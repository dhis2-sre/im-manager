// Package testenv owns ephemeral services shared by test fixtures. The runner
// exports their addresses to every Go test process; direct go test runs start
// services lazily and close them from TestMain.
package testenv

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/go-redis/redis"
	"github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const ConfigEnv = "IM_TEST_SERVICES"
const Template = "im_test_template"
const RedisPool = "im-test-free-databases"
const RedisDatabases = 64
const Region = "eu-west-1"
const AccessKey = "minioadmin"
const SecretKey = "minioadmin"

// Config contains only addresses of disposable test services, never application
// configuration. DB 0 in Redis is reserved for coordinating fixture leases.
type Config struct {
	Postgres storage.PostgresqlConfig
	Redis    string
	S3       string
	MinIO    string
}

type Environment struct {
	mu         sync.Mutex
	containers []testcontainers.Container
	postgres   func() (storage.PostgresqlConfig, error)
	redis      func() (string, error)
	s3         func() (string, error)
	minio      func() (string, error)
}

func New() *Environment {
	e := &Environment{}
	e.postgres = sync.OnceValues(e.startPostgres)
	e.redis = sync.OnceValues(e.startRedis)
	e.s3 = sync.OnceValues(func() (string, error) {
		return e.startEndpoint(testcontainers.ContainerRequest{
			Image: "localstack/localstack:3.0.0", ExposedPorts: []string{"4566/tcp"},
			Env:        map[string]string{"SERVICES": "s3", "DEFAULT_REGION": Region},
			WaitingFor: wait.ForHTTP("/_localstack/health").WithPort("4566/tcp"),
		}, "4566/tcp", "http")
	})
	e.minio = sync.OnceValues(func() (string, error) {
		return e.startEndpoint(testcontainers.ContainerRequest{
			Image: "quay.io/minio/minio:RELEASE.2025-01-20T14-49-07Z", ExposedPorts: []string{"9000/tcp"},
			Env:        map[string]string{"MINIO_ROOT_USER": AccessKey, "MINIO_ROOT_PASSWORD": SecretKey},
			Cmd:        []string{"server", "/data"},
			WaitingFor: wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp"),
		}, "9000/tcp", "")
	})
	return e
}

var local = New()

// Resolve uses the runner's services when supplied. Missing services are errors:
// silently falling back would conceal a broken runner configuration.
func Resolve(service string) (Config, error) {
	if value, ok := os.LookupEnv(ConfigEnv); ok {
		var c Config
		if err := json.Unmarshal([]byte(value), &c); err != nil {
			return c, fmt.Errorf("parse %s: %w", ConfigEnv, err)
		}
		present := map[string]bool{"postgres": c.Postgres.Host != "", "redis": c.Redis != "", "s3": c.S3 != "", "minio": c.MinIO != ""}
		if !present[service] {
			return c, fmt.Errorf("runner did not start %s", service)
		}
		return c, nil
	}
	return local.Start([]string{service})
}

func CloseLocal() error { return local.Close() }

// Start starts independent services concurrently. Call Close even on failure.
func (e *Environment) Start(services []string) (Config, error) {
	var c Config
	var mu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	for _, name := range services {
		wg.Go(func() {
			started := time.Now()
			var err error
			var address string
			var pg storage.PostgresqlConfig
			switch name {
			case "postgres":
				pg, err = e.postgres()
			case "redis":
				address, err = e.redis()
			case "s3":
				address, err = e.s3()
			case "minio":
				address, err = e.minio()
			default:
				err = fmt.Errorf("unknown test service %q", name)
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("start %s: %w", name, err))
				return
			}
			switch name {
			case "postgres":
				c.Postgres = pg
			case "redis":
				c.Redis = address
			case "s3":
				c.S3 = address
			case "minio":
				c.MinIO = address
			}
			fmt.Fprintf(os.Stderr, "test service %s ready in %s\n", name, time.Since(started).Round(time.Millisecond))
		})
	}
	wg.Wait()
	return c, errors.Join(errs...)
}

func (e *Environment) startEndpoint(req testcontainers.ContainerRequest, port, scheme string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	// Failed readiness checks can still return an allocated container.
	if c != nil {
		e.mu.Lock()
		e.containers = append(e.containers, c)
		e.mu.Unlock()
	}
	if err != nil {
		return "", err
	}
	return c.PortEndpoint(ctx, port, scheme)
}

func (e *Environment) startPostgres() (storage.PostgresqlConfig, error) {
	c := storage.PostgresqlConfig{Username: "im", Password: "im", DatabaseName: Template}
	address, err := e.startEndpoint(testcontainers.ContainerRequest{
		Image: "postgres:16.2", ExposedPorts: []string{"5432/tcp"},
		Env:        map[string]string{"POSTGRES_USER": c.Username, "POSTGRES_PASSWORD": c.Password, "POSTGRES_DB": Template},
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
	}, "5432/tcp", "")
	if err != nil {
		return c, err
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return c, err
	}
	c.Host = host
	c.Port, err = strconv.Atoi(port)
	if err != nil {
		return c, err
	}
	db, err := storage.NewDatabase(slog.New(slog.NewTextHandler(os.Stderr, nil)), c)
	if err != nil {
		return c, err
	}
	pool, err := db.DB()
	if err != nil {
		return c, err
	}
	// PostgreSQL cannot clone a template with open sessions.
	if err := pool.Close(); err != nil {
		return c, err
	}
	admin, err := Admin(c)
	if err != nil {
		return c, err
	}
	defer admin.Close()
	_, err = admin.Exec("ALTER DATABASE " + pq.QuoteIdentifier(Template) + " WITH ALLOW_CONNECTIONS false")
	return c, err
}

// Admin connects outside the template so CREATE/DROP DATABASE can run safely.
func Admin(c storage.PostgresqlConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable connect_timeout=10", c.Host, c.Port, c.Username, c.Password))
	if err == nil {
		db.SetMaxOpenConns(2)
	}
	return db, err
}

func (e *Environment) startRedis() (string, error) {
	address, err := e.startEndpoint(testcontainers.ContainerRequest{
		Image: "redis:6.0.9", ExposedPorts: []string{"6379/tcp"},
		Cmd:        []string{"redis-server", "--databases", strconv.Itoa(RedisDatabases), "--save", "", "--appendonly", "no"},
		WaitingFor: wait.ForLog("Ready to accept connections"),
	}, "6379/tcp", "")
	if err != nil {
		return "", err
	}
	client := redis.NewClient(&redis.Options{Addr: address})
	defer client.Close()
	ids := make([]interface{}, 0, RedisDatabases-1)
	for i := 1; i < RedisDatabases; i++ {
		ids = append(ids, i)
	}
	return address, client.SAdd(RedisPool, ids...).Err()
}

// Close runs after all test binaries exit. It also handles partial startup.
func (e *Environment) Close() error {
	e.mu.Lock()
	containers := e.containers
	e.containers = nil
	e.mu.Unlock()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	for _, c := range containers {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := c.Terminate(ctx); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}
