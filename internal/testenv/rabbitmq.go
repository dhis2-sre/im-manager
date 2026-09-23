package testenv

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	mobynet "github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// RabbitMQConfig describes one disposable broker shared by isolated virtual hosts.
type RabbitMQConfig struct {
	AMQP       string
	Stream     string
	Management string
	Username   string
	Password   string
}

// RabbitMQ returns the runner broker or starts a package-local broker lazily.
func (e *Environment) RabbitMQ() (RabbitMQConfig, error) {
	c, supplied, err := runnerConfig()
	if err != nil {
		return RabbitMQConfig{}, err
	}
	if supplied {
		if c.RabbitMQ.AMQP == "" || c.RabbitMQ.Stream == "" || c.RabbitMQ.Management == "" {
			return RabbitMQConfig{}, fmt.Errorf("runner did not start rabbitmq")
		}
		return c.RabbitMQ, nil
	}
	return e.rabbitmq()
}

func (e *Environment) startRabbitMQ() (RabbitMQConfig, error) {
	var config RabbitMQConfig
	// The stream protocol advertises a reconnect address. Reserve a host port
	// before startup so the advertised port matches Docker's published port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return config, err
	}
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	if err := listener.Close(); err != nil {
		return config, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        rabbitMQImage,
			ExposedPorts: []string{"5672/tcp", "5552/tcp", "15672/tcp"},
			Env: map[string]string{
				"RABBITMQ_DEFAULT_USER": "guest", "RABBITMQ_DEFAULT_PASS": "guest",
			},
			Files: []testcontainers.ContainerFile{
				{
					Reader:            strings.NewReader("[rabbitmq_management,rabbitmq_stream,rabbitmq_stream_management]."),
					ContainerFilePath: "/etc/rabbitmq/enabled_plugins", FileMode: 0o444,
				},
				{
					Reader:            strings.NewReader(fmt.Sprintf("loopback_users.guest = false\ndisk_free_limit.absolute = 100MB\nstream.advertised_host = localhost\nstream.advertised_port = %s\n", port)),
					ContainerFilePath: "/etc/rabbitmq/conf.d/20-test.conf", FileMode: 0o444,
				},
			},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("5552/tcp"),
				wait.ForHTTP("/api/overview").WithPort("15672/tcp").WithBasicAuth("guest", "guest"),
			),
			HostConfigModifier: func(hc *container.HostConfig) {
				if hc.PortBindings == nil {
					hc.PortBindings = mobynet.PortMap{}
				}
				hc.PortBindings[mobynet.MustParsePort("5552/tcp")] = []mobynet.PortBinding{{HostPort: port}}
			},
		},
	})
	// Keep partial startup allocations so runner/TestMain cleanup owns them too.
	if c != nil {
		e.mu.Lock()
		e.containers = append(e.containers, c)
		e.mu.Unlock()
	}
	if err != nil {
		return config, err
	}
	config.Username, config.Password = "guest", "guest"
	config.AMQP, err = c.PortEndpoint(ctx, "5672/tcp", "amqp")
	if err != nil {
		return config, err
	}
	config.Stream, err = c.PortEndpoint(ctx, "5552/tcp", "rabbitmq-stream")
	if err != nil {
		return config, err
	}
	config.Management, err = c.PortEndpoint(ctx, "15672/tcp", "http")
	return config, err
}

// URI attaches credentials and the fixture's virtual host to a broker endpoint.
func (c RabbitMQConfig) URI(endpoint, vhost string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(c.Username, c.Password)
	u.Path = "/" + vhost
	return u.String(), nil
}
