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

type CreateInstanceRequest struct {
	Name      string `json:"name" binding:"required,dns_rfc1035_label"`
	GroupName string `json:"groupName" binding:"required"`
	StackName string `json:"stackName" binding:"required"`
}

// Create instance
// swagger:route POST /instances createInstance
//
// Create instance
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
func (h Handler) Create(c *gin.Context) {
	var request CreateInstanceRequest
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

	userWithGroups, err := h.userClient.FindUserById(token, user.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance := &model.Instance{
		Name:      request.Name,
		UserID:    user.ID,
		GroupName: request.GroupName,
		StackName: request.StackName,
	}

	canWrite := handler.CanWriteInstance(userWithGroups, instance)
	if !canWrite {
		unauthorized := apperror.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	savedInstance, err := h.instanceService.Create(instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, savedInstance)
}

type ParameterRequest struct {
	StackParameter string `json:"stackParameter" binding:"required"`
	Value          string `json:"value" binding:"required"`
}

type DeployInstanceRequest struct {
	RequiredParameters []ParameterRequest `json:"requiredParameters"`
	OptionalParameters []ParameterRequest `json:"optionalParameters"`
}

// Deploy instance
// swagger:route POST /instances/{id}/deploy deployInstance
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
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		badRequest := apperror.NewBadRequest("error parsing id")
		_ = c.Error(badRequest)
		return
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

	accessToken, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userWithGroups, err := h.userClient.FindUserById(accessToken, user.ID)
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

	canWrite := handler.CanWriteInstance(userWithGroups, instance)
	if !canWrite {
		unauthorized := apperror.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	group, err := h.userClient.FindGroupByName(accessToken, instance.GroupName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance.RequiredParameters = convertRequiredParameters(instance, request.RequiredParameters)
	instance.OptionalParameters = convertOptionalParameters(instance, request.OptionalParameters)

	err = h.instanceService.Deploy(accessToken, instance, group)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, instance)
}

// LinkDeploy instance
// swagger:route POST /instances/{id}/link/{newInstanceId} linkDeployInstance
//
// Deploy and link with an existing instance
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
func (h Handler) LinkDeploy(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		badRequest := apperror.NewBadRequest("error parsing id")
		_ = c.Error(badRequest)
		return
	}

	newIdParam := c.Param("newInstanceId")
	newId, err := strconv.ParseUint(newIdParam, 10, 64)
	if err != nil {
		badRequest := apperror.NewBadRequest("error parsing new id")
		_ = c.Error(badRequest)
		return
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

	accessToken, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userWithGroups, err := h.userClient.FindUserById(accessToken, user.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindWithDecryptedParametersById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canWrite := handler.CanWriteInstance(userWithGroups, instance)
	if !canWrite {
		unauthorized := apperror.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	newInstance, err := h.instanceService.FindById(uint(newId))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	stack, err := h.stackService.Find(instance.StackName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	newStack, err := h.stackService.Find(newInstance.StackName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// Consumed required parameters
	for _, v := range newStack.RequiredParameters {
		if v.Consumed && v.Name != newStack.HostnameVariable {
			parameter, err := instance.FindRequiredParameter(v.Name)
			if err != nil {
				_ = c.Error(err)
				return
			}
			parameterRequest := ParameterRequest{
				StackParameter: v.Name,
				Value:          parameter.Value,
			}
			request.RequiredParameters = append(request.RequiredParameters, parameterRequest)
		}
	}

	// Consumed optional parameters
	for _, v := range newStack.OptionalParameters {
		if v.Consumed && v.Name != newStack.HostnameVariable {
			parameter, err := instance.FindOptionalParameter(v.Name)
			if err != nil {
				// TODO: Better error handling
				stackParameter, serr := stack.FindOptionalParameter(v.Name)
				if serr != nil {
					notFound := apperror.NewNotFound("optional stack parameter", v.Name)
					_ = c.Error(notFound)
					return
				}
				parameter.Value = stackParameter.DefaultValue
			}
			parameterRequest := ParameterRequest{
				StackParameter: v.Name,
				Value:          parameter.Value,
			}
			request.OptionalParameters = append(request.OptionalParameters, parameterRequest)
		}
	}

	// Hostname parameter
	// TODO: Is hostname always a required parameter?
	if newStack.HostnameVariable != "" {
		hostnameParameter := ParameterRequest{
			StackParameter: newStack.HostnameVariable,
			Value:          fmt.Sprintf(stack.HostnamePattern, instance.Name, instance.GroupName),
		}
		request.RequiredParameters = append(request.RequiredParameters, hostnameParameter)
	}

	newInstance.RequiredParameters = convertRequiredParameters(newInstance, request.RequiredParameters)
	newInstance.OptionalParameters = convertOptionalParameters(newInstance, request.OptionalParameters)

	group, err := h.userClient.FindGroupByName(accessToken, instance.GroupName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.instanceService.Deploy(accessToken, newInstance, group)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, newInstance)
}

func convertRequiredParameters(instance *model.Instance, requestParameters []ParameterRequest) []model.InstanceRequiredParameter {
	if len(requestParameters) > 0 {
		parameters := make([]model.InstanceRequiredParameter, len(requestParameters))
		for i, parameter := range requestParameters {
			parameters[i] = model.InstanceRequiredParameter{
				InstanceID:             instance.ID,
				StackName:              instance.StackName,
				StackRequiredParameter: model.StackRequiredParameter{Name: parameter.StackParameter, StackName: instance.StackName},
				Value:                  parameter.Value,
			}
		}
		return parameters
	}
	return []model.InstanceRequiredParameter{}
}

func convertOptionalParameters(instance *model.Instance, requestParameters []ParameterRequest) []model.InstanceOptionalParameter {
	if len(requestParameters) > 0 {
		parameters := make([]model.InstanceOptionalParameter, len(requestParameters))
		for i, parameter := range requestParameters {
			parameters[i] = model.InstanceOptionalParameter{
				InstanceID:             instance.ID,
				StackName:              instance.StackName,
				StackOptionalParameter: model.StackOptionalParameter{Name: parameter.StackParameter, StackName: instance.StackName},
				Value:                  parameter.Value,
			}
		}
		return parameters
	}
	return []model.InstanceOptionalParameter{}
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

	userWithGroups, err := h.userClient.FindUserById(token, user.ID)
	if err != nil {
		notFound := apperror.NewNotFound("user", strconv.Itoa(int(user.ID)))
		_ = c.Error(notFound)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canWrite := handler.CanWriteInstance(userWithGroups, instance)
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
// swagger:route GET /instances/{id} findInstanceById
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

	token, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userWithGroups, err := h.userClient.FindUserById(token, user.ID)
	if err != nil {
		notFound := apperror.NewNotFound("user", strconv.Itoa(int(user.ID)))
		_ = c.Error(notFound)
		return
	}

	instance, err := h.instanceService.FindWithParametersById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanReadInstance(userWithGroups, instance)
	if !canRead {
		unauthorized := apperror.NewUnauthorized("read access denied")
		_ = c.Error(unauthorized)
		return
	}

	c.JSON(http.StatusOK, instance)
}

// FindByIdWithDecryptedParameters instance
// swagger:route GET /instances/{id}/parameters findInstanceByIdWithParameters
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
func (h Handler) FindByIdWithDecryptedParameters(c *gin.Context) {
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

	userWithGroups, err := h.userClient.FindUserById(token, user.ID)
	if err != nil {
		notFound := apperror.NewNotFound("user", strconv.Itoa(int(user.ID)))
		_ = c.Error(notFound)
		return
	}

	instance, err := h.instanceService.FindWithDecryptedParametersById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanWriteInstance(userWithGroups, instance)
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

	selector := c.Query("selector")
	// We currently only support streaming of logs from DHIS2 and the database. And we want to make sure logs from any other pods are off limit
	if selector != "" && selector != "data" {
		badRequest := apperror.NewBadRequest("selector can only be empty or \"data\"")
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

	userWithGroups, err := h.userClient.FindUserById(token, user.ID)
	if err != nil {
		notFound := apperror.NewNotFound("user", strconv.Itoa(int(user.ID)))
		_ = c.Error(notFound)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		notFound := apperror.NewNotFound("instance", idParam)
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanReadInstance(userWithGroups, instance)
	if !canRead {
		unauthorized := apperror.NewUnauthorized("read access denied")
		_ = c.Error(unauthorized)
		return
	}

	group, err := h.userClient.FindGroupByName(token, instance.GroupName)
	if err != nil {
		_ = c.Error(err)
	}

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

	token, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userWithGroups, err := h.userClient.FindUserById(token, user.ID)
	if err != nil {
		notFound := apperror.NewNotFound("user", strconv.Itoa(int(user.ID)))
		_ = c.Error(notFound)
		return
	}

	instance, err := h.instanceService.FindByNameAndGroup(instanceName, groupName)
	if err != nil {
		notFound := apperror.NewNotFound("instance", instanceName)
		_ = c.Error(notFound)
		return
	}

	canRead := handler.CanReadInstance(userWithGroups, instance)
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

	token, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userWithGroups, err := h.userClient.FindUserById(token, user.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	groups := userWithGroups.Groups
	instances, err := h.instanceService.FindInstances(groups)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, h.groupsWithInstances(groups, instances))
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

/*
type RunJobResponse struct {
	RunId string `json:"runId"`
}

// Save instance
// swagger:route POST /instances/{id}/save saveInstance
//
// Save instance database
//
// Security:
//  oauth2:
//
// responses:
//   200:
//   401: Error
//   403: Error
//   404: Error
//   415: Error
func (h Handler) Save(c *gin.Context) {
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		badRequest := apperror.NewBadRequest("error parsing id")
		_ = c.Error(badRequest)
		return
	}

	token, err := handler.GetTokenFromHttpAuthHeader(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindById(uint(id))
	if err != nil {
		_ = c.Error(err)
		return
	}

	runJobRequest := &jobModels.RunJobRequest{
		GroupID: uint64(instance.GroupID),
		Payload: map[string]string{
			"key": "val",
		},
		TargetID: id,
	}

	runId, err := h.jobClient.Run(token, uint(3), runJobRequest)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, RunJobResponse{runId})
}
*/
