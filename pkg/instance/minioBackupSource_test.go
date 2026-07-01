package instance

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMinioClient struct {
	objects []minio.ObjectInfo
}

func (f fakeMinioClient) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	ch := make(chan minio.ObjectInfo, len(f.objects))
	for _, obj := range f.objects {
		ch <- obj
	}
	close(ch)
	return ch
}

func (f fakeMinioClient) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	return nil, errors.New("not implemented")
}

func TestMinioBackupSourceListSurfacesListingError(t *testing.T) {
	client := fakeMinioClient{
		objects: []minio.ObjectInfo{
			{Key: "apps/app1/manifest.json"},
			{Err: errors.New("connection reset")},
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	source := NewMinioBackupSource(logger, client, "dhis2")

	ch, err := source.List(context.Background())
	require.NoError(t, err)

	var received []BackupObject
	for obj := range ch {
		received = append(received, obj)
	}

	require.Len(t, received, 2)
	assert.Equal(t, "apps/app1/manifest.json", received[0].Path)
	require.Error(t, received[1].Err)
	assert.ErrorContains(t, received[1].Err, "connection reset")
}
