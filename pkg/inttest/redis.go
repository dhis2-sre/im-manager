package inttest

import (
	"testing"

	"github.com/go-redis/redis"
	"github.com/orlangure/gnomock"
	gnomockRedis "github.com/orlangure/gnomock/preset/redis"
	"github.com/stretchr/testify/require"
)

func SetupRedis(t *testing.T) *redis.Client {
	container, err := gnomock.Start(gnomockRedis.Preset())
	require.NoError(t, err, "failed to start Redis")
	t.Cleanup(func() { require.NoError(t, gnomock.Stop(container), "failed to stop Redis") })

	client := &redis.Options{
		Addr:     container.DefaultAddress(),
		Password: "",
		DB:       0,
	}
	return redis.NewClient(client)
}
