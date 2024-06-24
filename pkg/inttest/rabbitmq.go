// Package inttest provides setup functions that create a RabbitMQ container. We are using the
// management image for RabbitMQ so you can debug and interact with tests using its admin panel. Use
// a debugger, adjust timeouts waiting for a message or add a time.Sleep and find the exposed
// management port to login to the UI. You will find it easier to debug if your test configures the
// consumers connection and or consumer tag prefix.
package inttest

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/docker/go-connections/nat"
	amqpgo "github.com/rabbitmq/amqp091-go"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const amqpPort = "5672"
const natAMQPPort = amqpPort + "/tcp"
const streamPort = "5552"
const natStreamPort = streamPort + "/tcp"

// SetupRabbitMQAMQP creates a RabbitMQ with an AMQP client ready to send messages to it.
func SetupRabbitMQAMQP(t *testing.T, options ...rabbitMQOption) *AMQP {
	t.Helper()
	require := require.New(t)
	ctx := context.TODO()

	net, err := network.New(ctx)
	require.NoError(err, "failed setting up Docker network")
	t.Cleanup(func() {
		require.NoError(net.Remove(ctx), "failed to remove the Docker network")
	})

	rabbitMQContainer, err := NewRabbitMQ(ctx, WithNetwork(net.Name, "rabbitmq"))
	require.NoError(err, "failed setting up RabbitMQ")
	t.Cleanup(func() {
		require.NoError(rabbitMQContainer.Terminate(ctx), "failed to terminate RabbitMQ")
	})

	URI, err := rabbitMQContainer.AMQPURI(ctx)
	require.NoError(err, "failed to get RabbitMQ AMQP URI")
	conn, err := amqpgo.Dial(URI)
	require.NoError(err, "failed setting up AMQP connection")
	channel, err := conn.Channel()
	require.NoError(err, "failed setting up AMQP channel")

	return &AMQP{
		rabbitMQContainer: rabbitMQContainer,
		conn:              conn,
		Channel:           channel,
	}
}

// AMQP allows making requests to RabbitMQ. It does so by opening a connection and channel to
// RabbitMQ via the low-level github.com/rabbitmq/amqp091-go library.
type AMQP struct {
	rabbitMQContainer *rabbitmqContainer
	conn              *amqpgo.Connection // Connection established with RabbitMQ
	Channel           *amqpgo.Channel    // Channel established with RabbitMQ
}

// URI is the AMQP URI going to RabbitMQ.
func (a *AMQP) URI(t *testing.T) string {
	t.Helper()

	URI, err := a.rabbitMQContainer.AMQPURI(context.TODO())
	require.NoError(t, err, "failed to get RabbitMQ URI")
	return URI
}

// SetupRabbitStream creates a RabbitMQ container with a streaming environment ready to create a
// producer or consumer.
func SetupRabbitStream(t *testing.T) *Stream {
	t.Helper()
	require := require.New(t)
	ctx := context.TODO()

	net, err := network.New(ctx)
	require.NoError(err, "failed setting up Docker network")
	t.Cleanup(func() {
		require.NoError(net.Remove(ctx), "failed to remove the Docker network")
	})

	rabbitMQContainer, err := NewRabbitMQ(ctx, WithNetwork(net.Name, "rabbitmq"), WithStreamingExposed())
	require.NoError(err, "failed setting up RabbitMQ")
	t.Cleanup(func() {
		require.NoError(rabbitMQContainer.Terminate(ctx), "failed to terminate RabbitMQ")
	})

	URI, err := rabbitMQContainer.StreamURI(ctx)
	require.NoError(err, "failed to get RabbitMQ stream URI")
	env, err := stream.NewEnvironment(
		stream.NewEnvironmentOptions().
			SetUri(URI))
	require.NoError(err, "failed to create new RabbitMQ stream environment")

	return &Stream{
		rabbitMQContainer: rabbitMQContainer,
		Environment:       env,
	}
}

// Stream allows making requests to RabbitMQ. It does so by opening a stream environment via
// github.com/rabbitmq/rabbitmq-stream-go-client.
type Stream struct {
	rabbitMQContainer *rabbitmqContainer
	Environment       *stream.Environment
}

// StreamURI is the stream URI going to RabbitMQ.
func (s *Stream) StreamURI(t *testing.T) string {
	t.Helper()

	URI, err := s.rabbitMQContainer.StreamURI(context.TODO())
	require.NoError(t, err, "failed to get RabbitMQ stream URI")
	return URI
}

func (s *Stream) StreamPort(t *testing.T) string {
	t.Helper()

	port, err := s.rabbitMQContainer.ExposedStreamPort(context.TODO())
	require.NoError(t, err, "failed to get RabbitMQ stream port")
	return port
}

