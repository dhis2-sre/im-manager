package inspector

import (
	"context"
	"log/slog"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/instance"
)

func NewInspector(logger *slog.Logger, service *instance.Service, handlers ...Handler) inspector {
	return inspector{logger, handlers, service}
}

type inspector struct {
	logger          *slog.Logger
	handlers        []Handler
	instanceService *instance.Service
}

func (i inspector) Inspect(ctx context.Context) {
	for {
		time.Sleep(2 * time.Minute)

		i.logger.InfoContext(ctx, "Starting inspection...")

		deployments, err := i.instanceService.FindAllDeployments(ctx)
		if err != nil {
			i.logger.ErrorContext(ctx, "failed to find deployments", "error", err)
			continue
		}
		for _, deployment := range deployments {
			for _, handler := range i.handlers {
				err := handler.Handle(ctx, deployment)
				if err != nil {
					i.logger.ErrorContext(ctx, "failed to handle instance", "error", err)
					continue
				}
			}
		}

		i.logger.InfoContext(ctx, "Inspection ended")
	}
}
