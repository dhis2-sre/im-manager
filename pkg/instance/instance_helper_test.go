package instance_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	builder := &instanceBuilder{}

	for _, opt := range opts {
		opt(builder)
	}

	payload := map[string]any{}

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
	client.PatchJSON(t, path, strings.NewReader(string(jsonData)), &updatedInstance, inttest.WithAuthToken(authToken))

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

// minioPodName returns the name of the single minio pod belonging to the given deployment,
// disambiguated by im-deployment-id so it is safe under parallel subtests sharing a namespace.
func minioPodName(t *testing.T, k8sClient *inttest.K8sClient, namespace string, deploymentID uint) string {
	t.Helper()
	selector := fmt.Sprintf("im-type=minio,im-deployment-id=%d", deploymentID)
	pods, err := k8sClient.Client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector})
	require.NoError(t, err)
	require.Len(t, pods.Items, 1, "expected exactly one minio pod for selector %q", selector)
	return pods.Items[0].Name
}

// extractTarGzEntries unpacks a gzip'd tar into a map of regular-file path -> contents, stripping
// any leading "./" so callers can assert on logical object keys.
func extractTarGzEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer gr.Close()

	tr := tar.NewReader(gr)
	entries := make(map[string][]byte)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		if header.Typeflag != tar.TypeReg {
			continue
		}
		var buf bytes.Buffer
		_, err = io.Copy(&buf, tr) //nolint:gosec // test data, trusted archive
		require.NoError(t, err)
		entries[strings.TrimPrefix(header.Name, "./")] = buf.Bytes()
	}
	return entries
}
