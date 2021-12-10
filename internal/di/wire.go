//+build wireinject

package di

import (
	"github.com/dhis2-sre/im-manager/internal/client"
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/google/wire"
	"gorm.io/gorm"
	"log"
)

type Environment struct {
	Config          config.Config
	StackService    stack.Service
	StackHandler    stack.Handler
	InstanceHandler instance.Handler
}

func ProvideEnvironment(
	config config.Config,
	stackService stack.Service,
	stackHandler stack.Handler,
	instanceHandler instance.Handler,
) Environment {
	return Environment{
		config,
		stackService,
		stackHandler,
		instanceHandler,
	}
}

func GetEnvironment() Environment {
	wire.Build(
		config.ProvideConfig,

		provideDatabase,

		stack.ProvideRepository,
		stack.ProvideService,
		stack.ProvideHandler,

		client.ProvideUser,
		instance.ProvideHelmfileService,
		instance.ProvideKubernetesService,
		instance.ProvideRepository,
		instance.ProvideService,
		instance.ProvideHandler,

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
