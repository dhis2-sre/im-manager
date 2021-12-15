package instance

import (
	"bufio"
	"fmt"
	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	userClient "github.com/dhis2-sre/im-users/pkg/client"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"net/http"
	"strconv"
)

func ProvideHandler(
	userClient userClient.Client,
	instanceService Service,
) Handler {
	return Handler{
		userClient,
		instanceService,
	}
}

type Handler struct {
	userClient      userClient.Client
	instanceService Service
}

type ParameterRequest struct {
	StackParameterID uint   `json:"stackParameterId" binding:"required"`
	Value            string `json:"value" binding:"required"`
}

type CreateInstanceRequest struct {
	Name               string             `json:"name" binding:"required"`
	GroupID            uint               `json:"groupId" binding:"required"`
	StackID            uint               `json:"stackId" binding:"required"`
	RequiredParameters []ParameterRequest `json:"requiredParameters"`
	OptionalParameters []ParameterRequest `json:"optionalParameters"`
}

// Create godoc
// @Summary Create instance
// @Description Create instance
// @Tags Restricted
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{} //model.Instance
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Router /instances [post]
// @Param createInstanceRequest body CreateInstanceRequest true "Create instance request"
// @Security OAuth2Password
func (h Handler) Create(c *gin.Context) {
	var request CreateInstanceRequest

	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	requiredParameters := convertRequiredParameters(&request.RequiredParameters)
	optionalParameters := convertOptionalParameters(&request.OptionalParameters)

	instance := &model.Instance{
		Name:               request.Name,
		UserID:             uint(user.ID),
		GroupID:            request.GroupID,
		StackID:            request.StackID,
		RequiredParameters: *requiredParameters,
		OptionalParameters: *optionalParameters,
	}

	userWithGroups, err := h.userClient.FindUserById(uint(user.ID))
	if err != nil {
		_ = c.Error(err)
		return
	}

	log.Printf("%+v", userWithGroups)

	canWrite := handler.CanWriteInstance(userWithGroups, instance)
	if !canWrite {
		unauthorized := apperror.NewUnauthorized("Write access denied")
		_ = c.Error(unauthorized)
		return
	}

	if err := h.instanceService.Create(instance); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, instance)
}

func convertRequiredParameters(requestParameters *[]ParameterRequest) *[]model.InstanceRequiredParameter {
	if len(*requestParameters) > 0 {
		var parameters = make([]model.InstanceRequiredParameter, len(*requestParameters))
		for i, parameter := range *requestParameters {
			parameters[i] = model.InstanceRequiredParameter{
				StackRequiredParameterID: parameter.StackParameterID,
				Value:                    parameter.Value,
			}
		}
		return &parameters
	}
	return &[]model.InstanceRequiredParameter{}
}

func convertOptionalParameters(requestParameters *[]ParameterRequest) *[]model.InstanceOptionalParameter {
	if len(*requestParameters) > 0 {
		var parameters = make([]model.InstanceOptionalParameter, len(*requestParameters))
		for i, parameter := range *requestParameters {
			parameters[i] = model.InstanceOptionalParameter{
				StackOptionalParameterID: parameter.StackParameterID,
				Value:                    parameter.Value,
			}
		}
		return &parameters
	}
	return &[]model.InstanceOptionalParameter{}
}

// Delete godoc
// @Summary Delete instance by id
// @Description Delete instance by id...
// @Tags Restricted
// @Accept json
// @Produce json
// @Success 202 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /instances/{id} [delete]
// @Param id path string true "Instance id"
// @Security OAuth2Password
func (h Handler) Delete(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		badRequest := apperror.NewBadRequest("Error parsing id")
		_ = c.Error(badRequest)
		return
	}

	user, err := handler.GetUserFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userWithGroups, err := h.userClient.FindUserById(uint(user.ID))
	if err != nil {
		notFound := apperror.NewNotFound("user", strconv.Itoa(int(user.ID)))
		_ = c.Error(notFound)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", strconv.Itoa(id))
		_ = c.Error(notFound)
		return
	}

	canWrite := handler.CanWriteInstance(userWithGroups, instance)
	if !canWrite {
		unauthorized := apperror.NewUnauthorized("Write access denied")
		_ = c.Error(unauthorized)
		return
	}

	err = h.instanceService.Delete(instance.ID)
	if err != nil {
		message := fmt.Sprintf("Unable to delete instance: %s", err)
		internal := apperror.NewInternal(message)
		_ = c.Error(internal)
		return
	}

	c.Status(http.StatusAccepted)
}

