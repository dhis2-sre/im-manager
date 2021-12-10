// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//+build !wireinject

package di

import (
	"github.com/dhis2-sre/im-manager/internal/client"
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
	handler := stack.ProvideHandler(service)
	clientClient := client.ProvideUser(configConfig)
	instanceRepository := instance.ProvideRepository(db)
	kubernetesService := instance.ProvideKubernetesService()
	helmfileService := instance.ProvideHelmfileService(service, configConfig)
	instanceService := instance.ProvideService(configConfig, instanceRepository, clientClient, kubernetesService, helmfileService)
	instanceHandler := instance.ProvideHandler(clientClient, instanceService)
	environment := ProvideEnvironment(configConfig, service, handler, instanceHandler)
	return environment
}

// wire.go:

type Environment struct {
	Config          config.Config
	StackService    stack.Service
	StackHandler    stack.Handler
	InstanceHandler instance.Handler
}

func ProvideEnvironment(config2 config.Config,

	stackService stack.Service,
	stackHandler stack.Handler,
	instanceHandler instance.Handler,
) Environment {
	return Environment{config2, stackService,
		stackHandler,
		instanceHandler,
	}
}

func provideDatabase(c config.Config) *gorm.DB {
	database, err := storage.ProvideDatabase(c)
	if err != nil {
		log.Fatalln(err)
	}
	return database
}
