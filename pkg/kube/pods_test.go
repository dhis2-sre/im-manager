package kube

import (
	"context"
	"io"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodLogs(t *testing.T) {
	pod := componentTestPod("mydb-0", "db")
	pod.Spec.Containers = []v1.Container{{Name: "postgres"}, {Name: "sidecar"}}
	client := &Client{Clientset: fake.NewSimpleClientset(pod)}

	stream, err := client.PodLogs(context.Background(), "ns", "mydb-0", "", nil)
	require.NoError(t, err)
	defer stream.Close()

	logs, err := io.ReadAll(stream)
	require.NoError(t, err)
	assert.NotEmpty(t, logs)
}

func TestPodLogsPodNotFound(t *testing.T) {
	client := &Client{Clientset: fake.NewSimpleClientset()}

	_, err := client.PodLogs(context.Background(), "ns", "mydb-0", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mydb-0")
}

func TestPodLogsOfNamedContainer(t *testing.T) {
	pod := componentTestPod("mydb-0", "db")
	pod.Spec.Containers = []v1.Container{{Name: "postgres"}, {Name: "sidecar"}}
	client := &Client{Clientset: fake.NewSimpleClientset(pod)}

	stream, err := client.PodLogs(context.Background(), "ns", "mydb-0", "sidecar", nil)
	require.NoError(t, err)
	require.NoError(t, stream.Close())

	_, err = client.PodLogs(context.Background(), "ns", "mydb-0", "no-such-container", nil)
	require.Error(t, err)
	assert.True(t, errdef.IsNotFound(err), "expected a not found error, got %v", err)
}

func TestPodLogsWithoutContainers(t *testing.T) {
	pod := componentTestPod("mydb-0", "db")
	pod.Spec.Containers = nil
	client := &Client{Clientset: fake.NewSimpleClientset(pod)}

	_, err := client.PodLogs(context.Background(), "ns", "mydb-0", "", nil)
	require.Error(t, err)
	assert.True(t, errdef.IsNotFound(err), "expected a not found error, got %v", err)
}

func TestContainerName(t *testing.T) {
	pod := componentTestPod("mydb-0", "db")
	pod.Spec.Containers = []v1.Container{{Name: "postgres"}, {Name: "sidecar"}}

	container, err := containerName(*pod, "")
	require.NoError(t, err)
	assert.Equal(t, "postgres", container, "a caller naming no container gets the first one")

	container, err = containerName(*pod, "sidecar")
	require.NoError(t, err)
	assert.Equal(t, "sidecar", container)

	_, err = containerName(*pod, "postgres-exporter")
	require.Error(t, err)
	assert.True(t, errdef.IsNotFound(err), "expected a not found error, got %v", err)
}
