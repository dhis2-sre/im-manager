package stack

import "github.com/dhis2-sre/im-manager/pkg/model"

// swagger:parameters stack
type _ struct {
	// in: path
	// required: true
	Name string `json:"name"`
}

// swagger:response StackResponse
type _ struct {
	//in: body
	_ model.Stack
}

// swagger:response StacksResponse
type _ struct {
	//in: body
	_ *[]model.Stack
}
