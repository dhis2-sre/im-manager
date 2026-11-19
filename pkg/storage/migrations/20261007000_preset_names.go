package migrations

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// presetNames drops the unique indexes that made a preset and a deployment compete for the same name.
// AutoMigrate has already created their replacements: deployments are unique by name, group and
// whether they are a preset, and an instance is unique by its deployment and stack.
func presetNames() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20261007000",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.Exec("DROP INDEX IF EXISTS deployment_name_group_idx").Error; err != nil {
				return err
			}
			return tx.Exec("DROP INDEX IF EXISTS deployment_instance_name_group_stack_idx").Error
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS deployment_name_group_idx ON deployments (name, group_name)").Error; err != nil {
				return err
			}
			return tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS deployment_instance_name_group_stack_idx ON deployment_instances (name, group_name, stack_name)").Error
		},
	}
}
