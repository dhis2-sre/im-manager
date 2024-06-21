package event

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/ha"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
	"golang.org/x/exp/maps"
)

func NewHandler(logger *slog.Logger, env *stream.Environment, streamName string) Handler {
	return Handler{
		logger:     logger,
		env:        env,
		streamName: streamName,
	}
}

type Handler struct {
	logger     *slog.Logger
	env        *stream.Environment
	streamName string
}

func (h Handler) StreamEvents(c *gin.Context) {
	// swagger:route GET /events streamSSE
	//
	// Stream events
	//
	// Stream events...
	//
	// responses:
	//   200: Stream
	//   400: Error
	//   403: Error
	//   404: Error
	//   415: Error
	//
	// security:
	//   oauth2:
	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userGroups := mapUserGroups(user)
	if len(userGroups) == 0 {
		_ = c.Error(errdef.NewForbidden("you cannot stream events as you are not part of a group. Ask an administrator for help."))
		return
	}

	consumerName := fmt.Sprintf("user-%d-%s", user.ID, uuid.NewString())
	logger := h.logger.WithGroup("consumer").With("name", consumerName)

	// check offset to return 400 before any other header in case of an error
	offsetSpec, err := computeOffsetSpec(c)
	if err != nil {
		logger.Error("Failed to compute RabbitMQ offset spec", "error", err)
		_ = c.Error(err)
		return
	}
	retry := computeRetry()
	logger = h.logger.With("retry", retry)

	filter := stream.NewConsumerFilter(maps.Keys(userGroups), true, postFilter(logger, user.ID, userGroups))
	opts := stream.NewConsumerOptions().
		SetConsumerName(consumerName).
		SetClientProvidedName(consumerName).
		SetManualCommit().
		SetOffset(offsetSpec).
		SetFilter(filter)
	sseEvents, messageHandler := createMessageHandler(c.Request.Context().Done(), logger, retry)
	consumer, err := ha.NewReliableConsumer(h.env, h.streamName, opts, messageHandler)
	if err != nil {
		logger.Error("Failed to create RabbitMQ consumer", "error", err)
		_ = c.Error(err)
		return
	}
	defer consumer.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()
	logger.Info("Connection established for sending SSE events", "offsetSpec", offsetSpec)

	for {
		select {
		case <-c.Request.Context().Done():
			logger.Info("Request canceled, returning from handler")
			return
		case sseEvent := <-sseEvents:
			c.Render(-1, sseEvent)
			c.Writer.Flush()
		}
	}
}

// computeOffsetSpec computes the offset from which the SSE client will stream RabbitMQ messages
// from. By default clients will receive the next message that is published to the events stream.
// This means they will not receive "old" events. SSE clients send a "Last-Event-ID" HTTP header on
// re-connect. The "Last-Event-ID" value is a RabbitMQ offset we send in the SSE id field. Clients
// can thus resume where they left off.
func computeOffsetSpec(c *gin.Context) (stream.OffsetSpecification, error) {
	lastEventID := c.GetHeader("Last-Event-ID")
	if lastEventID == "" {
		return stream.OffsetSpecification{}.Next(), nil
	}

	lastOffset, err := strconv.ParseInt(lastEventID, 10, 64)
	if err != nil {
		return stream.OffsetSpecification{}, errdef.NewBadRequest("invalid header %q value: %v", "Last-Event-ID", err)
	}

	return stream.OffsetSpecification{}.Offset(lastOffset + 1), nil
}

// computeRetry computes the SSE computeRetry value in milliseconds. It is composed of a base of 3 seconds with an
// additional jitter of up to 1 second.
func computeRetry() uint {
	var base, maxJitter uint = 3_000, 1_001
	// math rand v2 has the better API and is good enough for computing the jitter
	// uses a half-open interval [0,n) so 1000ms+1ms
	return base + rand.UintN(maxJitter) //nolint:gosec
}

func mapUserGroups(user *model.User) map[string]struct{} {
	result := make(map[string]struct{}, len(user.Groups))
	for _, group := range user.Groups {
		result[group.Name] = struct{}{}
	}
	return result
}

// postFilter is a RabbitMQ stream post filter that is applied client side. This is necessary as the
// server side filter is probabilistic and can let false positives through. (see
// https://www.rabbitmq.com/blog/2023/10/16/stream-filtering) The filter must be simple and fast.
func postFilter(logger *slog.Logger, userID uint, userGroupsMap map[string]struct{}) stream.PostFilter {
	return func(message *amqp.Message) bool {
		return isUserMessageOwner(logger, userID, message.ApplicationProperties) && isUserPartOfMessageGroup(logger, userGroupsMap, message.ApplicationProperties)
	}
}

