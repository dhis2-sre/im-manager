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

  jobClient "github.com/dhis2-sre/im-job/pkg/client"

	"github.com/dhis2-sre/im-manager/pkg/integration"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/internal/server"
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	userClient "github.com/dhis2-sre/im-user/pkg/client"
	"github.com/dhis2-sre/rabbitmq"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.New()

	db, err := storage.NewDatabase(cfg)
	if err != nil {
		return err
	}

	stackSvc := stack.NewService(stack.NewRepository(db))

	instanceRepo := instance.NewRepository(db, cfg)
	uc := userClient.New(cfg.UserService.Host, cfg.UserService.BasePath)
	helmfileSvc := instance.NewHelmfileService(stackSvc, cfg)
	instanceSvc := instance.NewService(cfg, instanceRepo, uc, stackSvc, helmfileSvc)

	dockerHubClient := integration.NewDockerHubClient(cfg.DockerHub.Username, cfg.DockerHub.Password)

	integrationHandler := integration.NewHandler(dockerHubClient, cfg.InstanceService.Host, cfg.DatabaseManagerService.Host)

	err = stack.LoadStacks("./stacks", stackSvc)
	if err != nil {
		return err
	}

	consumer, err := rabbitmq.NewConsumer(
		cfg.RabbitMqURL.GetUrl(),
		rabbitmq.WithConsumerPrefix("im-manager"),
	)
	if err != nil {
		return err
	}
	defer consumer.Close()

	ttlDestroyConsumer := instance.NewTTLDestroyConsumer(cfg.UserService.Username, cfg.UserService.Password, uc, consumer, instanceSvc)
	err = ttlDestroyConsumer.Consume()
	if err != nil {
		return err
	}

	stackHandler := stack.NewHandler(stackSvc)
	instanceHandler := instance.NewHandler(uc, instanceSvc, stackSvc)
	authMiddleware, err := handler.NewAuthentication(cfg)
	if err != nil {
		return err
	}

	r := server.GetEngine(cfg.BasePath, stackHandler, instanceHandler, integrationHandler, authMiddleware)
	return r.Run()
}
