package instance

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/dhis2-sre/im-manager/pkg/stack"

	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
)

func NewHandler(
	userService userServiceHandler,
	groupService groupServiceHandler,
	instanceService Service,
	stackService stack.Service,
) Handler {
	return Handler{
		userService,
		groupService,
		instanceService,
		stackService,
	}
}

type Service interface {
	ConsumeParameters(source, destination *model.Instance) error
	Pause(token string, instance *model.Instance) error
	Resume(token string, instance *model.Instance) error
	Reset(token string, instance *model.Instance) error
	Restart(token string, instance *model.Instance, typeSelector string) error
	Save(instance *model.Instance) (*model.Instance, error)
	Deploy(token string, instance *model.Instance) error
	FindById(id uint) (*model.Instance, error)
	FindByIdDecrypted(id uint) (*model.Instance, error)
	FindByNameAndGroup(instance string, group string) (*model.Instance, error)
	Delete(token string, id uint) error
	Logs(instance *model.Instance, group *model.Group, typeSelector string) (io.ReadCloser, error)
	FindInstances(user *model.User, presets bool) ([]GroupWithInstances, error)
	Link(source, destination *model.Instance) error
}

type Handler struct {
	userService     userServiceHandler
	groupService    groupServiceHandler
	instanceService Service
	stackService    stack.Service
}

type userServiceHandler interface {
	FindById(id uint) (*model.User, error)
}

type groupServiceHandler interface {
	Find(name string) (*model.Group, error)
}

type DeployInstanceRequest struct {
	Name               string                            `json:"name" binding:"required,dns_rfc1035_label"`
	Group              string                            `json:"groupName" binding:"required"`
	Stack              string                            `json:"stackName" binding:"required"`
	RequiredParameters []model.InstanceRequiredParameter `json:"requiredParameters"`
	OptionalParameters []model.InstanceOptionalParameter `json:"optionalParameters"`
	SourceInstance     uint                              `json:"sourceInstance"`
	PresetInstance     uint                              `json:"presetInstance"`
}

