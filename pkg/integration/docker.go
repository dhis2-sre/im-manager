package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

func NewDockerHubClient(username, password string) dockerHubClient {
	return dockerHubClient{username, password}
}

type dockerHubClient struct {
	username, password string
}

func (d dockerHubClient) GetTags(organization string, repository string) ([]string, error) {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags?page_size=10000", organization, repository)

	token, err := d.getToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("JWT %s", token))

	client := http.Client{}

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

func (d dockerHubClient) GetImages(organization string) ([]string, error) {
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s?page_size=10000", organization)

	token, err := d.getToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("JWT %s", token))

	client := http.Client{}

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

func (d dockerHubClient) getToken() (string, error) {
	url := "https://hub.docker.com/v2/users/login"
	contentType := "application/json"
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
