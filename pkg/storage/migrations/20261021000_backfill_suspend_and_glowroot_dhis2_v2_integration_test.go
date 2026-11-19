package migrations_test

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage/migrations"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/clause"
)

func TestBackfillSuspendAndGlowrootDHIS2V2(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	group := model.Group{Name: "group", Hostname: "group.example.org", Deployable: true}
	require.NoError(t, db.Omit(clause.Associations).Create(&group).Error)
	user := model.User{Email: "user@example.org"}
	require.NoError(t, db.Create(&user).Error)

	createInstance := func(name, stackName string, parameters model.DeploymentInstanceParameters) uint {
		deployment := model.Deployment{Name: name, GroupName: group.Name, UserID: user.ID}
		require.NoError(t, db.Create(&deployment).Error)
		instance := model.DeploymentInstance{Name: name, GroupName: group.Name, StackName: stackName, DeploymentID: deployment.ID, Parameters: parameters}
		require.NoError(t, db.Create(&instance).Error)
		return instance.ID
	}
	unset := createInstance("unset", "dhis2-v2", model.DeploymentInstanceParameters{})
	suspendable := createInstance("suspendable", "dhis2-v2", model.DeploymentInstanceParameters{"ALLOW_SUSPEND": {Value: "true"}})
	otherStack := createInstance("other-stack", "whoami-go", model.DeploymentInstanceParameters{})

	for _, migration := range migrations.All() {
		if migration.ID == "20261021000" {
			require.NoError(t, migration.Migrate(db))
		}
	}

	parameters := func(instanceID uint) map[string]string {
		var rows []model.DeploymentInstanceParameter
		require.NoError(t, db.Where("deployment_instance_id = ?", instanceID).Find(&rows).Error)
		values := map[string]string{}
		for _, row := range rows {
			values[row.ParameterName] = row.Value
		}
		return values
	}

	require.Equal(t, map[string]string{"ALLOW_SUSPEND": "false", "DEPLOY_GLOWROOT": "false"}, parameters(unset))
	require.Equal(t, map[string]string{"ALLOW_SUSPEND": "true", "DEPLOY_GLOWROOT": "false"}, parameters(suspendable))
	require.Empty(t, parameters(otherStack))
}
