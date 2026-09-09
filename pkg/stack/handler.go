package stack

import (
	"fmt"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/kube"

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

	s.Companions = make([]StackCompanionResponse, len(stack.Companions))
	for i, companion := range stack.Companions {
		s.Companions[i] = StackCompanionResponse{Name: companion.Stack.Name, When: companion.When}
	}

	s.ParameterGroups = stack.ParameterGroups

	for parameterName, parameter := range stack.Parameters {
		s.Parameters = append(s.Parameters, StackParameterResponse{
			ParameterName: parameterName,
			DisplayName:   parameter.DisplayName,
			DefaultValue:  parameter.DefaultValue,
			Consumed:      parameter.Consumed,
			Priority:      parameter.Priority,
			Sensitive:     parameter.Sensitive,
			Group:         parameter.Group,
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
	Group         string  `json:"group,omitempty"`
}

// swagger:model Stack
type StackResponse struct {
	Name            string                   `json:"name"`
	Parameters      []StackParameterResponse `json:"parameters,omitempty"`
	ParameterGroups []ParameterGroup         `json:"parameterGroups,omitempty"`
	Requires        []StackResponse          `json:"requires,omitempty"`
	Companions      []StackCompanionResponse `json:"companions,omitempty"`
}

// StackCompanionResponse names a stack that can be deployed alongside this one. Like Requires it
// carries only the name, so a client fetches that stack to learn its parameters. When it has a
// condition, the companion applies only while that condition holds over this stack's parameters.
// swagger:model StackCompanion
type StackCompanionResponse struct {
	Name string          `json:"name"`
	When *kube.Condition `json:"when,omitempty"`
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
