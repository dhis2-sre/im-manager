package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// dropLegacyDeploymentNameIndex drops idx_name_and_group from deployments or deployment_instances. It is the unique
// index those tables had before #706 gave every index its own name, and AutoMigrate never removes an index whose name
// left the model, so databases created before then still refuse a deployment that shares its name with a preset.
// Index names are unique per schema, so only one table can hold it, and it is left alone when it is the databases one.
func dropLegacyDeploymentNameIndex() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20261019000",
		Migrate: func(tx *gorm.DB) error {
			return tx.Exec(`DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'idx_name_and_group' AND tablename IN ('deployments', 'deployment_instances')) THEN
		DROP INDEX idx_name_and_group;
	END IF;
END $$`).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
