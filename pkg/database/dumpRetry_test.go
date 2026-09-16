package database

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTransientPostgresState(t *testing.T) {
	// The message Bitnami's initdb-script phase produces when it stops its temporary server, which
	// is what a save issued moments after a deploy used to fail on.
	shuttingDown := errors.New(`command terminated with exit code 1: pg_dump: error: connection to server at "localhost" (::1), port 5432 failed: FATAL:  the database system is shutting down`)
	assert.True(t, isTransientPostgresState(shuttingDown))

	assert.True(t, isTransientPostgresState(errors.New("FATAL: the database system is starting up")))
	assert.True(t, isTransientPostgresState(errors.New("dial tcp 127.0.0.1:5432: connect: connection refused")))

	// A real dump problem must not be retried, it will not fix itself.
	assert.False(t, isTransientPostgresState(errors.New(`pg_dump: error: query failed: ERROR:  permission denied for table x`)))
	assert.False(t, isTransientPostgresState(errors.New("no such pod")))
}

func TestDumpWithRetries(t *testing.T) {
	service := Service{logger: slog.Default()}

	t.Run("SucceedsOncePostgresFinishesRestarting", func(t *testing.T) {
		attempts := 0
		size, err := service.dumpWithRetries(context.Background(), func() (int64, error) {
			attempts++
			if attempts < 2 {
				return 0, errors.New("FATAL: the database system is shutting down")
			}
			return 4096, nil
		})

		require.NoError(t, err)
		assert.Equal(t, int64(4096), size)
		assert.Equal(t, 2, attempts, "the first attempt hit the restart, the second succeeded")
	})

	t.Run("DoesNotRetryARealFailure", func(t *testing.T) {
		attempts := 0
		_, err := service.dumpWithRetries(context.Background(), func() (int64, error) {
			attempts++
			return 0, errors.New("pg_dump: error: permission denied for table dhis2_chart_seed_complete")
		})

		require.ErrorContains(t, err, "permission denied")
		assert.Equal(t, 1, attempts, "a genuine dump error is reported immediately")
	})

	t.Run("StopsWhenTheContextIsCancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		_, err := service.dumpWithRetries(ctx, func() (int64, error) {
			attempts++
			cancel()
			return 0, errors.New("the database system is starting up")
		})

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, attempts)
	})
}
