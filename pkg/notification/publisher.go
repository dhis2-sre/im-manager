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
	// Deliberately not SetProducerName. A name makes the producer a deduplication reference, and
	// RabbitMQ then discards every message whose publishing id it has already seen for that name,
	// while still confirming it, so nothing downstream can tell. Deduplication needs publishing ids
	// that are monotonic in the order they reach the broker, which we cannot offer: Send assigns the
	// id and enqueues the message as separate steps, and we publish from concurrent goroutines, so
	// two events can be numbered in one order and sent in another. The one that arrives below the
	// high water mark is dropped silently. We would rather have a duplicate event, which a client
	// can collapse, than a missing one nobody can detect. SetClientProvidedName only labels the
	// connection and carries none of this. What we give up is the deduplication of the reliable
	// producer's own re-sends after a reconnect, so delivery is at least once.
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

// PublishTransient streams an event to the group without persisting a notification and without an
// owner, so every group member's live connection receives it and it never appears in the
// notification bell. Meant for high-frequency ephemeral state like component status.
func (p *Publisher) PublishTransient(ctx context.Context, groupName, kind string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		p.logger.ErrorContext(ctx, "Failed to marshal transient event payload", "kind", kind, "error", err)
		return
	}

	msg := amqp.NewMessage(data)
	msg.ApplicationProperties = map[string]any{
		"group": groupName,
		"kind":  kind,
	}

	if err := p.producer.Send(msg); err != nil {
		p.logger.ErrorContext(ctx, "Failed to send transient event to RabbitMQ", "kind", kind, "error", err)
	}
}
