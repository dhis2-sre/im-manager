package inspector

import (
	"context"
	"log/slog"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

func NewTTLDestroyHandler(logger *slog.Logger, instanceService instanceService) ttlDestroyHandler {
	return ttlDestroyHandler{logger, instanceService}
}

type instanceService interface {
	DeleteDeployment(ctx context.Context, deployment *model.Deployment) error
	FindDecryptedDeploymentById(ctx context.Context, id uint) (*model.Deployment, error)
}

type ttlDestroyHandler struct {
	logger          *slog.Logger
	instanceService instanceService
}

func (t ttlDestroyHandler) Handle(ctx context.Context, deployment model.Deployment) error {
	t.logger.Info("TTL handler invoked", "deploymentId", deployment.ID)

	if t.ttlBeforeNow(deployment.CreatedAt, deployment.TTL) {
		decryptedDeployment, err := t.instanceService.FindDecryptedDeploymentById(ctx, deployment.ID)
		if err != nil {
			t.logger.ErrorContext(ctx, "TTL handler failed to decrypt deployment", "deploymentId", deployment.ID, "error", err)
			return err
		}

		err = t.instanceService.DeleteDeployment(ctx, decryptedDeployment)
		if err != nil {
			t.logger.ErrorContext(ctx, "TTL destroy failed", "deploymentId", deployment.ID, "error", err)
			return err
		}
		t.logger.Info("TTL destroy completed", "deploymentId", deployment.ID)
	}

	return nil
}

// ttlBeforeNow returns true if creationTimestamp + ttl is before now.
// ttl is the deployments time-to-live in seconds.
func (t ttlDestroyHandler) ttlBeforeNow(creationTimestamp time.Time, ttl uint) bool {
	expiration := creationTimestamp.Add(time.Duration(ttl) * time.Second)
	return expiration.Before(time.Now())
}
