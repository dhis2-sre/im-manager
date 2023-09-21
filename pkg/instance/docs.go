package instance

import "github.com/dhis2-sre/im-manager/pkg/model"

// swagger:parameters deployInstance
type _ struct {
	// Deploy instance request body parameter
	// in: body
	// required: true
	Payload DeployInstanceRequest

	// preset
	// in: query
	// required: false
	// type: string
	// description: define deployment as a preset
	Preset string `json:"preset"`
}

// swagger:parameters updateInstance
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`

	// Update instance request body parameter
	// in: body
	// required: true
	Payload UpdateInstanceRequest
}

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

// swagger:parameters deleteInstance findById findByIdDecrypted saveInstance pauseInstance resumeInstance resetInstance findDeploymentById deployDeployment deleteDeployment
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
}

// swagger:response InstanceLogsResponse
type _ struct {
	// in: body
	_ string
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

// swagger:response
type Error struct {
	// The error message
	// in: body
	Message string
}

// swagger:response GroupsWithInstances
type _ struct {
	// in: body
	_ []GroupsWithInstances
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

// swagger:response saveInstance
type _ struct {
	// in: body
	_ model.DeploymentInstance
}
