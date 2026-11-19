package storage

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/storage/migrations"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/uptrace/opentelemetry-go-extra/otelgorm"

	"github.com/dhis2-sre/im-manager/pkg/model"
	slogGorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresqlConfig struct {
	Host         string
	Port         int
	Username     string
	Password     string
	DatabaseName string
	LogQueries   bool
}

func NewDatabase(logger *slog.Logger, c PostgresqlConfig) (*gorm.DB, error) {
	db, err := ConnectDatabase(logger, c)
	if err != nil {
		return nil, err
	}
	if err := migrateDatabase(db); err != nil {
		if pool, poolErr := db.DB(); poolErr == nil {
			_ = pool.Close()
		}
		return nil, err
	}
	return db, nil
}

// ConnectDatabase opens a pool without changing the schema. Production startup
// uses NewDatabase; test fixtures use this after cloning a migrated template.
func ConnectDatabase(logger *slog.Logger, c PostgresqlConfig) (*gorm.DB, error) {
	gormLoggerOpts := []slogGorm.Option{
		slogGorm.WithHandler(logger.Handler()),
		slogGorm.WithRecordNotFoundError(),
		slogGorm.WithSlowThreshold(200 * time.Millisecond),
	}
	if c.LogQueries {
		gormLoggerOpts = append(gormLoggerOpts, slogGorm.WithTraceAll())
	}
	gormLogger := slogGorm.New(gormLoggerOpts...)

	databaseConfig := gorm.Config{
		Logger:         gormLogger,
		TranslateError: true,
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable", c.Host, c.Username, c.Password, c.DatabaseName, c.Port)
	db, err := gorm.Open(postgres.Open(dsn), &databaseConfig)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(1 * time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize otelgorm: %v", err)
	}
	return db, nil
}

func migrateDatabase(db *gorm.DB) error {
	err := db.AutoMigrate(
		&model.Deployment{},
		&model.DeploymentInstance{},
		&model.DeploymentInstanceParameter{},

		&model.User{},
		&model.Group{},
		&model.Cluster{},

		&model.Database{},
		&model.Lock{},
		&model.ExternalDownload{},

		&model.Notification{},
	)
	if err != nil {
		return fmt.Errorf("failed to open Gorm session: %v", err)
	}

	err = db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error
	if err != nil {
		return fmt.Errorf("failed to create pg_trgm extension: %v", err)
	}

	m := gormigrate.New(db, gormigrate.DefaultOptions, migrations.All())
	if err := m.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// GORM needs an explicit GIN index for the trigram operator class.
	sql := "CREATE INDEX IF NOT EXISTS idx_databases_description ON databases USING gin (description gin_trgm_ops)"
	err = db.Exec(sql).Error
	if err != nil {
		return err
	}

	return nil
}
