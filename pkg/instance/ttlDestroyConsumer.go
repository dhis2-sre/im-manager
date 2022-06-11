package instance

import (
	"encoding/json"
	"log"

	"github.com/dhis2-sre/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ttlDestroyConsumer struct {
	consumer        *rabbitmq.Consumer
	instanceService Service
}

func ProvideTtlDestroyConsumer(consumer *rabbitmq.Consumer, instanceService Service) *ttlDestroyConsumer {
	return &ttlDestroyConsumer{consumer, instanceService}
}

func (c *ttlDestroyConsumer) Consume() error {
	_, err := c.consumer.Consume("ttl-destroy", func(d amqp.Delivery) {
		payload := struct{ ID uint }{}

		if err := json.Unmarshal(d.Body, &payload); err != nil {
			log.Printf("Error unmarshalling ttl-destroy message: %v\n", err)
			return
		}

		err := c.instanceService.Delete(payload.ID)
		if err != nil {
			log.Printf("Error deleting instance %d: %v\n", payload.ID, err)
			return
		}

		err = d.Ack(false)
		if err != nil {
			log.Printf("Error acknowledging ttl-destroy message for instance %d: %v\n", payload.ID, err)
		}

		log.Printf("Deleted instance %d since TTL expired\n", payload.ID)
	})
	return err
}
