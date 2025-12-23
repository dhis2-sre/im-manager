package inttest

import (
	"log/slog"
	"os"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/storage"
	_ "github.com/lib/pq" // postgres driver
	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/postgres"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// SetupDB creates a PostgreSQL container. Gorm is connected to the DB and runs the migrations.
func SetupDB(t *testing.T) *gorm.DB {
	t.Helper()

	container, err := gnomock.Start(
		postgres.Preset(
			postgres.WithUser("im", "im"),
			postgres.WithDatabase("test_im"),
		),
	)
	require.NoError(t, err, "failed to start DB")
	t.Cleanup(func() { require.NoError(t, gnomock.Stop(container), "failed to stop DB") })

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	db, err := storage.NewDatabase(logger, storage.PostgresqlConfig{
		Host:         container.Host,
		Port:         container.DefaultPort(),
		Username:     "im",
		Password:     "im",
		DatabaseName: "test_im",
	})
	require.NoError(t, err, "failed to setup DB")

	sql := "CREATE EXTENSION IF NOT EXISTS pg_trgm"
	err = db.Exec(sql).Error
	require.NoError(t, err, "failed to create DB extension")

	return db
}
