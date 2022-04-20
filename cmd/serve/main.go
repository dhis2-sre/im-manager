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
	"log"

	"github.com/dhis2-sre/im-manager/internal/di"
	"github.com/dhis2-sre/im-manager/internal/server"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/stack"
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
