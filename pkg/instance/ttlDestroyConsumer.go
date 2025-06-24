package instance

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/internal/middleware"

	"github.com/dhis2-sre/rabbitmq-client/pkg/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ttlDestroyConsumer struct {
	logger          *slog.Logger
	consumer        *rabbitmq.Consumer
	instanceService instanceService
}

type instanceService interface {
	Delete(ctx context.Context, id uint) error
	FindDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error)
}

//goland:noinspection GoExportedFuncWithUnexportedType
func NewTTLDestroyConsumer(logger *slog.Logger, consumer *rabbitmq.Consumer, instanceService instanceService) *ttlDestroyConsumer {
	return &ttlDestroyConsumer{
		logger:          logger,
		consumer:        consumer,
		instanceService: instanceService,
	}
}

func (c *ttlDestroyConsumer) Consume() error {
	_, err := c.consumer.Consume("ttl-destroy", func(d amqp.Delivery) {
		ctx := context.Background()
		ctx = middleware.NewContextWithCorrelationID(ctx, d.CorrelationId)

		payload := struct{ ID uint }{}

		if err := json.Unmarshal(d.Body, &payload); err != nil {
			c.logger.ErrorContext(ctx, "Error unmarshalling ttl-destroy message", "error", err)
			err := d.Nack(false, false)
			if err != nil {
				c.logger.ErrorContext(ctx, "Error negatively acknowledging ttl-destroy message", "error", err)
				return
			}
			return
		}

		instance, err := c.instanceService.FindDeploymentInstanceById(ctx, payload.ID)
		if err != nil {
			c.logger.ErrorContext(ctx, "Error finding instance", "instanceId", payload.ID, "error", err)
			err := d.Nack(false, false)
			if err != nil {
				c.logger.ErrorContext(ctx, "Error acknowledging", "correlationId", d.CorrelationId, "error", err)
			}
			return
		}

		err = c.instanceService.Delete(ctx, instance.ID)
		if err != nil {
			if errdef.IsNotFound(err) {
				err := d.Ack(false)
				if err != nil {
					c.logger.ErrorContext(ctx, "Error acknowledging ttl-destroy message after deleting instance", "instanceId", instance.ID, "error", err)
					return
				}
			}
			c.logger.ErrorContext(ctx, "Error deleting instance", "instanceId", instance.ID, "error", err)
			return
		}
		c.logger.InfoContext(ctx, "Deleted expired instance", "instanceId", instance.ID, "name", instance.Name, "group", instance.GroupName)

		err = d.Ack(false)
		if err != nil {
			c.logger.ErrorContext(ctx, "Error acknowledging ttl-destroy message for instance", "instanceId", instance.ID, "error", err)
		}
	})
	return err
}
