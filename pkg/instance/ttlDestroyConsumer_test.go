package instance_test

import (
	"context"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/token"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func (s *ttlSuite) TestConsumeDeletesInstance() {
	require := s.Require()

	uc := &userClient{}
	uc.On("SignIn", "username", "password").Return(&token.Tokens{
		AccessToken: "dummy-token",
	}, nil)

	consumer, err := rabbitmq.NewConsumer(
		s.rabbitURI,
		rabbitmq.WithConsumerPrefix("im-manager"),
	)
	require.NoError(err)
	defer func() { require.NoError(consumer.Close()) }()

	is := &instanceService{}
	is.On("Delete", "token", uint(1)).Return(nil)

	td := instance.NewTTLDestroyConsumer(consumer, is)
	require.NoError(td.Consume())

	require.NoError(s.amqpClient.ch.PublishWithContext(context.TODO(), "", "ttl-destroy", false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Body:         []byte(`{"ID": 1}`),
	}))

	require.Eventually(func() bool {
		return is.AssertExpectations(s.T()) && uc.AssertExpectations(s.T())
	}, s.timeout, time.Second)
}

type userClient struct {
	mock.Mock
}

func (u *userClient) SignIn(username, password string) (*token.Tokens, error) {
	args := u.Called(username, password)
	tokens := args.Get(0).(*token.Tokens)
	err := args.Error(1)
	return tokens, err
}

type instanceService struct {
	mock.Mock
}

func (is *instanceService) Delete(token string, id uint) error {
	args := is.Called(token, id)
	return args.Error(0)
}

type ttlSuite struct {
	suite.Suite
	rabbitURI  string
	amqpClient *amqpTestClient
	timeout    time.Duration
}

func TestSuiteTTLDestroyConsumer(t *testing.T) {
	suite.Run(t, new(ttlSuite))
}

func (s *ttlSuite) SetupSuite() {
	s.timeout = time.Second * 30
}

func (s *ttlSuite) SetupTest() {
	require := s.Require()

	amqpURI := "amqp://guest:guest@rabbitmq:5672"
	ac, err := setupAMQPTestClient(amqpURI)
	require.NoError(err, "failed setting up AMQP client")
	s.rabbitURI = amqpURI
	s.amqpClient = ac
}

func (s *ttlSuite) TearDownTest() {
	s.Require().NoError(s.amqpClient.conn.Close())
}

type amqpTestClient struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func setupAMQPTestClient(uri string) (*amqpTestClient, error) {
	c, err := amqp.Dial(uri)
	if err != nil {
		return nil, err
	}
	ch, err := c.Channel()
	if err != nil {
		return nil, err
	}

	return &amqpTestClient{conn: c, ch: ch}, nil
}
