package instance_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type instanceBuilder struct {
	deploymentID uint
	stackName    string
	parameters   model.DeploymentInstanceParameters
	public       *bool
}

type InstanceOption func(*instanceBuilder)

func WithParameter(key, value string) InstanceOption {
	return func(ib *instanceBuilder) {
		if ib.parameters == nil {
			ib.parameters = make(model.DeploymentInstanceParameters)
		}
		ib.parameters[key] = model.DeploymentInstanceParameter{Value: value}
	}
}

func WithPublic(public bool) InstanceOption {
	return func(ib *instanceBuilder) {
		ib.public = &public
	}
}

func createInstance(t *testing.T, client *inttest.HTTPClient, deploymentID uint, stackName string, authToken string, opts ...InstanceOption) model.DeploymentInstance {
	t.Helper()

	builder := &instanceBuilder{
		deploymentID: deploymentID,
		stackName:    stackName,
	}

	for _, opt := range opts {
		opt(builder)
	}

	payload := map[string]any{
		"stackName": builder.stackName,
	}

	if len(builder.parameters) > 0 {
		payload["parameters"] = builder.parameters
	}

	if builder.public != nil {
		payload["public"] = *builder.public
	}

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err, "failed to marshal instance payload")

	var instance model.DeploymentInstance
	path := fmt.Sprintf("/deployments/%d/instance", deploymentID)
	client.PostJSON(t, path, strings.NewReader(string(jsonData)), &instance, inttest.WithAuthToken(authToken))

	assert.Equal(t, deploymentID, instance.DeploymentID)
	assert.Equal(t, "group-name", instance.GroupName)
	assert.Equal(t, stackName, instance.StackName)

	return instance
}

func updateInstance(t *testing.T, client *inttest.HTTPClient, instance model.DeploymentInstance, authToken string, opts ...InstanceOption) model.DeploymentInstance {
	t.Helper()

	builder := &instanceBuilder{
		stackName: instance.StackName,
	}

	for _, opt := range opts {
		opt(builder)
	}

	payload := map[string]any{
		"stackName": builder.stackName,
	}

	if len(builder.parameters) > 0 {
		payload["parameters"] = builder.parameters
	}

	if builder.public != nil {
		payload["public"] = *builder.public
	}

	jsonData, err := json.Marshal(payload)
	require.NoError(t, err, "failed to marshal update payload")

	var updatedInstance model.DeploymentInstance
	path := fmt.Sprintf("/deployments/%d/instance/%d", instance.DeploymentID, instance.ID)
	client.PutJSON(t, path, strings.NewReader(string(jsonData)), &updatedInstance, inttest.WithAuthToken(authToken))

	return updatedInstance
}

func createWhoamiInstance(t *testing.T, client *inttest.HTTPClient, deploymentID uint, authToken string, opts ...InstanceOption) model.DeploymentInstance {
	return createInstance(t, client, deploymentID, "whoami-go", authToken, opts...)
}

func createDHIS2DBInstance(t *testing.T, client *inttest.HTTPClient, deploymentID uint, databaseID, authToken string, opts ...InstanceOption) model.DeploymentInstance {
	return createInstance(t, client, deploymentID, "dhis2-db", authToken, append([]InstanceOption{WithParameter("DATABASE_ID", databaseID)}, opts...)...)
}

func createMinioInstance(t *testing.T, client *inttest.HTTPClient, deploymentID uint, authToken string, opts ...InstanceOption) model.DeploymentInstance {
	return createInstance(t, client, deploymentID, "minio", authToken, opts...)
}

func createDHIS2CoreInstance(t *testing.T, client *inttest.HTTPClient, deploymentID uint, authToken string, opts ...InstanceOption) model.DeploymentInstance {
	return createInstance(t, client, deploymentID, "dhis2-core", authToken, opts...)
}
