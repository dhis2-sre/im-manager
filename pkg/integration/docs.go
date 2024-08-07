package integration

// swagger:parameters postIntegration
type _ struct {
	// Integration request body
	// in: body
	// required: true
	Body Request
}

// Response depends on the input and can be either a list or a map
type Response struct{}

// swagger:response Response
type _ struct {
	// in: body
	_ Response
}

// swagger:parameters imageExists
type _ struct {
	// in: path
	// required: true
	Repository string `json:"repository"`

	// in: path
	// required: true
	Tag uint `json:"tag"`
}
