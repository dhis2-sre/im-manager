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
			log.Println(err)
			return
		}

		err := c.instanceService.Delete(payload.ID)
		if err != nil {
			log.Println(err)
			return
		}

		err = d.Ack(false)
		if err != nil {
			log.Println(err)
		}
	})
	return err
}
