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

// recordingExecutor records every Exec call and can be scripted with per-call
// stdout to write and errors to return, keyed by call index.
type recordingExecutor struct {
	calls   [][]string
	stdouts map[int]string
	stderrs map[int]string
	errs    map[int]error
}

func (r *recordingExecutor) Exec(ctx context.Context, namespace, podName, container string, command []string, stdout, stderr io.Writer) error {
	i := len(r.calls)
	r.calls = append(r.calls, command)
	if s := r.stdouts[i]; s != "" {
		_, _ = io.WriteString(stdout, s)
	}
	if s := r.stderrs[i]; s != "" {
		_, _ = io.WriteString(stderr, s)
	}
	return r.errs[i]
}

func TestMinioExecSourceMirrorsTarsAndCleansUp(t *testing.T) {
	exec := &recordingExecutor{stdouts: map[int]string{1: "tar-bytes"}} // call 1 = tar
	src := minioExecSource{executor: exec, namespace: "ns", podName: "pod", container: "minio", tmpDir: "/tmp/im-filestore-backup-x"}

	var buf strings.Builder
	require.NoError(t, src.stream(context.Background(), &buf))

	assert.Equal(t, "tar-bytes", buf.String())
	require.Len(t, exec.calls, 3)
	assert.Equal(t, []string{"env", "MC_HOST_backup=http://dhisdhis:dhisdhis@127.0.0.1:9000", "mc", "mirror", "--quiet", "backup/dhis2", "/tmp/im-filestore-backup-x"}, exec.calls[0])
	assert.Equal(t, []string{"tar", "-C", "/tmp/im-filestore-backup-x", "-czf", "-", "."}, exec.calls[1])
	assert.Equal(t, []string{"rm", "-rf", "/tmp/im-filestore-backup-x"}, exec.calls[2])
}

func TestMinioExecSourceMirrorErrorSkipsTarButCleansUp(t *testing.T) {
	exec := &recordingExecutor{
		errs:    map[int]error{0: fmt.Errorf("exit 1")},
		stderrs: map[int]string{0: "mc: mirror failed"},
	}
	src := minioExecSource{executor: exec, namespace: "ns", podName: "pod", container: "minio", tmpDir: "/tmp/im-filestore-backup-x"}

	err := src.stream(context.Background(), io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit 1")
	assert.Contains(t, err.Error(), "mc: mirror failed")

	require.Len(t, exec.calls, 2) // mirror (failed), then cleanup; no tar
	assert.Equal(t, "mc", exec.calls[0][2])
	assert.Equal(t, []string{"rm", "-rf", "/tmp/im-filestore-backup-x"}, exec.calls[1])
}

func TestMinioTempDir(t *testing.T) {
	assert.Equal(t, "/tmp/im-filestore-backup-saved-copy", minioTempDir("saved-copy"))
	assert.Equal(t, "/tmp/im-filestore-backup-a-b-c", minioTempDir("a/b c")) // non-path-safe chars replaced
}

func TestFilesystemTarCommand(t *testing.T) {
	cmd := filesystemTarCommand("/opt/dhis2/")
	assert.Equal(t, []string{"tar", "-C", "/opt/dhis2/files", "-czf", "-", "."}, cmd)
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
