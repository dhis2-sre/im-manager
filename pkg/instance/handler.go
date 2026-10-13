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
	Reset(ctx context.Context, token string, deploymentId, instanceId uint, ttl uint, userID uint) error
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
	Preset      bool   `json:"preset"`
	// Overwrite replaces a preset of the same name instead of refusing it. It only applies to presets.
	Overwrite bool `json:"overwrite"`
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
		Preset:      request.Preset,
	}

	canWrite := handler.CanWriteDeployment(user, deployment)
	if !canWrite {
		unauthorized := errdef.NewUnauthorized("write access denied")
		_ = c.Error(unauthorized)
		return
	}

	if request.Overwrite && !request.Preset {
		_ = c.Error(errdef.NewBadRequest("overwrite only applies to presets"))
		return
	}

	err = h.saveDeployment(ctx, user, deployment, request.Overwrite)
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

func (h Handler) saveDeployment(ctx context.Context, user *model.User, deployment *model.Deployment, overwrite bool) error {
	if !overwrite {
		return h.instanceService.SaveDeployment(ctx, deployment)
	}

	existing, err := h.instanceService.FindPreset(ctx, deployment.GroupName, deployment.Name)
	if errdef.IsNotFound(err) {
		return h.instanceService.SaveDeployment(ctx, deployment)
	}
	if err != nil {
		return err
	}

	if !handler.CanWriteDeployment(user, existing) {
		return errdef.NewUnauthorized("write access to preset %q denied", existing.Name)
	}

	return h.instanceService.ReplacePreset(ctx, existing.ID, deployment)
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
	// Check whether a deployment name is free to use in a group. Presets have their own names, so pass preset=true to check the name of a preset.
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
	var request struct {
		Group  string `form:"group" binding:"required"`
		Name   string `form:"name" binding:"required"`
		Preset bool   `form:"preset"`
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

	exists, err := h.instanceService.DeploymentNameExists(ctx, group.Name, request.Name, request.Preset)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if exists {
		c.JSON(http.StatusOK, DeploymentNameAvailability{Reason: fmt.Sprintf("a %s named %q already exists in group %q", deploymentKind(request.Preset), request.Name, group.Name)})
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

	stackNames := map[string]bool{}
	for _, instance := range request.Instances {
		if stackNames[instance.StackName] {
			_ = c.Error(errdef.NewBadRequest("stack %q is listed more than once", instance.StackName))
			return
		}
		stackNames[instance.StackName] = true
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
		deployment := &model.Deployment{
			UserID:      user.ID,
			Name:        name,
			Description: request.Description,
			GroupName:   group.Name,
			TTL:         request.TTL,
		}
		for _, instance := range request.Instances {
			parameters := make(model.DeploymentInstanceParameters, len(instance.Parameters))
			for parameterName, parameter := range instance.Parameters {
				parameters[parameterName] = model.DeploymentInstanceParameter{ParameterName: parameterName, Value: parameter.Value}
			}
			deployment.Instances = append(deployment.Instances, &model.DeploymentInstance{
				Name:         name,
				Group:        group,
				GroupName:    group.Name,
				StackName:    instance.StackName,
				Public:       instance.Public,
				Parameters:   parameters,
				DeployStatus: model.DeployStatusPending,
			})
		}
		deployments[i] = deployment
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

	err = h.deploymentService.Reset(ctx, token, deployment.ID, instance.ID, deployment.TTL, user.ID)
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
	// swagger:route GET /deployments/presets listPresets
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

// swagger:model EditInstanceRequest
type EditInstanceRequest struct {
	Parameters Parameters `json:"parameters"`
	Public     *bool      `json:"public"`
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

	edit := Edit{Description: request.Description, TTL: request.TTL}
	if len(request.Instances) > 0 {
		edit.Instances = make(map[string]InstanceEdit, len(request.Instances))
		for stackName, instanceRequest := range request.Instances {
			edit.Instances[stackName] = InstanceEdit(instanceRequest)
		}
	}

	edited, err := h.deploymentService.EditDeployment(ctx, token, id, edit, user.ID)
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
