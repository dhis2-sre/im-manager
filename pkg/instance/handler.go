package instance

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
)

func NewHandler(groupService groupServiceHandler, instanceService *service, defaultTTL uint) Handler {
	return Handler{
		groupService,
		instanceService,
		defaultTTL,
	}
}

type Handler struct {
	groupService    groupServiceHandler
	instanceService *service
	defaultTTL      uint
}

type groupServiceHandler interface {
	Find(name string) (*model.Group, error)
}

type SaveDeploymentRequest struct {
	Name        string `json:"name" binding:"required,dns_rfc1035_label"`
	Description string `json:"description"`
	Group       string `json:"group" binding:"required"`
	TTL         uint   `json:"ttl"`
	Source      uint   `json:"source"` // TODO: Create from source eg. other deployment
	Preset      uint   `json:"preset"` // TODO: Create as preset
}

func (h Handler) SaveDeployment(c *gin.Context) {
	// swagger:route POST /deployments saveDeployment
	//
	// Save a deployment
	//
	// Save a deployment...
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: Deployment
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	var request SaveDeploymentRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	group, err := h.groupService.Find(request.Group)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if !group.Deployable {
		forbidden := errdef.NewForbidden("group isn't deployable: %s", group.Name)
		_ = c.Error(forbidden)
		return
	}

	if request.TTL == 0 {
		request.TTL = h.defaultTTL
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment := &model.Deployment{
		UserID:      user.ID,
		Name:        request.Name,
		Description: request.Description,
		GroupName:   request.Group,
		TTL:         request.TTL,
	}

	group, err = h.groupService.Find(request.Group)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if !group.Deployable {
		forbidden := errdef.NewForbidden("group isn't deployable: %s", group.Name)
		_ = c.Error(forbidden)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		unauthorized := errdef.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	// TODO: If request.Source, load deployment... Maybe only support this for instances
	err = h.instanceService.SaveDeployment(deployment)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, deployment)
}

// FindDeploymentById deployment
func (h Handler) FindDeploymentById(c *gin.Context) {
	// swagger:route GET /deployments/{id} findDeploymentById
	//
	// Find a deployment
	//
	// Find a deployment by id
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: Deployment
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canRead := handler.CanReadDeployment(user, deployment)
	if !canRead {
		unauthorized := errdef.NewUnauthorized("read access denied")
		_ = c.Error(unauthorized)
		return
	}

	c.JSON(http.StatusOK, deployment)
}

type parameter struct{ Value string }

type parameters map[string]parameter

type SaveInstanceRequest struct {
	StackName string `json:"stackName"`
	Preset    bool   `json:"preset"`
	PresetID  uint   `json:"presetId"` // The preset id this link is created from
	//	Public     bool                      `json:"public"`
	Parameters parameters `json:"parameters"`
}

func (h Handler) SaveInstance(c *gin.Context) {
	// swagger:route POST /deployments/{id}/instance saveInstance
	//
	// Save an instance
	//
	// Save an instance...
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: DeploymentInstance
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	var request SaveInstanceRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	deploymentId, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	// Convert request parameters to LinkParameters
	params := make(model.Parameters, len(request.Parameters))
	for k, v := range request.Parameters {
		params[k] = model.DeploymentInstanceParameter{
			ParameterName: k,
			Value:         v.Value,
		}
	}

	instance := &model.DeploymentInstance{
		DeploymentID: deploymentId,
		StackName:    request.StackName,
		Parameters:   params,
		Preset:       request.Preset,
		//		PresetID:   0,
		//		Public:     false,
	}

	err := h.instanceService.SaveInstance(instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, instance)
}

type DeployInstanceRequest struct {
	Name           string                    `json:"name" binding:"required,dns_rfc1035_label"`
	Group          string                    `json:"groupName" binding:"required"`
	Description    string                    `json:"description"`
	Stack          string                    `json:"stackName" binding:"required"`
	Public         bool                      `json:"public"`
	TTL            uint                      `json:"ttl"`
	Parameters     []model.InstanceParameter `json:"parameters"`
	SourceInstance uint                      `json:"sourceInstance"`
	PresetInstance uint                      `json:"presetInstance"`
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

	group, err := h.groupService.Find(request.Group)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if !group.Deployable {
		forbidden := errdef.NewForbidden("group isn't deployable: %s", group.Name)
		_ = c.Error(forbidden)
		return
	}

	if request.TTL == 0 {
		request.TTL = h.defaultTTL
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
		Name:        request.Name,
		UserID:      user.ID,
		GroupName:   request.Group,
		Description: request.Description,
		StackName:   request.Stack,
		Public:      request.Public,
		TTL:         request.TTL,
		Parameters:  request.Parameters,
		Preset:      preset,
		PresetID:    request.PresetInstance,
	}

	canWrite := handler.CanWriteInstance(user, i)
	if !canWrite {
		unauthorized := errdef.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	err = h.instanceService.Save(i)
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

	err = h.instanceService.Save(instance)
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
		c.JSON(http.StatusCreated, instance)
		return
	}
	c.JSON(http.StatusAccepted, instance)
}

func (h Handler) consumeParameters(user *model.User, sourceInstanceId uint, instance *model.Instance, preset bool) error {
	sourceInstance, err := h.instanceService.FindByIdDecrypted(sourceInstanceId)
	if err != nil {
		return err
	}

	if preset && !sourceInstance.Preset {
		return errdef.NewUnauthorized("instance (id: %d) isn't a preset", sourceInstance.ID)
	}

	if preset && sourceInstance.StackName != instance.StackName {
		return errdef.NewUnauthorized("preset stack (%s) doesn't match instance stack (%s)", sourceInstance.StackName, instance.StackName)
	}

	canReadSource := handler.CanReadInstance(user, sourceInstance)
	if !canReadSource {
		return errdef.NewUnauthorized("read access to source instance (id: %d) denied", sourceInstance.ID)
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
	TTL        uint                      `json:"ttl"`
	Parameters []model.InstanceParameter `json:"parameters"`
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
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	var request UpdateInstanceRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	if request.TTL == 0 {
		request.TTL = h.defaultTTL
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

	instance, err := h.instanceService.FindById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteInstance(user, instance)
	if !canWrite {
		unauthorized := errdef.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	if instance.TTL != h.defaultTTL {
		instance.TTL = request.TTL
	}
	instance.Parameters = request.Parameters

	err = h.instanceService.Save(instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	decrypted, err := h.instanceService.FindByIdDecrypted(instance.ID)
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
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteInstance(user, instance)
	if !canWrite {
		_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("write access denied"))
		return
	}

	err = h.instanceService.Pause(instance)
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
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
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

	instance, err := h.instanceService.FindByIdDecrypted(id)
	if err != nil {
		_ = c.Error(err)
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
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindByIdDecrypted(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteInstance(user, instance)
	if !canWrite {
		_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("write access denied"))
		return
	}

	err = h.instanceService.Resume(instance)
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
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteInstance(user, instance)
	if !canWrite {
		_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("write access denied"))
		return
	}

	selector := c.Query("selector")
	err = h.instanceService.Restart(instance, selector)
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
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteInstance(user, instance)
	if !canWrite {
		unauthorized := errdef.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	err = h.instanceService.Delete(instance.ID)
	if err != nil {
		_ = c.Error(fmt.Errorf("unable to delete instance: %v", err))
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
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canRead := handler.CanReadInstance(user, instance)
	if !canRead {
		unauthorized := errdef.NewUnauthorized("read access denied")
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
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindByIdDecrypted(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canRead := handler.CanWriteInstance(user, instance)
	if !canRead {
		unauthorized := errdef.NewUnauthorized("read access denied")
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
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canRead := handler.CanReadInstance(user, instance)
	if !canRead {
		unauthorized := errdef.NewUnauthorized("read access denied")
		_ = c.Error(unauthorized)
		return
	}

	group, err := h.groupService.Find(instance.GroupName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	selector := c.Query("selector")
	r, err := h.instanceService.Logs(instance, group, selector)
	if err != nil {
		_ = c.Error(err)
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
		badRequest := errdef.NewBadRequest("missing group name")
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
		_ = c.Error(err)
		return
	}

	canRead := handler.CanReadInstance(user, instance)
	if !canRead {
		unauthorized := errdef.NewUnauthorized("read access denied")
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
	//	200: []GroupsWithInstances
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
	//	200: []GroupsWithInstances
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

// ListPublicInstances instances
func (h Handler) ListPublicInstances(c *gin.Context) {
	// swagger:route GET /public/instances listPublicInstances
	//
	// List Public Instances
	//
	// List all public instances
	//
	// responses:
	//	200: []GroupsWithInstances
	instances, err := h.instanceService.FindPublicInstances()
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, instances)
}
