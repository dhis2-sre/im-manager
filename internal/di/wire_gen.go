// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package di

import (
	"github.com/dhis2-sre/im-manager/internal/client"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"gorm.io/gorm"
	"log"
)

// Injectors from wire.go:

func GetEnvironment() Environment {
	configConfig := config.ProvideConfig()
	db := provideDatabase(configConfig)
	repository := stack.ProvideRepository(db)
	service := stack.ProvideService(repository)
	stackHandler := stack.ProvideHandler(service)
	instanceRepository := instance.ProvideRepository(db)
	clientClient := client.ProvideUserService(configConfig)
	kubernetesService := instance.ProvideKubernetesService()
	helmfileService := instance.ProvideHelmfileService(service, configConfig)
	instanceService := instance.ProvideService(configConfig, instanceRepository, clientClient, kubernetesService, helmfileService)
	client2 := client.ProvideJobService(configConfig)
	instanceHandler := instance.ProvideHandler(clientClient, client2, instanceService)
	authenticationMiddleware := handler.ProvideAuthentication(configConfig)
	environment := ProvideEnvironment(configConfig, service, stackHandler, instanceService, instanceHandler, authenticationMiddleware)
	return environment
}

// wire.go:

type Environment struct {
	Config                   config.Config
	StackService             stack.Service
	StackHandler             stack.Handler
	InstanceService          instance.Service
	InstanceHandler          instance.Handler
	AuthenticationMiddleware handler.AuthenticationMiddleware
}

func ProvideEnvironment(config2 config.Config,

	stackService stack.Service,
	stackHandler stack.Handler,
	instanceService instance.Service,
	instanceHandler instance.Handler,
	authenticationMiddleware handler.AuthenticationMiddleware,
) Environment {
	return Environment{config2, stackService,
		stackHandler,
		instanceService,
		instanceHandler,
		authenticationMiddleware,
	}
}

func provideDatabase(c config.Config) *gorm.DB {
	database, err := storage.ProvideDatabase(c)
	if err != nil {
		log.Fatalln(err)
	}
	return database
}
