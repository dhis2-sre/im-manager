package event_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
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
	amqpClient := inttest.SetupRabbitMQ(t, inttest.WithStreamingExposed())

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
	store := NewEventEmitter(t, env, streamName, repository)
	defer store.Close()

	t.Log("Sending messages before users are subscribed")
	// users should only get the next published message after they subscribed so these messages
	// should not be received by anyone
	store.emit(t, "db-update", sharedGroup.Name, user1)
	store.emit(t, "db-update", sharedGroup.Name, user2)
	store.emit(t, "db-update", sharedGroup.Name, nil)
	store.emit(t, "instance-update", sharedGroup.Name, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	ctxUser1, cancelUser1 := context.WithCancelCause(ctx)
	user1Messages := streamEvents(t, ctxUser1, client.ServerURL+"/events", user1, nil)

	t.Log("Sending messages after user1 subscribed")
	// TODO(ivo) I want to block only until the user is subscribed, so I can then test the offset
	// spec next config
	time.Sleep(5 * time.Second)
	event5 := store.emit(t, "db-update", sharedGroup.Name, user1)
	store.emit(t, "db-update", sharedGroup.Name, user2)
	event7 := store.emit(t, "db-update", sharedGroup.Name, nil)
	event8 := store.emit(t, "instance-update", sharedGroup.Name, nil)

	wantUser1Messages := []sseEvent{event5, event7, event8}

	t.Log("Waiting on messages for user1...")
	var gotUser1Messages []sseEvent
	for len(wantUser1Messages) != len(gotUser1Messages) {
		select {
		case <-ctx.Done():
			assert.Fail(t, "Timed out waiting on messages.")
		case msg := <-user1Messages:
			gotUser1Messages = append(gotUser1Messages, msg)
		}
	}
	assert.EqualValuesf(t, wantUser1Messages, gotUser1Messages, "mismatch in expected messages for user %d", user1.ID)

	ctxUser2, cancelUser2 := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancelUser2()
	user2Messages := streamEvents(t, ctxUser2, client.ServerURL+"/events", user2, nil)

	t.Log("Sending messages after user2 subscribed")
	// TODO(ivo) I want to block only until the user is subscribed, so I can then test the offset
	// spec next config
	time.Sleep(5 * time.Second)
	event9 := store.emit(t, "db-update", sharedGroup.Name, user1)
	event10 := store.emit(t, "db-update", sharedGroup.Name, user2)
	event11 := store.emit(t, "db-update", sharedGroup.Name, nil)
	event12 := store.emit(t, "instance-update", groupExclusiveToUser2.Name, nil)

	wantUser1Messages = []sseEvent{event9, event11}
	wantUser2Messages := []sseEvent{event10, event11, event12}

	t.Log("Waiting on messages for both users...")
	gotUser1Messages = nil
	var gotUser2Messages []sseEvent
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

	cancelUser1(errors.New("drop connection"))
	<-user1Messages // wait for user1 to be unsubscribed before sending new messages to test Last-Event-ID

	t.Log("Sending messages after user1 dropped its connection")
	event13 := store.emit(t, "instance-update", groupExclusiveToUser2.Name, nil)
	event14 := store.emit(t, "db-update", sharedGroup.Name, nil)
	event15 := store.emit(t, "instance-update", sharedGroup.Name, nil)

	// When a SSE client disconnects it will send the HTTP header Last-Event-ID with the ID of the
	// event it last received. We want to then send the event after that.
	user1Messages = streamEvents(t, ctxUser2, client.ServerURL+"/events", user1, &event11.ID)

	wantUser1Messages = []sseEvent{event14, event15}
	wantUser2Messages = []sseEvent{event13, event14, event15}

	t.Log("Waiting on messages for both users...")
	gotUser1Messages, gotUser2Messages = nil, nil
	for len(wantUser1Messages) != len(gotUser1Messages) || len(wantUser2Messages) != len(gotUser2Messages) {
		select {
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
}

func streamEvents(t *testing.T, ctx context.Context, url string, user *model.User, lastEventId *int64) <-chan sseEvent {
	out := make(chan sseEvent)
	go func() {
		sseClient := sse.NewClient(url + fmt.Sprintf("?user=%d", user.ID))
		if lastEventId != nil {
			sseClient.Headers["Last-Event-ID"] = strconv.FormatInt(*lastEventId, 10)
			t.Logf("User %d starts to stream from %q from Last-Event-ID %d", user.ID, url, *lastEventId)
		} else {
			t.Logf("User %d starts to stream from %q from next event", user.ID, url)
		}

		err := sseClient.SubscribeWithContext(ctx, "", func(msg *sse.Event) {
			select {
			case <-ctx.Done():
				return
			case out <- translateEvent(t, msg):
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

// sseEvent is the struct we use to assert on received SSE events.
type sseEvent struct {
	ID    int64
	Event string
	Data  []byte
}

func translateEvent(t *testing.T, msg *sse.Event) sseEvent {
	id, err := strconv.ParseInt(string(msg.ID), 10, 64)
	require.NoError(t, err, "failed to parse SSE event ID")
	return sseEvent{
		ID:    id,
		Event: string(msg.Event),
		Data:  msg.Data,
	}
}

type eventEmitter struct {
	repository     event.Repository
	producer       *ha.ReliableProducer
	eventCount     int64
	eventPublished chan error
}

func NewEventEmitter(t *testing.T, env *stream.Environment, streamName string, repository event.Repository) eventEmitter {
	producerName := "eventTestProducer"
	eventPublished := make(chan error)
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
					eventPublished <- nil
				} else {
					eventPublished <- fmt.Errorf("failed to publish message with publishing_id=%d", msgStatus.GetMessage().GetPublishingId())
				}
			}
		}()
	})
	require.NoError(t, err, "failed to create RabbitMQ producer")
	return eventEmitter{repository: repository, producer: producer, eventPublished: eventPublished}
}

func (es *eventEmitter) Close() error {
	return es.producer.Close()
}

// emit emits an instance manager event and returns the SSE event we expect an eligible user to
// receive via /events. This is a blocking operation.
func (es *eventEmitter) emit(t *testing.T, kind, group string, owner *model.User) sseEvent {
	streamOffset := es.eventCount
	es.eventCount++

	data := []byte(strconv.FormatInt(es.eventCount, 10))
	// TODO(ivo) I think this should move to another test, lets discuss.
	err := es.repository.Create(model.Event{
		Kind:      kind,
		GroupName: group,
		User:      owner,
		Payload:   string(data),
	})
	require.NoError(t, err, "failed to store event in the database")

	message := amqp.NewMessage(data)
	// set a publishing id for deduplication
	message.SetPublishingId(es.eventCount)
	// set properties used for filtering
	message.ApplicationProperties = map[string]interface{}{"group": group}
	if owner != nil {
		message.ApplicationProperties["owner"] = strconv.Itoa(int(owner.ID))
	}
	// set property that dictates the SSE event type
	message.ApplicationProperties["type"] = kind
	err = es.producer.Send(message)
	require.NoErrorf(t, err, "failed to send message to RabbitMQ stream of kind %q, group %q, user %v", kind, group, owner)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		assert.FailNow(t, "Timed out waiting on sent message.")
	case err := <-es.eventPublished:
		require.NoError(t, err)
		t.Logf("Sent event %d", es.eventCount)
		return sseEvent{
			ID:    streamOffset,
			Event: kind,
			Data:  data,
		}
	}
	return sseEvent{}
}
