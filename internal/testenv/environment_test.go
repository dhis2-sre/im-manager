package testenv

import (
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/stretchr/testify/require"
)

func TestTypedServicesUseRunnerConfig(t *testing.T) {
	config := Config{
		RabbitMQ: RabbitMQConfig{AMQP: "amqp://runner:5672", Stream: "rabbitmq-stream://runner:5552", Management: "http://runner:15672"},
		Postgres: storage.PostgresqlConfig{Host: "runner-postgres", Port: 5432},
		Redis:    "runner-redis:6379", S3: "http://runner-s3:4566", MinIO: "runner-minio:9000",
	}
	encoded, err := json.Marshal(config)
	require.NoError(t, err)
	// No local startup functions: these calls must use only the supplied config.
	e := &Environment{}
	accessors := []struct {
		name string
		get  func() (any, error)
		want any
	}{
		{"rabbitmq", func() (any, error) { return e.RabbitMQ() }, config.RabbitMQ},
		{"postgres", func() (any, error) { return e.Postgres() }, config.Postgres},
		{"redis", func() (any, error) { return e.Redis() }, config.Redis},
		{"s3", func() (any, error) { return e.S3() }, config.S3},
		{"minio", func() (any, error) { return e.MinIO() }, config.MinIO},
	}
	for _, accessor := range accessors {
		t.Run(accessor.name, func(t *testing.T) {
			t.Setenv(ConfigEnv, string(encoded))
			got, err := accessor.get()
			require.NoError(t, err)
			require.Equal(t, accessor.want, got)

			t.Setenv(ConfigEnv, `{}`)
			_, err = accessor.get()
			require.EqualError(t, err, "runner did not start "+accessor.name)

			t.Setenv(ConfigEnv, `broken-json`)
			_, err = accessor.get()
			require.ErrorContains(t, err, "parse "+ConfigEnv)
		})
	}
}

func TestTypedServicesStartOnlyRequestedLocalService(t *testing.T) {
	t.Setenv(ConfigEnv, "")
	require.NoError(t, os.Unsetenv(ConfigEnv))
	starts := 0
	e := &Environment{
		redis: sync.OnceValues(func() (string, error) {
			starts++
			return "local-redis:6379", nil
		}),
	}
	// Concurrent fixture access must share the same lazy startup. Other services
	// have no startup functions and must remain untouched.
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			address, err := e.Redis()
			if err != nil || address != "local-redis:6379" {
				t.Errorf("Redis() = %q, %v", address, err)
			}
		})
	}
	wg.Wait()
	require.Equal(t, 1, starts)
}
