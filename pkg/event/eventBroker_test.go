package event

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBroker_Subscribe(t *testing.T) {
	eventBroker := NewEventBroker()

	eventBroker.Subscribe(model.User{ID: 123})

	assert.Len(t, eventBroker.subscribers, 1)
	assert.Equal(t, eventBroker.subscribers[123].user.ID, uint(123))
}

func TestBroker_Subscribe_MultipleSubscribers(t *testing.T) {
	eventBroker := NewEventBroker()

	eventBroker.Subscribe(model.User{ID: 123})
	eventBroker.Subscribe(model.User{ID: 321})

	assert.Len(t, eventBroker.subscribers, 2)
	assert.Equal(t, eventBroker.subscribers[123].user.ID, uint(123))
	assert.Equal(t, eventBroker.subscribers[321].user.ID, uint(321))
}

func TestBroker_Unsubscribe(t *testing.T) {
	eventBroker := NewEventBroker()
	eventBroker.Subscribe(model.User{ID: 123})

	assert.Len(t, eventBroker.subscribers, 1)

	eventBroker.Unsubscribe(123)

	assert.Len(t, eventBroker.subscribers, 0)
}

func TestBroker_Receive(t *testing.T) {
	eventBroker := NewEventBroker()
	eventBroker.Subscribe(model.User{ID: 123})
	eventBroker.Send(123, Event{
		Type:    "type",
		Message: "message",
	})

	event, ok := eventBroker.Receive(123)

	assert.True(t, ok)
	assert.Equal(t, "type", event.Type)
	assert.Equal(t, "message", event.Message)
}

func TestBroker_Receive_MultipleSubscribers(t *testing.T) {
	eventBroker := NewEventBroker()
	eventBroker.Subscribe(model.User{ID: 123})
	eventBroker.Subscribe(model.User{ID: 321})
	eventBroker.Send(123, Event{
		Type:    "type",
		Message: "message",
	})

	eventBroker.Send(321, Event{
		Type:    "type2",
		Message: "message2",
	})

	event, ok := eventBroker.Receive(123)
	event2, ok2 := eventBroker.Receive(321)

	assert.True(t, ok)
	assert.Equal(t, "type", event.Type)
	assert.Equal(t, "message", event.Message)

	assert.True(t, ok2)
	assert.Equal(t, "type2", event2.Type)
	assert.Equal(t, "message2", event2.Message)
}

func TestBroker_Send(t *testing.T) {
	eventBroker := NewEventBroker()
	eventBroker.Subscribe(model.User{ID: 123})

	ok := eventBroker.Send(123, Event{
		Type:    "type",
		Message: "message",
	})

	assert.True(t, ok)
}

func TestBroker_Send_NoSubscriber(t *testing.T) {
	eventBroker := NewEventBroker()

	ok := eventBroker.Send(123, Event{
		Type:    "type",
		Message: "message",
	})

	assert.False(t, ok)
}
