package event

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
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

	consumerName := fmt.Sprintf("events-%d", user.ID)
	logger := h.logger.WithGroup("consumer").With("name", consumerName)

	// Set the RabbitMQ offset to the Last-Event-ID seen by the SSE client
	// This ensures that the SSE client resumes consuming from the stream where it left off
	// before the connection dropped. Otherwise, let the SSE client start streaming from the
	// first new message that is added to the stream.
	// https://web.dev/articles/eventsource-basics
	var offset atomic.Int64
	var offsetSpec stream.OffsetSpecification
	lastEventID := c.GetHeader("Last-Event-ID")
	if lastEventID == "" {
		offsetSpec = stream.OffsetSpecification{}.Next()
		logger.Info("User subscribed to events starting from the next published message")
	} else {
		lastOffset, err := strconv.ParseInt(lastEventID, 10, 64)
		if err != nil {
			_ = c.AbortWithError(400, fmt.Errorf("invalid %q value: %v", "Last-Event-ID", err))
			return
		}
		offset.Store(lastOffset + 1) // we want to send what the client has not already seen
		logger.Info("User subscribed to events starting from the Last-Event-ID", "lastEventId", lastOffset)
		offsetSpec = stream.OffsetSpecification{}.Offset(lastOffset + 1)
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	w := c.Writer
	clientGone := w.CloseNotify()

	userGroups := userGroups(user)
	filter := stream.NewConsumerFilter(maps.Keys(userGroups), true, postFilter(logger, user.ID, userGroups))
	opts := stream.NewConsumerOptions().
		SetConsumerName(consumerName).
		SetClientProvidedName(consumerName).
		SetManualCommit().
		SetOffset(offsetSpec).
		SetFilter(filter)
	sseEvents, messageHandler := createMessageHandler(or(c.Request.Context(), clientGone), logger)
	consumer, err := ha.NewReliableConsumer(h.env, h.streamName, opts, messageHandler)
	if err != nil {
		logger.Error("Failed to create RabbitMQ consumer", "error", err)
		_ = c.AbortWithError(500, err)
	}
	defer consumer.Close()

	logger.Info("Connection established for sending SSE events")
	for {
		select {
		case <-c.Request.Context().Done():
		case <-clientGone:
			logger.Info("Request canceled, returning from handler")
			return
		case sseEvent := <-sseEvents:
			c.Render(-1, sseEvent)
			w.Flush()
		}
	}
}

func userGroups(user *model.User) map[string]struct{} {
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
	// TODO(ivo) how to pass the user.ID as is without having to stringify and parse it again. Type
	// assertion is causing me a headache
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
func isUserPartOfMessageGroup(logger *slog.Logger, userGroupsMap map[string]struct{}, applicationProperties map[string]any) bool {
	group, ok := applicationProperties["group"]
	if !ok {
		return true
	}

	messageGroup, ok := group.(string)
	if !ok {
		logger.Error("Failed to type assert RabbitMQ message application property to a string", "messageProperty", "group", "messagePropertyValue", group)
		return false
	}

	_, ok = userGroupsMap[messageGroup]

	return ok
}

// or closes the channel it returns when one of context or done channel are closed.
func or(ctx context.Context, done <-chan bool) <-chan struct{} {
	out := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
		case <-done:
			close(out)
			return
		}
	}()
	return out
}

// createMessageHandler returns stream.MessagesHandler that will transform RabbitMQ messages of
// instance manager events into SSE events. These SSE events are sent via the read-only channel
// returned. This is to avoid race conditions when writing the data out to the HTTP response writer.
// Only one Go routine should write to the HTTP response writer. The RabbitMQ stream client runs our
// stream.MessagesHandler in a separate Go routine.
func createMessageHandler(done <-chan struct{}, logger *slog.Logger) (<-chan sse.Event, stream.MessagesHandler) {
	out := make(chan sse.Event)
	return out, func(consumerContext stream.ConsumerContext, message *amqp.Message) {
		select {
		case <-done:
			logger.Info("Request canceled, returning from messageHandler")
			close(out)
			return
		default:
			sseEvent, err := mapMessageToEvent(consumerContext.Consumer.GetOffset(), message)
			if err != nil {
				logger.Error("Failed to map AMQP message", "error", err)
				return
			}
			logger.Debug("Transformed AMQP message to SSE event", "message", sseEvent)

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
func mapMessageToEvent(offset int64, message *amqp.Message) (sse.Event, error) {
	var data string
	if len(message.Data) == 0 {
		return sse.Event{}, errors.New("received no data")
	}

	data = string(message.Data[0])

	var eventType string
	if event, ok := message.ApplicationProperties["type"]; ok {
		if kind, ok := event.(string); ok {
			eventType = kind
		} else {
			return sse.Event{}, fmt.Errorf("type assertion of RabbitMQ message application property %q failed, value=%v", "type", event)
		}
	}

	sseEvent := sse.Event{
		Id:   strconv.FormatInt(offset, 10),
		Data: data,
	}
	if eventType != "" { // SSE named event
		sseEvent.Event = eventType
	}
	return sseEvent, nil
}
