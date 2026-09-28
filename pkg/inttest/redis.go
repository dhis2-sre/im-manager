package inttest

import (
	"strconv"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/internal/testenv"
	"github.com/go-redis/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SetupRedis leases one logical database. The pool lives in DB 0 so allocation
// is atomic across Go test processes, including concurrent package runs.
func SetupRedis(t *testing.T) *redis.Client {
	t.Helper()
	address, err := testenv.Shared.Redis()
	require.NoError(t, err)
	admin := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { assert.NoError(t, admin.Close()) })

	var value string
	deadline := time.Now().Add(30 * time.Second)
	for {
		value, err = admin.SPop(testenv.RedisPool).Result()
		if err != redis.Nil {
			break
		}
		require.True(t, time.Now().Before(deadline), "Redis fixture pool exhausted; reduce fixture concurrency")
		time.Sleep(10 * time.Millisecond)
	}
	require.NoError(t, err)
	index, err := strconv.Atoi(value)
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: address, DB: index})
	t.Cleanup(func() {
		// Never release a dirty database. Fixture workers must stop before cleanup.
		err := client.FlushDB().Err()
		assert.NoError(t, err)
		assert.NoError(t, client.Close())
		if err == nil {
			assert.NoError(t, admin.SAdd(testenv.RedisPool, value).Err())
		}
	})
	require.NoError(t, client.FlushDB().Err())
	return client
}
