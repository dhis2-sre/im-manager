// Package classification Instance Manager Manager Service.
//
// Manager Service as part of the Instance Manager environment
//
// Terms Of Service:
//
// there are no TOS at this moment, use at your own risk we take no responsibility
//
//    Version: 0.1.0
//    License: TODO
//    Contact: <info@dhis2.org> https://github.com/dhis2-sre/im-manager
//
//    Consumes:
//      - application/json
//
//    Produces:
//      - application/json
//
//    SecurityDefinitions:
//      oauth2:
//        type: oauth2
//        tokenUrl: /not-valid--endpoint-is-served-from-the-im-user-service
//        refreshUrl: /not-valid--endpoint-is-served-from-the-im-user-service
//        flow: password
// swagger:meta
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/dhis2-sre/im-manager/internal/di"
	"github.com/dhis2-sre/im-manager/internal/server"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/rabbitmq"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	environment := di.GetEnvironment()

	stack.LoadStacks(environment.StackService)

	consumer, err := rabbitMQConnect(environment.Config.RabbitMqURL.GetUrl())
	if err != nil {
		return err
	}
	defer consumer.Close()

	ttlDestroyConsumer := instance.ProvideTtlDestroyConsumer(consumer, environment.InstanceService)
	err = ttlDestroyConsumer.Consume()
	if err != nil {
		return err
	}

	r := server.GetEngine(environment)
	return r.Run()
}

func rabbitMQConnect(rabbitMqURL string) (*rabbitmq.Consumer, error) {
	var err error
	var consumer *rabbitmq.Consumer
	for i, max := 0, 15; i < max; i++ {
		consumer, err = rabbitmq.NewConsumer(
			rabbitMqURL,
			rabbitmq.WithConsumerPrefix("im-manager"),
		)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to RabbitMQ (attempt %d/%d): %s\n", i+1, max, err)
		time.Sleep(time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %s", err)
	}

	return consumer, nil
}
