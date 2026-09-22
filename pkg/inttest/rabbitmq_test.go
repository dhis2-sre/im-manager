package inttest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/internal/testenv"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	amqpgo "github.com/rabbitmq/amqp091-go"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
	"github.com/stretchr/testify/require"
)

func TestRabbitStreamsAcrossProcesses(t *testing.T) {
	const name = "same-stream"
	rabbit := inttest.SetupRabbitStream(t)
	exists, err := rabbit.Environment.StreamExists(name)
	require.NoError(t, err)
	require.False(t, exists)
	require.NoError(t, rabbit.Environment.DeclareStream(name, stream.NewStreamOptions()))
	if os.Getenv("IM_TEST_RABBIT_CHILD") == "1" {
		return
	}
	config, err := testenv.Shared.RabbitMQ()
	require.NoError(t, err)
	encoded, err := json.Marshal(testenv.Config{RabbitMQ: config})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRabbitStreamsAcrossProcesses$", "-test.count=1") // #nosec G204 -- re-executes this test binary with a fixed selector.
	cmd.Env = append(os.Environ(), testenv.ConfigEnv+"="+string(encoded), "IM_TEST_RABBIT_CHILD=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	exists, err = rabbit.Environment.StreamExists(name)
	require.NoError(t, err)
	require.True(t, exists, "child cleanup must preserve the parent stream")
}

func TestRabbitStreamCleanup(t *testing.T) {
	var rabbit *inttest.Stream
	t.Run("fixture", func(t *testing.T) {
		rabbit = inttest.SetupRabbitStream(t)
		require.NoError(t, rabbit.Environment.DeclareStream("cleanup-stream", stream.NewStreamOptions()))
	})
	require.True(t, rabbit.Environment.IsClosed())
	assertRabbitVhostDeleted(t, rabbit.StreamURI(t))
}

func TestRabbitAMQPIsolationAndCleanup(t *testing.T) {
	left := inttest.SetupRabbitMQAMQP(t)
	_, err := left.Channel.QueueDeclare("same-queue", false, false, false, false, nil)
	require.NoError(t, err)
	require.NoError(t, left.Channel.PublishWithContext(context.Background(), "", "same-queue", false, false, amqpgo.Publishing{Body: []byte("parent")}))
	var right *inttest.AMQP
	t.Run("other fixture", func(t *testing.T) {
		right = inttest.SetupRabbitMQAMQP(t)
		_, err := right.Channel.QueueDeclare("same-queue", false, false, false, false, nil)
		require.NoError(t, err)
		_, received, err := right.Channel.Get("same-queue", true)
		require.NoError(t, err)
		require.False(t, received, "messages must not cross virtual hosts")
	})
	require.True(t, right.Channel.IsClosed())
	assertRabbitVhostDeleted(t, right.URI(t))
	message, received, err := left.Channel.Get("same-queue", true)
	require.NoError(t, err)
	require.True(t, received)
	require.Equal(t, []byte("parent"), message.Body)
}

func assertRabbitVhostDeleted(t *testing.T, uri string) {
	t.Helper()
	config, err := testenv.Shared.RabbitMQ()
	require.NoError(t, err)
	u, err := url.Parse(uri)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.Management+"/api/vhosts/"+url.PathEscape(strings.TrimPrefix(u.Path, "/")), nil)
	require.NoError(t, err)
	req.SetBasicAuth(config.Username, config.Password)
	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusNotFound, response.StatusCode)
}
