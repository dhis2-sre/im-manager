package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func Test_dockerHubClient_GetImages(t *testing.T) {
	hubClient := NewDockerHubClient("", "")
	hubClient.client = &clientMockImages{}

	organization := "dhis2"
	images, err := hubClient.GetImages(organization)
	require.NoError(t, err)

	assert.ElementsMatch(t, images, []string{"core", "core-dev"})
}

type clientMockImages struct {
}

func (c *clientMockImages) Do(*http.Request) (*http.Response, error) {
	results := []struct{ Name string }{
		{
			Name: "core",
		},
		{
			Name: "core-dev",
		},
	}
	return do(results)
}

func Test_dockerHubClient_GetTags(t *testing.T) {
	hubClient := NewDockerHubClient("", "")
	hubClient.client = &clientMockTags{}

	organization := "dhis2"
	repository := "core"
	tags, err := hubClient.GetTags(organization, repository)
	require.NoError(t, err)

	assert.ElementsMatch(t, tags, []string{"2.39.0", "2.38.0", "2.37.0"})
}

type clientMockTags struct {
}

func (c *clientMockTags) Do(*http.Request) (*http.Response, error) {
	results := []struct{ Name string }{
		{
			Name: "2.39.0",
		},
		{
			Name: "2.38.0",
		},
		{
			Name: "2.37.0",
		},
	}
	return do(results)
}

func do(results []struct{ Name string }) (*http.Response, error) {
	res := dockerHubResponse{Results: results}

	data, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	reader := bytes.NewReader(data)
	return &http.Response{
		Body: io.NopCloser(reader),
	}, nil
}
