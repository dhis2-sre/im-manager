//+build wireinject

package di

import (
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/google/wire"
	"gorm.io/gorm"
	"log"
)

type Environment struct {
	Config       config.Config
	StackHandler stack.Handler
}

func ProvideEnvironment(
	config config.Config,
	stackHandler stack.Handler,
) Environment {
	return Environment{
		config,
		stackHandler,
	}
}

func GetEnvironment() Environment {
	wire.Build(
		config.ProvideConfig,

		provideDatabase,

		stack.ProvideRepository,
		stack.ProvideService,
		stack.ProvideHandler,

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
