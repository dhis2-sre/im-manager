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

	"github.com/dhis2-sre/im-manager/internal/client"
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
	cfg := config.ProvideConfig()

	db, err := storage.ProvideDatabase(cfg)
	if err != nil {
		return err
	}

	repository := stack.ProvideRepository(db)
	stackSvc := stack.ProvideService(repository)

	instanceRepository := instance.ProvideRepository(db)
	uc := userClient.New(cfg.UserService.Host, cfg.UserService.BasePath)
	kubernetesSvc := instance.ProvideKubernetesService()
	helmfileSvc := instance.ProvideHelmfileService(stackSvc, cfg)
	instanceSvc := instance.ProvideService(cfg, instanceRepository, uc, kubernetesSvc, helmfileSvc)

	stack.LoadStacks(stackSvc)

	consumer, err := rabbitmq.NewConsumer(
		cfg.RabbitMqURL.GetUrl(),
		rabbitmq.WithConsumerPrefix("im-manager"),
	)
	if err != nil {
		return err
	}
	defer consumer.Close()

	ttlDestroyConsumer := instance.ProvideTtlDestroyConsumer(cfg.UserService.Username, cfg.UserService.Password, uc, consumer, instanceSvc)
	err = ttlDestroyConsumer.Consume()
	if err != nil {
		return err
	}

	stackHandler := stack.ProvideHandler(stackSvc)
	jobSvc := client.ProvideJobService(cfg)
	instanceHandler := instance.ProvideHandler(uc, jobSvc, instanceSvc)
	authMiddleware := handler.ProvideAuthentication(cfg)

	r := server.GetEngine(cfg.BasePath, stackHandler, instanceHandler, authMiddleware)
	return r.Run()
}
