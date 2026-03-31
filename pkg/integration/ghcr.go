package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewGhcrClient() *ghcrClient {
	return &ghcrClient{client: &http.Client{}}
}

type ghcrClient struct {
	client *http.Client
}

func (g *ghcrClient) getToken(organization, repository string) (string, error) {
	url := fmt.Sprintf("https://ghcr.io/token?scope=repository:%s/%s:pull", organization, repository)
	response, err := g.client.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	token := struct {
		Token string `json:"token"`
	}{}
	if err := json.Unmarshal(b, &token); err != nil {
		return "", err
	}

	return token.Token, nil
}

func (g *ghcrClient) GetTags(organization, repository string) ([]string, error) {
	token, err := g.getToken(organization, repository)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://ghcr.io/v2/%s/%s/tags/list", organization, repository)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	response, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	body := struct {
		Tags []string `json:"tags"`
	}{}
	if err := json.Unmarshal(b, &body); err != nil {
		return nil, err
	}

	return body.Tags, nil
}

func (g *ghcrClient) ImageExists(organization, repository, tag string) error {
	tags, err := g.GetTags(organization, repository)
	if err != nil {
		return err
	}

	for _, t := range tags {
		if t == tag {
			return nil
		}
	}

	return fmt.Errorf("%s/%s:%s not found", organization, repository, tag)
}

func (g *ghcrClient) GetImages(_ string) ([]string, error) {
	return nil, nil
}
