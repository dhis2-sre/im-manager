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
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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

// A core with no -Xmx finds no cgroup limit, since the chart sets a memory request and no limit, and
// takes a quarter of whatever /proc/meminfo reports, which inside the k3s container is the runner.
const (
	testCoreJavaOpts = "-Xmx1g"
	// The same bound in bytes, as the JVM reports MaxHeapSize.
	testCoreMaxHeapSize = int64(1) << 30
)

func createDHIS2CoreInstance(t *testing.T, client *inttest.HTTPClient, deploymentID uint, authToken string, opts ...InstanceOption) model.DeploymentInstance {
	// Prepended, so a caller that wants a different heap can still override it.
	opts = append([]InstanceOption{WithParameter("JAVA_OPTS", testCoreJavaOpts)}, opts...)
	return createInstance(t, client, deploymentID, "dhis2-core", authToken, opts...)
}

// podExecer is the part of the kubernetes service these helpers need.
type podExecer interface {
	Exec(ctx context.Context, namespace, podName, container string, command []string, stdout, stderr io.Writer) error
}

// assertCoreHeapBounded reads the ceiling a throwaway JVM in the core container is given. The chart
// appends javaOpts to JAVA_TOOL_OPTIONS, which every JVM there honours and none shows on its command
// line, so this is the readable surface; {command line} rather than {ergonomic} is what distinguishes
// a bound that was set from one the JVM derived from the node.
func assertCoreHeapBounded(t *testing.T, executor podExecer, namespace, pod, container string, expected int64) {
	t.Helper()

	var flags, stderr strings.Builder
	require.NoError(t, executor.Exec(context.Background(), namespace, pod, container,
		[]string{"sh", "-c", `java -XX:+PrintFlagsFinal -version 2>/dev/null | grep MaxHeapSize`}, &flags, &stderr),
		"reading the core's heap ceiling failed: %s", stderr.String())

	line := strings.TrimSpace(flags.String())
	fields := strings.Fields(line)
	require.GreaterOrEqual(t, len(fields), 4, "unexpected MaxHeapSize line: %q", line)
	actual, err := strconv.ParseInt(fields[3], 10, 64)
	require.NoError(t, err, "unexpected MaxHeapSize line: %q", line)

	assert.Equal(t, expected, actual, "the core's heap is not bounded to %s: %q", testCoreJavaOpts, line)
	assert.Contains(t, line, "{command line}", "the core's heap ceiling came from the node, not from %s: %q", testCoreJavaOpts, line)
}

// minioPodName returns the name of the deployment's single minio pod.
func minioPodName(t *testing.T, k8sClient *inttest.K8sClient, namespace string, deploymentID uint) string {
	t.Helper()
	selector := fmt.Sprintf("im-type=minio,im-deployment-id=%d", deploymentID)
	pods, err := k8sClient.Client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector})
	require.NoError(t, err)
	require.Len(t, pods.Items, 1, "expected exactly one minio pod for selector %q", selector)
	return pods.Items[0].Name
}

// waitForCorePodRunning polls until the core instance's default pod is Running, returning its name
// and primary container.
func waitForCorePodRunning(t *testing.T, k8sClient *inttest.K8sClient, namespace string, instanceID uint, timeout time.Duration) (string, string) {
	t.Helper()
	selector := fmt.Sprintf("im-id=%d,im-default=true", instanceID)
	deadline := time.Now().Add(timeout)
	for {
		pods, err := k8sClient.Client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector})
		require.NoError(t, err)
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning && len(pod.Spec.Containers) > 0 {
				return pod.Name, pod.Spec.Containers[0].Name
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for a Running core pod for selector %q", timeout, selector)
		}
		time.Sleep(2 * time.Second)
	}
}

// extractTarGzEntries unpacks a gzip'd tar into a map of file path -> contents, stripping any leading "./".
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

// rawTarGzNames returns the raw tar entry names for regular files, without stripping
// the leading "./". Restore (seed-minio.sh: tar x + mc mirror) reproduces the original
// object key only if the exec backends tar with "./"-relative names, so asserting the raw
// name guards that format.
func rawTarGzNames(t *testing.T, data []byte) []string {
	t.Helper()
	gr, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer gr.Close()

	tr := tar.NewReader(gr)
	var names []string
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		if header.Typeflag != tar.TypeReg {
			continue
		}
		names = append(names, header.Name)
	}
	return names
}
