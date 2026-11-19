package deployment

import "github.com/dhis2-sre/im-manager/pkg/model"

const kindFilestoreBackup = "filestore-backup"

// filestoreEvent is the JSON payload published for filestore-backup events. It matches the wire
// format these events had when they were published from the database package.
type filestoreEvent struct {
	Status       string `json:"status"`
	DatabaseID   uint   `json:"databaseId"`
	DatabaseName string `json:"databaseName"`
	Error        string `json:"error,omitempty"`
}

func newFilestoreEvent(db *model.Database, status, errMsg string) filestoreEvent {
	return filestoreEvent{
		Status:       status,
		DatabaseID:   db.ID,
		DatabaseName: db.Name,
		Error:        errMsg,
	}
}

const kindDeployment = "deployment"

// deploymentEvent is the JSON payload published while a deployment deploys. Per-instance events are
// progress and are streamed without being persisted; the deployment-level terminal event is the one
// worth keeping, so it is the only one that reaches the notification bell.
type deploymentEvent struct {
	Status         string `json:"status"`
	DeploymentID   uint   `json:"deploymentId"`
	DeploymentName string `json:"deploymentName"`
	InstanceID     uint   `json:"instanceId,omitempty"`
	StackName      string `json:"stackName,omitempty"`
	Error          string `json:"error,omitempty"`
}

func newInstanceEvent(deployment *model.Deployment, instance *model.DeploymentInstance, status, errMsg string) deploymentEvent {
	return deploymentEvent{
		Status:         status,
		DeploymentID:   deployment.ID,
		DeploymentName: deployment.Name,
		InstanceID:     instance.ID,
		StackName:      instance.StackName,
		Error:          errMsg,
	}
}

func newDeploymentEvent(deployment *model.Deployment, status, errMsg string) deploymentEvent {
	return deploymentEvent{
		Status:         status,
		DeploymentID:   deployment.ID,
		DeploymentName: deployment.Name,
		Error:          errMsg,
	}
}
