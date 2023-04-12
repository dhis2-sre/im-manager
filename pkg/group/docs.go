package group

// swagger:parameters groupCreate
type _ struct {
	// Refresh token request body parameter
	// in: body
	// required: true
	Body CreateGroupRequest
}

// swagger:parameters addUserToGroup
type _ struct {
	// in: path
	// required: true
	Group string `json:"group"`

	// in: path
	// required: true
	UserID uint `json:"userId"`
}

// swagger:parameters addClusterConfigurationToGroup
type _ struct {
	// in: path
	// required: true
	Group string `json:"group"`

	// SOPS encrypted Kubernetes configuration file
	// in: formData
	// required: true
	// swagger:file
	Body CreateClusterConfigurationRequest
}

// swagger:parameters findGroupByName
type _ struct {
	// in: path
	// required: true
	Name string `json:"name"`
}
