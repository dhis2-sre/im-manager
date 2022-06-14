package instance

import (
	"encoding/json"
	"log"

	"github.com/dhis2-sre/im-manager/pkg/config"
	userClient "github.com/dhis2-sre/im-user/pkg/client"

	"github.com/dhis2-sre/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ttlDestroyConsumer struct {
	config          config.Config
	userClient      userClient.Client
	consumer        *rabbitmq.Consumer
	instanceService Service
}

func ProvideTtlDestroyConsumer(config config.Config, userClient userClient.Client, consumer *rabbitmq.Consumer, instanceService Service) *ttlDestroyConsumer {
	return &ttlDestroyConsumer{config, userClient, consumer, instanceService}
}

func (c *ttlDestroyConsumer) Consume() error {
	_, err := c.consumer.Consume("ttl-destroy", func(d amqp.Delivery) {
		payload := struct{ ID uint }{}

		if err := json.Unmarshal(d.Body, &payload); err != nil {
			log.Println(err)
			return
		}

		tokens, err := c.userClient.SignIn(c.config.UserService.Username, c.config.UserService.Password)
		if err != nil {
			log.Println(err)
			return
		}

		err = c.instanceService.Delete(tokens.AccessToken, payload.ID)
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
