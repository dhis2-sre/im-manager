package stack

// swagger:parameters stack
type _ struct {
	// in: path
	// required: true
	Name string `json:"name"`
}

// swagger:response Stack
type StackBody struct {
	//in: body
	Body Stack
}

// swagger:response Stacks
type StacksBody struct {
	//in: body
	Body []Stack
}
