package cluster

import (
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/gin-gonic/gin"
)

func NewHandler(clusterService Service) Handler {
	return Handler{clusterService}
}

type Handler struct {
	clusterService Service
}

type CreateClusterRequest struct {
	Name                    string                `json:"name" form:"name" binding:"required"`
	Description             string                `json:"description" form:"description" binding:"required"`
	KubernetesConfiguration *multipart.FileHeader `form:"kubernetesConfiguration"`
}

// Create cluster
func (h Handler) Create(c *gin.Context) {
	// swagger:route POST /clusters clusterCreate
	//
	// Save cluster
	//
	// Save a cluster...
	//
	// security:
	//   oauth2:
	//
	// responses:
	//   201: Cluster
	//   400: Error
	//   401: Error
	//   403: Error
	//   415: Error
	var request CreateClusterRequest

	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	kubernetesConfiguration, err := h.getBytes(request.KubernetesConfiguration)
	if err != nil {
		_ = c.Error(err)
		return
	}

	cluster, err := h.clusterService.Save(c.Request.Context(), request.Name, request.Description, kubernetesConfiguration)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, cluster)
}

type UpdateClusterRequest struct {
	Name                    string                `json:"name"`
	Description             string                `json:"description"`
	KubernetesConfiguration *multipart.FileHeader `form:"kubernetesConfiguration"`
}

// Update cluster
func (h Handler) Update(c *gin.Context) {
	// swagger:route PUT /clusters/{id} clusterUpdate
	//
	// Update cluster
	//
	// Update a cluster...
	//
	// security:
	//   oauth2:
	//
	// responses:
	//   200: Cluster
	//   400: Error
	//   401: Error
	//   403: Error
	//   404: Error
	//   415: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		_ = c.Error(errdef.NewBadRequest("invalid cluster id"))
		return
	}

	var request UpdateClusterRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	kubernetesConfiguration, err := h.getBytes(request.KubernetesConfiguration)
	if err != nil {
		_ = c.Error(err)
		return
	}

	cluster, err := h.clusterService.Update(c.Request.Context(), uint(id), request.Name, request.Description, kubernetesConfiguration)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, cluster)
}

// Delete cluster
func (h Handler) Delete(c *gin.Context) {
	// swagger:route DELETE /clusters/{id} clusterDelete
	//
	// Delete cluster
	//
	// Delete a cluster...
	//
	// security:
	//   oauth2:
	//
	// responses:
	//   204:
	//   400: Error
	//   401: Error
	//   403: Error
	//   404: Error
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		_ = c.Error(errdef.NewBadRequest("invalid cluster id"))
	}

	err = h.clusterService.Delete(c.Request.Context(), uint(id))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

// Find cluster by id
func (h Handler) Find(c *gin.Context) {
	// swagger:route GET /clusters/{id} findClusterById
	//
	// Find cluster
	//
	// Find a cluster by its id
	//
	// responses:
	//   200: Cluster
	//   400: Error
	//   401: Error
	//   403: Error
	//   404: Error
	//
	// security:
	//   oauth2:
	idParam := c.Param("id")
	id, err := strconv.ParseUint(idParam, 10, 32)
	if err != nil {
		_ = c.Error(errdef.NewBadRequest("invalid cluster id"))
	}

	cluster, err := h.clusterService.Find(c.Request.Context(), uint(id))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, cluster)
}

// FindAll find all clusters
func (h Handler) FindAll(c *gin.Context) {
	// swagger:route GET /clusters findAllClusters
	//
	// Find all clusters
	//
	// Find all clusters...
	//
	// responses:
	//   200: Clusters
	//   401: Error
	//
	// security:
	//   oauth2:
	clusters, err := h.clusterService.FindAll(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, clusters)
}

func (h Handler) getBytes(file *multipart.FileHeader) ([]byte, error) {
	if file == nil {
		return nil, nil
	}

	openedFile, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer openedFile.Close()

	bytes, err := io.ReadAll(openedFile)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}
