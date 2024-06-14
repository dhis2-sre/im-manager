package event_test

import (
	"context"
	"errors"
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
		Name:     "group1",
		Hostname: "hostname1",
	}
	err := db.Create(&sharedGroup).Error
	require.NoError(t, err)

	user1 := &model.User{
		Email:      "user1@dhis2.org",
		EmailToken: uuid.New(),
		Groups: []model.Group{
			sharedGroup,
		},
	}
	err = db.Create(user1).Error
	require.NoError(t, err)

	groupExclusiveToUser2 := model.Group{
		Name:     "group2",
		Hostname: "hostname2",
	}
	err = db.Create(&groupExclusiveToUser2).Error
	require.NoError(t, err)

	user2 := &model.User{
		Email:      "user2@dhis2.org",
		EmailToken: uuid.New(),
		Groups: []model.Group{
			sharedGroup,
			groupExclusiveToUser2,
		},
	}
	err = db.Create(user2).Error
	require.NoError(t, err)

	env, err := stream.NewEnvironment(
		stream.NewEnvironmentOptions().
			SetUri(amqpClient.StreamURI(t)))
	require.NoError(t, err, "failed to create new RabbitMQ stream environment")

	streamName := "events"
	err = env.DeclareStream(streamName,
		stream.NewStreamOptions().
			SetMaxSegmentSizeBytes(stream.ByteCapacity{}.MB(1)).
			SetMaxLengthBytes(stream.ByteCapacity{}.MB(20)))
	require.NoError(t, err, "failed to declare RabbitMQ stream")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	// this is only to allow testing using multiple users without bringing in all our auth stack
	authenticator := func(ctx *gin.Context) {
		userParam := ctx.Query("user")
		userID, err := strconv.ParseUint(userParam, 10, 64)
		require.NoErrorf(t, err, "failed to parse query param user=%q into user ID", userParam)

		if userID == uint64(user1.ID) {
			ctx.Set("user", user1)
			return
		} else if userID == uint64(user2.ID) {
			ctx.Set("user", user2)
			return
		}

		require.FailNow(t, "provide query param user=userID in the test")
	}
	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		eventHandler := event.NewHandler(logger, env, streamName)
		event.Routes(engine, authenticator, eventHandler)
	})

	repository := event.NewRepository(db)
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
	store := eventStorer{repository: repository, producer: producer}

	// TODO(ivo) should we wait for the publish confirmation before subscribing?
	event1 := store.storeEvent(t, "db-update", sharedGroup.Name, user1)
	event2 := store.storeEvent(t, "db-update", sharedGroup.Name, user2)
	event3 := store.storeEvent(t, "db-update", sharedGroup.Name, nil)
	event4 := store.storeEvent(t, "instance-update", sharedGroup.Name, nil)

	wantUser1Messages := []*sse.Event{event1, event3, event4}
	wantUser2Messages := []*sse.Event{event2, event3, event4}

	ctxUser2, cancelUser2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelUser2()
	ctxUser1, cancelUser1 := context.WithCancelCause(ctxUser2)

	user1Messages := streamEvents(t, ctxUser1, client.ServerURL+"/events", user1)
	user2Messages := streamEvents(t, ctxUser2, client.ServerURL+"/events", user2)

	t.Log("Waiting on messages...")
	var gotUser1Messages, gotUser2Messages []*sse.Event
	for len(wantUser1Messages) != len(gotUser1Messages) || len(wantUser2Messages) != len(gotUser2Messages) {
		select {
		case <-ctxUser1.Done():
		case <-ctxUser2.Done():
			assert.FailNow(t, "Timed out waiting on messages.")
		case msg := <-user1Messages:
			gotUser1Messages = append(gotUser1Messages, msg)
		case msg := <-user2Messages:
			gotUser2Messages = append(gotUser2Messages, msg)
		}
	}

	assert.EqualValuesf(t, wantUser1Messages, gotUser1Messages, "mismatch in expected messages for user %d", user1.ID)
	assert.EqualValuesf(t, wantUser2Messages, gotUser2Messages, "mismatch in expected messages for user %d", user2.ID)
	t.Log("Every user got their expected first batch of messages")

	cancelUser1(errors.New("drop connection"))
	<-user1Messages // wait for user1 to be unsubscribed before sending new messages to test offset tracking
	ctxUser1, cancelUser1 = context.WithCancelCause(ctxUser2)
	defer cancelUser1(nil)

	event5 := store.storeEvent(t, "instance-update", groupExclusiveToUser2.Name, nil)
	event6 := store.storeEvent(t, "db-update", sharedGroup.Name, nil)
	event7 := store.storeEvent(t, "instance-update", sharedGroup.Name, nil)

	user1Messages = streamEvents(t, ctxUser2, client.ServerURL+"/events", user1)

	wantUser1Messages = []*sse.Event{event6, event7}
	wantUser2Messages = []*sse.Event{event5, event6, event7}

	t.Log("Waiting on messages...")
	gotUser1Messages, gotUser2Messages = nil, nil
	for len(wantUser1Messages) != len(gotUser1Messages) || len(wantUser2Messages) != len(gotUser2Messages) {
		select {
		case <-ctxUser1.Done():
		case <-ctxUser2.Done():
			assert.FailNow(t, "Timed out waiting on messages.")
		case msg := <-user1Messages:
			gotUser1Messages = append(gotUser1Messages, msg)
		case msg := <-user2Messages:
			gotUser2Messages = append(gotUser2Messages, msg)
		}
	}

	assert.EqualValuesf(t, wantUser1Messages, gotUser1Messages, "mismatch in expected messages for user %d", user1.ID)
	assert.EqualValuesf(t, wantUser2Messages, gotUser2Messages, "mismatch in expected messages for user %d", user2.ID)
	t.Log("Every user got their expected second batch of messages")
}

func streamEvents(t *testing.T, ctx context.Context, url string, user *model.User) <-chan *sse.Event {
	out := make(chan *sse.Event)
	go func() {
		sseClient := sse.NewClient(url + fmt.Sprintf("?user=%d", user.ID))
		t.Logf("User %d starts to stream from %q", user.ID, url)
		err := sseClient.SubscribeWithContext(ctx, "", func(msg *sse.Event) {
			select {
			case <-ctx.Done():
				return
			case out <- msg:
			}
		})
		require.NoError(t, err, "failed to stream from %q for user %d", url, user.ID)
	}()
	go func() {
		<-ctx.Done()
		t.Logf("User %d stops to stream from %q due: %v", user.ID, url, context.Cause(ctx))
		close(out)
	}()
	return out
}

type eventStorer struct {
	repository   event.Repository
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

	data := []byte(strconv.Itoa(messageCount))
	// TODO(ivo) replace directly sending a message to RabbitMQ by calling an event repository to
	// store an event in the DB
	err := es.repository.Create(model.Event{
		Kind:      kind,
		GroupName: group,
		User:      owner,
		Payload:   string(data),
	})
	require.NoError(t, err, "failed to store event in the database")

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
	err = es.producer.Send(message)
	require.NoErrorf(t, err, "failed to send message to RabbitMQ stream of kind %q, group %q, user %v", kind, group, owner)

	return &sse.Event{
		Event: []byte(kind),
		Data:  data,
	}
}
