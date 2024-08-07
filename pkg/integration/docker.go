package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/errdef"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewDockerHubClient(username, password string) *dockerHubClient {
	return &dockerHubClient{
		username: username,
		password: password,
		client:   &http.Client{},
	}
}

type dockerHubClient struct {
	username, password string
	client             *http.Client
}

type dockerHubResponse struct {
	Results []struct {
		Name string
	}
}

func (d *dockerHubClient) GetTags(organization string, repository string) ([]string, error) {
	token, err := d.getToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags?page_size=10000", organization, repository)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("JWT %s", token))

	response, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	body := dockerHubResponse{}
	err = json.Unmarshal(b, &body)
	if err != nil {
		return nil, err
	}

	return mapResponseToNames(body), nil
}

func (d *dockerHubClient) ImageExists(organization, repository, tag string) error {
	token, err := d.getToken()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags/%s", organization, repository, tag)
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("JWT %s", token))

	response, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		return nil
	}

	if response.StatusCode == http.StatusNotFound {
		return errdef.NewNotFound("%s/%s:%s is not found", organization, repository, tag)
	}

	return fmt.Errorf("unexpected status code %d", response.StatusCode)
}

func mapResponseToNames(response dockerHubResponse) []string {
	names := make([]string, len(response.Results))
	for i, result := range response.Results {
		names[i] = result.Name
	}
	return names
}

func (d *dockerHubClient) GetImages(organization string) ([]string, error) {
	token, err := d.getToken()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s?page_size=10000", organization)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("JWT %s", token))

	response, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	body := dockerHubResponse{}
	err = json.Unmarshal(b, &body)
	if err != nil {
		return nil, err
	}

	return mapResponseToNames(body), nil
}

func (d *dockerHubClient) getToken() (string, error) {
	body := struct {
		Username string
		Password string
	}{
		Username: d.username,
		Password: d.password,
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(body)
	if err != nil {
		return "", err
	}

	response, err := http.Post("https://hub.docker.com/v2/users/login", "application/json", &buf)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

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
