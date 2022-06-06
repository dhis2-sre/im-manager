package instance_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"github.com/dhis2-sre/rabbitmq"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
)

func (s *ttlSuite) TestConsumeDeletesInstance() {
	require := s.Require()

	consumer, err := rabbitmq.NewConsumer(
		s.rabbitURI,
		rabbitmq.WithConsumerPrefix("im-manager"),
	)
	require.NoError(err)
	defer func() { require.NoError(consumer.Close()) }()

	is := &instanceService{}
	is.On("Delete", uint(1)).Return(nil)

	td := instance.ProvideTtlDestroyConsumer(consumer, is)
	require.NoError(td.Consume())

	require.NoError(s.amqpClient.ch.Publish("", "ttl-destroy", false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Body:         []byte(`{"ID": 1}`),
	}))

	require.Eventually(func() bool {
		return is.AssertExpectations(s.T())
	}, s.timeout, time.Second)
}

type instanceService struct {
	mock.Mock
}

func (is *instanceService) Delete(id uint) error {
	args := is.Called(id)
	return args.Error(0)
}

func (is *instanceService) Create(instance *model.Instance) (*model.Instance, error) { return nil, nil }

func (is *instanceService) Deploy(token string, instance *model.Instance, group *models.Group) error {
	return nil
}

func (is *instanceService) FindById(id uint) (*model.Instance, error) { return nil, nil }

func (is *instanceService) Logs(instance *model.Instance, group *models.Group, selector string) (io.ReadCloser, error) {
	return nil, nil
}
func (is *instanceService) FindWithParametersById(id uint) (*model.Instance, error) { return nil, nil }

func (is *instanceService) FindWithDecryptedParametersById(id uint) (*model.Instance, error) {
	return nil, nil
}

func (is *instanceService) FindByNameAndGroup(instance string, groupId string) (*model.Instance, error) {
	return nil, nil
}

func (is *instanceService) FindInstances(groups []*models.Group) ([]*model.Instance, error) {
	return nil, nil
}

type ttlSuite struct {
	suite.Suite
	ctx         context.Context
	network     testcontainers.Network
	networkName string
	rabbitC     *rabbitmqContainer
	rabbitURI   string
	amqpClient  *amqpTestClient
	timeout     time.Duration
}

func TestSuiteTTLDestroyConsumer(t *testing.T) {
	suite.Run(t, new(ttlSuite))
}

func (s *ttlSuite) SetupSuite() {
	ctx := context.TODO()

	name := "test_ttl-" + uuid.NewString()
	net, err := setupNetwork(ctx, name)
	s.Require().NoError(err, "failed setting up Docker network")
	s.network = net
	s.networkName = name
	s.timeout = time.Second * 30
}

func (s *ttlSuite) TearDownSuite() {
	ctx := context.TODO()

	err := s.network.Remove(ctx)
	s.Require().NoError(err, "failed tearing down Docker network")
}

func (s *ttlSuite) SetupTest() {
	require := s.Require()

	ctx := context.Background()
	rbc, err := NewRabbitMQ(ctx,
		WithNetwork(s.networkName, "rabbitmq"),
	)
	require.NoError(err, "failed setting up RabbitMQ")

	ac, err := setupAMQPTestClient(rbc.amqpURI)
	require.NoError(err, "failed setting up AMQP client")

	s.ctx = ctx
	s.rabbitC = rbc
	s.rabbitURI = rbc.amqpURI
	s.amqpClient = ac
}

func (s *ttlSuite) TearDownTest() {
	require := s.Require()

	require.NoError(s.amqpClient.conn.Close())
	require.NoError(s.rabbitC.Terminate(s.ctx))
}
