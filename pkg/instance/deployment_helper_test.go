package instance_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type deploymentBuilder struct {
	name        string
	groupName   string
	description string
	ttl         *uint
	public      *bool
}

type DeploymentOption func(*deploymentBuilder)

func WithDescription(description string) DeploymentOption {
	return func(db *deploymentBuilder) {
		db.description = description
	}
}

func WithTTL(ttl uint) DeploymentOption {
	return func(db *deploymentBuilder) {
		db.ttl = &ttl
	}
}

func WithDeploymentPublic(public bool) DeploymentOption {
	return func(db *deploymentBuilder) {
		db.public = &public
	}
}

func createDeployment(t *testing.T, client *inttest.HTTPClient, name string, authToken string, opts ...DeploymentOption) model.Deployment {
	t.Helper()

	builder := &deploymentBuilder{
		name:      name,
		groupName: "group-name", // default, could be made configurable if needed
	}

	for _, opt := range opts {
		opt(builder)
	}

	payload := map[string]interface{}{
		"name":  builder.name,
		"group": builder.groupName,
	}

	if builder.description != "" {
		payload["description"] = builder.description
	}

	if builder.ttl != nil {
		payload["ttl"] = *builder.ttl
	}

	if builder.public != nil {
		payload["public"] = *builder.public
	}

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err, "failed to marshal deployment payload")

	var deployment model.Deployment
	client.PostJSON(t, "/deployments", strings.NewReader(string(jsonData)), &deployment, inttest.WithAuthToken(authToken))

	assert.Equal(t, name, deployment.Name)
	assert.Equal(t, builder.groupName, deployment.GroupName)
	if builder.description != "" {
		assert.Equal(t, builder.description, deployment.Description)
	}

	return deployment
}

func deployDeployment(t *testing.T, client *inttest.HTTPClient, deploymentID uint, authToken string) {
	t.Helper()
	path := fmt.Sprintf("/deployments/%d/deploy", deploymentID)
	client.Do(t, http.MethodPost, path, nil, http.StatusOK, inttest.WithAuthToken(authToken))
}

func destroyDeployment(t *testing.T, client *inttest.HTTPClient, deploymentID uint, authToken string) {
	t.Helper()
	path := fmt.Sprintf("/deployments/%d", deploymentID)
	client.Do(t, http.MethodDelete, path, nil, http.StatusAccepted, inttest.WithAuthToken(authToken))
}

func updateDeployment(t *testing.T, client *inttest.HTTPClient, deploymentID uint, authToken string, opts ...DeploymentOption) model.Deployment {
	t.Helper()

	builder := &deploymentBuilder{}
	for _, opt := range opts {
		opt(builder)
	}

	payload := make(map[string]interface{})
	if builder.description != "" {
		payload["description"] = builder.description
	}
	if builder.ttl != nil {
		payload["ttl"] = *builder.ttl
	}

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err, "failed to marshal update payload")

	var updatedDeployment model.Deployment
	path := fmt.Sprintf("/deployments/%d", deploymentID)
	client.PutJSON(t, path, strings.NewReader(string(jsonData)), &updatedDeployment, inttest.WithAuthToken(authToken))

	return updatedDeployment
}
