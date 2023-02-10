package instance

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"

	"github.com/dhis2-sre/im-manager/pkg/stack"

	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"github.com/gin-gonic/gin"
)

func NewHandler(
	usrClient userClientHandler,
	instanceService Service,
	stackService stack.Service,
) Handler {
	return Handler{
		usrClient,
		instanceService,
		stackService,
	}
}

type Service interface {
	ConsumeParameters(source, destination *model.Instance) error
	Pause(token string, instance *model.Instance) error
	Restart(token string, instance *model.Instance, typeSelector string) error
	Save(instance *model.Instance) (*model.Instance, error)
	Deploy(token string, instance *model.Instance) error
	FindById(id uint) (*model.Instance, error)
	FindByIdDecrypted(id uint) (*model.Instance, error)
	FindByNameAndGroup(instance string, group string) (*model.Instance, error)
	Delete(token string, id uint) error
	Logs(instance *model.Instance, group *models.Group, typeSelector string) (io.ReadCloser, error)
	FindInstances(groups []*models.Group, presets bool) ([]*model.Instance, error)
	Link(source, destination *model.Instance) error
}

type Handler struct {
	userClient      userClientHandler
	instanceService Service
	stackService    stack.Service
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
	PresetInstance     uint                              `json:"presetInstance"`
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
		UserID:             uint(user.ID),
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

func (h Handler) consumeParameters(user *models.User, sourceInstanceId uint, instance *model.Instance, preset bool) error {
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

	selector := c.Query("selector")
	err = h.instanceService.Restart(token, instance, selector)
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

	groups := h.uniqueUserGroups(user)
	instances, err := h.instanceService.FindInstances(groups, false)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, h.groupsWithInstances(instances))
}

func (h Handler) uniqueUserGroups(user *models.User) []*models.Group {
	groups := append(user.Groups, user.AdminGroups...)
	return removeDuplicates(groups)
}

// ListPresets presets
// swagger:route GET /presets listPresets
//
// List presets
//
// Security:
//  oauth2:
//
// responses:
//   200: []GroupWithInstances
//   401: Error
//   403: Error
//   415: Error
func (h Handler) ListPresets(c *gin.Context) {
	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	groups := h.uniqueUserGroups(user)
	presets, err := h.instanceService.FindInstances(groups, true)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, h.groupsWithInstances(presets))
}

func (h Handler) groupsWithInstances(instances []*model.Instance) []GroupWithInstances {
	groups := h.uniqueInstanceGroups(instances)
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

func (h Handler) uniqueInstanceGroups(instances []*model.Instance) []*models.Group {
	groups := make([]*models.Group, len(instances))
	for i, instance := range instances {
		groups[i] = &models.Group{Name: instance.GroupName}
	}
	return removeDuplicates(groups)
}

type ByName []*models.Group

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

func removeDuplicates(s []*models.Group) []*models.Group {
	if len(s) < 1 {
		return s
	}

	sort.Sort(ByName(s))

	prev := 1
	for curr := 1; curr < len(s); curr++ {
		if s[curr-1].Name != s[curr].Name {
			s[prev] = s[curr]
			prev++
		}
	}

	return s[:prev]
}
