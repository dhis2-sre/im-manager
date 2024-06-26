package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/gin-gonic/gin"
)

func NewHandler(client DockerHubClient, instanceManagerHost, databaseManagerHost string) Handler {
	return Handler{client, instanceManagerHost, databaseManagerHost}
}

type Handler struct {
	dockerHubClient                          DockerHubClient
	instanceManagerHost, databaseManagerHost string
}

type DockerHubClient interface {
	GetImages(organization string) ([]string, error)
	GetTags(organization, repository string) ([]string, error)
}

type Request struct {
	Key     string `json:"key" binding:"required"`
	Payload any    `json:"payload"`
}

// Integrations ...
func (h Handler) Integrations(c *gin.Context) {
	// swagger:route POST /integrations postIntegration
	//
	// Integration
	//
	// Return integration for a given key
	//
	// Security:
	//  oauth2:
	//
	// responses:
	//   200: Response
	//   401: Error
	//   403: Error
	//   415: Error
	var request Request
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	if request.Key == "IMAGE_REPOSITORY" {
		payload := request.Payload.(map[string]any)

		org, ok := payload["organization"]
		if !ok {
			_ = c.Error(fmt.Errorf("payload doesn't contain \"organization\""))
			return
		}

		organization, ok := org.(string)
		if !ok {
			_ = c.Error(fmt.Errorf("\"organization\" must be a string"))
			return
		}

		images, err := h.dockerHubClient.GetImages(organization)
		if err != nil {
			_ = c.Error(err)
			return
		}

		sort.Strings(images)

		c.JSON(http.StatusOK, images)
		return
	}

	if request.Key == "IMAGE_TAG" {
		payload := request.Payload.(map[string]any)

		org, ok := payload["organization"]
		if !ok {
			_ = c.Error(fmt.Errorf("payload doesn't contain \"organization\""))
			return
		}
		organization, ok := org.(string)
		if !ok {
			_ = c.Error(fmt.Errorf("\"organization\" must be a string"))
			return
		}

		rep, ok := payload["repository"]
		if !ok {
			_ = c.Error(fmt.Errorf("payload doesn't contain \"repository\""))
			return
		}
		repository, ok := rep.(string)
		if !ok {
			_ = c.Error(fmt.Errorf("\"repository\" must be a string"))
			return
		}

		tags, err := h.dockerHubClient.GetTags(organization, repository)
		if err != nil {
			_ = c.Error(err)
			return
		}

		sort.Slice(tags, func(i, j int) bool { return tags[i] > tags[j] })

		c.JSON(http.StatusOK, tags)
		return
	}

	if request.Key == "IMAGE_PULL_POLICY" {
		policies := []string{"Always", "IfNotPresent", "Never"}

		c.JSON(http.StatusOK, policies)
		return
	}

	if request.Key == "DATABASE_ID" {
		token, err := handler.GetTokenFromRequest(c)
		if err != nil {
			_ = c.Error(err)
			return
		}

		url := fmt.Sprintf("http://%s/databases", h.databaseManagerHost)
		databases, err := getDatabases(token, url)
		if err != nil {
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusOK, databases)
		return
	}

	if request.Key == "PRESET_ID" {
		token, err := handler.GetTokenFromRequest(c)
		if err != nil {
			_ = c.Error(err)
			return
		}

		url := fmt.Sprintf("http://%s/presets", h.instanceManagerHost)
		presets, err := getInstances(token, url)
		if err != nil {
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusOK, presets)
		return
	}

	if request.Key == "SOURCE_ID" {
		token, err := handler.GetTokenFromRequest(c)
		if err != nil {
			_ = c.Error(err)
			return
		}

		url := fmt.Sprintf("http://%s/instances", h.instanceManagerHost)
		presets, err := getInstances(token, url)
		if err != nil {
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusOK, presets)
		return
	}

	c.Status(http.StatusNotFound)
}

func getDatabases(token string, url string) (map[uint]string, error) {
	b, err := httpGet(token, url)
	if err != nil {
		return nil, err
	}

	var groups []struct {
		Name      string
		Databases []struct {
			ID   uint
			Name string
		}
	}
	err = json.Unmarshal(b, &groups)
	if err != nil {
		return nil, err
	}

	databases := make(map[uint]string)
	for _, group := range groups {
		for _, database := range group.Databases {
			databases[database.ID] = fmt.Sprintf("%s/%s", group.Name, database.Name)
		}
	}

	return databases, nil
}

func getInstances(token string, url string) (map[uint]string, error) {
	b, err := httpGet(token, url)
	if err != nil {
		return nil, err
	}

	var groups []struct {
		Name      string
		Instances []struct {
			ID   uint
			Name string
		}
	}
	err = json.Unmarshal(b, &groups)
	if err != nil {
		return nil, err
	}

	instances := make(map[uint]string)
	for _, group := range groups {
		for _, instance := range group.Instances {
			instances[instance.ID] = fmt.Sprintf("%s/%s", group.Name, instance.Name)
		}
	}

	return instances, nil
}

func httpGet(token string, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := http.Client{}

	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return b, err
}
