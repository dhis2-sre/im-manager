package instance_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/rabbitmq-client/pkg/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConsumeDeletesInstance(t *testing.T) {
	t.Parallel()

	amqpClient := inttest.SetupRabbitMQAMQP(t)

	consumer, err := rabbitmq.NewConsumer(
		amqpClient.URI(t),
		rabbitmq.WithConnectionName(t.Name()),
		rabbitmq.WithConsumerTagPrefix(t.Name()),
	)
	require.NoError(t, err, "failed to create new RabbitMQ consumer")
	defer func() { require.NoError(t, consumer.Close()) }()

	is := &instanceService{}
	is.On("Delete", uint(1)).Return(nil)
	is.On("FindDeploymentInstanceById", uint(1)).Return(&model.DeploymentInstance{}, nil)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	td := instance.NewTTLDestroyConsumer(logger, consumer, is)
	require.NoError(t, td.Consume())

	require.NoError(t, amqpClient.Channel.PublishWithContext(context.TODO(), "", "ttl-destroy", false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Body:         []byte(`{"ID": 1}`),
	}), "failed to send message with key \"ttl-destroy\"")

	require.Eventually(t, func() bool {
		return is.AssertExpectations(t)
	}, time.Second*10, time.Second)
}

type instanceService struct {
	mock.Mock
}

func (is *instanceService) FindDeploymentInstanceById(id uint) (*model.DeploymentInstance, error) {
	args := is.Called(id)
	return &model.DeploymentInstance{ID: 1}, args.Error(1)
}

func (is *instanceService) Delete(id uint) error {
	args := is.Called(id)
	return args.Error(0)
}
