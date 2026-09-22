package migrations

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// backfillDeployStatus gives instances that existed before deploy status was recorded the only
// answer that is true of them: they were deployed, and the best available timestamp is the row's
// own. Leaving them empty would make every pre-existing instance look like it had never been
// deployed, which is the state the new status is meant to distinguish from a failed deploy.
func backfillDeployStatus() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260922000",
		Migrate: func(tx *gorm.DB) error {
			// UpdateColumns rather than Updates so a backfill does not make every existing
			// instance look like it was touched today.
			return tx.Model(&model.DeploymentInstance{}).
				Where("deploy_status IS NULL OR deploy_status = ?", "").
				UpdateColumns(map[string]any{
					"deploy_status": model.DeployStatusDeployed,
					"deployed_at":   gorm.Expr("updated_at"),
				}).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Model(&model.DeploymentInstance{}).
				Where("deploy_status = ?", model.DeployStatusDeployed).
				UpdateColumns(map[string]any{"deploy_status": "", "deployed_at": nil}).Error
		},
	}
}
