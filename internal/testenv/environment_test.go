package testenv

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

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

func TestStartAllIsConcurrent(t *testing.T) {
	want := []string{"postgres", "redis", "s3", "minio", "rabbitmq"}
	entered := make(chan string, 5)
	release := make(chan struct{})
	done := make(chan struct{})
	wait := func(name string) { entered <- name; <-release }
	postgresErr := errors.New("postgres startup failed")
	redisErr := errors.New("redis startup failed")
	e := &Environment{
		postgres: func() (storage.PostgresqlConfig, error) {
			wait("postgres")
			return storage.PostgresqlConfig{}, postgresErr
		},
		redis: func() (string, error) { wait("redis"); return "", redisErr },
		s3:    func() (string, error) { wait("s3"); return "http://s3:4566", nil },
		minio: func() (string, error) { wait("minio"); return "minio:9000", nil },
		rabbitmq: func() (RabbitMQConfig, error) {
			wait("rabbitmq")
			return RabbitMQConfig{Management: "http://rabbitmq:15672"}, nil
		},
	}
	var config Config
	var err error
	go func() {
		defer close(done)
		config, err = e.StartAll()
	}()
	// Release blocked starts even when an assertion fails.
	unblock := sync.OnceFunc(func() { close(release) })
	defer func() { unblock(); <-done }()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	var got []string
	for range want {
		select {
		case service := <-entered:
			got = append(got, service)
		case <-timer.C:
			t.Fatal("services did not start concurrently")
		}
	}
	require.ElementsMatch(t, want, got)
	unblock()
	<-done
	require.Empty(t, entered, "started an unexpected service")
	require.ErrorIs(t, err, postgresErr)
	require.ErrorIs(t, err, redisErr)
	require.Equal(t, "http://s3:4566", config.S3, "successful starts survive other startup failures")
	require.Equal(t, "minio:9000", config.MinIO)
	require.Equal(t, "http://rabbitmq:15672", config.RabbitMQ.Management)
}
