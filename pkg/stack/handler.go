package stack

import (
	"fmt"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/gin-gonic/gin"
)

func ProvideHandler(service Service) Handler {
	return Handler{
		service,
	}
}

type Handler struct {
	service Service
}

// Find godoc
// @Summary Find stack by name
// @Description Find stack by name...
// @Tags Restricted
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /stacks/{name} [get]
// @Param name path string true "Stack name"
// @Security OAuth2Password
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

// FindAll godoc
// @Summary Find all stacks
// @Description Find all stacks...
// @Tags Restricted
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /stacks [get]
// @Security OAuth2Password
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
