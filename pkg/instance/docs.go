package instance

// swagger:parameters deployInstance
type _ struct {
	// Deploy instance request body parameter
	// in: body
	// required: true
	Payload DeployInstanceRequest
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

// swagger:parameters pauseInstance
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
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

// swagger:parameters deleteInstance findById findByIdDecrypted saveInstance
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

// swagger:response
type _ struct {
	// in: body
	_ GroupWithInstances
}
