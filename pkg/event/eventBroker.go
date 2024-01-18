package event

import (
	"sync"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"golang.org/x/exp/maps"
)

func NewEventBroker() *Broker {
	return &Broker{
		subscribers: make(map[uint]Subscriber),
		lock:        sync.Mutex{},
	}
}

type Event struct {
	Type    string
	Message string
}

type Subscriber struct {
	user    model.User
	channel chan Event
}

type Broker struct {
	subscribers map[uint]Subscriber
	lock        sync.Mutex
}

func (e *Broker) Subscribe(user model.User) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.subscribers[user.ID] = Subscriber{
		user:    user,
		channel: make(chan Event, 1),
	}
}

func (e *Broker) Unsubscribe(id uint) {
	e.lock.Lock()
	defer e.lock.Unlock()
	// TODO: Possible panic? Yes..
	close(e.subscribers[id].channel)
	delete(e.subscribers, id)
}

func (e *Broker) Subscribers() []model.User {
	keys := maps.Keys(e.subscribers)
	subscribers := make([]model.User, len(keys))
	for i, key := range keys {
		subscribers[i] = e.subscribers[key].user
	}
	return subscribers
}

func (e *Broker) Send(id uint, event Event) bool {
	if subscriber, ok := e.subscribers[id]; ok {
		subscriber.channel <- event
		return true
	}
	return false
}

func (e *Broker) Receive(id uint) (Event, bool) {
	if subscriber, ok := e.subscribers[id]; ok {
		return <-subscriber.channel, true
	}
	return Event{}, false
}
