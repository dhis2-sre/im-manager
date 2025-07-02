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
		CreatedAt: time.Now().Add(time.Minute * -10),
		TTL:       300,
	}
	instanceService := &mockInstanceService{}
	instanceService.On("DeleteDeployment", ctx, &deployment).Return(nil)

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
