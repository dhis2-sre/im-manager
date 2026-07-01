package instance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	v1 "k8s.io/api/core/v1"
)

const (
	filestoreReadWorkers    = 16
	filestoreReadByteBudget = 256 << 20 // 256 MiB
)

// Objects larger than this stream straight into the tar instead of being buffered,
// so a single object can't exceed filestoreReadByteBudget.
var filestoreMaxBufferedObject int64 = 32 << 20 // 32 MiB

type filestoreStreamer interface {
	stream(ctx context.Context, w io.Writer) error
}

// writeTarGz builds a gzip'd tar from source. Objects up to filestoreMaxBufferedObject
// are fetched concurrently (bounded by filestoreReadWorkers and filestoreReadByteBudget);
// larger ones are streamed directly. A single goroutine owns the non-thread-safe tar.Writer.
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

	g.Go(func() error {
		defer close(results)

		fetchers, ctx := errgroup.WithContext(ctx)
		fetchers.SetLimit(filestoreReadWorkers)

		for object := range objectCh {
			if object.Err != nil {
				return object.Err
			}
			if object.Size > filestoreMaxBufferedObject {
				select {
				case results <- fetchedObject{object: object}:
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}
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

	g.Go(func() error {
		for f := range results {
			if f.data == nil {
				if err := streamTarObject(ctx, tw, source, f.object); err != nil {
					return err
				}
				continue
			}
			if err := writeTarEntry(tw, f.object, int64(len(f.data)), bytes.NewReader(f.data)); err != nil {
				return err
			}
		}
		return nil
	})

	return g.Wait()
}

// objectWeight is how many bytes an object reserves while it is fetched, capped at the
// total budget so one oversized object can still be fetched instead of waiting forever.
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

func streamTarObject(ctx context.Context, tw *tar.Writer, source BackupSource, object BackupObject) error {
	reader, err := source.Get(ctx, object.Path)
	if err != nil {
		return fmt.Errorf("failed to get object %s: %v", object.Path, err)
	}
	defer reader.Close()

	return writeTarEntry(tw, object, object.Size, reader)
}

func writeTarEntry(tw *tar.Writer, object BackupObject, size int64, r io.Reader) error {
	header := &tar.Header{
		Name:    object.Path,
		Size:    size,
		Mode:    0644,
		ModTime: object.LastModified,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header for %s: %v", object.Path, err)
	}
	if _, err := io.Copy(tw, r); err != nil {
		return fmt.Errorf("write object %s to tar: %v", object.Path, err)
	}
	return nil
}

// restoreTarGzToBucket uploads each file in a gzip'd tar to bucket under its key,
// stripping the filesystem backend's leading "./" so keys match across backends.
func restoreTarGzToBucket(ctx context.Context, client *minio.Client, bucket string, r io.Reader) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip stream: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %v", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		key := strings.TrimPrefix(header.Name, "./")
		if key == "" {
			continue
		}
		if _, err := client.PutObject(ctx, bucket, key, tr, header.Size, minio.PutObjectOptions{}); err != nil {
			return fmt.Errorf("put object %s: %v", key, err)
		}
	}
}

// ensureBucket creates bucket if absent so a restore can populate a fresh one.
func ensureBucket(ctx context.Context, client *minio.Client, bucket, region string) error {
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check bucket %q: %v", bucket, err)
	}
	if exists {
		return nil
	}
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}); err != nil {
		return fmt.Errorf("create bucket %q: %v", bucket, err)
	}
	return nil
}

// s3APISource streams a tar.gz of objects listed over the S3 API, for the external S3 backend.
type s3APISource struct {
	source BackupSource
}

func (s s3APISource) stream(ctx context.Context, w io.Writer) error {
	return writeTarGz(ctx, s.source, w)
}

type podExecutor interface {
	Exec(ctx context.Context, namespace, podName, container string, command []string, stdout, stderr io.Writer) error
}