// isUserMessageOwner determines if the user is allowed to receive the message. This function only
// considers the "owner" property of a message. Messages that have no owner can be read by the user.
// Messages that have an owner can only be read by the user if the "owner" property value can be
// parsed and matches the userID.
func isUserMessageOwner(logger *slog.Logger, userID uint, applicationProperties map[string]any) bool {
	owner, ok := applicationProperties["owner"]
	if !ok {
		return true
	}

	messageOwner, ok := owner.(string)
	if !ok {
		logger.Error("Failed to type assert RabbitMQ message application property to a string", "messageProperty", "owner", "messagePropertyValue", owner)
		return false
	}

	messageOwnerID, err := strconv.ParseUint(messageOwner, 10, 64)
	if err != nil {
		logger.Error("Failed to parse RabbitMQ message application property to a uint", "messageProperty", "owner", "messagePropertyValue", owner, "error", err)
		return false
	}

	return messageOwnerID == uint64(userID)
}

// isUserPartOfMessageGroup determines if the user is allowed to receive the message. This function
// only considers the "group" property of a message. Messages that have no group can be read by the
// user. Messages that have a group can only be read by the user if the "group" property value can
// be parsed and matches one of the userGroupsMap.
func isUserPartOfMessageGroup(logger *slog.Logger, userGroups map[string]struct{}, applicationProperties map[string]any) bool {
	group, ok := applicationProperties["group"]
	if !ok {
		return true
	}

	messageGroup, ok := group.(string)
	if !ok {
		logger.Error("Failed to type assert RabbitMQ message application property to a string", "messageProperty", "group", "messagePropertyValue", group)
		return false
	}

	_, ok = userGroups[messageGroup]
	return ok
}

// createMessageHandler returns stream.MessagesHandler that will transform RabbitMQ messages of
// instance manager events into SSE events. These SSE events are sent via the read-only channel
// returned. This is to avoid race conditions when writing the data out to the HTTP response writer.
// Only one Go routine should write to the HTTP response writer. The RabbitMQ stream client runs our
// stream.MessagesHandler in a separate Go routine.
func createMessageHandler(done <-chan struct{}, logger *slog.Logger, retry uint) (<-chan sse.Event, stream.MessagesHandler) {
	out := make(chan sse.Event)
	return out, func(consumerContext stream.ConsumerContext, message *amqp.Message) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("RabbitMQ message handler panicked", "recover", r)
				// We assume that we cannot recover from a panic in a message handler. We thus panic
				// again. We do want to log any panic to be notfied.
				panic(r)
			}
		}()

		select {
		case <-done:
			logger.Info("Request canceled, returning from messageHandler")
			close(out)
			return
		default:
			sseEvent, err := mapMessageToEvent(retry, consumerContext.Consumer.GetOffset(), message)
			if err != nil {
				logger.Error("Failed to map AMQP message", "error", err)
				return
			}

			select {
			case <-done:
				logger.Info("Request canceled, returning from messageHandler")
				close(out)
				return
			case out <- sseEvent:
			}
		}
	}
}

// mapMessageToEvent maps an AMQP message of an instance manager event to an SSE event. No error is
// returned if the message could be processed and an SSE event should be sent. Do not send an SSE
// event when an error is returned.
func mapMessageToEvent(retry uint, offset int64, message *amqp.Message) (sse.Event, error) {
	if len(message.Data) == 0 {
		return sse.Event{}, errors.New("received no data")
	}

	kindProperty, ok := message.ApplicationProperties["kind"]
	if !ok {
		return sse.Event{}, errors.New(`RabbitMQ message is missing application property "kind"`)
	}
	kind, ok := kindProperty.(string)
	if !ok {
		return sse.Event{}, fmt.Errorf("type assertion of RabbitMQ message application property %q failed, value=%v", "type", kindProperty)
	}

	// text/event-stream is text based. Thus our data needs to be converted to a string. Gin
	// sse.Event marshalls the Data field using fmt.Sprint which uses the default formatting verb %v
	// which for a []byte would print `[65]` for []byte{"A"} instead of `A`
	data := string(message.Data[0])
	return sse.Event{
		Id:    strconv.FormatInt(offset, 10),
		Data:  data,
		Retry: retry,
		Event: kind,
	}, nil
}
