package inspector

import (
	"context"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

type Handler interface {
	Handle(ctx context.Context, deployment model.Deployment) error
}
