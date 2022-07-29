package instance_test

import (
	"io"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"github.com/dhis2-sre/rabbitmq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func (s *ttlSuite) TestConsumeDeletesInstance() {
	require := s.Require()

	uc := &userClient{}
	uc.On("SignIn", "username", "password").Return(&models.Tokens{
		AccessToken: "token",
	}, nil)

	consumer, err := rabbitmq.NewConsumer(
		s.rabbitURI,
		rabbitmq.WithConsumerPrefix("im-manager"),
	)
	require.NoError(err)
	defer func() { require.NoError(consumer.Close()) }()

	is := &instanceService{}
	is.On("Delete", "token", uint(1)).Return(nil)

	td := instance.NewTTLDestroyConsumer("username", "password", uc, consumer, is)
	require.NoError(td.Consume())

	require.NoError(s.amqpClient.ch.Publish("", "ttl-destroy", false, false, amqp.Publishing{
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

func (u *userClient) SignIn(username, password string) (*models.Tokens, error) {
	args := u.Called(username, password)
	tokens := args.Get(0).(*models.Tokens)
	err := args.Error(1)
	return tokens, err
}

type instanceService struct {
	mock.Mock
}

func (is *instanceService) ConsumeParameters(sourceInstance, destinationInstance *model.Instance) error {
	return nil
}

func (is *instanceService) Save(instance *model.Instance) (*model.Instance, error) {
	return nil, nil
}

func (is *instanceService) Link(source, destination *model.Instance) error {
	return nil
}

func (is *instanceService) Restart(token string, id uint) error {
	return nil
}

func (is *instanceService) Delete(token string, id uint) error {
	args := is.Called(token, id)
	return args.Error(0)
}

func (is *instanceService) Deploy(token string, instance *model.Instance) error {
	return nil
}

func (is *instanceService) FindById(id uint) (*model.Instance, error) { return nil, nil }

func (is *instanceService) Logs(instance *model.Instance, group *models.Group, selector string) (io.ReadCloser, error) {
	return nil, nil
}
func (is *instanceService) FindByIdWithParameters(id uint) (*model.Instance, error) { return nil, nil }

func (is *instanceService) FindByIdWithDecryptedParameters(id uint) (*model.Instance, error) {
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

func setupAMQPTestClient(URI string) (*amqpTestClient, error) {
	c, err := amqp.Dial(URI)
	if err != nil {
		return nil, err
	}
	ch, err := c.Channel()
	if err != nil {
		return nil, err
	}

	return &amqpTestClient{conn: c, ch: ch}, nil
}
