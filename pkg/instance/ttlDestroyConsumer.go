package instance

import (
	"encoding/json"
	"github.com/dhis2-sre/instance-queue/pgk/queue"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
)

func ProvideTtlDestroyConsumer(url string, instanceService Service) *ttlDestroyConsumer {
	abstractConsumer := &queue.AbstractConsumer{}
	consumer := &ttlDestroyConsumer{abstractConsumer, instanceService}
	abstractConsumer.Consumer = consumer
	abstractConsumer.Url = url
	return consumer
}

type ttlDestroyConsumer struct {
	*queue.AbstractConsumer
	instanceService Service
}

func (c *ttlDestroyConsumer) Channel() string {
	return "ttl-destroy"
}

func (c *ttlDestroyConsumer) Consume(d amqp.Delivery) {
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
}
