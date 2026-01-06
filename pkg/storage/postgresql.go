package storage

import (
	"fmt"
	"log/slog"
	"time"

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
}

func NewDatabase(logger *slog.Logger, c PostgresqlConfig) (*gorm.DB, error) {
	gormLogger := slogGorm.New(
		slogGorm.WithHandler(logger.Handler()),
		slogGorm.WithRecordNotFoundError(),
		slogGorm.WithSlowThreshold(1000*time.Millisecond),
	)

	databaseConfig := gorm.Config{
		Logger:         gormLogger,
		TranslateError: true,
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable", c.Host, c.Username, c.Password, c.DatabaseName, c.Port)
	db, err := gorm.Open(postgres.Open(dsn), &databaseConfig)
	if err != nil {
		return nil, err
	}

	if err := db.Use(otelgorm.NewPlugin()); err != nil {
		return nil, fmt.Errorf("failed to initialize otelgorm: %v", err)
	}
	err = db.AutoMigrate(
		&model.Deployment{},
		&model.DeploymentInstance{},
		&model.DeploymentInstanceParameter{},

		&model.User{},
		&model.Group{},
		&model.Cluster{},

		&model.Database{},
		&model.Lock{},
		&model.ExternalDownload{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open Gorm session: %v", err)
	}

	err = db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error
	if err != nil {
		return nil, fmt.Errorf("failed to create pg_trgm extension: %v", err)
	}

	// GORM Doesn't handle the creation of gin indexes very well so the index is created manually here
	//dev-1           | [00] Starting service
	//database-1      | 2025-09-21 19:29:19.904 UTC [98] ERROR:  operator class "gin_trgm_ops" does not exist for access method "btree"
	//database-1      | 2025-09-21 19:29:19.904 UTC [98] STATEMENT:  CREATE INDEX IF NOT EXISTS "idx_databases_description" ON "databases" (description gin_trgm_ops)
	//dev-1           | [00] {"time":"2025-09-21T19:29:19.905025751Z","level":"ERROR","msg":"ERROR: operator class \"gin_trgm_ops\" does not exist for access method \"btree\" (SQLSTATE 42704)","error":"ERROR: operator class \"gin_trgm_ops\" does not exist for access method \"btree\" (SQLSTATE 42704)","query":"CREATE INDEX IF NOT EXISTS \"idx_databases_description\" ON \"databases\" (description gin_trgm_ops)","duration":414270,"rows":0,"file":"/src/pkg/storage/postgresql.go:45"}
	//dev-1           | [00] im-manager exited due to: failed to setup DB: failed to open Gorm session: ERROR: operator class "gin_trgm_ops" does not exist for access method "btree" (SQLSTATE 42704)exit status 1
	//dev-1           | [00] (error exit: exit status 1)
	//dev-1           | [00] Killing service
	sql := "CREATE INDEX IF NOT EXISTS idx_databases_description ON databases USING gin (description gin_trgm_ops)"
	err = db.Exec(sql).Error
	if err != nil {
		return nil, err
	}

	return db, nil
}
