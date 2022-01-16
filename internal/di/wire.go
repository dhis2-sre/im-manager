//go:build wireinject
// +build wireinject

package di

import (
	"github.com/dhis2-sre/im-manager/internal/client"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/google/wire"
	"gorm.io/gorm"
	"log"
)

type Environment struct {
	Config                   config.Config
	StackService             stack.Service
	StackHandler             stack.Handler
	InstanceService          instance.Service
	InstanceHandler          instance.Handler
	AuthenticationMiddleware handler.AuthenticationMiddleware
}

func ProvideEnvironment(
	config config.Config,
	stackService stack.Service,
	stackHandler stack.Handler,
	instanceService instance.Service,
	instanceHandler instance.Handler,
	authenticationMiddleware handler.AuthenticationMiddleware,
) Environment {
	return Environment{
		config,
		stackService,
		stackHandler,
		instanceService,
		instanceHandler,
		authenticationMiddleware,
	}
}

func GetEnvironment() Environment {
	wire.Build(
		config.ProvideConfig,

		provideDatabase,

		stack.ProvideRepository,
		stack.ProvideService,
		stack.ProvideHandler,

		client.ProvideUserService,
		client.ProvideJobService,
		instance.ProvideHelmfileService,
		instance.ProvideKubernetesService,
		instance.ProvideRepository,
		instance.ProvideService,
		instance.ProvideHandler,

		handler.ProvideAuthentication,

		ProvideEnvironment,
	)
	return Environment{}
}

func provideDatabase(c config.Config) *gorm.DB {
	database, err := storage.ProvideDatabase(c)
	if err != nil {
		log.Fatalln(err)
	}
	return database
}
