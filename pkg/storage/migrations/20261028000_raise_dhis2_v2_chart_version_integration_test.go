package migrations_test

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage/migrations"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/clause"
)

func TestRaiseDHIS2V2ChartVersion(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	group := model.Group{Name: "group", Hostname: "group.example.org", Deployable: true}
	require.NoError(t, db.Omit(clause.Associations).Create(&group).Error)
	user := model.User{Email: "user@example.org"}
	require.NoError(t, db.Create(&user).Error)

	createInstance := func(name, stackName, chartVersion string) uint {
		deployment := model.Deployment{Name: name, GroupName: group.Name, UserID: user.ID}
		require.NoError(t, db.Create(&deployment).Error)
		parameters := model.DeploymentInstanceParameters{"CHART_VERSION": {Value: chartVersion}}
		instance := model.DeploymentInstance{Name: name, GroupName: group.Name, StackName: stackName, DeploymentID: deployment.ID, Parameters: parameters}
		require.NoError(t, db.Create(&instance).Error)
		return instance.ID
	}
	onOneOne := createInstance("one-one", "dhis2-v2", "1.1.0")
	onOneThree := createInstance("one-three", "dhis2-v2", "1.3.0")
	pinned := createInstance("pinned", "dhis2-v2", "1.0.1")
	otherStack := createInstance("other-stack", "whoami-go", "1.3.0")

	for _, migration := range migrations.All() {
		if migration.ID == "20261028000" {
			require.NoError(t, migration.Migrate(db))
		}
	}

	chartVersion := func(instanceID uint) string {
		var parameter model.DeploymentInstanceParameter
		require.NoError(t, db.Where("deployment_instance_id = ? AND parameter_name = ?", instanceID, "CHART_VERSION").First(&parameter).Error)
		return parameter.Value
	}

	require.Equal(t, "1.4.0", chartVersion(onOneOne))
	require.Equal(t, "1.4.0", chartVersion(onOneThree))
	require.Equal(t, "1.0.1", chartVersion(pinned))
	require.Equal(t, "1.3.0", chartVersion(otherStack))
}
