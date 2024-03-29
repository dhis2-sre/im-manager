package instance

import (
	"encoding/json"
	"log"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ttlDestroyConsumer struct {
	consumer        *rabbitmq.Consumer
	instanceDeleter deleter
}

type deleter interface {
	Delete(id uint) error
}

//goland:noinspection GoExportedFuncWithUnexportedType
func NewTTLDestroyConsumer(consumer *rabbitmq.Consumer, instanceDeleter deleter) *ttlDestroyConsumer {
	return &ttlDestroyConsumer{
		consumer:        consumer,
		instanceDeleter: instanceDeleter,
	}
}

func (c *ttlDestroyConsumer) Consume() error {
	_, err := c.consumer.Consume("ttl-destroy", func(d amqp.Delivery) {
		payload := struct{ ID uint }{}

		if err := json.Unmarshal(d.Body, &payload); err != nil {
			log.Printf("Error unmarshalling ttl-destroy message: %v\n", err)
			err := d.Nack(false, false)
			if err != nil {
				log.Printf("Error negatively acknowledging ttl-destroy message: %v\n", err)
				return
			}
			return
		}

		err := c.instanceDeleter.Delete(payload.ID)
		if err != nil {
			if errdef.IsNotFound(err) {
				err := d.Ack(false)
				if err != nil {
					log.Printf("Error acknowledging ttl-destroy message for instance %d: %v\n", payload.ID, err)
					return
				}
			}
			log.Printf("Error deleting instance %d: %v\n", payload.ID, err)
			return
		}
		log.Printf("Deleted instance %d since TTL expired\n", payload.ID)

		err = d.Ack(false)
		if err != nil {
			log.Printf("Error acknowledging ttl-destroy message for instance %d: %v\n", payload.ID, err)
		}
	})
	return err
}
