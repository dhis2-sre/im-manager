package inttest

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/config"
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

	db, err := storage.NewDatabase(config.Postgresql{
		Host:         container.Host,
		Port:         container.DefaultPort(),
		Username:     "im",
		Password:     "im",
		DatabaseName: "test_im",
	})
	require.NoError(t, err, "failed to setup DB")
	return db
}
