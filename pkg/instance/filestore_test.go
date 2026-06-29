package instance

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// fakeBackupSource serves a fixed set of objects from memory.
type fakeBackupSource struct {
	objects map[string][]byte
}

func (f fakeBackupSource) List(ctx context.Context) (<-chan BackupObject, error) {
	ch := make(chan BackupObject)
	go func() {
		defer close(ch)
		for path, data := range f.objects {
			select {
			case ch <- BackupObject{Path: path, Size: int64(len(data))}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (f fakeBackupSource) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	data, ok := f.objects[path]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", path)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func TestWriteTarGzConcurrent(t *testing.T) {
	objects := make(map[string][]byte)
	for i := 0; i < 500; i++ { // more objects than filestoreReadWorkers, to exercise the pool
		objects[fmt.Sprintf("apps/app-%03d/file.txt", i)] = []byte(fmt.Sprintf("content-%d", i))
	}

	var buf bytes.Buffer
	require.NoError(t, writeTarGz(context.Background(), fakeBackupSource{objects: objects}, &buf))

	entries := extractTarGz(t, buf.Bytes())
	require.Len(t, entries, len(objects)) // no objects dropped despite concurrency
	for path, data := range objects {
		assert.Equal(t, data, entries[path], "content mismatch for %s", path)
	}
}

// failingBackupSource lists objects but fails Get for one path.
type failingBackupSource struct {
	objects  map[string][]byte
	failPath string
}

func (f failingBackupSource) List(ctx context.Context) (<-chan BackupObject, error) {
	ch := make(chan BackupObject)
	go func() {
		defer close(ch)
		for path, data := range f.objects {
			select {
			case ch <- BackupObject{Path: path, Size: int64(len(data))}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}

func (f failingBackupSource) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	if path == f.failPath {
		return nil, fmt.Errorf("boom fetching %s", path)
	}
	return io.NopCloser(bytes.NewReader(f.objects[path])), nil
}

func TestWriteTarGzPropagatesGetError(t *testing.T) {
	objects := make(map[string][]byte)
	for i := 0; i < 200; i++ { // more objects than filestoreReadWorkers, so a failure races other fetches
		objects[fmt.Sprintf("apps/app-%03d/file.txt", i)] = []byte("x")
	}
	src := failingBackupSource{objects: objects, failPath: "apps/app-100/file.txt"}

	// in a goroutine so a deadlock regression fails the test instead of hanging it
	done := make(chan error, 1)
	go func() { done <- writeTarGz(context.Background(), src, io.Discard) }()

	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "boom")
	case <-time.After(15 * time.Second):
		t.Fatal("writeTarGz did not return - the concurrent fetch likely deadlocked on a Get error")
	}
}

func TestWriteTarGzStreamsLargeObjects(t *testing.T) {
	// lower the buffering threshold so small test objects exercise both the buffered and the direct-stream paths
	orig := filestoreMaxBufferedObject
	filestoreMaxBufferedObject = 4
	defer func() { filestoreMaxBufferedObject = orig }()

	objects := map[string][]byte{
		"small/a.txt": []byte("hi"),                    // <= threshold: buffered
		"large/b.bin": bytes.Repeat([]byte("x"), 1024), // > threshold: streamed directly
	}

	var buf bytes.Buffer
	require.NoError(t, writeTarGz(context.Background(), fakeBackupSource{objects: objects}, &buf))

	entries := extractTarGz(t, buf.Bytes())
	require.Len(t, entries, len(objects))
	for path, data := range objects {
		assert.Equal(t, data, entries[path], "content mismatch for %s", path)
	}
}

func TestFilestoreStreamerForS3(t *testing.T) {
	s := Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	core := &model.DeploymentInstance{Parameters: model.DeploymentInstanceParameters{
		"STORAGE_TYPE": {Value: "s3"},
		"S3_BUCKET":    {Value: "my-bucket"},
		"S3_REGION":    {Value: "eu-west-1"},
		"S3_IDENTITY":  {Value: "identity"},
		"S3_SECRET":    {Value: "secret"},
	}}

	// only s3 is unit-testable here; minio/filesystem resolve a pod and are covered by integration tests
	streamer, err := s.filestoreStreamerFor(core, model.Cluster{})
	require.NoError(t, err)
	assert.IsType(t, s3APISource{}, streamer)
}

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
	s := execStreamer{podExec: podExec{executor: exec, namespace: "ns", podName: "pod", container: "minio"}, command: []string{"sh", "-c", "x"}}

	var buf strings.Builder
	require.NoError(t, s.stream(context.Background(), &buf))

	assert.Equal(t, "tar-bytes", buf.String())
	assert.Equal(t, "ns", exec.gotNamespace)
	assert.Equal(t, "pod", exec.gotPod)
	assert.Equal(t, "minio", exec.gotContainer)
}

func TestExecStreamerWrapsStderrOnError(t *testing.T) {
	exec := &fakeExecutor{stderr: "mc: boom", err: fmt.Errorf("exit 1")}
	s := execStreamer{podExec: podExec{executor: exec, namespace: "ns", podName: "pod", container: "minio"}, command: []string{"sh"}}

	err := s.stream(context.Background(), io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit 1")
	assert.Contains(t, err.Error(), "mc: boom")
}

// recordingExecutor records every Exec call; per-call stdout and errors are keyed by call index.
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
	exec := &recordingExecutor{stdouts: map[int]string{
		0: "/tmp/im-filestore-backup-abc123\n", // call 0 = mktemp (trailing newline must be trimmed)
		2: "tar-bytes",                         // call 2 = tar
	}}
	src := minioExecSource{podExec{executor: exec, namespace: "ns", podName: "pod", container: "minio"}}

	var buf strings.Builder
	require.NoError(t, src.stream(context.Background(), &buf))

	assert.Equal(t, "tar-bytes", buf.String())
	require.Len(t, exec.calls, 4)
	assert.Equal(t, []string{"mktemp", "-d", "/tmp/im-filestore-backup-XXXXXXXX"}, exec.calls[0])
	assert.Equal(t, []string{"env", "MC_HOST_backup=http://dhisdhis:dhisdhis@127.0.0.1:9000", "mc", "mirror", "--quiet", "backup/dhis2", "/tmp/im-filestore-backup-abc123"}, exec.calls[1])
	assert.Equal(t, []string{"tar", "-C", "/tmp/im-filestore-backup-abc123", "-czf", "-", "."}, exec.calls[2])
	assert.Equal(t, []string{"rm", "-rf", "/tmp/im-filestore-backup-abc123"}, exec.calls[3])
}

func TestMinioExecSourceMirrorErrorSkipsTarButCleansUp(t *testing.T) {
	exec := &recordingExecutor{
		stdouts: map[int]string{0: "/tmp/im-filestore-backup-abc123"}, // call 0 = mktemp
		errs:    map[int]error{1: fmt.Errorf("exit 1")},               // call 1 = mirror
		stderrs: map[int]string{1: "mc: mirror failed"},
	}
	src := minioExecSource{podExec{executor: exec, namespace: "ns", podName: "pod", container: "minio"}}

	err := src.stream(context.Background(), io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exit 1")
	assert.Contains(t, err.Error(), "mc: mirror failed")

	require.Len(t, exec.calls, 3) // mktemp, mirror (failed), then cleanup; no tar
	assert.Equal(t, "mc", exec.calls[1][2])
	assert.Equal(t, []string{"rm", "-rf", "/tmp/im-filestore-backup-abc123"}, exec.calls[2])
}

func TestMinioExecSourceMktempErrorSkipsCleanup(t *testing.T) {
	exec := &recordingExecutor{
		errs:    map[int]error{0: fmt.Errorf("exit 1")}, // call 0 = mktemp
		stderrs: map[int]string{0: "mktemp: no space"},
	}
	src := minioExecSource{podExec{executor: exec, namespace: "ns", podName: "pod", container: "minio"}}

	err := src.stream(context.Background(), io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mktemp: no space")

	require.Len(t, exec.calls, 1) // mktemp failed; nothing to mirror, tar, or clean up
}

func TestMinioExecSourceEmptyMktempPathErrors(t *testing.T) {
	exec := &recordingExecutor{stdouts: map[int]string{0: "  \n"}} // mktemp produced no path
	src := minioExecSource{podExec{executor: exec, namespace: "ns", podName: "pod", container: "minio"}}

	err := src.stream(context.Background(), io.Discard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path")

	require.Len(t, exec.calls, 1) // no rm -rf with an empty path
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
