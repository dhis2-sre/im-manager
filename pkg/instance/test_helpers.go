package instance

// Test helpers used across packages. Kept in the main package so they can be
// reused by external tests that depend on instance models and services.

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

func CreateTestDeploymentRecord(db *gorm.DB, userID uint, name, groupName string) *model.Deployment {
	deployment := &model.Deployment{
		UserID:    userID,
		Name:      name,
		GroupName: groupName,
	}

	db.Create(deployment)

	return deployment
}

func CreateTestInstanceRecord(db *gorm.DB, deploymentID uint, group *model.Group, stackName, instanceName, groupName string, params model.DeploymentInstanceParameters) *model.DeploymentInstance {
	instance := &model.DeploymentInstance{
		Name:         instanceName,
		GroupName:    groupName,
		StackName:    stackName,
		DeploymentID: deploymentID,
		Group:        group,
		Parameters:   params,
	}

	db.Create(instance)

	return instance
}
