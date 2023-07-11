package inttest

import (
	"fmt"
	"testing"

	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
)

// SetupRabbitMQ creates a RabbitMQ container returning an AMQP client ready to send messages to it.
func SetupRabbitMQ(t *testing.T) *amqpTestClient {
	t.Helper()

	container, err := gnomock.Start(
		rabbitmq.Preset(
			rabbitmq.WithUser("im", "im"),
		),
	)
	require.NoError(t, err, "failed to start RabbitMQ")
	t.Cleanup(func() { require.NoError(t, gnomock.Stop(container), "failed to stop RabbitMQ") })

	URI := fmt.Sprintf(
		"amqp://%s:%s@%s",
		"im", "im",
		container.DefaultAddress(),
	)
	conn, err := amqp.Dial(URI)
	require.NoErrorf(t, err, "failed to connect to RabbitMQ", URI)
	t.Cleanup(func() {
		require.NoErrorf(t, conn.Close(), "failed to close connection to RabbitMQ")
	})

	ch, err := conn.Channel()
	require.NoErrorf(t, err, "failed to open channel to RabbitMQ")
	t.Cleanup(func() {
		require.NoErrorf(t, ch.Close(), "failed to close channel to RabbitMQ")
	})

	return &amqpTestClient{Channel: ch, URI: URI}
}

type amqpTestClient struct {
	Channel *amqp.Channel
	URI     string
}