// FindById godoc
// @Summary Find instance by id
// @Description Find instance by id...
// @Tags Restricted
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /instances/{id} [get]
// @Param id path string true "Instance id"
// @Security OAuth2Password
func (h Handler) FindById(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		badRequest := apperror.NewBadRequest("Error parsing id")
		_ = c.Error(badRequest)
		return
	}

	user, err := handler.GetUserFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userWithGroups, err := h.userClient.FindUserById(uint(user.ID))
	if err != nil {
		notFound := apperror.NewNotFound("user", strconv.Itoa(int(user.ID)))
		_ = c.Error(notFound)
		return
	}

	instance, err := h.instanceService.FindWithParametersById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", strconv.Itoa(id))
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanReadInstance(userWithGroups, instance)
	if !canRead {
		unauthorized := apperror.NewUnauthorized("Read access denied")
		_ = c.Error(unauthorized)
		return
	}

	c.JSON(http.StatusOK, instance)
}

func (h Handler) Logs(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		badRequest := apperror.NewBadRequest("Error parsing id")
		_ = c.Error(badRequest)
		return
	}

	user, err := handler.GetUserFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userWithGroups, err := h.userClient.FindUserById(uint(user.ID))
	if err != nil {
		notFound := apperror.NewNotFound("user", strconv.Itoa(int(user.ID)))
		_ = c.Error(notFound)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", strconv.Itoa(id))
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanReadInstance(userWithGroups, instance)
	if !canRead {
		unauthorized := apperror.NewUnauthorized("Read access denied")
		_ = c.Error(unauthorized)
		return
	}

	readCloser, err := h.instanceService.Logs(instance)

	if err != nil {
		conflict := apperror.NewConflict(err.Error())
		_ = c.Error(conflict)
		return
	}

	defer func(readCloser io.ReadCloser) {
		err := readCloser.Close()
		if err != nil {
			_ = c.Error(err)
		}
	}(readCloser)

	bufferedReader := bufio.NewReader(readCloser)

	c.Stream(func(writer io.Writer) bool {
		readBytes, err := bufferedReader.ReadBytes('\n')
		if err != nil {
			return false
		}

		_, err = writer.Write(readBytes)
		return err == nil
	})

	c.Status(http.StatusOK)
}

// NameToId godoc
// @Summary Instance id by instance name
// @Description Return instance id given instance name
// @Tags Restricted
// @Accept json
// @Produce json
// @Success 200 {object} string
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} string
// @Router /instances-name-to-id/{group}/{name} [get]
// @Param group path string true "Instance group"
// @Param name path string true "Instance name"
// @Security OAuth2Password
func (h Handler) NameToId(c *gin.Context) {
	name := c.Param("name")
	groupIdParam := c.Param("groupId")
	groupId, err := strconv.Atoi(groupIdParam)
	if err != nil {
		badRequest := apperror.NewBadRequest("Error parsing id")
		_ = c.Error(badRequest)
		return
	}

	user, err := handler.GetUserFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userWithGroups, err := h.userClient.FindUserById(uint(user.ID))
	if err != nil {
		notFound := apperror.NewNotFound("user", strconv.Itoa(int(user.ID)))
		_ = c.Error(notFound)
		return
	}

	instance, err := h.instanceService.FindByNameAndGroup(name, uint(groupId))
	if err != nil {
		notFound := apperror.NewNotFound("instance", name)
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanReadInstance(userWithGroups, instance)
	if !canRead {
		unauthorized := apperror.NewUnauthorized("Read access denied")
		_ = c.Error(unauthorized)
		return
	}

	c.JSON(http.StatusOK, instance.ID)
}
