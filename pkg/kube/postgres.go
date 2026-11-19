package kube

import (
	"context"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

// PostgresAccess is implemented by components that can locate their primary PostgreSQL pod for
// exec-based operations such as pg_dump. The pod and container differ per technology: Bitnami runs
// a StatefulSet with a "postgresql" container while CloudNativePG labels its current primary with
// cnpg.io/instanceRole and uses a "postgres" container.
type PostgresAccess interface {
	PostgresPod(ctx context.Context, client *Client, instance *model.DeploymentInstance) (pod string, container string, err error)
}

// FindPostgresAccess returns the postgres component among the given components, or a not-found
// error for stacks without one.
func FindPostgresAccess(components []Component) (PostgresAccess, error) {
	for _, component := range components {
		if access, ok := component.(PostgresAccess); ok {
			return access, nil
		}
	}
	return nil, errdef.NewNotFound("no postgres component found")
}
