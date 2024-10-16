package event_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/event"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

	rabbitmq := inttest.SetupRabbitStream(t)
	streamName := "test-event-handler"
	err = rabbitmq.Environment.DeclareStream(streamName,
		stream.NewStreamOptions().
			SetMaxSegmentSizeBytes(stream.ByteCapacity{}.MB(1)).
			SetMaxLengthBytes(stream.ByteCapacity{}.MB(20)))
	require.NoError(t, err, "failed to declare RabbitMQ stream")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	// this is only to allow testing using multiple users without bringing in all our auth stack
	authenticator := func(c *gin.Context) {
		userParam := c.Query("user")
		userID, err := strconv.ParseUint(userParam, 10, 64)
		require.NoErrorf(t, err, "failed to parse query param user=%q into user ID", userParam)

		if userID == uint64(user1.ID) {
			ctx := model.NewContextWithUser(c.Request.Context(), user1)
			c.Request = c.Request.WithContext(ctx)
			return
		} else if userID == uint64(user2.ID) {
			ctx := model.NewContextWithUser(c.Request.Context(), user2)
			c.Request = c.Request.WithContext(ctx)
			return
		}

		require.FailNow(t, "provide query param user=userID in the test")
	}
	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		eventHandler := event.NewHandler(logger, rabbitmq.Environment, streamName)
		event.Routes(engine, authenticator, eventHandler)
	})

	eventEmitter := NewEventEmitter(t, rabbitmq.Environment, streamName)
	defer eventEmitter.Close()

	t.Log("Sending messages before users are subscribed")
	// users should only get the next published message after they subscribed so these messages
	// should not be received by anyone
	eventEmitter.emit(t, "db-update", sharedGroup.Name, user1)
	eventEmitter.emit(t, "db-update", sharedGroup.Name, user2)
	eventEmitter.emit(t, "db-update", sharedGroup.Name, nil)
	eventEmitter.emit(t, "instance-update", sharedGroup.Name, nil)

	ctx, cancel := context.WithTimeoutCause(context.Background(), 100*time.Second, errors.New("test timed out"))
	defer cancel()
	ctxUser1, cancelUser1 := context.WithCancelCause(ctx)
	defer cancelUser1(nil)
	user1Messages := streamEvents(t, ctxUser1, client, user1, nil)

	t.Log("Sending messages after user1 subscribed")
	event5 := eventEmitter.emit(t, "db-update", sharedGroup.Name, user1)
	eventEmitter.emit(t, "db-update", sharedGroup.Name, user2)
	event7 := eventEmitter.emit(t, "db-update", sharedGroup.Name, nil)
	event8 := eventEmitter.emit(t, "instance-update", sharedGroup.Name, nil)

	wantUser1Messages := []sseEvent{event5, event7, event8}

	t.Log("Waiting on messages for user1...")
	var gotUser1Messages []sseEvent
	for len(wantUser1Messages) != len(gotUser1Messages) {
		select {
		case <-ctx.Done():
			require.Fail(t, "Timed out waiting on messages.")
		case msg := <-user1Messages:
			gotUser1Messages = append(gotUser1Messages, msg)
		}
	}
	require.EqualValuesf(t, wantUser1Messages, gotUser1Messages, "mismatch in expected messages for user %d", user1.ID)
	t.Log("Got correct messages for user1")

	ctxUser2, cancelUser2 := context.WithTimeoutCause(ctx, 50*time.Second, errors.New("user2 context timed out"))
	defer cancelUser2()
	user2Messages := streamEvents(t, ctxUser2, client, user2, nil)

	t.Log("Sending messages after user2 subscribed")
	event9 := eventEmitter.emit(t, "db-update", sharedGroup.Name, user1)
	event10 := eventEmitter.emit(t, "db-update", sharedGroup.Name, user2)
	event11 := eventEmitter.emit(t, "db-update", sharedGroup.Name, nil)
	event12 := eventEmitter.emit(t, "instance-update", groupExclusiveToUser2.Name, nil)

	wantUser1Messages = []sseEvent{event9, event11}
	wantUser2Messages := []sseEvent{event10, event11, event12}

	t.Log("Waiting on messages for both users...")
	gotUser1Messages = nil
	var gotUser2Messages []sseEvent
	for len(wantUser1Messages) != len(gotUser1Messages) || len(wantUser2Messages) != len(gotUser2Messages) {
		select {
		case <-ctxUser1.Done():
		case <-ctxUser2.Done():
			require.FailNow(t, "Timed out waiting on messages.")
		case msg := <-user1Messages:
			gotUser1Messages = append(gotUser1Messages, msg)
		case msg := <-user2Messages:
			gotUser2Messages = append(gotUser2Messages, msg)
		}
	}
	require.EqualValuesf(t, wantUser1Messages, gotUser1Messages, "mismatch in expected messages for user %d", user1.ID)
	require.EqualValuesf(t, wantUser2Messages, gotUser2Messages, "mismatch in expected messages for user %d", user2.ID)
	t.Log("Got correct messages for both users")

	cancelUser1(errors.New("drop connection"))
	<-user1Messages // wait for user1 to be unsubscribed before sending new messages to test Last-Event-ID

	t.Log("Sending messages after user1 dropped its connection")
	event13 := eventEmitter.emit(t, "instance-update", groupExclusiveToUser2.Name, nil)
	event14 := eventEmitter.emit(t, "db-update", sharedGroup.Name, nil)
	event15 := eventEmitter.emit(t, "instance-update", sharedGroup.Name, nil)

	// When an SSE client disconnects it will send the HTTP header Last-Event-ID with the ID of the
	// event it last received. We want to then send the event after that.
	user1Messages = streamEvents(t, ctxUser2, client, user1, &event11.ID)

	wantUser1Messages = []sseEvent{event14, event15}
	wantUser2Messages = []sseEvent{event13, event14, event15}

	t.Log("Waiting on messages for both users...")
	gotUser1Messages, gotUser2Messages = nil, nil
	for len(wantUser1Messages) != len(gotUser1Messages) || len(wantUser2Messages) != len(gotUser2Messages) {
		select {
		case <-ctxUser2.Done():
			require.FailNow(t, "Timed out waiting on messages.")
		case msg := <-user1Messages:
			gotUser1Messages = append(gotUser1Messages, msg)
		case msg := <-user2Messages:
			gotUser2Messages = append(gotUser2Messages, msg)
		}
	}
	assert.EqualValuesf(t, wantUser1Messages, gotUser1Messages, "mismatch in expected messages for user %d", user1.ID)
	assert.EqualValuesf(t, wantUser2Messages, gotUser2Messages, "mismatch in expected messages for user %d", user2.ID)
	t.Log("Got correct messages for both users")
}

