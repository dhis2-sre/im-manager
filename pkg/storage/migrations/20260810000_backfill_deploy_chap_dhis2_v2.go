package migrations

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// backfillDeployChapDHIS2V2 gives dhis2-v2 instances deployed before the parameter existed a
// DEPLOY_CHAP value. The helmfile environment is built from an instance's stored parameters alone,
// with no fallback to the stack definition's default, so a requiredEnv the instance has no row for
// fails every later helmfile run, including the destroy that would otherwise clean the instance up.
func backfillDeployChapDHIS2V2() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260810000",
		Migrate: func(tx *gorm.DB) error {
			var instances []model.DeploymentInstance
			if err := tx.Where("stack_name = ?", "dhis2-v2").Find(&instances).Error; err != nil {
				return err
			}
			for _, instance := range instances {
				param := model.DeploymentInstanceParameter{
					DeploymentInstanceID: instance.ID,
					ParameterName:        "DEPLOY_CHAP",
					StackName:            "dhis2-v2",
					Value:                "false",
				}
				if err := tx.FirstOrCreate(&param, model.DeploymentInstanceParameter{
					DeploymentInstanceID: instance.ID,
					ParameterName:        "DEPLOY_CHAP",
				}).Error; err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Where("parameter_name = ? AND stack_name = ?", "DEPLOY_CHAP", "dhis2-v2").
				Delete(&model.DeploymentInstanceParameter{}).Error
		},
	}
}
