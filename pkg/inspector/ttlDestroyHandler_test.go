package inspector

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_TTLDestroyHandler_NotExpired(t *testing.T) {
	instanceService := &mockInstanceService{}
	handler := NewTTLDestroyHandler(slog.Default(), instanceService)

	deployment := model.Deployment{
		CreatedAt: time.Now(),
		TTL:       300,
	}

	err := handler.Handle(context.TODO(), deployment)

	require.NoError(t, err)
	instanceService.AssertExpectations(t)
}

func Test_TTLDestroyHandler_Expired(t *testing.T) {
	ctx := context.TODO()
	deployment := model.Deployment{
		ID:        123,
		CreatedAt: time.Now().Add(time.Minute * -10),
		TTL:       300,
	}
	decryptedDeployment := &model.Deployment{
		ID:        123,
		CreatedAt: deployment.CreatedAt,
		TTL:       300,
	}
	instanceService := &mockInstanceService{}
	instanceService.On("FindDecryptedDeploymentById", ctx, deployment.ID).Return(decryptedDeployment, nil)
	instanceService.On("DeleteDeployment", ctx, decryptedDeployment).Return(nil)

	handler := NewTTLDestroyHandler(slog.Default(), instanceService)

	err := handler.Handle(ctx, deployment)

	require.NoError(t, err)
	instanceService.AssertExpectations(t)
}

type mockInstanceService struct{ mock.Mock }

func (m *mockInstanceService) DeleteDeployment(ctx context.Context, deployment *model.Deployment) error {
	called := m.Called(ctx, deployment)
	return called.Error(0)
}

func (m *mockInstanceService) FindDecryptedDeploymentById(ctx context.Context, id uint) (*model.Deployment, error) {
	called := m.Called(ctx, id)
	return called.Get(0).(*model.Deployment), called.Error(1)
}
