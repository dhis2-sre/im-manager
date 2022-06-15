package instance

import (
	"encoding/json"
	"log"

	"github.com/dhis2-sre/im-user/swagger/sdk/models"

	"github.com/dhis2-sre/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ttlDestroyConsumer struct {
	usrClientUsername string
	usrClientPassword string
	usrAuth           userAuth
	consumer          *rabbitmq.Consumer
	instanceService   Service
}

type userAuth interface {
	SignIn(username, password string) (*models.Tokens, error)
}

func ProvideTtlDestroyConsumer(userClientUsername, userClientPassword string, usrAuth userAuth, consumer *rabbitmq.Consumer, instanceService Service) *ttlDestroyConsumer {
	return &ttlDestroyConsumer{
		usrClientUsername: userClientUsername,
		usrClientPassword: userClientPassword,
		usrAuth:           usrAuth,
		consumer:          consumer,
		instanceService:   instanceService,
	}
}

func (c *ttlDestroyConsumer) Consume() error {
	_, err := c.consumer.Consume("ttl-destroy", func(d amqp.Delivery) {
		payload := struct{ ID uint }{}

		if err := json.Unmarshal(d.Body, &payload); err != nil {
			log.Printf("Error unmarshalling ttl-destroy message: %v\n", err)
			return
		}

		tokens, err := c.usrAuth.SignIn(c.usrClientUsername, c.usrClientPassword)
		if err != nil {
			log.Printf("Error signing in to im-user: %v\n", err)
			return
		}

		err = c.instanceService.Delete(tokens.AccessToken, payload.ID)
		if err != nil {
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
