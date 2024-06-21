package instance

import (
	"bufio"
	"fmt"
	"io"
	"net/http"

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
	Public      bool   `json:"public"`
	TTL         uint   `json:"ttl"`
}

func (h Handler) DeployDeployment(c *gin.Context) {
	// swagger:route POST /deployments/{id}/deploy deployDeployment
	//
	// Deploy a deployment
	//
	// Deploy a deployment...
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
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDecryptedDeploymentById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		unauthorized := errdef.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	if len(deployment.Instances) == 0 {
		badRequest := errdef.NewBadRequest("deployment contains no instances")
		_ = c.Error(badRequest)
		return
	}

	token, err := handler.GetTokenFromRequest(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.instanceService.DeployDeployment(token, deployment)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, deployment)
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

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if request.TTL == 0 {
		request.TTL = h.defaultTTL
	}

	deployment := &model.Deployment{
		UserID:      user.ID,
		Name:        request.Name,
		Description: request.Description,
		GroupName:   request.Group,
		Public:      request.Public,
		TTL:         request.TTL,
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		unauthorized := errdef.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

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

type parameter struct {
	Value string `json:"value"`
}

type parameters map[string]parameter

type SaveInstanceRequest struct {
	StackName  string     `json:"stackName"`
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

	deployment, err := h.instanceService.FindDeploymentById(deploymentId)
	if err != nil {
		_ = c.Error(err)
		return
	}

	parameters := make(model.DeploymentInstanceParameters, len(request.Parameters))
	for name, parameter := range request.Parameters {
		parameters[name] = model.DeploymentInstanceParameter{
			ParameterName: name,
			Value:         parameter.Value,
		}
	}

	instance := &model.DeploymentInstance{
		DeploymentID: deploymentId,
		Name:         deployment.Name,
		Group:        deployment.Group,
		GroupName:    deployment.GroupName,
		StackName:    request.StackName,
		Parameters:   parameters,
	}

	err = h.instanceService.SaveInstance(instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, instance)
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

	instance, err := h.instanceService.FindDeploymentInstanceById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(instance.DeploymentID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
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

	token, err := handler.GetTokenFromRequest(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindDecryptedDeploymentInstanceById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(instance.DeploymentID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("write access denied"))
		return
	}

	err = h.instanceService.Reset(token, instance, deployment.TTL)
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

	instance, err := h.instanceService.FindDecryptedDeploymentInstanceById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(instance.DeploymentID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
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

	instance, err := h.instanceService.FindDeploymentInstanceById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(instance.DeploymentID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
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

// DeleteDeploymentInstance delete deployment instance by id
func (h Handler) DeleteDeploymentInstance(c *gin.Context) {
	// swagger:route DELETE /deployments/{id}/instance/{instanceId} deleteDeploymentInstance
	//
	// Delete deployment instance
	//
	// Delete a deployment instance by id
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
	deploymentId, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	instanceId, ok := handler.GetPathParameter(c, "instanceId")
	if !ok {
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(deploymentId)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		unauthorized := errdef.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	err = h.instanceService.DeleteInstance(deploymentId, instanceId)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
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

	instance, err := h.instanceService.FindDeploymentInstanceById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(instance.DeploymentID)
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

// FindDeployments deployments
func (h Handler) FindDeployments(c *gin.Context) {
	// swagger:route GET /deployments listDeployments
	//
	// Find deployments
	//
	// Find all deployments accessible by the user
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: []GroupsWithDeployments
	//	401: Error
	//	403: Error
	//	415: Error
	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	groupsWithDeployments, err := h.instanceService.FindDeployments(user)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, groupsWithDeployments)
}

// FindPublicDeployments publicDeployments
func (h Handler) FindPublicDeployments(c *gin.Context) {
	// swagger:route GET /deployments/public listPublicDeployments
	//
	// Find public deployments
	//
	// Find all public deployments
	//
	// responses:
	//	200: []GroupsWithDeployments
	//	401: Error
	//	403: Error
	//	415: Error
	groupsWithDeployments, err := h.instanceService.FindPublicDeployments()
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, groupsWithDeployments)
}

// DeleteDeployment deployment by id
func (h Handler) DeleteDeployment(c *gin.Context) {
	// swagger:route DELETE /deployments/{id} deleteDeployment
	//
	// Delete deployment
	//
	// Delete an deployment by id
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

	deployment, err := h.instanceService.FindDeploymentById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		unauthorized := errdef.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	err = h.instanceService.DeleteDeployment(deployment)
	if err != nil {
		_ = c.Error(fmt.Errorf("unable to delete deployment: %v", err))
		return
	}

	c.Status(http.StatusAccepted)
}

// Status returns the status of an instance
func (h Handler) Status(c *gin.Context) {
	// swagger:route GET /instances/{id}/status status
	//
	// Get instance status
	//
	// Get instance status...
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: Status
	//	401: Error
	//	403: Error
	//	404: Error
	//	409: Error
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

	instance, err := h.instanceService.FindDeploymentInstanceById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(instance.DeploymentID)
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

	status, err := h.instanceService.GetStatus(instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, status)
}
