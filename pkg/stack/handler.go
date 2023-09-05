package stack

import (
	"fmt"
	"net/http"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/gin-gonic/gin"
)

func NewHandler(service Service) Handler {
	return Handler{
		service,
	}
}

type Service interface {
	Find(name string) (*model.Stack, error)
	FindAll() ([]model.Stack, error)
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
	//   200: StackResponse
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

	c.JSON(http.StatusOK, stack)
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
	//   200: StacksResponse
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
