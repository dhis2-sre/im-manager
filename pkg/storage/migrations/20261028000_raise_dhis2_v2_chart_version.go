package migrations

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// raiseDHIS2V2ChartVersion moves the instances still on a former default chart version to 1.4.0, so
// their next deploy sets dhis2.environment on the database and drops the pod disruption budget that
// 1.3.0 made optional. Versions someone chose themselves are left alone, and the rollback returns
// every moved instance to 1.3.0 since the version each came from is not kept.
func raiseDHIS2V2ChartVersion() *gormigrate.Migration {
	const (
		stackName       = "dhis2-v2"
		parameterName   = "CHART_VERSION"
		previousDefault = "1.3.0"
		newDefault      = "1.4.0"
	)
	formerDefaults := []string{"1.1.0", previousDefault}

	update := func(tx *gorm.DB, from []string, to string) error {
		return tx.Model(&model.DeploymentInstanceParameter{}).
			Where("stack_name = ? AND parameter_name = ? AND value IN ?", stackName, parameterName, from).
			Update("value", to).Error
	}

	return &gormigrate.Migration{
		ID: "20261028000",
		Migrate: func(tx *gorm.DB) error {
			return update(tx, formerDefaults, newDefault)
		},
		Rollback: func(tx *gorm.DB) error {
			return update(tx, []string{newDefault}, previousDefault)
		},
	}
}
