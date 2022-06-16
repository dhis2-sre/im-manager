package stack

import (
	"fmt"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) Handler {
	return Handler{
		service,
	}
}

// Find stack
// swagger:route GET /stacks/{name} stack
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
func (h Handler) Find(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		badRequest := apperror.NewBadRequest("stack name missing")
		_ = c.Error(badRequest)
		return
	}

	stack, err := h.service.Find(name)
	if err != nil {
		notFound := apperror.NewNotFound("stack", name)
		_ = c.Error(notFound)
		return
	}

	c.JSON(http.StatusOK, stack)
}

// FindAll stack
// swagger:route GET /stacks stacks
//
// Find all stacks
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
func (h Handler) FindAll(c *gin.Context) {
	stacks, err := h.service.FindAll()
	if err != nil {
		message := fmt.Sprintf("Error loading stacks: %s", err)
		internal := apperror.NewInternal(message)
		_ = c.Error(internal)
		return
	}
	c.JSON(http.StatusOK, stacks)
}
