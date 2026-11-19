package migrations

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// backfillSuspendAndGlowrootDHIS2V2 gives dhis2-v2 instances deployed before ALLOW_SUSPEND and
// DEPLOY_GLOWROOT existed a value for each, since a requiredEnv the instance has no row for fails
// every later helmfile run. Both are backfilled as false so instances that have been running
// without suspension keep doing so until their owner opts in.
func backfillSuspendAndGlowrootDHIS2V2() *gormigrate.Migration {
	const stackName = "dhis2-v2"
	parameterNames := []string{"ALLOW_SUSPEND", "DEPLOY_GLOWROOT"}

	return &gormigrate.Migration{
		ID: "20261021000",
		Migrate: func(tx *gorm.DB) error {
			var instances []model.DeploymentInstance
			if err := tx.Where("stack_name = ?", stackName).Find(&instances).Error; err != nil {
				return err
			}
			for _, instance := range instances {
				for _, parameterName := range parameterNames {
					parameter := model.DeploymentInstanceParameter{
						DeploymentInstanceID: instance.ID,
						ParameterName:        parameterName,
						StackName:            stackName,
						Value:                "false",
					}
					if err := tx.FirstOrCreate(&parameter, model.DeploymentInstanceParameter{
						DeploymentInstanceID: instance.ID,
						ParameterName:        parameterName,
					}).Error; err != nil {
						return err
					}
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Where("parameter_name IN ? AND stack_name = ?", parameterNames, stackName).
				Delete(&model.DeploymentInstanceParameter{}).Error
		},
	}
}
