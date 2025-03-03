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

	// TODO: Remove this and just return the stack once the front end has caught up
	s := Stack{}
	s.Name = stack.Name

	requires := make([]Stack, len(stack.Requires))
	for i, require := range stack.Requires {
		requires[i] = Stack{
			Name: require.Name,
		}
	}
	s.Requires = requires

	for parameterName, parameter := range stack.Parameters {
		s.Parameters = append(s.Parameters, StackParameter{
			ParameterName: parameterName,
			DisplayName:   parameter.DisplayName,
			DefaultValue:  parameter.DefaultValue,
			Consumed:      parameter.Consumed,
			Priority:      parameter.Priority,
			Sensitive:     parameter.Sensitive,
		})
	}

	c.JSON(http.StatusOK, s)
}

// swagger:model StackParameter
type StackParameter struct {
	ParameterName string  `json:"parameterName"`
	DisplayName   string  `json:"displayName"`
	DefaultValue  *string `json:"defaultValue,omitempty"`
	Consumed      bool    `json:"consumed"`
	Priority      uint    `json:"priority"`
	Sensitive     bool    `json:"sensitive"`
}

// swagger:model Stack
type Stack struct {
	Name       string           `json:"name"`
	Parameters []StackParameter `json:"parameters,omitempty"`
	Requires   []Stack          `json:"requires,omitempty"`
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
	c.JSON(http.StatusOK, stacks)
}
