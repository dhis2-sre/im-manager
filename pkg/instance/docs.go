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

// swagger:parameters deleteInstance findById findByIdDecrypted saveInstance pauseInstance resumeInstance resetInstance findDeploymentById deployDeployment deleteDeployment status
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
type _ struct {
	// in: body
	_ string
}

// swagger:response Status
type _ struct {
	// in: body
	_ InstanceStatus
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
type _ struct {
	// The error message
	// in: body
	Message string
}

// swagger:response GroupsWithDeployments
type _ struct {
	// in: body
	_ []GroupsWithDeployments
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
type _ struct {
	// in: body
	_ model.DeploymentInstance
}
