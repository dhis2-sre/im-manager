// Package inttest provides setup functions that create a RabbitMQ container. These help us test our
// consumer and producer under different network conditions like all connections being dropped. We
// are using the management image for RabbitMQ so you can debug and interact with tests using its
// admin panel. Use a debugger, adjust timeouts waiting for a message or add a time.Sleep and find
// the exposed management port to login to the UI. You will find it easier to debug if your test
// configures the consumers connection and or consumer tag prefix.
package inttest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	amqpgo "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SetupRabbitMQ creates a RabbitMQ with an AMQP client ready to send messages to it.
func SetupRabbitMQ(t *testing.T, options ...rabbitMQOption) *AMQP {
	t.Helper()
	require := require.New(t)
	ctx := context.TODO()

	net, err := network.New(ctx)
	require.NoError(err, "failed setting up Docker network")
	t.Cleanup(func() {
		require.NoError(net.Remove(ctx), "failed to remove the Docker network")
	})

	options = append(options, WithNetwork(net.Name, "rabbitmq"))
	rabbitMQContainer, err := NewRabbitMQ(ctx, options...)
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
// RabbitMQ directly via the low-level github.com/rabbitmq/amqp091-go library. Access the actual
// amqp091-go channel for specific use cases where our defaults don't work.
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

// StreamURI is the stream URI going to RabbitMQ.
func (a *AMQP) StreamURI(t *testing.T) string {
	t.Helper()

	URI, err := a.rabbitMQContainer.StreamURI(context.TODO())
	require.NoError(t, err, "failed to get RabbitMQ stream URI")
	return URI
}

func (a *AMQP) StreamPort(t *testing.T) string {
	t.Helper()

	port, err := a.rabbitMQContainer.ExposedStreamPort(context.TODO())
	require.NoError(t, err, "failed to get RabbitMQ stream port")
	return port
}

// Publish a message to given queue.
func (a *AMQP) Publish(t *testing.T, queue, message string) {
	t.Helper()

	err := a.Channel.PublishWithContext(context.TODO(), "", queue, false, false, amqpgo.Publishing{
		DeliveryMode: amqpgo.Persistent,
		Body:         []byte(message),
	})
	require.NoError(t, err, "failed to publish message to queue %q", queue)
}

// PublishEvery publishes a message to given queue in given interval until done is closed. This will
// go directly to RabbitMQ using URI.
func (a *AMQP) PublishEvery(t *testing.T, tick time.Duration, done chan struct{}, queue, message string) {
	t.Helper()

	go func() {
		timer := time.NewTicker(tick)
		defer timer.Stop()
		for {
			select {
			case <-done:
				return
			case <-timer.C:
				a.Publish(t, queue, message)
			}
		}
	}()
}

func (a *AMQP) Restart(t *testing.T) {
	t.Helper()

	require.NoError(t, a.rabbitMQContainer.Stop(context.TODO(), nil), "failed to stop RabbitMQ")
	require.NoError(t, a.rabbitMQContainer.Start(context.TODO()), "failed to start RabbitMQ")

	URI, err := a.rabbitMQContainer.AMQPURI(context.TODO())
	require.NoError(t, err, "failed to get RabbitMQ AMQP URI")
	conn, err := amqpgo.Dial(URI)
	require.NoError(t, err, "failed setting up AMQP connection")
	a.conn = conn
	channel, err := conn.Channel()
	require.NoError(t, err, "failed setting up AMQP channel")
	a.Channel = channel

	t.Logf("Restarted RabbitMQ with new connection and channel for publishing to URI %q", URI)
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
	port, err := rc.MappedPort(ctx, nat.Port("5672/tcp"))
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
	port, err := rc.MappedPort(ctx, nat.Port("5552/tcp"))
	if err != nil {
		return "", err
	}
	return port.Port(), nil
}

type rabbitMQOptions struct {
	user            string
	pw              string
	network         string
	networkAlias    string
	exposeStreaming bool
}

type rabbitMQOption func(*rabbitMQOptions)

func WithUser(user string) rabbitMQOption {
	return func(options *rabbitMQOptions) {
		options.user = user
	}
}

func WithPassword(pw string) rabbitMQOption {
	return func(options *rabbitMQOptions) {
		options.pw = pw
	}
}

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
	opts := &rabbitMQOptions{
		user: "rabbitmq",
		pw:   "rabbitmq",
	}
	for _, o := range options {
		o(opts)
	}

	AMQPPort := "5672"
	natAMQPPort := AMQPPort + "/tcp"
	natPortMgmt := "15672/tcp"
	exposedPorts := []string{natAMQPPort, natPortMgmt}
	if opts.exposeStreaming {
		exposedPorts = append(exposedPorts, "5552:5552/tcp")
	}
	req := testcontainers.ContainerRequest{
		Image: "rabbitmq:3.13-management-alpine",
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER":               opts.user,
			"RABBITMQ_DEFAULT_PASS":               opts.pw,
			"RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS": `-rabbitmq_stream advertised_host localhost`,
		},
		ExposedPorts: exposedPorts,
		Files: []testcontainers.ContainerFile{
			{
				Reader:            strings.NewReader(`[rabbitmq_management, rabbitmq_management_agent, rabbitmq_stream, rabbitmq_stream_management].`),
				ContainerFilePath: "/etc/rabbitmq/enabled_plugins",
				FileMode:          0o444,
			},
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort(nat.Port(natAMQPPort)),
			WaitForRabbitMQ(opts.user, opts.pw, natAMQPPort),
		).WithDeadline(time.Minute),
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
		internalPort: AMQPPort,
		user:         opts.user,
		pw:           opts.pw,
	}, nil
}

