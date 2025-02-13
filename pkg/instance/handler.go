package instance

import (
	"bufio"
	"fmt"
	"github.com/dhis2-sre/im-manager/pkg/group"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"io"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
)

func NewHandler(stackService stack.Service, groupService group.Service, instanceService *Service, defaultTTL uint) Handler {
	return Handler{
		stackService:    stackService,
		groupService:    groupService,
		instanceService: instanceService,
		defaultTTL:      defaultTTL,
	}
}

type Handler struct {
	stackService    stack.Service
	groupService    group.Service
	instanceService *Service
	defaultTTL      uint
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

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDecryptedDeploymentById(ctx, id)
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

	err = h.instanceService.DeployDeployment(ctx, token, deployment)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.stripDeploymentSensitiveParameterValues(deployment)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, deployment)
}

func (h Handler) stripDeploymentSensitiveParameterValues(deployment *model.Deployment) error {
	for _, instance := range deployment.Instances {
		err := h.stripInstanceSensitiveParameterValues(instance)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h Handler) stripInstanceSensitiveParameterValues(instance *model.DeploymentInstance) error {
	stack, err := h.stackService.Find(instance.StackName)
	if err != nil {
		return err
	}

	for index, parameter := range instance.Parameters {
		if stack.Parameters[parameter.ParameterName].Sensitive {
			parameter.Value = "!!!redacted!!!"
			instance.Parameters[index] = parameter
		}
	}
	return nil
}

type SaveDeploymentRequest struct {
	Name        string `json:"name" binding:"required,dns_rfc1035_label"`
	Description string `json:"description"`
	Group       string `json:"group" binding:"required"`
	TTL         uint   `json:"ttl"`
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

	ctx := c.Request.Context()
	group, err := h.groupService.Find(ctx, request.Group)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if !group.Deployable {
		forbidden := errdef.NewForbidden("group isn't deployable: %s", group.Name)
		_ = c.Error(forbidden)
		return
	}

	user, err := handler.GetUserFromContext(ctx)
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
		TTL:         request.TTL,
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		unauthorized := errdef.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	err = h.instanceService.SaveDeployment(ctx, deployment)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.stripDeploymentSensitiveParameterValues(deployment)
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

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(ctx, id)
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

	err = h.stripDeploymentSensitiveParameterValues(deployment)
	if err != nil {
		_ = c.Error(err)
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
	Public     bool       `json:"public"`
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

	ctx := c.Request.Context()
	deployment, err := h.instanceService.FindDeploymentById(ctx, deploymentId)
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
		Public:       request.Public,
		Parameters:   parameters,
	}

	err = h.instanceService.SaveInstance(ctx, instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.stripInstanceSensitiveParameterValues(instance)
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

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindDeploymentInstanceById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(ctx, instance.DeploymentID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("write access denied"))
		return
	}

	err = h.instanceService.Pause(ctx, instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// InstanceWithDetails instance
func (h Handler) InstanceWithDetails(c *gin.Context) {
	// swagger:route PUT /instances/{id}/details instanceWithDetails
	//
	// Instance with details
	//
	// Returns the details of an instance including parameters
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

	ctx := c.Request.Context()

	instance, err := h.instanceService.FindDeploymentInstanceById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.stripInstanceSensitiveParameterValues(instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, instance)
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

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindDecryptedDeploymentInstanceById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(ctx, instance.DeploymentID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("write access denied"))
		return
	}

	err = h.instanceService.Reset(ctx, token, instance, deployment.TTL)
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

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindDecryptedDeploymentInstanceById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(ctx, instance.DeploymentID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("write access denied"))
		return
	}

	err = h.instanceService.Resume(ctx, instance)
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

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindDeploymentInstanceById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(ctx, instance.DeploymentID)
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
	err = h.instanceService.Restart(ctx, instance, selector)
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

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(ctx, deploymentId)
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

	err = h.instanceService.DeleteInstance(ctx, deploymentId, instanceId)
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

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindDeploymentInstanceById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(ctx, instance.DeploymentID)
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

	group, err := h.groupService.Find(ctx, instance.GroupName)
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
	//	200: GroupsWithDeployments
	//	401: Error
	//	403: Error
	//	415: Error
	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	groupsWithDeployments, err := h.instanceService.FindDeployments(ctx, user)
	if err != nil {
		_ = c.Error(err)
		return
	}

	for i, group := range groupsWithDeployments {
		for j, deployment := range group.Deployments {
			err := h.stripDeploymentSensitiveParameterValues(deployment)
			if err != nil {
				_ = c.Error(err)
				return
			}
			groupsWithDeployments[i].Deployments[j] = deployment
		}
	}

	c.JSON(http.StatusOK, groupsWithDeployments)
}

// FindPublicInstances list public available instances
func (h Handler) FindPublicInstances(c *gin.Context) {
	// swagger:route GET /deployments/public findPublicInstances
	//
	// Find public deployments
	//
	// Find all public deployments
	//
	// responses:
	//	200: GroupsWithPublicInstances
	//	401: Error
	//	403: Error
	//	415: Error
	groupsWithInstances, err := h.instanceService.FindPublicInstances(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, groupsWithInstances)
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

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDecryptedDeploymentById(ctx, id)
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

	err = h.instanceService.DeleteDeployment(ctx, deployment)
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

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.instanceService.FindDeploymentInstanceById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	deployment, err := h.instanceService.FindDeploymentById(ctx, instance.DeploymentID)
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
