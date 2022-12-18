package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"golang.org/x/exp/slices"

	"github.com/stretchr/testify/assert"
)

func Test_dockerHubClient_GetImages(t *testing.T) {
	hubClient := NewDockerHubClient("", "")
	hubClient.client = &clientMockImages{}

	organization := "dhis2"
	images, err := hubClient.GetImages(organization)
	require.NoError(t, err)

	assert.True(t, slices.Contains(images, "core"))
	assert.True(t, slices.Contains(images, "core-dev"))
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
	res := dockerHubResponse{Results: results}

	data, err := json.Marshal(res)
	if err != nil {
		log.Fatal(err)
	}
	reader := bytes.NewReader(data)
	return &http.Response{
		Body: io.NopCloser(reader),
	}, nil
}

func Test_dockerHubClient_GetTags(t *testing.T) {
	hubClient := NewDockerHubClient("", "")
	hubClient.client = &clientMockTags{}

	organization := "dhis2"
	images, err := hubClient.GetImages(organization)
	require.NoError(t, err)

	assert.True(t, slices.Contains(images, "2.39.0"))
	assert.True(t, slices.Contains(images, "2.38.0"))
	assert.True(t, slices.Contains(images, "2.37.0"))
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
	res := dockerHubResponse{Results: results}

	data, err := json.Marshal(res)
	if err != nil {
		log.Fatal(err)
	}
	reader := bytes.NewReader(data)
	return &http.Response{
		Body: io.NopCloser(reader),
	}, nil
}
