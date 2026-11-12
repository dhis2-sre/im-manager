package instance

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/dhis2-sre/im-manager/pkg/stack"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func NewHandler(stackService stack.Service, groupService groupServiceHandler, instanceService *Service, deploymentService deploymentService, defaultTTL uint) Handler {
	return Handler{
		stackService:      stackService,
		groupService:      groupService,
		instanceService:   instanceService,
		deploymentService: deploymentService,
		defaultTTL:        defaultTTL,
	}
}

type Handler struct {
	stackService      stack.Service
	groupService      groupServiceHandler
	instanceService   *Service
	deploymentService deploymentService
	defaultTTL        uint
}

type groupServiceHandler interface {
	Find(ctx context.Context, name string) (*model.Group, error)
}

type deploymentService interface {
	StartDeployment(ctx context.Context, token string, deploymentId uint, userID uint) error
	StartDeployments(ctx context.Context, token string, deploymentIds []uint, userID uint) error
	UpdateDeployment(ctx context.Context, token string, deploymentId uint, ttl uint, description string, userID uint) (*model.Deployment, error)
	EditDeployment(ctx context.Context, token string, deploymentId uint, edit Edit, userID uint) (*model.Deployment, error)
	UpdateInstance(ctx context.Context, token string, deploymentId, instanceId uint, parameters Parameters, public *bool) (*model.DeploymentInstance, error)
	Reset(ctx context.Context, token string, deploymentId, instanceId uint, componentName string, ttl uint, userID uint) error
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
	//	202:
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

	err = h.deploymentService.StartDeployment(ctx, token, deployment.ID, user.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
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
			parameter.Value = "***"
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

	if err := kube.Reachable(group.Cluster); err != nil {
		_ = c.Error(errdef.NewBadRequest("group %q can't be deployed to: %v", group.Name, err))
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

// swagger:model DeploymentNameAvailability
type DeploymentNameAvailability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (h Handler) DeploymentNameAvailability(c *gin.Context) {
	// swagger:route GET /deployments/availability deploymentNameAvailability
	//
	// Deployment name availability
	//
	// Check whether a deployment name is free to use in a group.
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: DeploymentNameAvailability
	//	400: Error
	//	401: Error
	//	403: Error
	//	404: Error
	h.nameAvailability(c, false)
}

func (h Handler) PresetNameAvailability(c *gin.Context) {
	// swagger:route GET /presets/availability presetNameAvailability
	//
	// Preset name availability
	//
	// Check whether a preset name is free to use in a group. Presets and deployments have separate names, so a deployment's name does not make it taken.
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: DeploymentNameAvailability
	//	400: Error
	//	401: Error
	//	403: Error
	//	404: Error
	h.nameAvailability(c, true)
}

// nameAvailability answers whether a deployment, or a preset when preset is true, may take the name in the group.
func (h Handler) nameAvailability(c *gin.Context, preset bool) {
	var request struct {
		Group string `form:"group" binding:"required"`
		Name  string `form:"name" binding:"required"`
	}
	if err := c.ShouldBindQuery(&request); err != nil {
		_ = c.Error(errdef.NewBadRequest("group and name query parameters are required"))
		return
	}

	ctx := c.Request.Context()
	group, err := h.groupService.Find(ctx, request.Group)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if !handler.CanWriteDeployment(user, &model.Deployment{UserID: user.ID, GroupName: group.Name}) {
		_ = c.Error(errdef.NewUnauthorized("write access denied"))
		return
	}

	if !group.Deployable {
		_ = c.Error(errdef.NewForbidden("group isn't deployable: %s", group.Name))
		return
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok && v.Var(request.Name, "dns_rfc1035_label") != nil {
		c.JSON(http.StatusOK, DeploymentNameAvailability{Reason: "name must be at most 63 characters of lowercase letters, digits and '-', start with a letter and end with a letter or digit"})
		return
	}

	exists, err := h.instanceService.DeploymentNameExists(ctx, group.Name, request.Name, preset)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if exists {
		c.JSON(http.StatusOK, DeploymentNameAvailability{Reason: fmt.Sprintf("a %s named %q already exists in group %q", deploymentKind(preset), request.Name, group.Name)})
		return
	}

	c.JSON(http.StatusOK, DeploymentNameAvailability{Available: true})
}

// MaxBulkDeployments caps how many deployments one bulk deploy creates.
const MaxBulkDeployments = 50

type BulkDeployRequest struct {
	// Prefix names the deployments prefix-1 to prefix-count
	Prefix      string                `json:"prefix" binding:"required"`
	Count       uint                  `json:"count" binding:"required,min=1,max=50"`
	Group       string                `json:"group" binding:"required"`
	Description string                `json:"description"`
	TTL         uint                  `json:"ttl"`
	Instances   []SaveInstanceRequest `json:"instances" binding:"required,min=1,dive"`
}

func bulkDeploymentNames(prefix string, count uint) []string {
	names := []string{}
	for i := uint(1); i <= count && i <= MaxBulkDeployments; i++ {
		names = append(names, fmt.Sprintf("%s-%d", prefix, i))
	}
	return names
}

func invalidBulkNamesReason(prefix string, count uint) string {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return ""
	}
	for _, name := range bulkDeploymentNames(prefix, count) {
		if v.Var(name, "dns_rfc1035_label") != nil {
			return fmt.Sprintf("%q isn't a valid name: names must be at most 63 characters of lowercase letters, digits and '-' and start with a letter", name)
		}
	}
	return ""
}

func (h Handler) BulkDeploy(c *gin.Context) {
	// swagger:route POST /deployments/bulk bulkDeploy
	//
	// Bulk deploy
	//
	// Create count deployments named prefix-1 to prefix-count with the same instances and deploy them, a few at a time. Either every deployment is created or none is: if any name is taken the request fails with 409 and creates nothing.
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	202: BulkDeployResponse
	//	400: Error
	//	401: Error
	//	403: Error
	//	404: Error
	//	409: Error
	//	415: Error
	var request BulkDeployRequest
	if err := handler.DataBinder(c, &request); err != nil {
		if !errdef.IsBadRequest(err) {
			err = errdef.NewBadRequest("prefix, group, a count between 1 and %d and at least one instance are required: %v", MaxBulkDeployments, err)
		}
		_ = c.Error(err)
		return
	}

	if reason := invalidBulkNamesReason(request.Prefix, request.Count); reason != "" {
		_ = c.Error(errdef.NewBadRequest("%s", reason))
		return
	}

	if err := rejectRepeatedStacks(request.Instances); err != nil {
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
		_ = c.Error(errdef.NewForbidden("group isn't deployable: %s", group.Name))
		return
	}

	if err := kube.Reachable(group.Cluster); err != nil {
		_ = c.Error(errdef.NewBadRequest("group %q can't be deployed to: %v", group.Name, err))
		return
	}

	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if !handler.CanWriteDeployment(user, &model.Deployment{UserID: user.ID, GroupName: group.Name}) {
		_ = c.Error(errdef.NewUnauthorized("write access denied"))
		return
	}

	token, err := handler.GetTokenFromRequest(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if request.TTL == 0 {
		request.TTL = h.defaultTTL
	}

	names := bulkDeploymentNames(request.Prefix, request.Count)
	deployments := make([]*model.Deployment, len(names))
	ids := make([]uint, len(names))
	for i, name := range names {
		deployments[i] = &model.Deployment{
			UserID:      user.ID,
			Name:        name,
			Description: request.Description,
			GroupName:   group.Name,
			TTL:         request.TTL,
			Instances:   newInstances(name, group, request.Instances, model.DeployStatusPending),
		}
	}

	err = h.instanceService.SaveDeployments(ctx, group.Name, deployments)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response := BulkDeployResponse{Deployments: make([]BulkDeployment, len(deployments))}
	for i, deployment := range deployments {
		ids[i] = deployment.ID
		response.Deployments[i] = BulkDeployment{ID: deployment.ID, Name: deployment.Name}
	}

	err = h.deploymentService.StartDeployments(ctx, token, ids, user.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusAccepted, response)
}

// bindRequest binds the request body and reports a body that fails validation as a bad request rather than as an internal error.
func bindRequest(c *gin.Context, request any) error {
	err := handler.DataBinder(c, request)
	if err != nil && !errdef.IsBadRequest(err) {
		return errdef.NewBadRequest("invalid request body: %v", err)
	}
	return err
}

func rejectRepeatedStacks(requests []SaveInstanceRequest) error {
	stackNames := map[string]bool{}
	for _, request := range requests {
		if stackNames[request.StackName] {
			return errdef.NewBadRequest("stack %q is listed more than once", request.StackName)
		}
		stackNames[request.StackName] = true
	}
	return nil
}

func newInstances(name string, group *model.Group, requests []SaveInstanceRequest, status model.DeployStatus) []*model.DeploymentInstance {
	instances := make([]*model.DeploymentInstance, len(requests))
	for i, request := range requests {
		parameters := make(model.DeploymentInstanceParameters, len(request.Parameters))
		for parameterName, parameter := range request.Parameters {
			parameters[parameterName] = model.DeploymentInstanceParameter{ParameterName: parameterName, Value: parameter.Value}
		}
		instances[i] = &model.DeploymentInstance{
			Name:         name,
			Group:        group,
			GroupName:    group.Name,
			StackName:    request.StackName,
			Public:       request.Public,
			Parameters:   parameters,
			DeployStatus: status,
		}
	}
	return instances
}

// swagger:model BulkDeployResponse
type BulkDeployResponse struct {
	Deployments []BulkDeployment `json:"deployments"`
}

type BulkDeployment struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// swagger:model BulkDeploymentNameAvailability
type BulkDeploymentNameAvailability struct {
	Names  []string `json:"names"`
	Taken  []string `json:"taken"`
	Reason string   `json:"reason,omitempty"`
}

func (h Handler) BulkDeploymentNameAvailability(c *gin.Context) {
	// swagger:route GET /deployments/bulk/availability bulkDeploymentNameAvailability
	//
	// Bulk deployment name availability
	//
	// List the names a bulk deploy with the given prefix and count would create and which of them are taken. The bulk deploy checks again when it creates them.
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: BulkDeploymentNameAvailability
	//	400: Error
	//	401: Error
	//	403: Error
	//	404: Error
	var request struct {
		Group  string `form:"group" binding:"required"`
		Prefix string `form:"prefix" binding:"required"`
		Count  uint   `form:"count" binding:"required,min=1,max=50"`
	}
	if err := c.ShouldBindQuery(&request); err != nil {
		_ = c.Error(errdef.NewBadRequest("group, prefix and a count between 1 and %d are required", MaxBulkDeployments))
		return
	}

	ctx := c.Request.Context()
	group, err := h.groupService.Find(ctx, request.Group)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if !handler.CanWriteDeployment(user, &model.Deployment{UserID: user.ID, GroupName: group.Name}) {
		_ = c.Error(errdef.NewUnauthorized("write access denied"))
		return
	}

	if !group.Deployable {
		_ = c.Error(errdef.NewForbidden("group isn't deployable: %s", group.Name))
		return
	}

	names := bulkDeploymentNames(request.Prefix, request.Count)
	if reason := invalidBulkNamesReason(request.Prefix, request.Count); reason != "" {
		c.JSON(http.StatusOK, BulkDeploymentNameAvailability{Names: names, Taken: []string{}, Reason: reason})
		return
	}

	taken, err := h.instanceService.TakenDeploymentNames(ctx, group.Name, names)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, BulkDeploymentNameAvailability{Names: names, Taken: taken})
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

type Parameter struct {
	Value string `json:"value"`
}

type Parameters map[string]Parameter

type SaveInstanceRequest struct {
	StackName  string     `json:"stackName"`
	Parameters Parameters `json:"parameters"`
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

	if canWrite := handler.CanWriteDeployment(user, deployment); !canWrite {
		_ = c.AbortWithError(http.StatusUnauthorized, fmt.Errorf("write access denied"))
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
	// Resetting an instance will completely destroy it and redeploy using the same parameters. With a
	// selector only that component's data is reset, the database of a dhis2-v2 instance for example:
	// the redeploy seeds it again while the other components keep their data.
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

	err = h.deploymentService.Reset(ctx, token, deployment.ID, instance.ID, c.Query("selector"), deployment.TTL, user.ID)
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
	replica := c.Query("replica")
	err = h.instanceService.Restart(ctx, instance, selector, replica)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// DeploymentComponents deployment components
func (h Handler) DeploymentComponents(c *gin.Context) {
	// swagger:route GET /deployments/{id}/components deploymentComponents
	//
	// Deployment components
	//
	// List the components of every instance in a deployment along with their supported operations and live replicas
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: DeploymentComponents
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
		_ = c.Error(errdef.NewUnauthorized("read access denied"))
		return
	}

	components, err := h.instanceService.DeploymentComponents(ctx, deployment)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, components)
}

// Components instance components
func (h Handler) Components(c *gin.Context) {
	// swagger:route GET /instances/{id}/components instanceComponents
	//
	// Instance components
	//
	// List the components of an instance along with their supported operations and live replicas
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: Components
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
		_ = c.Error(errdef.NewUnauthorized("read access denied"))
		return
	}

	components, err := h.instanceService.Components(ctx, instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, components)
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

// logTailLines reads the "tail" query parameter: absent means the default amount of the log, 0
// means all of it, and anything else is that many lines from the end.
func logTailLines(tail string) (*int64, error) {
	if tail == "" {
		lines := DefaultLogTailLines
		return &lines, nil
	}

	lines, err := strconv.ParseInt(tail, 10, 64)
	if err != nil || lines < 0 {
		return nil, errdef.NewBadRequest("tail must be a positive number of lines, or 0 for the whole log")
	}
	if lines == 0 {
		return nil, nil
	}

	return &lines, nil
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
	replica := c.Query("replica")
	container := c.Query("container")

	tailLines, err := logTailLines(c.Query("tail"))
	if err != nil {
		_ = c.Error(err)
		return
	}

	r, err := h.instanceService.Logs(ctx, instance, group, selector, replica, container, tailLines)
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

func (h Handler) FindPresets(c *gin.Context) {
	// swagger:route GET /presets listPresets
	//
	// Find presets
	//
	// Find the presets in the groups the user belongs to. A preset is a deployment that is saved but never deployed, kept to start new deployments from.
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: Presets
	//	401: Error
	//	403: Error
	//	415: Error
	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	presets, err := h.instanceService.FindPresets(ctx, user)
	if err != nil {
		_ = c.Error(err)
		return
	}

	for _, preset := range presets {
		err := h.stripDeploymentSensitiveParameterValues(preset)
		if err != nil {
			_ = c.Error(err)
			return
		}
	}

	c.JSON(http.StatusOK, presets)
}

type SavePresetRequest struct {
	Name        string                `json:"name" binding:"required,dns_rfc1035_label"`
	Description string                `json:"description"`
	Group       string                `json:"group" binding:"required"`
	TTL         uint                  `json:"ttl"`
	Instances   []SaveInstanceRequest `json:"instances" binding:"required,min=1,dive"`
}

func (h Handler) SavePreset(c *gin.Context) {
	// swagger:route POST /presets savePreset
	//
	// Save a preset
	//
	// Save a preset with all its instances in one request. Either the preset is created with every instance or nothing is.
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	201: Deployment
	//	400: Error
	//	401: Error
	//	403: Error
	//	404: Error
	//	409: Error
	//	415: Error
	var request SavePresetRequest
	if err := bindRequest(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	if err := rejectRepeatedStacks(request.Instances); err != nil {
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
		_ = c.Error(errdef.NewForbidden("group isn't deployable: %s", group.Name))
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

	preset := &model.Deployment{
		UserID:      user.ID,
		Name:        request.Name,
		Description: request.Description,
		GroupName:   group.Name,
		TTL:         request.TTL,
		Preset:      true,
		Instances:   newInstances(request.Name, group, request.Instances, ""),
	}

	if !handler.CanWriteDeployment(user, preset) {
		_ = c.Error(errdef.NewUnauthorized("write access denied"))
		return
	}

	if err := h.instanceService.SavePreset(ctx, preset); err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.stripDeploymentSensitiveParameterValues(preset); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, preset)
}

// findAuthorizedPreset finds the preset named by the id path parameter that the user may access, and reports a failure on the context.
func (h Handler) findAuthorizedPreset(c *gin.Context, mayAccess func(*model.User, *model.Deployment) bool) (*model.Deployment, *model.User, bool) {
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return nil, nil, false
	}

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return nil, nil, false
	}

	preset, err := h.instanceService.FindPresetById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return nil, nil, false
	}

	if !mayAccess(user, preset) {
		_ = c.Error(errdef.NewUnauthorized("access to preset %d denied", id))
		return nil, nil, false
	}

	return preset, user, true
}

func (h Handler) FindPresetById(c *gin.Context) {
	// swagger:route GET /presets/{id} findPresetById
	//
	// Find a preset
	//
	// Find a preset by id
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
	preset, _, ok := h.findAuthorizedPreset(c, handler.CanReadDeployment)
	if !ok {
		return
	}

	if err := h.stripDeploymentSensitiveParameterValues(preset); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, preset)
}

// EditPresetRequest is a deployment edit that can also rename the preset and move it to another group.
//
// swagger:model EditPresetRequest
type EditPresetRequest struct {
	Name *string `json:"name" binding:"omitempty,dns_rfc1035_label"`
	// Group moves the preset to the named group
	Group       *string                        `json:"group"`
	Description *string                        `json:"description"`
	TTL         *uint                          `json:"ttl"`
	Instances   map[string]EditInstanceRequest `json:"instances"`
}

func (h Handler) EditPreset(c *gin.Context) {
	// swagger:route PATCH /presets/{id} editPreset
	//
	// Edit a preset
	//
	// Edit a preset and everything in it in one request, including its name and group. Like a deployment edit, whatever the request leaves out stays unchanged, so masked sensitive values are kept by not sending them.
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	200: Deployment
	//	400: Error
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	preset, user, ok := h.findAuthorizedPreset(c, handler.CanWriteDeployment)
	if !ok {
		return
	}

	var request EditPresetRequest
	if err := bindRequest(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	edit := PresetEdit{
		Edit: Edit{Description: request.Description, TTL: request.TTL, Instances: instanceEdits(request.Instances)},
		Name: request.Name,
	}

	ctx := c.Request.Context()
	if request.Group != nil && *request.Group != preset.GroupName {
		group, err := h.groupService.Find(ctx, *request.Group)
		if err != nil {
			_ = c.Error(err)
			return
		}

		if !group.Deployable {
			_ = c.Error(errdef.NewForbidden("group isn't deployable: %s", group.Name))
			return
		}

		if !handler.CanWriteDeployment(user, &model.Deployment{UserID: user.ID, GroupName: group.Name}) {
			_ = c.Error(errdef.NewUnauthorized("write access denied"))
			return
		}

		edit.Group = group
	}

	edited, err := h.instanceService.EditPreset(ctx, preset.ID, edit)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.stripDeploymentSensitiveParameterValues(edited); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, edited)
}

func (h Handler) DeletePreset(c *gin.Context) {
	// swagger:route DELETE /presets/{id} deletePreset
	//
	// Delete a preset
	//
	// Delete a preset by id
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	204:
	//	401: Error
	//	403: Error
	//	404: Error
	//	409: Error
	//	415: Error
	preset, _, ok := h.findAuthorizedPreset(c, handler.CanWriteDeployment)
	if !ok {
		return
	}

	if err := h.instanceService.DeleteDeployment(c.Request.Context(), preset); err != nil {
		_ = c.Error(fmt.Errorf("unable to delete preset: %w", err))
		return
	}

	c.Status(http.StatusNoContent)
}

type CreateDeploymentFromPresetRequest struct {
	Name string `json:"name" binding:"required,dns_rfc1035_label"`
	// Group defaults to the group of the preset
	Group       string  `json:"group"`
	Description *string `json:"description"`
	TTL         *uint   `json:"ttl"`
	// Instances change the preset's instances the way an edit does, keyed by stack name
	Instances map[string]EditInstanceRequest `json:"instances"`
}

func (h Handler) CreateDeploymentFromPreset(c *gin.Context) {
	// swagger:route POST /presets/{id}/deployments createDeploymentFromPreset
	//
	// Create a deployment from a preset
	//
	// Create a deployment holding the preset's instances, including the sensitive values a client only sees masked. Anything the request leaves out comes from the preset. The deployment is created, not deployed.
	//
	// Security:
	//	oauth2:
	//
	// responses:
	//	201: Deployment
	//	400: Error
	//	401: Error
	//	403: Error
	//	404: Error
	//	409: Error
	//	415: Error
	preset, user, ok := h.findAuthorizedPreset(c, handler.CanReadDeployment)
	if !ok {
		return
	}

	var request CreateDeploymentFromPresetRequest
	if err := bindRequest(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	if request.Group == "" {
		request.Group = preset.GroupName
	}

	if request.TTL != nil && *request.TTL == 0 {
		request.TTL = &h.defaultTTL
	}

	ctx := c.Request.Context()
	group, err := h.groupService.Find(ctx, request.Group)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if !group.Deployable {
		_ = c.Error(errdef.NewForbidden("group isn't deployable: %s", group.Name))
		return
	}

	if !handler.CanWriteDeployment(user, &model.Deployment{UserID: user.ID, GroupName: group.Name}) {
		_ = c.Error(errdef.NewUnauthorized("write access denied"))
		return
	}

	deployment, err := h.instanceService.CreateDeploymentFromPreset(ctx, preset.ID, DeploymentFromPreset{
		UserID:      user.ID,
		Name:        request.Name,
		Group:       group,
		Description: request.Description,
		TTL:         request.TTL,
		Instances:   instanceEdits(request.Instances),
	})
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.stripDeploymentSensitiveParameterValues(deployment); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, deployment)
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
		_ = c.Error(fmt.Errorf("unable to delete deployment: %w", err))
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

	status, err := h.instanceService.GetStatus(ctx, instance)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, status)
}

type UpdateInstanceRequest struct {
	Parameters Parameters `json:"parameters"`
	Public     *bool      `json:"public"`
}

// UpdateInstance partially updates an existing deployment instance. Only
// parameters present in the request body are modified; all others are
// preserved. Public is only changed when provided.
func (h Handler) UpdateInstance(c *gin.Context) {
	// swagger:route PATCH /deployments/{id}/instance/{instanceId} updateInstance
	//
	// Update a Deployment Instance
	//
	// Update a Deployment Instance ...
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
	deploymentId, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	instanceId, ok := handler.GetPathParameter(c, "instanceId")
	if !ok {
		return
	}

	var request UpdateInstanceRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
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

	token, err := handler.GetTokenFromRequest(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	instance, err := h.deploymentService.UpdateInstance(ctx, token, deploymentId, instanceId, request.Parameters, request.Public)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, instance)
}

// EditDeploymentRequest is a diff against the deployment, not a snapshot of it. An absent field, an
// absent stack and an absent parameter all mean unchanged, which is the only contract a form can
// honour when reads answer with *** in place of every sensitive value.
//
// swagger:model EditDeploymentRequest
type EditDeploymentRequest struct {
	Description *string `json:"description"`
	TTL         *uint   `json:"ttl"`
	// Instances are keyed by stack name, the key a deployment's instances are unique by and the only
	// one that can name a companion that does not exist yet.
	Instances map[string]EditInstanceRequest `json:"instances"`
}

func (request EditDeploymentRequest) edit() Edit {
	return Edit{Description: request.Description, TTL: request.TTL, Instances: instanceEdits(request.Instances)}
}

// swagger:model EditInstanceRequest
type EditInstanceRequest struct {
	Parameters Parameters `json:"parameters"`
	Public     *bool      `json:"public"`
}

func instanceEdits(requests map[string]EditInstanceRequest) map[string]InstanceEdit {
	if len(requests) == 0 {
		return nil
	}
	edits := make(map[string]InstanceEdit, len(requests))
	for stackName, request := range requests {
		edits[stackName] = InstanceEdit(request)
	}
	return edits
}

// EditDeployment edits a deployment
func (h Handler) EditDeployment(c *gin.Context) {
	// swagger:route PATCH /deployments/{id} editDeployment
	//
	// Edit a deployment
	//
	// Edit a deployment and everything in it in one request. Whatever the edit changes is redeployed
	// in the background; the response carries the deployment as it now stands.
	//
	// Security:
	//   oauth2:
	//
	// Responses:
	//   202: Deployment
	//   400: Error
	//   401: Error
	//   403: Error
	//   404: Error
	//   409: Error
	//   415: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	var request EditDeploymentRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
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

	deployment, err := h.instanceService.FindDeploymentById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if canWrite := handler.CanWriteDeployment(user, deployment); !canWrite {
		_ = c.Error(errdef.NewUnauthorized("write access denied"))
		return
	}

	edited, err := h.deploymentService.EditDeployment(ctx, token, id, request.edit(), user.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.stripDeploymentSensitiveParameterValues(edited); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusAccepted, edited)
}

type UpdateDeploymentRequest struct {
	TTL         uint   `json:"ttl"`
	Description string `json:"description"`
}

// UpdateDeployment updates an existing Deployment's TTL and description
func (h Handler) UpdateDeployment(c *gin.Context) {
	// swagger:route PUT /deployments/{id} updateDeployment
	//
	// Update a Deployment
	//
	// Update a Deployment ...
	//
	// Security:
	//   oauth2:
	//
	// Responses:
	//   200: Deployment
	//   400: Error
	//   401: Error
	//   403: Error
	//   404: Error
	//   500: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	var request UpdateDeploymentRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
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

	deployment, err := h.instanceService.FindDeploymentById(ctx, id)
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

	updatedDeployment, err := h.deploymentService.UpdateDeployment(ctx, token, id, request.TTL, request.Description, user.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.stripDeploymentSensitiveParameterValues(updatedDeployment)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, updatedDeployment)
}
