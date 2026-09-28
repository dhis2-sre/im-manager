package inttest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/internal/testenv"
	"github.com/google/uuid"
	amqpgo "github.com/rabbitmq/amqp091-go"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rabbitFixture owns one virtual host; the runner or TestMain owns the broker.
type rabbitFixture struct {
	config testenv.RabbitMQConfig
	vhost  string
}

func setupRabbit(t *testing.T) rabbitFixture {
	t.Helper()
	config, err := testenv.Shared.RabbitMQ()
	require.NoError(t, err)
	fixture := rabbitFixture{config: config, vhost: "im-test-" + uuid.NewString()}
	require.NoError(t, fixture.request(http.MethodPut, "/api/vhosts/"+fixture.vhost, `{}`))
	// Register immediately so permission or client initialization failures also clean up.
	t.Cleanup(func() {
		assert.NoError(t, fixture.request(http.MethodDelete, "/api/vhosts/"+fixture.vhost, ""))
	})
	require.NoError(t, fixture.request(http.MethodPut,
		"/api/permissions/"+fixture.vhost+"/"+url.PathEscape(config.Username),
		`{"configure":".*","write":".*","read":".*"}`))
	return fixture
}

func (f rabbitFixture) request(method, path, body string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, f.config.Management+path, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.SetBasicAuth(f.config.Username, f.config.Password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("RabbitMQ %s %s: HTTP %d: %s", method, path, resp.StatusCode, response)
	}
	return nil
}

// SetupRabbitMQAMQP creates an isolated virtual host and closes its clients
// before deleting the virtual host when this test and its subtests finish.
func SetupRabbitMQAMQP(t *testing.T) *AMQP {
	t.Helper()
	fixture := setupRabbit(t)
	uri, err := fixture.config.URI(fixture.config.AMQP, fixture.vhost)
	require.NoError(t, err)
	conn, err := amqpgo.Dial(uri)
	require.NoError(t, err)
	t.Cleanup(func() {
		if !conn.IsClosed() {
			assert.NoError(t, conn.Close())
		}
	})
	channel, err := conn.Channel()
	require.NoError(t, err)
	t.Cleanup(func() {
		if !channel.IsClosed() {
			assert.NoError(t, channel.Close())
		}
	})
	return &AMQP{Channel: channel, uri: uri}
}

type AMQP struct {
	Channel *amqpgo.Channel
	uri     string
}

func (a *AMQP) URI(t *testing.T) string {
	t.Helper()
	return a.uri
}

// SetupRabbitStream creates an isolated virtual host on the shared broker.
// Tests must close their producers/consumers before this fixture's cleanup.
func SetupRabbitStream(t *testing.T) *Stream {
	t.Helper()
	fixture := setupRabbit(t)
	uri, err := fixture.config.URI(fixture.config.Stream, fixture.vhost)
	require.NoError(t, err)
	env, err := stream.NewEnvironment(stream.NewEnvironmentOptions().SetUri(uri))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, env.Close()) })
	return &Stream{Environment: env, uri: uri}
}

type Stream struct {
	Environment *stream.Environment
	uri         string
}

func (s *Stream) StreamURI(t *testing.T) string {
	t.Helper()
	return s.uri
}

func (s *Stream) StreamPort(t *testing.T) string {
	t.Helper()
	u, err := url.Parse(s.uri)
	require.NoError(t, err)
	return u.Port()
}
