package instance

import "github.com/dhis2-sre/im-manager/pkg/model"

// swagger:parameters restartInstance
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`

	// selector
	// in: query
	// required: false
	// type: string
	// description: restart a specific deployment labeled with im-type=<selector>
	Selector string `json:"selector"`
}

// swagger:parameters instanceLogs
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`

	// selector
	// in: query
	// required: false
	// type: string
	// description: stream logs of a specific pod labeled with im-type=<selector>
	Selector string `json:"selector"`
}

// swagger:parameters deleteInstance findById findByIdDecrypted saveInstance pauseInstance resumeInstance resetInstance findDeploymentById deployDeployment deleteDeployment status instanceWithDetails filestoreBackup
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
}

// swagger:parameters deleteDeploymentInstance
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
	// in: path
	// required: true
	InstanceID uint `json:"instanceId"`
}

// swagger:response InstanceLogsResponse
type InstanceLogsBody struct {
	// in: body
	Body string
}

// swagger:response Status
type StatusBody struct {
	// in: body
	Body InstanceStatus
}

// swagger:parameters instanceNameToId
type _ struct {
	// in: path
	// required: true
	GroupName string `json:"groupName"`

	// in: path
	// required: true
	InstanceName string `json:"instanceName"`
}

// swagger:response Error
type ErrorBody struct {
	// The error message
	// in: body
	Message string
}

// swagger:response GroupsWithDeployments
type GroupsWithDeploymentsBody struct {
	// in: body
	Body []GroupWithDeployments
}

// swagger:response GroupsWithPublicInstances
type GroupsWithPublicInstancesBody struct {
	// in: body
	Body []GroupWithPublicInstances
}

// swagger:parameters saveDeployment
type _ struct {
	// Save deployment request body parameter
	// in: body
	// required: true
	Payload SaveDeploymentRequest
}

// swagger:parameters saveInstance
type _ struct {
	// Save instance request body parameter
	// in: body
	// required: true
	Payload SaveInstanceRequest
}

// swagger:response DeploymentInstance
type DeploymentInstanceBody struct {
	// in: body
	Body model.DeploymentInstance
}

// swagger:parameters updateInstance
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
	// in: path
	// required: true
	InstanceID uint `json:"instanceId"`
	// Update instance request body parameter
	// in: body
	// required: true
	Payload UpdateInstanceRequest
}

// swagger:parameters updateDeployment
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
	// Update deployment request body parameter
	// in: body
	// required: true
	Payload UpdateDeploymentRequest
}
