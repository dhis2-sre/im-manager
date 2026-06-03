package instance

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// filestoreStreamer writes a gzip'd tar of an instance's filestore to w.
// Tar entry names are logical object keys (relative paths), which makes the
// archive restore-compatible with stacks/minio/seed-minio.sh for every backend.
type filestoreStreamer interface {
	stream(ctx context.Context, w io.Writer) error
}

// writeTarGz builds a gzip'd tar from a BackupSource (object lister) and writes
// it to w. Entry names are object keys, matching what mc mirror restores.
func writeTarGz(ctx context.Context, source BackupSource, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	objectCh, err := source.List(ctx)
	if err != nil {
		return fmt.Errorf("list objects: %v", err)
	}

	for object := range objectCh {
		if object.Err != nil {
			return object.Err
		}
		if err := writeTarObject(ctx, tw, source, object); err != nil {
			return err
		}
	}
	return nil
}

func writeTarObject(ctx context.Context, tw *tar.Writer, source BackupSource, object BackupObject) error {
	reader, err := source.Get(ctx, object.Path)
	if err != nil {
		return fmt.Errorf("failed to get object %s: %v", object.Path, err)
	}
	defer reader.Close()

	header := &tar.Header{
		Name:    object.Path,
		Size:    object.Size,
		Mode:    0644,
		ModTime: object.LastModified,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header for %s: %v", object.Path, err)
	}
	if _, err := io.Copy(tw, reader); err != nil {
		return fmt.Errorf("copy object %s to tar: %v", object.Path, err)
	}
	return nil
}

// s3APISource streams a tar.gz built from objects listed over the S3 API.
// Used for the external-AWS-S3 storage backend (no pod, no port-forward).
type s3APISource struct {
	source BackupSource
}

func (s s3APISource) stream(ctx context.Context, w io.Writer) error {
	return writeTarGz(ctx, s.source, w)
}

// podExecutor runs a command in a pod container, streaming stdout/stderr.
// Implemented by kubernetesService.
type podExecutor interface {
	Exec(ctx context.Context, namespace, podName, container string, command []string, stdout, stderr io.Writer) error
}

// execStreamer streams a command's stdout (a gzip'd tar) out of a pod, the same
// way the database backup streams pg_dump. stderr is captured for errors.
type execStreamer struct {
	executor  podExecutor
	namespace string
	podName   string
	container string
	command   []string
}

func (e execStreamer) stream(ctx context.Context, w io.Writer) error {
	var stderr strings.Builder
	if err := e.executor.Exec(ctx, e.namespace, e.podName, e.container, e.command, w, &stderr); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

// minioMirrorCommand mirrors the dhis2 bucket to a temp dir then tars it to
// stdout. MinIO 2025.x stores xl.meta on disk, so the bucket must be read
// through mc to get logical object keys. mc and tar ship in the minio image.
func minioMirrorCommand() []string {
	script := strings.Join([]string{
		"set -e",
		"D=$(mktemp -d)",
		`trap 'rm -rf "$D"' EXIT`,
		"mc alias set bk http://127.0.0.1:9000 dhisdhis dhisdhis >/dev/null 2>&1",
		`mc mirror --quiet bk/dhis2 "$D" >/dev/null`,
		`tar -C "$D" -czf - .`,
	}, "; ")
	return []string{"sh", "-c", script}
}

// filesystemTarCommand tars DHIS2_HOME/files to stdout, emitting an empty
// archive when no files exist yet so backup never fails on a clean instance.
func filesystemTarCommand(dhis2Home string) []string {
	files := strings.TrimRight(dhis2Home, "/") + "/files"
	script := fmt.Sprintf(`if [ -d "%s" ]; then tar -C "%s" -czf - .; else tar -czf - -T /dev/null; fi`, files, files)
	return []string{"sh", "-c", script}
}

func storageType(core *model.DeploymentInstance) string {
	if p, ok := core.Parameters["STORAGE_TYPE"]; ok && p.Value != "" {
		return p.Value
	}
	return "minio"
}

// newAWSS3Client builds a minio-go client for the instance's external AWS bucket.
func newAWSS3Client(core *model.DeploymentInstance) (*minio.Client, error) {
	region := core.Parameters["S3_REGION"].Value
	identity := core.Parameters["S3_IDENTITY"].Value
	secret := core.Parameters["S3_SECRET"].Value
	return minio.New("s3.amazonaws.com", &minio.Options{
		Creds:  credentials.NewStaticV4(identity, secret, ""),
		Secure: true,
		Region: region,
	})
}

// filestoreStreamerFor selects the backend-specific streamer for an instance.
// minio + filesystem stream via pod exec; s3 reads the external bucket directly.
func (s Service) filestoreStreamerFor(core *model.DeploymentInstance, cluster model.Cluster) (filestoreStreamer, error) {
	switch storageType(core) {
	case "filesystem":
		ks, err := NewKubernetesService(cluster)
		if err != nil {
			return nil, err
		}
		pod, err := ks.getPod(core.ID, "")
		if err != nil {
			return nil, err
		}
		return execStreamer{
			executor:  ks,
			namespace: pod.Namespace,
			podName:   pod.Name,
			container: pod.Spec.Containers[0].Name,
			command:   filesystemTarCommand(core.Parameters["DHIS2_HOME"].Value),
		}, nil
	case "s3":
		client, err := newAWSS3Client(core)
		if err != nil {
			return nil, err
		}
		return s3APISource{source: NewMinioBackupSource(s.logger, client, core.Parameters["S3_BUCKET"].Value)}, nil
	default: // minio
		ks, err := NewKubernetesService(cluster)
		if err != nil {
			return nil, err
		}
		pod, err := ks.getPodByLabels(map[string]string{
			"im-type":          "minio",
			"im-deployment-id": fmt.Sprint(core.DeploymentID),
		})
		if err != nil {
			return nil, err
		}
		return execStreamer{
			executor:  ks,
			namespace: pod.Namespace,
			podName:   pod.Name,
			container: "minio",
			command:   minioMirrorCommand(),
		}, nil
	}
}
