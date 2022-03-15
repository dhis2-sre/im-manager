package docs

import (
	"github.com/dhis2-sre/im-manager/pkg/instance"
)

// swagger:parameters deleteInstance findInstanceById instanceLogs deployInstance saveInstance findInstanceByIdWithParameters
type IdParameter struct {
	// in: path
	// required: true
	ID uint `json:"id"`
}

// swagger:parameters instanceNameToId
type GroupIdParameter struct {
	// in: path
	// required: true
	GroupID uint `json:"groupId"`
}

// swagger:parameters instanceNameToId
type NameParameter struct {
	// in: path
	// required: true
	Name uint `json:"name"`
}

// swagger:response
type Error struct {
	// The error message
	//in: body
	Message string
}

// swagger:response
type GroupWithInstances struct {
	//in: body
	GroupWithInstances instance.GroupWithInstances
}

// swagger:parameters createInstance
type _ struct {
	// Create instance request body parameter
	// in: body
	// required: true
	Body instance.CreateInstanceRequest
}

// swagger:response
type RunJobResponse struct {
	//in: body
	RunJobResponse instance.RunJobResponse
}
