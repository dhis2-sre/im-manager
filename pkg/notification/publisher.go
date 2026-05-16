package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"

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
	counter    atomic.Int64
}

func NewPublisher(logger *slog.Logger, env *stream.Environment, streamName string, repo *repository) (*Publisher, error) {
	p := &Publisher{
		logger:     logger,
		repository: repo,
	}

	producerName := "notification-publisher"
	opts := stream.NewProducerOptions().
		SetProducerName(producerName).
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

func (p *Publisher) Publish(ctx context.Context, userID uint, groupName, kind string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	n := &model.Notification{
		UserID:    userID,
		GroupName: groupName,
		Kind:      kind,
		Data:      string(data),
	}
	if err := p.repository.create(ctx, n); err != nil {
		return fmt.Errorf("failed to persist notification: %w", err)
	}

	msg := amqp.NewMessage(data)
	msg.SetPublishingId(p.counter.Add(1))
	msg.ApplicationProperties = map[string]any{
		"group": groupName,
		"owner": strconv.FormatUint(uint64(userID), 10),
		"kind":  kind,
	}

	if err := p.producer.Send(msg); err != nil {
		p.logger.ErrorContext(ctx, "Failed to send notification to RabbitMQ", "kind", kind, "error", err)
	}

	return nil
}
