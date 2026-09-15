package inttest

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/internal/testenv"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// SetupDB clones the migrated template into a database owned by this fixture.
// Parent tests may share the fixture with their subtests.
func SetupDB(t *testing.T) *gorm.DB {
	t.Helper()
	config, err := testenv.Shared.Postgres()
	require.NoError(t, err)
	admin, err := testenv.Admin(config)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, admin.Close()) })

	name := "test_" + uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = admin.ExecContext(ctx, "CREATE DATABASE "+pq.QuoteIdentifier(name)+" TEMPLATE "+pq.QuoteIdentifier(testenv.Template))
	require.NoError(t, err, "clone test database")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := admin.ExecContext(ctx, "DROP DATABASE "+pq.QuoteIdentifier(name))
		assert.NoError(t, err, "drop test database after closing its clients")
	})
	config.DatabaseName = name
	db, err := storage.ConnectDatabase(slog.New(slog.NewTextHandler(os.Stdout, nil)), config)
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	// Bound the sum of connections from parallel fixtures on the shared server.
	pool.SetMaxOpenConns(5)
	pool.SetMaxIdleConns(2)
	t.Cleanup(func() { assert.NoError(t, pool.Close()) })
	return db
}
