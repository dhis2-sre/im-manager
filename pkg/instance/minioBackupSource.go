package instance

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/minio/minio-go/v7"
	"golang.org/x/sync/errgroup"
)

func NewMinioBackupSource(logger *slog.Logger, client MinioClient, bucket string) *MinioBackupSource {
	return &MinioBackupSource{logger, client, bucket}
}

// MinioClient defines the methods we need from MinIO client
type MinioClient interface {
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
}

// MinioBackupSource implements BackupSource for MinIO
type MinioBackupSource struct {
	logger *slog.Logger
	client MinioClient
	bucket string
}

// List implements BackupSource interface
func (m *MinioBackupSource) List(ctx context.Context) (<-chan BackupObject, error) {
	ch := make(chan BackupObject)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer close(ch)

		objectCh := m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{Recursive: true})

		for obj := range objectCh {
			if obj.Err != nil {
				return fmt.Errorf("list objects: %v", obj.Err)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- BackupObject{Path: obj.Key, Size: obj.Size, LastModified: obj.LastModified}:
			}
		}

		return nil
	})

	return ch, nil
}

// Get implements BackupSource interface
func (m *MinioBackupSource) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %v", path, err)
	}

	return obj, nil
}
