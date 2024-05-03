package storage

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	slogGorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewDatabase(logger *slog.Logger, c config.Postgresql) (*gorm.DB, error) {
	gormLogger := slogGorm.New(
		slogGorm.WithHandler(logger.WithGroup("db").Handler()),
		slogGorm.WithRecordNotFoundError(),
		slogGorm.WithSlowThreshold(5*time.Millisecond),
		slogGorm.WithContextValue("user", "user"),
	)
	databaseConfig := gorm.Config{
		Logger:         gormLogger,
		TranslateError: true,
	}

	host := c.Host
	port := c.Port
	username := c.Username
	password := c.Password
	name := c.DatabaseName
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable", host, username, password, name, port)
	db, err := gorm.Open(postgres.Open(dsn), &databaseConfig)
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(
		&model.Deployment{},
		&model.DeploymentInstance{},
		&model.DeploymentInstanceParameter{},

		&model.User{},
		&model.Group{},
		&model.ClusterConfiguration{},

		&model.Database{},
		&model.Lock{},
		&model.ExternalDownload{},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to open Gorm session: %v", err)
	}

	return db, nil
}
