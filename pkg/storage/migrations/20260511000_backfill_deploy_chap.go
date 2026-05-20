package migrations

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func backfillDeployChap() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "20260511000",
		Migrate: func(tx *gorm.DB) error {
			var instances []model.DeploymentInstance
			if err := tx.Where("stack_name = ?", "dhis2-core").Find(&instances).Error; err != nil {
				return err
			}
			for _, instance := range instances {
				param := model.DeploymentInstanceParameter{
					DeploymentInstanceID: instance.ID,
					ParameterName:        "DEPLOY_CHAP",
					StackName:            "dhis2-core",
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
			return tx.Where("parameter_name = ? AND stack_name = ?", "DEPLOY_CHAP", "dhis2-core").
				Delete(&model.DeploymentInstanceParameter{}).Error
		},
	}
}