// podExec runs commands in one pod container and wraps a failed command with its stderr.
type podExec struct {
	executor  podExecutor
	namespace string
	podName   string
	container string
}

func (p podExec) run(ctx context.Context, stdout io.Writer, command ...string) error {
	var stderr strings.Builder
	if err := p.executor.Exec(ctx, p.namespace, p.podName, p.container, command, stdout, &stderr); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}
	return nil
}

// execStreamer streams a command's stdout (a gzip'd tar) out of a pod.
type execStreamer struct {
	podExec
	command []string
}

func (e execStreamer) stream(ctx context.Context, w io.Writer) error {
	return e.run(ctx, w, e.command...)
}

// mc reads MC_HOST_<alias> to define a host alias inline; "backup" uses the minio stack defaults.
const minioClientHostEnv = "MC_HOST_backup=http://dhisdhis:dhisdhis@127.0.0.1:9000"

// minioExecSource backs up an in-cluster MinIO bucket by mirroring it to a pod temp dir
// with mc, then tarring that to w (mc can't tar to stdout and MinIO doesn't keep raw
// objects on disk). The staging copy needs ~filestore-size free ephemeral storage on the
// pod or it risks eviction.
type minioExecSource struct {
	podExec
}

func (m minioExecSource) stream(ctx context.Context, w io.Writer) error {
	var out strings.Builder
	if err := m.run(ctx, &out, "mktemp", "-d", "/tmp/im-filestore-backup-XXXXXXXX"); err != nil {
		return err
	}
	tmpDir := strings.TrimSpace(out.String())
	if tmpDir == "" {
		return fmt.Errorf("mktemp returned an empty path")
	}

	defer func() {
		// Detached from ctx so a cancelled or failed backup still removes the staging dir.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = m.run(cleanupCtx, io.Discard, "rm", "-rf", tmpDir)
	}()

	if err := m.run(ctx, io.Discard, "env", minioClientHostEnv, "mc", "mirror", "--quiet", "backup/dhis2", tmpDir); err != nil {
		return err
	}
	return m.run(ctx, w, "tar", "-C", tmpDir, "-czf", "-", ".")
}

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

func newExternalS3Client(core *model.DeploymentInstance) (*minio.Client, error) {
	endpoint := core.Parameters["S3_ENDPOINT"].Value
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}
	// minio.New wants a bare host; take TLS from the scheme, defaulting to HTTPS.
	secure := true
	if rest, ok := strings.CutPrefix(endpoint, "https://"); ok {
		endpoint = rest
	} else if rest, ok := strings.CutPrefix(endpoint, "http://"); ok {
		endpoint, secure = rest, false
	}
	region := core.Parameters["S3_REGION"].Value
	identity := core.Parameters["S3_IDENTITY"].Value
	secret := core.Parameters["S3_SECRET"].Value
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(identity, secret, ""),
		Secure: secure,
		Region: region,
	})
}

// filestoreStreamerFor selects the backend-specific streamer: minio and filesystem
// stream via pod exec, s3 reads the external bucket directly.
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
			podExec: podExec{
				executor:  ks,
				namespace: pod.Namespace,
				podName:   pod.Name,
				container: coreContainerName(pod, core.Name),
			},
			command: filesystemTarCommand(core.Parameters["DHIS2_HOME"].Value),
		}, nil
	case "s3":
		client, err := newExternalS3Client(core)
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
			podExec{
				executor:  ks,
				namespace: pod.Namespace,
				podName:   pod.Name,
				container: "minio",
			},
		}, nil
	}
}

// coreContainerName returns the container named after the release (the DHIS2 container),
// falling back to the first so a sidecar injected ahead of it isn't picked by mistake.
func coreContainerName(pod v1.Pod, instanceName string) string {
	for _, container := range pod.Spec.Containers {
		if container.Name == instanceName {
			return container.Name
		}
	}
	return pod.Spec.Containers[0].Name
}
