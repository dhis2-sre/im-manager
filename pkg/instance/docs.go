package instance

// swagger:parameters deployInstance
type _ struct {
	// Deploy instance request body parameter
	// in: body
	// required: true
	_ DeployInstanceRequest
}

// swagger:parameters updateInstance
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`

	// Update instance request body parameter
	// in: body
	// required: true
	_ UpdateInstanceRequest
}

// swagger:parameters restartInstance
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
}

// swagger:parameters linkDeployInstance
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`

	// in: path
	// required: true
	DestinationId uint `json:"destinationId"`

	// Deploy instance request body parameter
	// in: body
	// required: true
	_ DeployInstanceRequest
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
	Selector string `json:"selector"`
}

// swagger:parameters deleteInstance findInstanceById findInstanceByIdWithParameters saveInstance
type _ struct {
	// in: path
	// required: true
	ID uint `json:"id"`
}

// swagger:response InstanceLogsResponse
type _ struct {
	//in: body
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
	//in: body
	Message string
}

// swagger:response
type _ struct {
	//in: body
	_ GroupWithInstances
}
