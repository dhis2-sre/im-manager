package notification_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/notification"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The publisher must not deduplicate. RabbitMQ deduplicates per publisher reference by discarding
// anything whose publishing id it has already seen, and it confirms those discards, so the
// confirmation callback the publisher already has cannot notice. Only reading the stream back can.
//
// Delivery is at least once, since dropping the producer name also gave up the deduplication that
// collapsed the reliable producer's own re-sends. These tests therefore assert that nothing is
// missing rather than that nothing is repeated.
func TestPublisherDeliversEveryEvent(t *testing.T) {
	t.Parallel()

	rabbit := inttest.SetupRabbitStream(t)

	newPublisher := func(t *testing.T, streamName string) *notification.Publisher {
		t.Helper()
		publisher, err := notification.NewPublisher(slog.Default(), rabbit.Environment, streamName, nil)
		require.NoError(t, err, "failed to create publisher")
		return publisher
	}

	// A restart is what made this visible in production: a named producer is seeded from the
	// stream's high water mark, so a publisher whose publishing ids start over sends ids the broker
	// has already seen, and every one of them is discarded.
	t.Run("AcrossARestart", func(t *testing.T) {
		streamName := declareStream(t, rabbit, "restart")

		delivered := collect(t, rabbit.Environment, streamName)

		first := newPublisher(t, streamName)
		first.PublishTransient(context.Background(), "group-a", "event", payload{ID: "before-restart"})
		// Waiting for it to land rather than closing straight away, since Close does not reliably
		// flush a message sent a moment earlier, which would make this a test of that instead.
		delivered.waitFor(t, "before-restart")
		require.NoError(t, first.Close())

		second := newPublisher(t, streamName)
		t.Cleanup(func() { _ = second.Close() })
		second.PublishTransient(context.Background(), "group-a", "event", payload{ID: "after-restart"})

		delivered.waitFor(t, "after-restart")
	})

	// Publishing ids are assigned and enqueued as separate steps, so concurrent publishes can reach
	// the broker out of the order they were numbered in. Deduplication would silently discard
	// whichever arrived below the mark, which is the other reason this producer carries no name.
	t.Run("FromConcurrentGoroutines", func(t *testing.T) {
		streamName := declareStream(t, rabbit, "concurrent")

		publisher := newPublisher(t, streamName)
		t.Cleanup(func() { _ = publisher.Close() })

		const events = 20
		var wg sync.WaitGroup
		for i := range events {
			wg.Add(1)
			go func() {
				defer wg.Done()
				publisher.PublishTransient(context.Background(), "group-b", "event", payload{ID: fmt.Sprint(i)})
			}()
		}
		wg.Wait()

		delivered := collect(t, rabbit.Environment, streamName)
		for i := range events {
			delivered.waitFor(t, fmt.Sprint(i))
		}
	})
}

type payload struct {
	ID string `json:"id"`
}

func declareStream(t *testing.T, rabbit *inttest.Stream, name string) string {
	t.Helper()

	streamName := "events-" + name
	require.NoError(t, rabbit.Environment.DeclareStream(streamName, stream.NewStreamOptions()),
		"failed to declare stream %q", streamName)
	return streamName
}

// collect reads the stream from the start, recording the id of every payload it sees, so a test can
// wait for a particular event to arrive rather than guess how long publishing takes.
func collect(t *testing.T, env *stream.Environment, streamName string) *delivered {
	t.Helper()

	d := &delivered{ids: map[string]bool{}}
	consumer, err := env.NewConsumer(streamName, func(_ stream.ConsumerContext, msg *amqp.Message) {
		var p payload
		if err := json.Unmarshal(msg.GetData(), &p); err != nil {
			return
		}

		d.mu.Lock()
		defer d.mu.Unlock()
		d.ids[p.ID] = true
	}, stream.NewConsumerOptions().SetOffset(stream.OffsetSpecification{}.First()))
	require.NoError(t, err, "failed to create consumer")
	t.Cleanup(func() { _ = consumer.Close() })

	return d
}

type delivered struct {
	mu  sync.Mutex
	ids map[string]bool
}

func (d *delivered) waitFor(t *testing.T, id string) {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		seen := d.ids[id]
		d.mu.Unlock()
		if seen {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.Failf(t, "event never reached the stream", "no event with id %q arrived", id)
}
