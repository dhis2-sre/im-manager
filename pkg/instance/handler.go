package instance

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/dhis2-sre/im-manager/pkg/stack"

	jobClient "github.com/dhis2-sre/im-job/pkg/client"
	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	userClient      userClientHandler
	jobClient       jobClient.Client
	instanceService Service
	stackService    stack.Service
}

func NewHandler(
	usrClient userClientHandler,
	jobClient jobClient.Client,
	instanceService Service,
	stackService stack.Service,
) Handler {
	return Handler{
		usrClient,
		jobClient,
		instanceService,
		stackService,
	}
}

type userClientHandler interface {
	FindGroupByName(token string, name string) (*models.Group, error)
	FindUserById(token string, id uint) (*models.User, error)
}

type DeployInstanceRequest struct {
	Name               string                            `json:"name" binding:"required,dns_rfc1035_label"`
	Group              string                            `json:"groupName" binding:"required"`
	Stack              string                            `json:"stackName" binding:"required"`
	RequiredParameters []model.InstanceRequiredParameter `json:"requiredParameters"`
	OptionalParameters []model.InstanceOptionalParameter `json:"optionalParameters"`
	SourceInstance     uint                              `json:"sourceInstance"`
}

// Deploy instance
// swagger:route POST /instances deployInstance
//
// Deploy instance
//
// Security:
//  oauth2:
//
// responses:
//   201: Instance
//   401: Error
//   403: Error
//   404: Error
//   415: Error
func (h Handler) Deploy(c *gin.Context) {
	deploy := true
	if deployParam, ok := c.GetQuery("deploy"); ok {
		var err error
		if deploy, err = strconv.ParseBool(deployParam); err != nil {
			_ = c.Error(err)
			return
		}
	}

	var request DeployInstanceRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	token, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance := &model.Instance{
		Name:               request.Name,
		UserID:             uint(user.ID),
		GroupName:          request.Group,
		StackName:          request.Stack,
		RequiredParameters: request.RequiredParameters,
		OptionalParameters: request.OptionalParameters,
	}

	canWrite := handler.CanWriteInstance(user, instance)
	if !canWrite {
		unauthorized := apperror.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	if request.SourceInstance != 0 {
		sourceInstance, err := h.instanceService.FindByIdDecrypted(request.SourceInstance)
		if err != nil {
			_ = c.Error(err)
			return
		}

		canWriteSource := handler.CanWriteInstance(user, sourceInstance)
		if !canWriteSource {
			err := apperror.NewUnauthorized(fmt.Sprintf("write access to source instance (id: %d) denied", sourceInstance.ID))
			_ = c.Error(err)
			return
		}

		if sourceInstance.DeployLog == "" {
			err := fmt.Errorf("source instance %q not deployed", sourceInstance.Name)
			_ = c.Error(err)
			return
		}

		err = h.instanceService.ConsumeParameters(sourceInstance, instance)
		if err != nil {
			_ = c.Error(err)
			return
		}

		savedInstance, err := h.instanceService.Save(instance)
		if err != nil {
			_ = c.Error(err)
			return
		}

		err = h.instanceService.Link(sourceInstance, savedInstance)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}

	savedInstance, err := h.instanceService.Save(instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if deploy {
		err = h.instanceService.Deploy(token, savedInstance)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusCreated, savedInstance)
		return
	}
	c.JSON(http.StatusAccepted, savedInstance)
}

type UpdateInstanceRequest struct {
	RequiredParameters []model.InstanceRequiredParameter `json:"requiredParameters"`
	OptionalParameters []model.InstanceOptionalParameter `json:"optionalParameters"`
}

// Update instance
// swagger:route PUT /instances/{id} updateInstance
//
// Update an instance
//
// Security:
//  oauth2:
//
// responses:
//   204: Instance
//   401: Error
//   403: Error
//   404: Error
//   415: Error
func (h Handler) Update(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		badRequest := apperror.NewBadRequest("error parsing id")
		_ = c.Error(badRequest)
		return
	}

	var request UpdateInstanceRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	token, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canWrite := handler.CanWriteInstance(user, instance)
	if !canWrite {
		unauthorized := apperror.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	instance.RequiredParameters = request.RequiredParameters
	instance.OptionalParameters = request.OptionalParameters

	err = h.instanceService.Deploy(token, instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusAccepted, instance)
}

// Pause instance
// swagger:route PUT /instances/{id}/pause pauseInstance
//
// Pause instance
//
// Security:
//  oauth2:
//
// responses:
//   202:
//   401: Error
//   403: Error
//   404: Error
//   415: Error
func (h Handler) Pause(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("failed to parse id: %s", err))
		return
	}

	token, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canWrite := handler.CanWriteInstance(user, instance)
	if !canWrite {
		_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("write access denied"))
		return
	}

	err = h.instanceService.Pause(token, instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// Restart instance
// swagger:route PUT /instances/{id}/restart restartInstance
//
// Restart instance
//
// Security:
//  oauth2:
//
// responses:
//   202:
//   401: Error
//   403: Error
//   404: Error
//   415: Error
func (h Handler) Restart(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("failed to parse id: %s", err))
		return
	}

	token, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canWrite := handler.CanWriteInstance(user, instance)
	if !canWrite {
		_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("write access denied"))
		return
	}

	err = h.instanceService.Restart(token, instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// Delete instance by id
// swagger:route DELETE /instances/{id} deleteInstance
//
// Delete an instance by id
//
// Security:
//  oauth2:
//
// responses:
//   202:
//   401: Error
//   403: Error
//   404: Error
//   415: Error
func (h Handler) Delete(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		badRequest := apperror.NewBadRequest("error parsing id")
		_ = c.Error(badRequest)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	token, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canWrite := handler.CanWriteInstance(user, instance)
	if !canWrite {
		unauthorized := apperror.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	err = h.instanceService.Delete(token, instance.ID)
	if err != nil {
		message := fmt.Sprintf("Unable to delete instance: %s", err)
		internal := apperror.NewInternal(message)
		_ = c.Error(internal)
		return
	}

	c.Status(http.StatusAccepted)
}

// FindById instance
// swagger:route GET /instances/{id} findById
//
// Find instance by id
//
// Security:
//  oauth2:
//
// responses:
//   200: Instance
//   401: Error
//   403: Error
//   404: Error
//   415: Error
func (h Handler) FindById(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		badRequest := apperror.NewBadRequest("error parsing id")
		_ = c.Error(badRequest)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanReadInstance(user, instance)
	if !canRead {
		unauthorized := apperror.NewUnauthorized("read access denied")
		_ = c.Error(unauthorized)
		return
	}

	c.JSON(http.StatusOK, instance)
}

// FindByIdDecrypted instance
// swagger:route GET /instances/{id}/parameters findByIdDecrypted
//
// Find instance by id with decrypted parameters
//
// Security:
//  oauth2:
//
// responses:
//   200: Instance
//   401: Error
//   403: Error
//   404: Error
//   415: Error
func (h Handler) FindByIdDecrypted(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		badRequest := apperror.NewBadRequest("error parsing id")
		_ = c.Error(badRequest)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindByIdDecrypted(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanWriteInstance(user, instance)
	if !canRead {
		unauthorized := apperror.NewUnauthorized("read access denied")
		_ = c.Error(unauthorized)
		return
	}

	c.JSON(http.StatusOK, instance)
}

// Logs instance
// swagger:route GET /instances/{id}/logs instanceLogs
//
// Stream instance logs in real time
//
// Security:
//  oauth2:
//
// Responses:
//   200: InstanceLogsResponse
//   401: Error
//   403: Error
//   404: Error
//   415: Error
func (h Handler) Logs(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		badRequest := apperror.NewBadRequest("error parsing id")
		_ = c.Error(badRequest)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	token, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanReadInstance(user, instance)
	if !canRead {
		unauthorized := apperror.NewUnauthorized("read access denied")
		_ = c.Error(unauthorized)
		return
	}

	group, err := h.userClient.FindGroupByName(token, instance.GroupName)
	if err != nil {
		_ = c.Error(err)
	}

	selector := c.Query("selector")
	r, err := h.instanceService.Logs(instance, group, selector)
	if err != nil {
		conflict := apperror.NewConflict(err.Error())
		_ = c.Error(conflict)
		return
	}

	defer func(r io.ReadCloser) {
		err := r.Close()
		if err != nil {
			_ = c.Error(err)
		}
	}(r)

	bufferedReader := bufio.NewReader(r)

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

// NameToId instance
// swagger:route GET /instances-name-to-id/{groupName}/{instanceName} instanceNameToId
//
// Find instance id by name and group name
//
// Security:
//  oauth2:
//
// responses:
//   200: Instance
//   401: Error
//   403: Error
//   404: Error
//   415: Error
func (h Handler) NameToId(c *gin.Context) {
	instanceName := c.Param("instanceName")
	groupName := c.Param("groupName")
	if groupName == "" {
		badRequest := apperror.NewBadRequest("missing group name")
		_ = c.Error(badRequest)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindByNameAndGroup(instanceName, groupName)
	if err != nil {
		notFound := apperror.NewNotFound("instance", instanceName)
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanReadInstance(user, instance)
	if !canRead {
		unauthorized := apperror.NewUnauthorized("read access denied")
		_ = c.Error(unauthorized)
		return
	}

	c.JSON(http.StatusOK, instance.ID)
}

type GroupWithInstances struct {
	Name      string
	Hostname  string
	Instances []*model.Instance
}

// List instances
// swagger:route GET /instances listInstances
//
// List instances
//
// Security:
//  oauth2:
//
// responses:
//   200: []GroupWithInstances
//   401: Error
//   403: Error
//   415: Error
func (h Handler) List(c *gin.Context) {
	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instances, err := h.instanceService.FindInstances(user.Groups)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, h.groupsWithInstances(user.Groups, instances))
}

func (h Handler) groupsWithInstances(groups []*models.Group, instances []*model.Instance) []GroupWithInstances {
	groupsWithInstances := make([]GroupWithInstances, len(groups))
	for i, group := range groups {
		groupsWithInstances[i].Name = group.Name
		groupsWithInstances[i].Hostname = group.Hostname
		groupsWithInstances[i].Instances = h.filterByGroupId(instances, func(instance *model.Instance) bool {
			return instance.GroupName == group.Name
		})
	}
	return groupsWithInstances
}

func (h Handler) filterByGroupId(instances []*model.Instance, test func(instance *model.Instance) bool) (ret []*model.Instance) {
	for _, instance := range instances {
		if test(instance) {
			ret = append(ret, instance)
		}
	}
	return
}
