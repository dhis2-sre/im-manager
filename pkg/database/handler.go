package database

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func New(databaseService Service, userService userService, groupService groupService, instanceService instance.Service, stackService stack.Service) Handler {
	return Handler{
		databaseService,
		userService,
		groupService,
		instanceService,
		stackService,
	}
}

type userService interface {
	FindById(id uint) (*model.User, error)
}

type Handler struct {
	databaseService Service
	userService     userService
	groupService    groupService
	instanceService instance.Service
	stackService    stack.Service
}

type Service interface {
	Copy(id uint, d *model.Database, group *model.Group) error
	FindById(id uint) (*model.Database, error)
	FindByIdentifier(identifier string) (*model.Database, error)
	Lock(id uint, instanceId uint, userId uint) (*model.Lock, error)
	Unlock(id uint) error
	Upload(d *model.Database, group *model.Group, reader ReadAtSeeker, size int64) (*model.Database, error)
	Download(id uint, dst io.Writer, headers func(contentLength int64)) error
	Delete(id uint) error
	List(groups []model.Group) ([]model.Database, error)
	Update(d *model.Database) error
	CreateExternalDownload(databaseID uint, expiration time.Time) (*model.ExternalDownload, error)
	FindExternalDownload(uuid uuid.UUID) (*model.ExternalDownload, error)
	SaveAs(database *model.Database, instance *model.Instance, stack *model.Stack, newName string, format string) (*model.Database, error)
}

// Upload database
func (h Handler) Upload(c *gin.Context) {
	// swagger:route POST /databases uploadDatabase
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
	file, err := c.FormFile("database")
	if err != nil {
		_ = c.Error(err)
		return
	}

	groupName := c.PostForm("group")
	if groupName == "" {
		_ = c.Error(errors.New("group name not found"))
		return
	}

	d := &model.Database{
		Name:      file.Filename,
		GroupName: groupName,
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	group, err := h.groupService.Find(d.GroupName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	f, err := file.Open()
	if err != nil {
		_ = c.Error(err)
		return
	}

	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			_ = c.Error(err)
			return
		}
	}(f)

	header := c.GetHeader("Content-Length")
	contentLength, err := strconv.Atoi(header)
	if err != nil {
		_ = c.Error(err)
		return
	}

	save, err := h.databaseService.Upload(d, group, f, int64(contentLength))
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
	// Save database under a new name. If you want to simple save, you currently have to delete the old one and rename the new one
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

	instance, err := h.instanceService.FindByIdDecrypted(instanceId)
	if err != nil {
		_ = c.Error(err)
		return
	}

	stack, err := h.stackService.Find(instance.StackName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	databaseIdString, err := findParameter("DATABASE_ID", instance, stack)
	if err != nil {
		_ = c.Error(err)
		return
	}

	databaseId, err := strconv.ParseUint(databaseIdString, 10, 32)
	if err != nil {
		badRequest := errdef.NewBadRequest("error parsing databaseId: %s", databaseIdString)
		_ = c.Error(badRequest)
		return
	}

	database, err := h.databaseService.FindById(uint(databaseId))
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, database)
	if err != nil {
		_ = c.Error(err)
		return
	}

	save, err := h.databaseService.SaveAs(database, instance, stack, request.Name, request.Format)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, save)
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

	d := &model.Database{
		Name:      request.Name,
		GroupName: request.Group,
	}

	err := h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	group, err := h.groupService.Find(d.GroupName)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.databaseService.Copy(id, d, group); err != nil {
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
	d, err := h.databaseService.FindByIdentifier(identifier)
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

	_, err := h.instanceService.FindById(request.InstanceId)
	if err != nil {
		_ = c.Error(err)
		return
	}

	d, err := h.databaseService.FindById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	lock, err := h.databaseService.Lock(id, request.InstanceId, user.ID)
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

	d, err := h.databaseService.FindById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user, err := handler.GetUserFromContext(c)
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

	err = h.databaseService.Unlock(id)
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
	d, err := h.databaseService.FindByIdentifier(identifier)
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

	err = h.databaseService.Download(d.ID, c.Writer, func(contentLength int64) {
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

	d, err := h.databaseService.FindById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.databaseService.Delete(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// swagger:model GroupsWithDatabases
type GroupsWithDatabases struct {
	ID        uint
	Name      string
	Hostname  string
	Databases []model.Database
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
	user, err := handler.GetUserFromContext(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	d, err := h.databaseService.List(user.Groups)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, groupsWithDatabases(user.Groups, d))
}

func groupsWithDatabases(groups []model.Group, databases []model.Database) []GroupsWithDatabases {
	groupsWithDatabases := make([]GroupsWithDatabases, len(groups))
	for i, group := range groups {
		groupsWithDatabases[i].Name = group.Name
		groupsWithDatabases[i].Hostname = group.Hostname
		groupsWithDatabases[i].Databases = filterDatabases(databases, func(database *model.Database) bool {
			return database.GroupName == group.Name
		})
	}
	return groupsWithDatabases
}

func filterDatabases(databases []model.Database, test func(database *model.Database) bool) (ret []model.Database) {
	for i := range databases {
		if test(&databases[i]) {
			ret = append(ret, databases[i])
		}
	}
	return
}

type UpdateDatabaseRequest struct {
	Name string `json:"name" binding:"required"`
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

	d, err := h.databaseService.FindById(id)
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

	err = h.databaseService.Update(d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, d)
}

func (h Handler) canAccess(c *gin.Context, d *model.Database) error {
	user, err := handler.GetUserFromContext(c)
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
	Expiration time.Time `json:"expiration" binding:"required"`
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

	d, err := h.databaseService.FindById(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	err = h.canAccess(c, d)
	if err != nil {
		_ = c.Error(err)
		return
	}

	externalDownload, err := h.databaseService.CreateExternalDownload(d.ID, request.Expiration)
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
	// Download database
	//
	// Download database...
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

	download, err := h.databaseService.FindExternalDownload(id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	d, err := h.databaseService.FindById(download.DatabaseID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	_, file := path.Split(d.Url)
	c.Header("Content-Disposition", "attachment; filename="+file)
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Type", "application/octet-stream")

	err = h.databaseService.Download(d.ID, c.Writer, func(contentLength int64) {
		c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	})
	if err != nil {
		_ = c.Error(err)
		return
	}
}
