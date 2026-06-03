package instance

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeExecutor struct {
	gotNamespace string
	gotPod       string
	gotContainer string
	gotCommand   []string
	stdout       string
	stderr       string
	err          error
}

func (f *fakeExecutor) Exec(ctx context.Context, namespace, podName, container string, command []string, stdout, stderr io.Writer) error {
	f.gotNamespace, f.gotPod, f.gotContainer, f.gotCommand = namespace, podName, container, command
	if f.stdout != "" {
		_, _ = io.WriteString(stdout, f.stdout)
	}
	if f.stderr != "" {
		_, _ = io.WriteString(stderr, f.stderr)
	}
	return f.err
}

func TestExecStreamerCopiesStdout(t *testing.T) {
	exec := &fakeExecutor{stdout: "tar-bytes"}
	s := execStreamer{executor: exec, namespace: "ns", podName: "pod", container: "minio", command: []string{"sh", "-c", "x"}}

	var buf strings.Builder
	require.NoError(t, s.stream(context.Background(), &buf))

	assert.Equal(t, "tar-bytes", buf.String())
	assert.Equal(t, "ns", exec.gotNamespace)
	assert.Equal(t, "pod", exec.gotPod)
	assert.Equal(t, "minio", exec.gotContainer)
}

func TestExecStreamerWrapsStderrOnError(t *testing.T) {
	exec := &fakeExecutor{stderr: "mc: boom", err: fmt.Errorf("exit 1")}
	s := execStreamer{executor: exec, namespace: "ns", podName: "pod", container: "minio", command: []string{"sh"}}

	err := s.stream(context.Background(), io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit 1")
	assert.Contains(t, err.Error(), "mc: boom")
}

func TestMinioMirrorCommand(t *testing.T) {
	cmd := minioMirrorCommand()
	require.Len(t, cmd, 3)
	assert.Equal(t, []string{"sh", "-c"}, cmd[:2])
	assert.Contains(t, cmd[2], "mc mirror --quiet bk/dhis2")
	assert.Contains(t, cmd[2], `tar -C "$D" -czf - .`)
	assert.Contains(t, cmd[2], "mc alias set bk http://127.0.0.1:9000 dhisdhis dhisdhis")
}

func TestFilesystemTarCommand(t *testing.T) {
	cmd := filesystemTarCommand("/opt/dhis2/")
	require.Len(t, cmd, 3)
	assert.Equal(t, []string{"sh", "-c"}, cmd[:2])
	assert.Contains(t, cmd[2], `tar -C "/opt/dhis2/files" -czf - .`)
	assert.Contains(t, cmd[2], "tar -czf - -T /dev/null")
}

func TestGetPodByLabels(t *testing.T) {
	minioPod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "test-1-minio-abc",
		Namespace: "grp",
		Labels:    map[string]string{"im-type": "minio", "im-deployment-id": "7"},
	}}
	otherPod := &v1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "test-1-minio-other",
		Namespace: "grp",
		Labels:    map[string]string{"im-type": "minio", "im-deployment-id": "9"},
	}}
	ks := kubernetesService{client: fake.NewSimpleClientset(minioPod, otherPod)}

	pod, err := ks.getPodByLabels(map[string]string{"im-type": "minio", "im-deployment-id": "7"})
	require.NoError(t, err)
	assert.Equal(t, "test-1-minio-abc", pod.Name)
	assert.Equal(t, "grp", pod.Namespace)
}

func TestGetPodByLabelsNotFound(t *testing.T) {
	ks := kubernetesService{client: fake.NewSimpleClientset()}
	_, err := ks.getPodByLabels(map[string]string{"im-type": "minio", "im-deployment-id": "7"})
	require.Error(t, err)
}

func TestStorageTypeDefaultsToMinio(t *testing.T) {
	assert.Equal(t, "minio", storageType(&model.DeploymentInstance{}))
	withType := &model.DeploymentInstance{Parameters: model.DeploymentInstanceParameters{
		"STORAGE_TYPE": {Value: "filesystem"},
	}}
	assert.Equal(t, "filesystem", storageType(withType))
}