func streamEvents(t *testing.T, ctx context.Context, client *inttest.HTTPClient, user *model.User, lastEventId *int64) <-chan sseEvent {
	url := fmt.Sprintf("%s/events?user=%d", client.ServerURL, user.ID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	require.NoErrorf(t, err, "User %d failed to create request to stream from %q", user.ID, url)

	if lastEventId != nil {
		req.Header.Add("Last-Event-ID", strconv.FormatInt(*lastEventId, 10))
		t.Logf("User %d starts to stream from %q from Last-Event-ID %d", user.ID, url, *lastEventId)
	} else {
		t.Logf("User %d starts to stream from %q from next event", user.ID, url)
	}

	resp, err := client.Client.Do(req)
	require.NoErrorf(t, err, "User %d failed to stream from %q", user.ID, url)
	require.Equal(t, http.StatusOK, resp.StatusCode, "User %d failed to stream from %q", user.ID, url)

	out := make(chan sseEvent)
	go func() {
		defer resp.Body.Close()

		sc := bufio.NewScanner(resp.Body)
		var event sseEvent
		var gotData bool
		var newlineCount int
		for sc.Scan() {
			line := sc.Text()
			field, fieldValue, found := strings.Cut(line, ":")
			if found {
				switch field {
				case "id":
					id, err := strconv.ParseInt(fieldValue, 10, 64)
					require.NoError(t, err, "failed to parse SSE event ID in line %q", line)
					event.ID = id
				case "event":
					event.Event = fieldValue
				case "data":
					event.Data = fieldValue
					gotData = true
					newlineCount++
				}
			} else if gotData {
				newlineCount++
			}

			if newlineCount == 2 { // SSE event is done
				out <- event
				newlineCount = 0
			}
		}

		close(out)
		t.Logf("User %d stops to stream from %q due to: %v", user.ID, url, context.Cause(ctx))
		// sc.Err() is set to the cancellation cause or [context.Canceled] if the req ctx was
		// cancelled. We are only interested in any issues with reading the SSE event.
		if sc.Err() != context.Cause(ctx) && sc.Err() != ctx.Err() {
			require.NoErrorf(t, sc.Err(), "error scanning event stream from %q for user %d", url, user.ID)
		}
	}()

	return out
}

// sseEvent is the struct we use to assert on received SSE events.
type sseEvent struct {
	ID    int64
	Event string
	Data  string
}

// eventEmitter emits an event to RabbitMQ which can then be streamed via SSE from the event handler.
type eventEmitter struct {
	producer       *ha.ReliableProducer
	eventCount     int64
	eventPublished chan error
}

func NewEventEmitter(t *testing.T, env *stream.Environment, streamName string) eventEmitter {
	producerName := "test-event-handler"
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
	return eventEmitter{producer: producer, eventPublished: eventPublished}
}

func (es *eventEmitter) Close() error {
	return es.producer.Close()
}

// emit emits an instance manager event and returns the SSE event we expect an eligible user to
// receive via the event handler. This is a blocking operation. An event has an event counter that
// is 1-indexed for human readability. This event counter is also used for deduplication in RabbitMQ
// by setting it as the publishing id. Since we assert on the SSE event clients should receive we
// also need to assert on the SSE id field. The event handler sets the id field to the RabbitMQ
// offset (0-indexed) so SSE clients can resume on re-connect.
func (es *eventEmitter) emit(t *testing.T, kind, group string, owner *model.User) sseEvent {
	streamOffset := es.eventCount
	es.eventCount++

	data := strconv.FormatInt(es.eventCount, 10)
	message := amqp.NewMessage([]byte(data))
	// set a publishing id for deduplication
	message.SetPublishingId(es.eventCount)
	// set properties used for filtering
	message.ApplicationProperties = map[string]interface{}{"group": group}
	if owner != nil {
		message.ApplicationProperties["owner"] = strconv.Itoa(int(owner.ID))
	}
	// set property that dictates the SSE event field
	message.ApplicationProperties["kind"] = kind
	err := es.producer.Send(message)
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