// Deploy instance
func (h Handler) Deploy(c *gin.Context) {
	// swagger:route POST /instances deployInstance
	//
	// Deploy instance
	//
	// Deploy an instance...
	//
	// Security:
	//  oauth2:
	//
	// responses:
	//   201: Instance
	//   400: Error
	//   401: Error
	//   403: Error
	//   404: Error
	//   415: Error
	deploy := false
	if deployParam, ok := c.GetQuery("deploy"); ok {
		var err error
		if deploy, err = strconv.ParseBool(deployParam); err != nil {
			_ = c.Error(err)
			return
		}
	}

	preset := false
	if presetParam, ok := c.GetQuery("preset"); ok {
		var err error
		if preset, err = strconv.ParseBool(presetParam); err != nil {
			_ = c.Error(err)
			return
		}
	}

	if deploy && preset {
		_ = c.Error(fmt.Errorf("a preset can't be deployed, thus both deploy and preset can't be true"))
		return
	}

	deploy = !preset

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

	i := &model.Instance{
		Name:               request.Name,
		UserID:             user.ID,
		GroupName:          request.Group,
		StackName:          request.Stack,
		RequiredParameters: request.RequiredParameters,
		OptionalParameters: request.OptionalParameters,
		Preset:             preset,
		PresetID:           request.PresetInstance,
	}

	canWrite := handler.CanWriteInstance(user, i)
	if !canWrite {
		unauthorized := apperror.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	_, err = h.instanceService.Save(i)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindByIdDecrypted(i.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if request.PresetInstance != 0 {
		err = h.consumeParameters(user, request.PresetInstance, instance, true)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}

	if request.SourceInstance != 0 {
		err = h.consumeParameters(user, request.SourceInstance, instance, false)
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
		decryptedInstance, err := h.instanceService.FindByIdDecrypted(i.ID)
		if err != nil {
			_ = c.Error(err)
			return
		}

		err = h.instanceService.Deploy(token, decryptedInstance)
		if err != nil {
			_ = c.Error(err)
			return
		}
		c.JSON(http.StatusCreated, savedInstance)
		return
	}
	c.JSON(http.StatusAccepted, savedInstance)
}

func (h Handler) consumeParameters(user *model.User, sourceInstanceId uint, instance *model.Instance, preset bool) error {
	sourceInstance, err := h.instanceService.FindByIdDecrypted(sourceInstanceId)
	if err != nil {
		return err
	}

	if preset && !sourceInstance.Preset {
		return apperror.NewUnauthorized(fmt.Sprintf("instance (id: %d) isn't a preset", sourceInstance.ID))
	}

	if preset && sourceInstance.StackName != instance.StackName {
		return apperror.NewUnauthorized(fmt.Sprintf("preset stack (%s) doesn't match instance stack (%s)", sourceInstance.StackName, instance.StackName))
	}

	canReadSource := handler.CanReadInstance(user, sourceInstance)
	if !canReadSource {
		return apperror.NewUnauthorized(fmt.Sprintf("read access to source instance (id: %d) denied", sourceInstance.ID))
	}

	err = h.instanceService.ConsumeParameters(sourceInstance, instance)
	if err != nil {
		return err
	}

	if !preset {
		err = h.instanceService.Link(sourceInstance, instance)
		if err != nil {
			return err
		}
	}
	return nil
}

type UpdateInstanceRequest struct {
	RequiredParameters []model.InstanceRequiredParameter `json:"requiredParameters"`
	OptionalParameters []model.InstanceOptionalParameter `json:"optionalParameters"`
}

// Update instance
func (h Handler) Update(c *gin.Context) {
	// swagger:route PUT /instances/{id} updateInstance
	//
	// Update instance
	//
	// Update an instance...
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	204: Instance
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
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

	saved, err := h.instanceService.Save(instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	decrypted, err := h.instanceService.FindByIdDecrypted(saved.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.instanceService.Deploy(token, decrypted)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusAccepted, decrypted)
}

// Pause instance
func (h Handler) Pause(c *gin.Context) {
	// swagger:route PUT /instances/{id}/pause pauseInstance
	//
	// Pause instance
	//
	// Pause an instance. Pause can be called multiple times even on an already paused instance
	// (idempotent).
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	202:
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
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

// Reset instance
func (h Handler) Reset(c *gin.Context) {
	// swagger:route PUT /instances/{id}/reset resetInstance
	//
	// Reset instance
	//
	// Resetting an instance will completely destroy it and redeploy using the same parameters
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	202:
	//	400: Error
	//	401: Error
	//	403: Error
	//	404: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
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

	err = h.instanceService.Reset(token, instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// Resume paused instance
func (h Handler) Resume(c *gin.Context) {
	// swagger:route PUT /instances/{id}/resume resumeInstance
	//
	// Resume paused instance
	//
	// Resume a paused instance. Resume can be called multiple times even on an already running
	// instance (idempotent).
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	202:
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
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

	err = h.instanceService.Resume(token, instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// Restart instance
func (h Handler) Restart(c *gin.Context) {
	// swagger:route PUT /instances/{id}/restart restartInstance
	//
	// Restart instance
	//
	// Restart an instance...
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	202:
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
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

	selector := c.Query("selector")
	err = h.instanceService.Restart(token, instance, selector)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// Delete instance by id
func (h Handler) Delete(c *gin.Context) {
	// swagger:route DELETE /instances/{id} deleteInstance
	//
	// Delete instance
	//
	// Delete an instance by id
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	202:
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
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
func (h Handler) FindById(c *gin.Context) {
	// swagger:route GET /instances/{id} findById
	//
	// Find instance
	//
	// Find an instance by id
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: Instance
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
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
func (h Handler) FindByIdDecrypted(c *gin.Context) {
	// swagger:route GET /instances/{id}/parameters findByIdDecrypted
	//
	// Find decrypted instance
	//
	// Find instance by id with decrypted parameters
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: Instance
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
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
func (h Handler) Logs(c *gin.Context) {
	// swagger:route GET /instances/{id}/logs instanceLogs
	//
	// Stream logs
	//
	// Stream instance logs in real time
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	200: InstanceLogsResponse
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
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

	group, err := h.groupService.Find(instance.GroupName)
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
func (h Handler) NameToId(c *gin.Context) {
	// swagger:route GET /instances-name-to-id/{groupName}/{instanceName} instanceNameToId
	//
	// Find an instance
	//
	// Find instance id by name and group name
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: Instance
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
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

// ListInstances instances
func (h Handler) ListInstances(c *gin.Context) {
	// swagger:route GET /instances listInstances
	//
	// List instances
	//
	// List all instances accessible by the user
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: []GroupWithInstances
	//	401: Error
	//	403: Error
	//	415: Error
	h.findInstances(c, false)
}

// ListPresets presets
func (h Handler) ListPresets(c *gin.Context) {
	// swagger:route GET /presets listPresets
	//
	// List presets
	//
	// List all presets accessible by the user
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: []GroupWithInstances
	//	401: Error
	//	403: Error
	//	415: Error
	h.findInstances(c, true)
}

func (h Handler) findInstances(c *gin.Context, presets bool) {
	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instances, err := h.instanceService.FindInstances(user, presets)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, instances)
}
