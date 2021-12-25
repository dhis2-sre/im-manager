package main

import (
	"github.com/dhis2-sre/im-manager/internal/di"
	"github.com/dhis2-sre/im-manager/internal/server"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"log"
)

func main() {
	environment := di.GetEnvironment()

	stack.LoadStacks(environment.StackService)

	launchConsumers(environment)

	r := server.GetEngine(environment)
	if err := r.Run(); err != nil {
		log.Fatal(err)
	}
}

func launchConsumers(environment di.Environment) {
	rabbitMqURL := environment.Config.RabbitMqURL.GetUrl()
	instanceService := environment.InstanceService

	ttlDestroyConsumer := instance.ProvideTtlDestroyConsumer(rabbitMqURL, instanceService)
	go ttlDestroyConsumer.Launch()
}
