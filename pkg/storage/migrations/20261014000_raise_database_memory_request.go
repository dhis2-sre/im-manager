package migrations

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// raiseDatabaseMemoryRequest moves the instances still on the old database memory request default to
// the new one, so their next deploy reserves what PostgreSQL uses. The scheduler packed nodes by the
// old 256Mi while each database used about 1Gi, until nodes ran out of memory. Values someone chose
// themselves are left alone.
func raiseDatabaseMemoryRequest() *gormigrate.Migration {
	const (
		stackName     = "dhis2-v2"
		parameterName = "DB_RESOURCES_REQUESTS_MEMORY"
		oldDefault    = "256Mi"
		newDefault    = "1Gi"
	)

	update := func(tx *gorm.DB, from, to string) error {
		return tx.Model(&model.DeploymentInstanceParameter{}).
			Where("stack_name = ? AND parameter_name = ? AND value = ?", stackName, parameterName, from).
			Update("value", to).Error
	}

	return &gormigrate.Migration{
		ID: "20261014000",
		Migrate: func(tx *gorm.DB) error {
			return update(tx, oldDefault, newDefault)
		},
		Rollback: func(tx *gorm.DB) error {
			return update(tx, newDefault, oldDefault)
		},
	}
}
