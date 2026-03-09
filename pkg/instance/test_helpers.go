package instance

// Test helpers used across packages. Kept in the main package so they can be
// reused by external tests that depend on instance models and services.

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func CreateDeploymentRecord(t *testing.T, db *gorm.DB, userID uint, name, groupName string) *model.Deployment {
	t.Helper()

	deployment := &model.Deployment{
		UserID:    userID,
		Name:      name,
		GroupName: groupName,
	}

	err := db.Create(deployment).Error
	require.NoError(t, err, "failed to create test deployment record")

	return deployment
}

func CreateInstanceRecord(t *testing.T, db *gorm.DB, deploymentID uint, group *model.Group, stackName, instanceName, groupName string, params model.DeploymentInstanceParameters) *model.DeploymentInstance {
	t.Helper()

	instance := &model.DeploymentInstance{
		Name:         instanceName,
		GroupName:    groupName,
		StackName:    stackName,
		DeploymentID: deploymentID,
		Group:        group,
		Parameters:   params,
	}

	err := db.Create(instance).Error
	require.NoError(t, err, "failed to create test instance record")

	return instance
}
