package instance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"os"
	"sort"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	minioContainer "github.com/testcontainers/testcontainers-go/modules/minio"
)

func TestBackupServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	container, minioClient := setupMinio(t, ctx)
	defer func() {
		require.NoError(t, testcontainers.TerminateContainer(container))
	}()

	minioBucket := "dhis2"
	require.NoError(t, minioClient.MakeBucket(ctx, minioBucket, minio.MakeBucketOptions{}))

	testFiles := map[string][]byte{
		"apps/app1/manifest.json": []byte(`{"name":"app1"}`),
		"userAvatar/uid1":         []byte("avatar-content"),
	}
	for name, content := range testFiles {
		_, err := minioClient.PutObject(ctx, minioBucket, name, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{})
		require.NoError(t, err)
	}

	s3Dir := t.TempDir()
	s3Bucket := "database-bucket"
	require.NoError(t, os.Mkdir(s3Dir+"/"+s3Bucket, 0o755))
	s3Test := inttest.SetupS3(t, s3Dir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	source := NewMinioBackupSource(logger, minioClient, minioBucket)
	backupService := NewBackupService(logger, source, s3Test.Client)

	s3Key := "group/save-name-fs.tar.gz"
	require.NoError(t, backupService.PerformBackup(ctx, s3Bucket, s3Key))

	tarContent := s3Test.GetObject(t, s3Bucket, s3Key)
	entries := extractTarGz(t, tarContent)

	var paths []string
	for p := range entries {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var expected []string
	for p := range testFiles {
		expected = append(expected, p)
	}
	sort.Strings(expected)

	assert.Equal(t, expected, paths)
	for name, content := range testFiles {
		assert.Equal(t, content, entries[name], "content mismatch for %s", name)
	}
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

func extractTarGz(t *testing.T, data []byte) map[string][]byte {
	t.Helper()

	gr, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer gr.Close()

	tr := tar.NewReader(gr)
	entries := make(map[string][]byte)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		content, err := io.ReadAll(tr)
		require.NoError(t, err)
		entries[hdr.Name] = content
	}
	return entries
}
