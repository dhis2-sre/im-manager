package database

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func NewHandler(logger *slog.Logger, databaseService *service, groupService groupService, instanceService instanceService, stackService stackService) Handler {
	return Handler{
		logger:          logger,
		databaseService: databaseService,
		groupService:    groupService,
		instanceService: instanceService,
		stackService:    stackService,
	}
}

type Handler struct {
	logger          *slog.Logger
	databaseService *service
	groupService    groupService
	instanceService instanceService
	stackService    stackService
}

type instanceService interface {
	FindDecryptedDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error)
	FindDeploymentById(ctx context.Context, id uint) (*model.Deployment, error)
	FilestoreBackup(ctx context.Context, instance *model.DeploymentInstance, name string, database *model.Database) error
}

type stackService interface {
	Find(name string) (*model.Stack, error)
}

// Upload database
func (h Handler) Upload(c *gin.Context) {
	// swagger:route PUT /databases uploadDatabase
	//
	// Upload database
	//
	// Upload database...
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	201: Database
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	groupName := strings.TrimSpace(c.GetHeader("X-Upload-Group"))
	if groupName == "" {
		_ = c.Error(errdef.NewBadRequest("X-Upload-Group header is required"))
		return
	}

	name := strings.TrimSpace(c.GetHeader("X-Upload-Name"))
	if name == "" {
		_ = c.Error(errdef.NewBadRequest("X-Upload-Name header is required"))
		return
	}

	description := strings.TrimSpace(c.GetHeader("X-Upload-Description"))

	contentType := c.GetHeader("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var contentLength int64
	if cl := c.GetHeader("Content-Length"); cl != "" {
		if parsed, err := strconv.ParseInt(cl, 10, 64); err == nil {
			contentLength = parsed
		}
	}

	databaseName := strings.Trim(name, "/")

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	d := model.Database{
		Name:        databaseName,
		Description: description,
		GroupName:   groupName,
		Type:        "database",
		UserID:      user.ID,
	}

	err = h.canAccess(c, &d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	group, err := h.groupService.Find(ctx, d.GroupName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	save, err := h.databaseService.StreamUpload(ctx, d, group, c.Request.Body, contentType, contentLength)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, save)
}

type saveAsRequest struct {
	// Name of the new database
	Name string `json:"name" binding:"required"`
	// Database dump format. Currently plain and custom are support, please see https://www.postgresql.org/docs/current/app-pgdump.html
	Format string `json:"format" binding:"required,oneOf=plain custom"`
	// TODO: Add InstanceId here rather than as path param?
	//	InstanceId uint   `json:"instanceId" binding:"required"`
	// TODO: Allow saving to another group, remember to ensure user is member of the other group
	//	Group  string `json:"group"`
}

// SaveAs database
func (h Handler) SaveAs(c *gin.Context) {
	// swagger:route POST /databases/save-as/{instanceId} saveAsDatabase
	//
	// "Save as" database
	//
	// Save database under a new name
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	201: Database
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	var request saveAsRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	instanceId, ok := handler.GetPathParameter(c, "instanceId")
	if !ok {
		return
	}

	//goland:noinspection GoImportUsedAsName
	ctx := c.Request.Context()
	instance, err := h.instanceService.FindDecryptedDeploymentInstanceById(ctx, instanceId)
	if err != nil {
		_ = c.Error(err)
		return
	}

	//goland:noinspection GoImportUsedAsName
	stack, err := h.stackService.Find(instance.StackName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	parameter := "DATABASE_ID"
	databaseId, exists := instance.Parameters[parameter]
	if !exists {
		_ = c.Error(fmt.Errorf("parameter %q not found", parameter))
	}

	database, err := h.databaseService.FindByIdentifier(ctx, databaseId.Value)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, database)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	savedDatabase, err := h.databaseService.SaveAs(ctx, user.ID, database, instance, stack, request.Name, request.Format, func(ctx context.Context, saved *model.Database) {
		h.logger.InfoContext(ctx, "Save an instances database as", "groupName", saved.GroupName, "databaseName", saved.Name, "instanceName", instance.Name)
	})
	if err != nil {
		_ = c.Error(err)
		return
	}

	// Backup file store
	deployment, err := h.instanceService.FindDeploymentById(ctx, instance.DeploymentID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	coreInstance, err := getInstanceByStack("dhis2-core", deployment.Instances)
	if err != nil {
		if errdef.IsNotFound(err) {
			c.JSON(http.StatusCreated, savedDatabase)
			return
		}
		_ = c.Error(err)
		return
	}

	err = h.instanceService.FilestoreBackup(ctx, coreInstance, savedDatabase.Name, savedDatabase)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, savedDatabase)
}

func getInstanceByStack(stack string, instances []*model.DeploymentInstance) (*model.DeploymentInstance, error) {
	for _, instance := range instances {
		if instance.StackName == stack {
			return instance, nil
		}
	}
	return nil, errdef.NewNotFound("failed to find instance of type %s", stack)
}

// Save database
func (h Handler) Save(c *gin.Context) {
	// swagger:route POST /databases/save/{instanceId} saveDatabase
	//
	// Save database
	//
	// Saving a database won't affect the instances running the database. However, it should be noted that if two unlocked databases are deployed from the same database they can both overwrite it. It's up to the users to ensure this doesn't happen accidentally.
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	202:
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	instanceId, ok := handler.GetPathParameter(c, "instanceId")
	if !ok {
		return
	}

	//goland:noinspection GoImportUsedAsName
	ctx := c.Request.Context()
	instance, err := h.instanceService.FindDecryptedDeploymentInstanceById(ctx, instanceId)
	if err != nil {
		_ = c.Error(err)
		return
	}

	//goland:noinspection GoImportUsedAsName
	stack, err := h.stackService.Find(instance.StackName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	parameter := "DATABASE_ID"
	databaseId, exists := instance.Parameters[parameter]
	if !exists {
		_ = c.Error(fmt.Errorf("parameter %q not found", parameter))
	}

	database, err := h.databaseService.FindByIdentifier(ctx, databaseId.Value)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, database)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.databaseService.Save(ctx, user.ID, database, instance, stack)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

type CopyDatabaseRequest struct {
	Name  string `json:"name" binding:"required"`
	Group string `json:"group" binding:"required"`
}

// Copy database
func (h Handler) Copy(c *gin.Context) {
	// swagger:route POST /databases/{id}/copy copyDatabase
	//
	// Copy database
	//
	// Copy database...
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	202: Database
	//	401: Error
	//	403: Error
	//	415: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	var request CopyDatabaseRequest
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

	d := &model.Database{
		Name:      request.Name,
		GroupName: request.Group,
		Type:      "database",
		UserID:    user.ID,
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	group, err := h.groupService.Find(ctx, d.GroupName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.databaseService.Copy(ctx, id, d, group); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, d)
}

// FindByIdentifier database
func (h Handler) FindByIdentifier(c *gin.Context) {
	// swagger:route GET /databases/{id} findDatabase
	//
	// Find database
	//
	// Find a database by its identifier. The identifier could be either the actual id of the database or the slug associated with it
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	200: Database
	//	400: Error
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	identifier := c.Param("id")
	d, err := h.databaseService.FindByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, d)
}

type LockDatabaseRequest struct {
	InstanceId uint `json:"instanceId" binding:"required"`
}

// Lock database
func (h Handler) Lock(c *gin.Context) {
	// swagger:route POST /databases/{id}/lock lockDatabaseById
	//
	// Lock database
	//
	// Lock database by id...
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	200: Lock
	//	401: Error
	//	403: Error
	//	404: Error
	//	409: Error
	//	415: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	var request LockDatabaseRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	ctx := c.Request.Context()
	d, err := h.databaseService.FindById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	lock, err := h.databaseService.Lock(ctx, id, request.InstanceId, user.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, lock)
}

// Unlock database
func (h Handler) Unlock(c *gin.Context) {
	// swagger:route DELETE /databases/{id}/lock unlockDatabaseById
	//
	// Unlock database
	//
	// Unlock database by id
	//
	// Security:
	//	oauth2:
	//
	// Responses:
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
	d, err := h.databaseService.FindById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if d.Lock == nil {
		c.String(http.StatusNoContent, "database not locked")
		return
	}

	canUnlock := handler.CanUnlock(user, d)
	if !canUnlock {
		forbidden := errdef.NewForbidden("access denied")
		_ = c.Error(forbidden)
		return
	}

	err = h.databaseService.Unlock(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// Download database
func (h Handler) Download(c *gin.Context) {
	// swagger:route GET /databases/{id}/download downloadDatabase
	//
	// Download database
	//
	// Download a database by its identifier. The identifier could be either the actual id of the database or the slug associated with it
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	200: DownloadDatabaseResponse
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	identifier := c.Param("id")
	ctx := c.Request.Context()
	d, err := h.databaseService.FindByIdentifier(ctx, identifier)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	_, file := path.Split(d.Url)
	c.Header("Content-Disposition", "attachment; filename="+file)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Type", "application/octet-stream")

	err = h.databaseService.Download(ctx, d.ID, c.Writer, func(contentLength int64) {
		c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
}

// Delete database
func (h Handler) Delete(c *gin.Context) {
	// swagger:route DELETE /databases/{id} deleteDatabaseById
	//
	// Delete database
	//
	// Delete database by id...
	//
	// Security:
	//	oauth2:
	//
	// Responses:
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
	d, err := h.databaseService.FindById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.databaseService.Delete(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// swagger:model GroupsWithDatabases
type GroupsWithDatabases struct {
	Name      string           `json:"name"`
	Databases []model.Database `json:"databases"`
}

// List databases
func (h Handler) List(c *gin.Context) {
	// swagger:route GET /databases listDatabases
	//
	// List databases
	//
	// List databases...
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	200: []GroupsWithDatabases
	//	401: Error
	//	403: Error
	//	415: Error
	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	databases, err := h.databaseService.List(ctx, user)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, databases)
}

type UpdateDatabaseRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description" binding:"required"`
}

// Update database
func (h Handler) Update(c *gin.Context) {
	// swagger:route PUT /databases/{id} updateDatabaseById
	//
	// Update database
	//
	// Update database by id
	// TODO: Race condition? If two clients request at the same time... Do we need a transaction between find and update
	//
	// Security:
	//   oauth2:
	//
	// Responses:
	//	200: Database
	//	401: Error
	//	403: Error
	//	415: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	var request UpdateDatabaseRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	ctx := c.Request.Context()
	d, err := h.databaseService.FindById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	d.Name = request.Name
	d.Description = request.Description

	err = h.databaseService.Update(ctx, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, d)
}

func (h Handler) canAccess(c *gin.Context, d *model.Database) error {
	user, err := handler.GetUserFromContext(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return err
	}

	can := handler.CanAccess(user, d)
	if !can {
		return errdef.NewForbidden("access denied")
	}

	return nil
}

type CreateExternalDatabaseRequest struct {
	// Expiration time in seconds
	Expiration uint `json:"expiration" binding:"required"`
}

// CreateExternalDownload database
func (h Handler) CreateExternalDownload(c *gin.Context) {
	// swagger:route POST /databases/{id}/external createExternalDownloadDatabase
	//
	// External download link
	//
	// Create link so the database can be downloaded without log in
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	200: CreateExternalDownloadResponse
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	var request CreateExternalDatabaseRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	ctx := c.Request.Context()
	d, err := h.databaseService.FindById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	externalDownload, err := h.databaseService.CreateExternalDownload(ctx, d.ID, request.Expiration)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, externalDownload)
}

// ExternalDownload database
func (h Handler) ExternalDownload(c *gin.Context) {
	// swagger:route GET /databases/external/{uuid} externalDownloadDatabase
	//
	// Externally download database
	//
	// Download a given database without authentication
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	200: DownloadDatabaseResponse
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	uuidParam := c.Param("uuid")
	if uuidParam == "" {
		badRequest := errdef.NewBadRequest("error missing uuid")
		_ = c.Error(badRequest)
		return
	}

	id, err := uuid.Parse(uuidParam)
	if err != nil {
		_ = c.Error(err)
		return
	}

	ctx := c.Request.Context()
	download, err := h.databaseService.FindExternalDownload(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	d, err := h.databaseService.FindById(ctx, download.DatabaseID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	_, file := path.Split(d.Url)
	c.Header("Content-Disposition", "attachment; filename="+file)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Type", "application/octet-stream")

	err = h.databaseService.Download(ctx, d.ID, c.Writer, func(contentLength int64) {
		c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
}
