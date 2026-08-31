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

func createDeployment(t *testing.T, client *inttest.HTTPClient, name string, authToken string, opts ...DeploymentOption) model.Deployment {
	t.Helper()

	builder := &deploymentBuilder{
		name:      name,
		groupName: "group-name", // hard coded, let's make this configurable if needed
	}

	for _, opt := range opts {
		opt(builder)
	}

	payload := map[string]any{
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

	payload := make(map[string]any)
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

/* heavyDeploys bounds how many subtests may hold a postgres or dhis2-core deployment at once. The
 * runner cannot carry four postgres instances and two cores together: they schedule, then starve,
 * fail their probes, restart, and every subtest waiting on readiness times out. Two runs of #1660
 * failed that way with every postgres and core pod unready simultaneously while every lightweight
 * pod stayed ready. FilestoreBackupFilesystemViaExec already opted out of the parallel batch for
 * this reason; this bounds the rest instead of serialising them. */
var heavyDeploys = make(chan struct{}, 2)

// acquireHeavyDeploy blocks until this subtest may deploy postgres or dhis2-core, releasing the slot
// once the subtest and its cleanup are done.
func acquireHeavyDeploy(t *testing.T) {
	t.Helper()
	heavyDeploys <- struct{}{}
	t.Cleanup(func() { <-heavyDeploys })
}
