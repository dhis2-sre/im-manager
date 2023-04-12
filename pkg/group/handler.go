package group

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
)

func NewHandler(
	groupService groupService,
) Handler {
	return Handler{
		groupService,
	}
}

type Handler struct {
	groupService groupService
}

type groupService interface {
	Create(name string, hostname string) (*model.Group, error)
	AddUser(groupName string, userId uint) error
	AddClusterConfiguration(clusterConfiguration *model.ClusterConfiguration) error
	GetClusterConfiguration(groupName string) (*model.ClusterConfiguration, error)
	Find(name string) (*model.Group, error)
	FindOrCreate(name string, hostname string) (*model.Group, error)
}

type CreateGroupRequest struct {
	Name     string `json:"name" binding:"required"`
	Hostname string `json:"hostname" binding:"required"`
}

// Create group
func (h Handler) Create(c *gin.Context) {
	// swagger:route POST /groups groupCreate
	//
	// Create group
	//
	// Create a group...
	//
	// security:
	//   oauth2:
	//
	// responses:
	//   201: Group
	//   400: Error
	//   401: Error
	//   403: Error
	//   415: Error
	var request CreateGroupRequest

	if err := handler.DataBinder(c, &request); err != nil {
		return
	}

	group, err := h.groupService.Create(request.Name, request.Hostname)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, group)
}

// AddUserToGroup group
func (h Handler) AddUserToGroup(c *gin.Context) {
	// swagger:route POST /groups/{group}/users/{userId} addUserToGroup
	//
	// Add user to group
	//
	// Add a user to a group...
	//
	// security:
	//   oauth2:
	//
	// responses:
	//   201: Group
	//   400: Error
	//   401: Error
	//   403: Error
	//   415: Error
	groupName := c.Param("group")
	userIdString := c.Param("userId")

	userId, err := strconv.ParseUint(userIdString, 10, 64)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("failed to parse userId: %s", err))
		return
	}

	err = h.groupService.AddUser(groupName, uint(userId))
	if err != nil {
		if errdef.IsNotFound(err) {
			_ = c.AbortWithError(http.StatusNotFound, err)
		} else {
			_ = c.Error(err)
		}
		return
	}

	c.Status(http.StatusCreated)
}

type CreateClusterConfigurationRequest struct {
	KubernetesConfiguration *multipart.FileHeader `form:"kubernetesConfiguration" binding:"required"`
}

// AddClusterConfiguration group
func (h Handler) AddClusterConfiguration(c *gin.Context) {
	// swagger:route POST /groups/{group}/cluster-configuration addClusterConfigurationToGroup
	//
	// Add cluster configuration to group
	//
	// Add a cluster configuration to a group. This will allow deploying to a remote cluster.
	// Currently only configurations with embedded access tokens are support.
	// The configuration needs to be encrypted using Mozilla Sops. Please see ./scripts/addClusterConfigToGroup.sh for an example of how this can be done.
	//
	// security:
	//   oauth2:
	//
	// responses:
	//   201: Group
	//   401: Error
	//   400: Error
	//   403: Error
	//   415: Error
	var request CreateClusterConfigurationRequest
	if err := handler.DataBinder(c, &request); err != nil {
		return
	}

	groupName := c.Param("group")
	if groupName == "" {
		_ = c.AbortWithError(http.StatusBadRequest, errors.New("group not found"))
		return
	}

	kubernetesConfiguration, err := h.getBytes(request.KubernetesConfiguration)
	if err != nil {
		_ = c.Error(err)
		return
	}

	clusterConfiguration := &model.ClusterConfiguration{
		GroupName:               groupName,
		KubernetesConfiguration: kubernetesConfiguration,
	}

	err = h.groupService.AddClusterConfiguration(clusterConfiguration)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusCreated)
}

func (h Handler) getBytes(file *multipart.FileHeader) ([]byte, error) {
	if file == nil {
		return nil, nil
	}

	openedFile, err := file.Open()
	if err != nil {
		return nil, err
	}

	bytes, err := io.ReadAll(openedFile)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// Find group by name
func (h Handler) Find(c *gin.Context) {
	// swagger:route GET /groups/{name} findGroupByName
	//
	// Find group
	//
	// Find a group by its name
	//
	// responses:
	//   200: Group
	//   401: Error
	//   403: Error
	//   404: Error
	//   415: Error
	//
	// security:
	//   oauth2:
	name := c.Param("name")

	group, err := h.groupService.Find(name)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, group)
}
