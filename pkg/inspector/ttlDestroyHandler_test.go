package inspector

import (
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/dhis2-sre/rabbitmq-client/pkg/rabbitmq"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_TTLDestroyHandler_NotExpired(t *testing.T) {
	producer := &mockQueueProducer{}
	handler := NewTTLDestroyHandler(slog.Default(), producer)
	now := strconv.Itoa(int(time.Now().Unix()))
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"im-instance-id":        "1",
				"im-creation-timestamp": now,
				"im-ttl":                "300",
			},
		},
	}

	err := handler.Handle(pod)

	require.NoError(t, err)
	producer.AssertExpectations(t)
}

func Test_TTLDestroyHandler_Expired(t *testing.T) {
	producer := &mockQueueProducer{}
	var channel rabbitmq.Channel = "ttl-destroy"
	producer.
		On("Produce", channel, mock.AnythingOfType("string"), struct{ ID uint }{ID: 1}).
		Return(nil)
	handler := NewTTLDestroyHandler(slog.Default(), producer)
	tenMinutesAgo := strconv.Itoa(int(time.Now().Add(time.Minute * -10).Unix()))
	pod := v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"im-instance-id":        "1",
				"im-creation-timestamp": tenMinutesAgo,
				"im-ttl":                "300",
			},
		},
	}

	err := handler.Handle(pod)

	require.NoError(t, err)
	producer.AssertExpectations(t)
}

type mockQueueProducer struct{ mock.Mock }

func (m *mockQueueProducer) Produce(channel rabbitmq.Channel, correlationId string, payload any) error {
	called := m.Called(channel, correlationId, payload)
	return called.Error(0)
}
