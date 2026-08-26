package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/ha"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/message"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
)

type Publisher struct {
	logger     *slog.Logger
	producer   *ha.ReliableProducer
	repository *repository
}

func NewPublisher(logger *slog.Logger, env *stream.Environment, streamName string, repo *repository) (*Publisher, error) {
	p := &Publisher{
		logger:     logger,
		repository: repo,
	}

	producerName := "notification-publisher"
	// Deliberately not SetProducerName: a named producer is a deduplication reference, and RabbitMQ
	// then silently drops (while still confirming) every message whose publishing id it has already
	// seen for that name. Publishing ids come from a counter that restarts at zero with the process,
	// so after a restart every event up to the stream's high water mark was discarded. This producer
	// wants each event appended, not deduplicated. SetClientProvidedName only labels the connection.
	opts := stream.NewProducerOptions().
		SetClientProvidedName(producerName).
		SetFilter(stream.NewProducerFilter(func(msg message.StreamMessage) string {
			return fmt.Sprintf("%s", msg.GetApplicationProperties()["group"])
		}))

	producer, err := ha.NewReliableProducer(env, streamName, opts, func(statuses []*stream.ConfirmationStatus) {
		for _, s := range statuses {
			if !s.IsConfirmed() {
				logger.Error("Failed to confirm RabbitMQ notification message", "publishingId", s.GetMessage().GetPublishingId())
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create notification publisher: %w", err)
	}

	p.producer = producer
	return p, nil
}

func (p *Publisher) Close() error {
	return p.producer.Close()
}

func (p *Publisher) Publish(ctx context.Context, userID uint, groupName, kind string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		p.logger.ErrorContext(ctx, "Failed to marshal notification payload", "kind", kind, "error", err)
		return
	}

	n := &model.Notification{
		UserID:    userID,
		GroupName: groupName,
		Kind:      kind,
		Data:      string(data),
	}
	if err := p.repository.create(ctx, n); err != nil {
		p.logger.ErrorContext(ctx, "Failed to persist notification", "kind", kind, "error", err)
		return
	}

	msg := amqp.NewMessage(data)
	msg.ApplicationProperties = map[string]any{
		"group": groupName,
		"owner": strconv.FormatUint(uint64(userID), 10),
		"kind":  kind,
	}

	if err := p.producer.Send(msg); err != nil {
		p.logger.ErrorContext(ctx, "Failed to send notification to RabbitMQ", "kind", kind, "error", err)
	}
}
