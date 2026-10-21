package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
)

// NewGhcrClient creates a GHCR client. Without credentials it only sees public packages; with a username and a
// token that can read packages it also sees the private packages that user has access to.
//
//goland:noinspection GoExportedFuncWithUnexportedType
func NewGhcrClient(username, password string) *ghcrClient {
	return &ghcrClient{client: &http.Client{}, username: username, password: password}
}

type ghcrClient struct {
	client   *http.Client
	username string
	password string
}

func (g *ghcrClient) getToken(organization, repository string) (string, error) {
	url := fmt.Sprintf("https://ghcr.io/token?scope=repository:%s/%s:pull", organization, repository)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if g.username != "" && g.password != "" {
		req.SetBasicAuth(g.username, g.password)
	}

	response, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ghcr token request failed with status %d", response.StatusCode)
	}

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

	url := fmt.Sprintf("https://ghcr.io/v2/%s/%s/tags/list?n=10000", organization, repository)
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

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ghcr tags request failed with status %d", response.StatusCode)
	}

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

	return slices.DeleteFunc(body.Tags, func(tag string) bool {
		return strings.HasPrefix(tag, "sha256-")
	}), nil
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

// GetImages GHCR-based stacks (e.g. CHAP) only use IMAGE_TAG, not IMAGE_REPOSITORY, so this is never called.
func (g *ghcrClient) GetImages(_ string) ([]string, error) {
	return nil, nil
}
