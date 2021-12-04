package stack

import (
	"fmt"
	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func ProvideHandler(service Service) Handler {
	return Handler{
		service,
	}
}

type Handler struct {
	service Service
}

// FindById godoc
// @Summary Find stack by id
// @Description Find stack by id...
// @Tags Restricted
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /stacks/{id} [get]
// @Param id path string true "Stack id"
// @Security OAuth2Password
func (h Handler) FindById(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		badRequest := apperror.NewBadRequest("Error parsing id")
		_ = c.Error(badRequest)
		return
	}

	stack, err := h.service.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("stack", idParam)
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
