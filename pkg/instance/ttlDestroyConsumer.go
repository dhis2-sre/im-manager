package instance

import (
	"encoding/json"
	"log/slog"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/rabbitmq-client/pkg/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ttlDestroyConsumer struct {
	logger          *slog.Logger
	consumer        *rabbitmq.Consumer
	instanceDeleter deleter
}

type deleter interface {
	Delete(id uint) error
}

//goland:noinspection GoExportedFuncWithUnexportedType
func NewTTLDestroyConsumer(logger *slog.Logger, consumer *rabbitmq.Consumer, instanceDeleter deleter) *ttlDestroyConsumer {
	return &ttlDestroyConsumer{
		logger:          logger,
		consumer:        consumer,
		instanceDeleter: instanceDeleter,
	}
}

func (c *ttlDestroyConsumer) Consume() error {
	_, err := c.consumer.Consume("ttl-destroy", func(d amqp.Delivery) {
		payload := struct{ ID uint }{}

		if err := json.Unmarshal(d.Body, &payload); err != nil {
			c.logger.Error("Error unmarshalling ttl-destroy message", "error", err)
			err := d.Nack(false, false)
			if err != nil {
				c.logger.Error("Error negatively acknowledging ttl-destroy message", "error", err)
				return
			}
			return
		}

		err := c.instanceDeleter.Delete(payload.ID)
		if err != nil {
			if errdef.IsNotFound(err) {
				err := d.Ack(false)
				if err != nil {
					c.logger.Error("Error acknowledging ttl-destroy message after deleting instance", "instanceId", payload.ID, "error", err)
					return
				}
			}
			c.logger.Error("Error deleting instance", "instanceId", payload.ID, "error", err)
			return
		}
		c.logger.Info("Deleted expired instance", "instanceId", payload.ID)

		err = d.Ack(false)
		if err != nil {
			c.logger.Error("Error acknowledging ttl-destroy message for instance", "instanceId", payload.ID, "error", err)
		}
	})
	return err
}