type rabbitmqContainer struct {
	testcontainers.Container
	user         string
	pw           string
	network      string
	networkAlias string
	internalPort string
}

func (rc *rabbitmqContainer) AMQPURI(ctx context.Context) (string, error) {
	ip, err := rc.Host(ctx)
	if err != nil {
		return "", err
	}
	port, err := rc.ExposedAMQPPort(ctx)
	if err != nil {
		return "", err
	}
	return amqpURI(rc.user, rc.pw, ip, port), nil
}

func (rc *rabbitmqContainer) ExposedAMQPPort(ctx context.Context) (string, error) {
	port, err := rc.MappedPort(ctx, nat.Port(natAMQPPort))
	if err != nil {
		return "", err
	}
	return port.Port(), nil
}

func (rc *rabbitmqContainer) StreamURI(ctx context.Context) (string, error) {
	ip, err := rc.Host(ctx)
	if err != nil {
		return "", err
	}
	port, err := rc.ExposedStreamPort(ctx)
	if err != nil {
		return "", err
	}
	return streamURI(rc.user, rc.pw, ip, port), nil
}

func (rc *rabbitmqContainer) ExposedStreamPort(ctx context.Context) (string, error) {
	port, err := rc.MappedPort(ctx, nat.Port(natStreamPort))
	if err != nil {
		return "", err
	}
	return port.Port(), nil
}

type rabbitMQOptions struct {
	network         string
	networkAlias    string
	exposeStreaming bool
}

type rabbitMQOption func(*rabbitMQOptions)

// WithNetwork connects the RabbitMQ container to a specific network and gives it an alias with
// which you can reach it on this network.
func WithNetwork(name, alias string) rabbitMQOption {
	return func(options *rabbitMQOptions) {
		options.network = name
		options.networkAlias = alias
	}
}

// WithStreamingExposed exposes RabbitMQ streaming port 5552 to a fixed port of 5552.
func WithStreamingExposed() rabbitMQOption {
	return func(options *rabbitMQOptions) {
		options.exposeStreaming = true
	}
}

// NewRabbitMQ creates a RabbitMQ container. The container will be listening and ready to accept
// connections. Connect using default user and password rabbitmq or the credentials you provided via
// the options.
func NewRabbitMQ(ctx context.Context, options ...rabbitMQOption) (*rabbitmqContainer, error) {
	opts := &rabbitMQOptions{}
	for _, o := range options {
		o(opts)
	}

	user := "guest"
	pw := "guest"
	natPortMgmt := "15672/tcp"
	exposedPorts := []string{natAMQPPort, natPortMgmt}
	if opts.exposeStreaming {
		exposedPorts = append(exposedPorts, fmt.Sprintf("%s:%s", streamPort, natStreamPort))
	}
	req := testcontainers.ContainerRequest{
		Image: "bitnami/rabbitmq:3.13",
		Env: map[string]string{
			"RABBITMQ_USERNAME":                    user,
			"RABBITMQ_PASSWORD":                    pw,
			"BITNAMI_DEBUG":                        "true",
			"RABBITMQ_MANAGEMENT_ALLOW_WEB_ACCESS": "true",
			"RABBITMQ_DISK_FREE_ABSOLUTE_LIMIT":    "100MB",
			"RABBITMQ_PLUGINS":                     "rabbitmq_management,rabbitmq_management_agent,rabbitmq_stream,rabbitmq_stream_management",
		},
		ExposedPorts: exposedPorts,
		Files: []testcontainers.ContainerFile{
			{
				Reader:            strings.NewReader(`SERVER_ADDITIONAL_ERL_ARGS="-rabbitmq_stream advertised_host localhost"`),
				ContainerFilePath: "/etc/rabbitmq/rabbitmq-env.conf",
				FileMode:          0o444,
			},
		},
		WaitingFor: wait.ForLog("Time to start RabbitMQ").WithOccurrence(2),
	}
	if opts.network != "" {
		req.Networks = []string{opts.network}
		req.NetworkAliases = map[string][]string{
			opts.network: {opts.networkAlias},
		}
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	return &rabbitmqContainer{
		Container:    container,
		network:      opts.network,
		networkAlias: opts.networkAlias,
		internalPort: amqpPort,
		user:         user,
		pw:           pw,
	}, nil
}

func amqpURI(user, pw, ip, port string) string {
	return fmt.Sprintf("amqp://%s:%s@%s:%s", user, pw, ip, port)
}

func streamURI(user, pw, ip, port string) string {
	return fmt.Sprintf("rabbitmq-stream://%s:%s@%s:%s", user, pw, ip, port)
}
