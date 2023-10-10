package stack

// swagger:parameters stack
type _ struct {
	// in: path
	// required: true
	Name string `json:"name"`
}

// swagger:response Stack
type _ struct {
	//in: body
	_ Stack
}

// swagger:response Stacks
type _ struct {
	//in: body
	_ []Stack
}
