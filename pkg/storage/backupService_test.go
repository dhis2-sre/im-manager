package storage

import (
	"bytes"
	"context"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/localstack"
	"github.com/testcontainers/testcontainers-go"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	minioContainer "github.com/testcontainers/testcontainers-go/modules/minio"
)

func TestBackupService(t *testing.T) {
	ctx := context.Background()
	container, err := minioContainer.Run(ctx, "minio/minio:latest")
	defer func() {
		err := testcontainers.TerminateContainer(container)
		require.NoError(t, err)
	}()
	require.NoError(t, err)

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(container.Password, container.Password, ""),
		Secure: false,
	})
	require.NoError(t, err)

	// Create test bucket and upload test file
	bucketName := "test-bucket"
	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
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

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	s3Dir := t.TempDir()
	s3Bucket := "database-bucket"
	err = os.Mkdir(s3Dir+"/"+s3Bucket, 0o755)
	require.NoError(t, err, "failed to create S3 output bucket")

	s3Container, err := gnomock.Start(
		localstack.Preset(
			localstack.WithServices(localstack.S3),
			localstack.WithS3Files(s3Dir+"/"+s3Bucket),
			localstack.WithVersion("2.1.0"),
		),
	)
	require.NoError(t, err, "failed to start S3")
	t.Cleanup(func() { require.NoError(t, gnomock.Stop(s3Container), "failed to stop S3") })
	/*
		s3Client := s3.NewFromConfig(
			aws.Config{
				Region: "",
				EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{
						URL:           fmt.Sprintf("http://%s/", s3Container.Host),
						SigningRegion: region,
					}, nil
				}),
			},
			func(o *s3.Options) {
				o.UsePathStyle = true
			},
		)
	*/
	backupService, err := NewBackupService(logger, minioClient, nil)
	require.NoError(t, err)

	err = backupService.PerformBackup(ctx, bucketName, "backup-bucket", "backup-key")
	require.NoError(t, err)
}
