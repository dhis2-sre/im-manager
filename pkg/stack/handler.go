package stack

import (
	"fmt"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/gin-gonic/gin"
)

func NewHandler(service Service) Handler {
	return Handler{
		service,
	}
}

type Handler struct {
	service Service
}

// Find stack
func (h Handler) Find(c *gin.Context) {
	// swagger:route GET /stacks/{name} stack
	//
	// Find stack
	//
	// Find stack by name
	//
	// Security:
	//  oauth2:
	//
	// Responses:
	//   200: Stack
	//   401: Error
	//   403: Error
	//   404: Error
	//   415: Error
	name := c.Param("name")
	if name == "" {
		badRequest := errdef.NewBadRequest("stack name missing")
		_ = c.Error(badRequest)
		return
	}

	stack, err := h.service.Find(name)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, toResponseStack(*stack))
}

func toResponseStack(stack Stack) StackResponse {
	s := StackResponse{Name: stack.Name}

	s.Requires = make([]StackResponse, len(stack.Requires))
	for i, require := range stack.Requires {
		s.Requires[i] = StackResponse{Name: require.Name}
	}

	for parameterName, parameter := range stack.Parameters {
		s.Parameters = append(s.Parameters, StackParameterResponse{
			ParameterName: parameterName,
			DisplayName:   parameter.DisplayName,
			DefaultValue:  parameter.DefaultValue,
			Consumed:      parameter.Consumed,
			Priority:      parameter.Priority,
			Sensitive:     parameter.Sensitive,
		})
	}

	return s
}

// swagger:model StackParameter
type StackParameterResponse struct {
	ParameterName string  `json:"parameterName"`
	DisplayName   string  `json:"displayName"`
	DefaultValue  *string `json:"defaultValue,omitempty"`
	Consumed      bool    `json:"consumed"`
	Priority      uint    `json:"priority"`
	Sensitive     bool    `json:"sensitive"`
}

// swagger:model Stack
type StackResponse struct {
	Name       string                   `json:"name"`
	Parameters []StackParameterResponse `json:"parameters,omitempty"`
	Requires   []StackResponse          `json:"requires,omitempty"`
}

// FindAll stack
func (h Handler) FindAll(c *gin.Context) {
	// swagger:route GET /stacks stacks
	//
	// Find all stacks
	//
	// Find all stacks...
	//
	// Security:
	//  oauth2:
	//
	// Responses:
	//   200: Stacks
	//   401: Error
	//   403: Error
	//   404: Error
	//   415: Error
	stacks, err := h.service.FindAll()
	if err != nil {
		_ = c.Error(fmt.Errorf("error loading stacks: %w", err))
		return
	}

	response := make([]StackResponse, len(stacks))
	for i, stack := range stacks {
		response[i] = toResponseStack(stack)
	}
	c.JSON(http.StatusOK, response)
}
