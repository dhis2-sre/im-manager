package group

import "github.com/dhis2-sre/im-manager/pkg/kube"

// swagger:parameters groupCreate
type _ struct {
	// Create group request body parameter
	// in: body
	// required: true
	Body CreateGroupRequest
}

// swagger:parameters addUserToGroup removeUserFromGroup addAdminUserToGroup removeAdminUserFromGroup
type _ struct {
	// in: path
	// required: true
	Group string `json:"group"`

	// in: path
	// required: true
	UserID uint `json:"userId"`
}

// swagger:parameters findGroupByName findGroupByNameWithDetails findResources
type _ struct {
	// in: path
	// required: true
	Name string `json:"name"`
}

// swagger:parameters groupUpdate
type _ struct {
	// in: path
	// required: true
	Name string `json:"name"`

	// Update group request body parameter
	// in: body
	// required: true
	Body UpdateGroupRequest
}

// swagger:parameters findAllGroupsByUser
type _ struct {
	// deployable
	// in: query
	// required: false
	// type: string
	// description: if true, only deployable groups are returned. Otherwise, all groups are returned
	Deployable string `json:"deployable"`
}

// swagger:response ClusterResources
type ClusterResourcesBody struct {
	// in: body
	Body kube.ClusterResources
}

// swagger:parameters addClusterToGroup removeClusterFromGroup
type _ struct {
	// in: path
	// required: true
	Group string `json:"group"`
	// in: path
	// required: true
	ClusterId string `json:"clusterId"`
}
