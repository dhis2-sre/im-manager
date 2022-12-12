package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/config"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	config config.Config
}

func NewHandler(config config.Config) Handler {
	return Handler{config}
}

type Request struct {
	Key     string `json:"key" binding:"required"`
	Payload any    `json:"payload"`
}

// Integrations ...
// swagger:route POST /integrations postIntegration
//
// Return integration for a given key
//
// Security:
//  oauth2:
//
// responses:
//   200: Any
//   401: Error
//   403: Error
//   415: Error
func (h Handler) Integrations(c *gin.Context) {
	var request Request
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	if request.Key == "IMAGE_REPOSITORY" {
		token, err := getDockerHubToken(h.config.DockerHub.Username, h.config.DockerHub.Password)
		if err != nil {
			_ = c.Error(err)
			return
		}

		payload := request.Payload.(map[string]any)
		organization := payload["organization"].(string)
		images, err := getDockerHubImages(token, organization)
		if err != nil {
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusOK, images)
		return
	}

	if request.Key == "IMAGE_TAG" {
		token, err := getDockerHubToken(h.config.DockerHub.Username, h.config.DockerHub.Password)
		if err != nil {
			_ = c.Error(err)
			return
		}

		payload := request.Payload.(map[string]any)
		organization := payload["organization"].(string)
		repository := payload["repository"].(string)
		tags, err := getDockerHubImageTags(token, organization, repository)
		if err != nil {
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusOK, tags)
		return
	}

	if request.Key == "DATABASE_ID" {
		token, err := handler.GetTokenFromHttpAuthHeader(c)
		if err != nil {
			_ = c.Error(err)
			return
		}

		url := fmt.Sprintf("http://%s/databases", h.config.DatabaseManagerService.Host)
		databases, err := getInstanceManagerDatabases(token, url)
		if err != nil {
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusOK, databases)
		return
	}

	if request.Key == "PRESET_ID" {
		token, err := handler.GetTokenFromHttpAuthHeader(c)
		if err != nil {
			_ = c.Error(err)
			return
		}

		url := "https://api.im.tons.test.c.dhis2.org/presets"
		presets, err := getInstances(token, url)
		if err != nil {
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusOK, presets)
		return
	}

	if request.Key == "SOURCE_ID" {
		token, err := handler.GetTokenFromHttpAuthHeader(c)
		if err != nil {
			_ = c.Error(err)
			return
		}

		// TODO: Put urls in config
		url := "https://api.im.tons.test.c.dhis2.org/instances"
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

func getInstanceManagerDatabases(token string, url string) (map[uint]string, error) {
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

func httpGet(token string, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := http.Client{
		Timeout: 30 * time.Second,
	}

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

func getDockerHubImageTags(token string, organization string, repository string) ([]string, error) {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags?page_size=10000", organization, repository)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("JWT %s", token))

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	body := struct {
		Results []struct {
			Name string
		}
	}{}
	err = json.Unmarshal(b, &body)
	if err != nil {
		return nil, err
	}

	tags := make([]string, len(body.Results))
	for i, image := range body.Results {
		tags[i] = image.Name
	}

	return tags, nil
}

func getDockerHubImages(token, organization string) ([]string, error) {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s?page_size=10000", organization)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("JWT %s", token))

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	body := struct {
		Results []struct {
			Name string
		}
	}{}
	err = json.Unmarshal(b, &body)
	if err != nil {
		return nil, err
	}

	images := make([]string, len(body.Results))
	for i, image := range body.Results {
		images[i] = image.Name
	}

	return images, nil
}

func getDockerHubToken(username, password string) (string, error) {
	// NewDockerImageIntegration(request.Key, request.Payload)
	url := "https://hub.docker.com/v2/users/login"
	contentType := "application/json"
	body := struct {
		Username string
		Password string
	}{
		Username: username,
		Password: password,
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(body)
	if err != nil {
		return "", err
	}

	response, err := http.Post(url, contentType, &buf)
	if err != nil {
		return "", err
	}

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	token := struct {
		Token string
	}{}
	if err := json.Unmarshal(b, &token); err != nil {
		return "", err
	}

	return token.Token, nil
}