func amqpURI(user, pw, ip, port string) string {
	return fmt.Sprintf("amqp://%s:%s@%s:%s", user, pw, ip, port)
}

func streamURI(user, pw, ip, port string) string {
	return fmt.Sprintf("rabbitmq-stream://%s:%s@%s:%s", user, pw, ip, port)
}

// rabbitMQstrategy implements testcontainers wait.Strategy to ensure RabbitMQ is up and we can
// connect to it using given credentials.
type rabbitMQStrategy struct {
	usr            string
	pw             string
	port           string
	startupTimeout time.Duration
}

func WaitForRabbitMQ(user, password, port string) *rabbitMQStrategy {
	return &rabbitMQStrategy{usr: user, pw: password, port: port, startupTimeout: time.Minute}
}

func (rbw *rabbitMQStrategy) WithStartupTimeout(timeout time.Duration) *rabbitMQStrategy {
	rbw.startupTimeout = timeout
	return rbw
}

func (rbw *rabbitMQStrategy) WaitUntilReady(ctx context.Context, target wait.StrategyTarget) error {
	// limit context to startupTimeout
	ctx, cancelContext := context.WithTimeout(ctx, rbw.startupTimeout)
	defer cancelContext()

	ipAddress, err := target.Host(ctx)
	if err != nil {
		return nil
	}

	waitInterval := 50 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s:%w", ctx.Err(), err)
		case <-time.After(waitInterval):
			port, err := target.MappedPort(ctx, nat.Port(rbw.port))
			if err != nil {
				return fmt.Errorf("No mapped port for RabbitMQ found: %w", err)
			}
			conn, err := rbw.connect(ipAddress, port.Port())
			if err != nil {
				fmt.Printf("Connection to RabbitMQ failed: %s\n", err)
				continue
			}
			defer conn.Close()

			return nil
		}
	}
}

func (rbw *rabbitMQStrategy) connect(ip, port string) (*amqpgo.Connection, error) {
	uri := amqpURI(rbw.usr, rbw.pw, ip, port)
	fmt.Printf("Waiting for RabbitMQ connection to: %q\n", uri)
	return amqpgo.Dial(uri)
}
