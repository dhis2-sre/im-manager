package event_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/event"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	sse "github.com/r3labs/sse/v2"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/ha"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/message"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventHandler(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	amqpClient := inttest.SetupRabbitMQ(t)

	sharedGroup := model.Group{
		Name:     "eventtest1",
		Hostname: "eventtest1",
	}
	user1 := &model.User{
		ID:         1,
		Email:      "user1@dhis2.org",
		EmailToken: uuid.New(),
		Groups: []model.Group{
			sharedGroup,
		},
	}
	db.Create(user1)
	groupExclusiveToUser2 := model.Group{
		Name:     "eventtest2",
		Hostname: "eventtest2",
	}
	user2 := &model.User{
		ID:         2,
		Email:      "user2@dhis2.org",
		EmailToken: uuid.New(),
		Groups: []model.Group{
			sharedGroup,
			groupExclusiveToUser2,
		},
	}
	db.Create(user2)

	portString := amqpClient.StreamPort(t)
	port, err := strconv.Atoi(portString)
	require.NoError(t, err)
	t.Logf("stream port %d", port)

	env, err := stream.NewEnvironment(
		stream.NewEnvironmentOptions().
			// SetUri(amqpClient.StreamURI(t)))
			SetHost("localhost").
			SetPort(port).
			SetUser("rabbitmq").
			SetPassword("rabbitmq"))
	require.NoError(t, err, "failed to create new RabbitMQ stream environment")

	streamName := "events"
	err = env.DeclareStream(streamName,
		stream.NewStreamOptions().
			SetMaxSegmentSizeBytes(stream.ByteCapacity{}.MB(1)).
			SetMaxLengthBytes(stream.ByteCapacity{}.MB(20)))
	require.NoError(t, err, "failed to declare RabbitMQ stream")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	authenticator := func(ctx *gin.Context) {
		ctx.Set("user", user1)
	}
	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		eventHandler := event.NewHandler(logger, env, streamName)
		event.Routes(engine, authenticator, eventHandler)
	})

	producerName := "eventTestProducer"
	opts := stream.NewProducerOptions().
		SetProducerName(producerName).
		SetClientProvidedName(producerName).
		SetFilter(
			// each message will get the group as filter key
			stream.NewProducerFilter(func(message message.StreamMessage) string {
				return fmt.Sprintf("%s", message.GetApplicationProperties()["group"])
			}))
	producer, err := ha.NewReliableProducer(env, streamName, opts, func(messageStatus []*stream.ConfirmationStatus) {
		go func() {
			for _, msgStatus := range messageStatus {
				if msgStatus.IsConfirmed() {
					t.Logf("Plublishing confirmed for message with publishing_id=%d", msgStatus.GetMessage().GetPublishingId())
				} else {
					t.Logf("Plublishing NOT confirmed for message with publishing_id=%d", msgStatus.GetMessage().GetPublishingId())
				}
			}
		}()
	})
	require.NoError(t, err, "failed to create RabbitMQ producer")
	defer producer.Close()
	store := eventStorer{producer: producer}

	// TODO(ivo) assert both users get messages for the group they are in if there is no owner
	// to do that I need to adapt the authenticator
	// TODO(ivo) assert that only the owner gets the message if there is an owner
	// TODO(ivo) assert that users get messages for different named events
	// store.storeEvent(t, "instance-update", group.Name, nil, "DHIS2 is ready")

	event1 := store.storeEvent(t, "db-update", sharedGroup.Name, user1)
	event2 := store.storeEvent(t, "db-update", sharedGroup.Name, user2)
	event3 := store.storeEvent(t, "db-update", sharedGroup.Name, nil)

	wantUser1Messages := []*sse.Event{event1, event3}
	wantUser2Messages := []*sse.Event{event2, event3}
	_ = wantUser2Messages

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	t.Log("User1 starts streaming /events")
	user1Messages := make(chan *sse.Event)
	go func(ctx context.Context) {
		sseClient := sse.NewClient(client.ServerURL + "/events")
		err = sseClient.SubscribeWithContext(ctx, "", func(msg *sse.Event) {
			user1Messages <- msg
		})
		require.NoError(t, err, "failed to stream from /events")
	}(ctx)
	// TODO(ivo) subscribe with a different user

	t.Log("Waiting on messages...")
	// TODO(ivo) do assert the messages we got on timeout
	var gotUser1Messages []*sse.Event
	for len(wantUser1Messages) != len(gotUser1Messages) {
		select {
		case msg := <-user1Messages:
			gotUser1Messages = append(gotUser1Messages, msg)
		case <-ctx.Done():
			assert.FailNow(t, "Timed out waiting on messages.")
		}
	}

	assert.EqualValuesf(t, wantUser1Messages, gotUser1Messages, "mismatch in expected messages for user %d", user1.ID)
}

type eventStorer struct {
	producer     *ha.ReliableProducer
	messageCount int
	m            sync.Mutex
}

// storeEvent stores an instance manager event and returns the SSE event we expect an eligible user
// to receive via /events.
func (es *eventStorer) storeEvent(t *testing.T, kind, group string, owner *model.User) *sse.Event {
	es.m.Lock()
	messageCount := es.messageCount
	es.messageCount++
	es.m.Unlock()

	// TODO(ivo) replace directly sending a message to RabbitMQ by calling an event repository to
	// store an event in the DB

	data := []byte(strconv.Itoa(messageCount))
	message := amqp.NewMessage(data)
	// set a publishing id for deduplication
	message.SetPublishingId(int64(messageCount))
	// set properties used for filtering
	message.ApplicationProperties = map[string]interface{}{"group": group}
	if owner != nil {
		message.ApplicationProperties["owner"] = strconv.Itoa(int(owner.ID))
	}
	// set property that dictates the SSE event type
	message.ApplicationProperties["type"] = kind
	err := es.producer.Send(message)
	require.NoErrorf(t, err, "failed to send message to RabbitMQ stream of kind %q, group %q, user %v", kind, group, owner)

	return &sse.Event{
		Event: []byte(kind),
		Data:  data,
	}
}
