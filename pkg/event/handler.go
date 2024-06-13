package event

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-contrib/sse"
	"github.com/gin-gonic/gin"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/ha"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
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
	logger.Info("User subscribed")

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	w := c.Writer
	clientGone := w.CloseNotify()

	sseEvents := make(chan sse.Event)

	// TODO(ivo) understand match unfiltered
	userGroups := sortedGroupNames(user.Groups)
	filter := stream.NewConsumerFilter(userGroups, true, postFilter(logger, user.ID, userGroups))
	opts := stream.NewConsumerOptions().
		// SetOffset(stream.OffsetSpecification{}.First()).
		SetConsumerName(consumerName).
		SetClientProvidedName(consumerName).
		SetFilter(filter)
	messageHandler := func(consumerContext stream.ConsumerContext, message *amqp.Message) {
		select {
		case <-c.Request.Context().Done():
		case <-clientGone:
			logger.Info("Request canceled, returning from messageHandler")
			// TODO anything else we should be doing?
			return
		default:
			var data string
			if len(message.Data) == 0 {
				logger.Error("Received no data")
				return
			}

			data = string(message.Data[0])

			var eventType string
			if event, ok := message.ApplicationProperties["type"]; ok {
				if kind, ok := event.(string); ok {
					eventType = kind
				} else {
					logger.Error("Failed to type assert RabbitMQ message application property to a string", "messageProperty", "type", "messagePropertyValue", event)
				}
			}

			logger.Info("Received message", "type", eventType, "data", message.Data)

			sseEvent := sse.Event{
				Data: data,
			}
			if eventType != "" { // SSE named event
				sseEvent.Event = eventType
			}

			select {
			case <-c.Request.Context().Done():
			case <-clientGone:
				logger.Info("Request canceled, returning from messageHandler")
				return
			case sseEvents <- sseEvent:
			}
		}
	}
	consumer, err := ha.NewReliableConsumer(h.env, h.streamName, opts, messageHandler)
	if err != nil {
		logger.Error("Failed to create RabbitMQ consumer", "error", err)
		_ = c.AbortWithError(500, err)
	}
	defer consumer.Close()

	logger.Info("Connection established for sending events via SSE")
	// TODO(ivo) think about this here
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

func sortedGroupNames(groups []model.Group) []string {
	var result []string
	for _, group := range groups {
		result = append(result, group.Name)
	}
	slices.Sort(result)
	return result
}

// postFilter is a RabbitMQ stream post filter that is applied client side. This is necessary as the
// server side filter is probabilistic and can let false positives through. (see
// https://www.rabbitmq.com/blog/2023/10/16/stream-filtering) The filter must be simple and fast.
// userGroups must be in sorted order!
func postFilter(logger *slog.Logger, userID uint, userGroups []string) func(message *amqp.Message) bool {
	// TODO(ivo) how to pass the user.ID as is without having to stringify and parse it again. Type
	// assertion is causing me a headache
	return func(message *amqp.Message) bool {
		return isUserMessageOwner(logger, userID, message.ApplicationProperties) && isUserPartOfMessageGroup(logger, userGroups, message.ApplicationProperties)
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
	if uint(messageOwnerID) != userID {
		return false
	}

	return true
}

// isUserPartOfMessageGroup determines if the user is allowed to receive the message. This function
// only considers the "group" property of a message. Messages that have no group can be read by the
// user. Messages that have a group can only be read by the user if the "group" property value can
// be parsed and matches one of the userGroups.
func isUserPartOfMessageGroup(logger *slog.Logger, userGroups []string, applicationProperties map[string]any) bool {
	group, ok := applicationProperties["group"]
	if !ok {
		return true
	}

	messageGroup, ok := group.(string)
	if !ok {
		logger.Error("Failed to type assert RabbitMQ message application property to a string", "messageProperty", "group", "messagePropertyValue", group)
		return false
	}

	if _, ok := slices.BinarySearch(userGroups, messageGroup); !ok {
		return false
	}

	return true
}
