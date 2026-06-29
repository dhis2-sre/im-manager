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
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const (
	// filestoreReadWorkers caps how many objects are fetched from the source
	// bucket concurrently.
	filestoreReadWorkers = 16
	// filestoreReadByteBudget caps the total bytes of fetched-but-not-yet-written
	// objects held in memory at once.
	filestoreReadByteBudget = 256 << 20 // 256 MiB
)

// filestoreStreamer writes a gzip'd tar of an instance's filestore to w.
type filestoreStreamer interface {
	stream(ctx context.Context, w io.Writer) error
}

// writeTarGz builds a gzip'd tar from a BackupSource and writes it to w. Objects
// are fetched concurrently; tar entry order does not matter because the archive
// is restored by object key.
func writeTarGz(ctx context.Context, source BackupSource, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	objectCh, err := source.List(ctx)
	if err != nil {
		return fmt.Errorf("list objects: %v", err)
	}

	type fetchedObject struct {
		object BackupObject
		data   []byte
	}
	results := make(chan fetchedObject)
	budget := semaphore.NewWeighted(filestoreReadByteBudget)

	g, ctx := errgroup.WithContext(ctx)

	// fetch objects concurrently, bounded by worker count and the byte budget
	g.Go(func() error {
		defer close(results)

		fetchers, ctx := errgroup.WithContext(ctx)
		fetchers.SetLimit(filestoreReadWorkers)

		for object := range objectCh {
			if object.Err != nil {
				return object.Err
			}
			object := object
			fetchers.Go(func() error {
				weight := objectWeight(object.Size)
				if err := budget.Acquire(ctx, weight); err != nil {
					return err
				}
				defer budget.Release(weight)

				data, err := readObject(ctx, source, object.Path)
				if err != nil {
					return err
				}
				select {
				case results <- fetchedObject{object: object, data: data}:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}
		return fetchers.Wait()
	})

	// one goroutine owns the tar.Writer, which is not concurrency-safe
	g.Go(func() error {
		for f := range results {
			if err := writeTarBytes(tw, f.object, f.data); err != nil {
				return err
			}
		}
		return nil
	})

	return g.Wait()
}

// objectWeight clamps size into [1, budget] so a single oversized object cannot
// exceed the semaphore's limit.
func objectWeight(size int64) int64 {
	switch {
	case size <= 0:
		return 1
	case size > filestoreReadByteBudget:
		return filestoreReadByteBudget
	default:
		return size
	}
}

func readObject(ctx context.Context, source BackupSource, path string) ([]byte, error) {
	reader, err := source.Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %v", path, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read object %s: %v", path, err)
	}
	return data, nil
}

func writeTarBytes(tw *tar.Writer, object BackupObject, data []byte) error {
	header := &tar.Header{
		Name:    object.Path,
		Size:    int64(len(data)),
		Mode:    0644,
		ModTime: object.LastModified,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header for %s: %v", object.Path, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write object %s to tar: %v", object.Path, err)
	}
	return nil
}

// s3APISource streams a tar.gz of objects listed over the S3 API, for the
// external S3 backend.
type s3APISource struct {
	source BackupSource
}

func (s s3APISource) stream(ctx context.Context, w io.Writer) error {
	return writeTarGz(ctx, s.source, w)
}

// podExecutor runs a command in a pod container, streaming stdout/stderr.
type podExecutor interface {
	Exec(ctx context.Context, namespace, podName, container string, command []string, stdout, stderr io.Writer) error
}

// execStreamer streams a command's stdout (a gzip'd tar) out of a pod.
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

// minioClientHostEnv registers the mc host alias "backup" inline via mc's
// MC_HOST_<alias> convention, with the minio stack's default credentials.
const minioClientHostEnv = "MC_HOST_backup=http://dhisdhis:dhisdhis@127.0.0.1:9000"

// minioExecSource backs up an in-cluster MinIO bucket by mirroring it to a temp
// dir with mc, then tarring that to w. Reading via mc is required because MinIO
// does not store the raw objects on disk.
type minioExecSource struct {
	executor  podExecutor
	namespace string
	podName   string
	container string
	tmpDir    string
}

func (m minioExecSource) stream(ctx context.Context, w io.Writer) error {
	defer func() {
		// best-effort cleanup
		_ = m.exec(ctx, io.Discard, "rm", "-rf", m.tmpDir)
	}()

	if err := m.exec(ctx, io.Discard, "env", minioClientHostEnv, "mc", "mirror", "--quiet", "backup/dhis2", m.tmpDir); err != nil {
		return err
	}
	return m.exec(ctx, w, "tar", "-C", m.tmpDir, "-czf", "-", ".")
}

func (m minioExecSource) exec(ctx context.Context, stdout io.Writer, command ...string) error {
	var stderr strings.Builder
	if err := m.executor.Exec(ctx, m.namespace, m.podName, m.container, command, stdout, &stderr); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

// minioTempDir returns a per-backup temp dir derived from the backup name so
// concurrent backups of the same pod don't collide.
func minioTempDir(backupName string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, backupName)
	return "/tmp/im-filestore-backup-" + safe
}

// filesystemTarCommand tars DHIS2_HOME/files to stdout as argv (no shell).
func filesystemTarCommand(dhis2Home string) []string {
	files := strings.TrimRight(dhis2Home, "/") + "/files"
	return []string{"tar", "-C", files, "-czf", "-", "."}
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

// filestoreStreamerFor selects the backend-specific streamer: minio and
// filesystem stream via pod exec, s3 reads the external bucket directly.
func (s Service) filestoreStreamerFor(core *model.DeploymentInstance, cluster model.Cluster, backupName string) (filestoreStreamer, error) {
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
		return minioExecSource{
			executor:  ks,
			namespace: pod.Namespace,
			podName:   pod.Name,
			container: "minio",
			tmpDir:    minioTempDir(backupName),
		}, nil
	}
}
