package instance

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
	minioContainer "github.com/testcontainers/testcontainers-go/modules/minio"
)

func TestBackupService(t *testing.T) {
	// TODO: Don't skip
	t.SkipNow()

	ctx := context.Background()
	container, minioClient := setupMinio(t, ctx)
	defer func() {
		err := testcontainers.TerminateContainer(container)
		require.NoError(t, err)
	}()

	// Create test bucket and upload test file
	bucketName := "test-bucket"
	err := minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	require.NoError(t, err)

	// Create and upload test files
	testFiles := map[string][]byte{
		"test1.txt": []byte("test content 1"),
		"test2.txt": []byte("test content 2"),
	}

	for fileName, content := range testFiles {
		_, err = minioClient.PutObject(ctx, bucketName, fileName, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{})
		require.NoError(t, err)
	}

	s3Dir := t.TempDir()
	s3Bucket := "database-bucket"
	err = os.Mkdir(s3Dir+"/"+s3Bucket, 0o755)
	require.NoError(t, err, "failed to create S3 output bucket")
	s3 := inttest.SetupS3(t, s3Dir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	source := NewMinioBackupSource(logger, minioClient, "dhis2")
	backupService := NewBackupService(logger, source, s3.Client)

	err = backupService.PerformBackup(ctx, bucketName, "backup-key")
	require.NoError(t, err)
}

func setupMinio(t *testing.T, ctx context.Context) (*minioContainer.MinioContainer, *minio.Client) {
	container, err := minioContainer.Run(ctx, "minio/minio:latest")
	require.NoError(t, err)

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(container.Password, container.Password, ""),
		Secure: false,
	})
	require.NoError(t, err)

	return container, minioClient
}
