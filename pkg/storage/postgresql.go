package storage

import (
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDatabase(c config.Postgresql) (*gorm.DB, error) {
	host := c.Host
	port := c.Port
	username := c.Username
	password := c.Password
	name := c.DatabaseName

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable", host, username, password, name, port)

	databaseConfig := gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	db, err := gorm.Open(postgres.Open(dsn), &databaseConfig)
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(
		&model.User{},
		&model.Group{},
		&model.ClusterConfiguration{},

		&model.Stack{},
		&model.StackRequiredParameter{},
		&model.StackOptionalParameter{},

		&model.Instance{},
		&model.Linked{},
		&model.InstanceRequiredParameter{},
		&model.InstanceOptionalParameter{},

		&model.Database{},
		&model.Lock{},
		&model.ExternalDownload{},
	)

	if err != nil {
		return nil, err
	}

	return db, nil
}
