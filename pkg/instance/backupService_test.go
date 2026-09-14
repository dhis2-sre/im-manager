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
	"github.com/dhis2-sre/im-manager/pkg/storage"
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
	// nil uploader: PerformBackup uses StreamUpload, which only needs the multipart client methods.
	backupService := NewBackupService(logger, storage.NewS3Client(logger, s3Test.Client, nil))

	s3Key := "group/save-name-fs.tar.gz"
	uploaded, err := backupService.PerformBackup(ctx, s3APISource{source}, s3Bucket, s3Key)
	require.NoError(t, err)

	tarContent := s3Test.GetObject(t, s3Bucket, s3Key)
	assert.Equal(t, int64(len(tarContent)), uploaded, "the reported size is what landed in S3, so it can be recorded on the file store")
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

// TestFilestoreBackupRestoreRoundTrip covers the external S3 backend end to end, the one backend
// whose restore runs inside IM rather than in a seed script: objects are backed up out of one
// bucket and restored into another, which has to reproduce the original keys byte for byte or a
// restored DHIS 2 references file store objects that are not where it left them.
func TestFilestoreBackupRestoreRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	container, minioClient := setupMinio(t, ctx)
	defer func() {
		require.NoError(t, testcontainers.TerminateContainer(container))
	}()

	sourceBucket := "round-trip-source"
	targetBucket := "round-trip-target"
	require.NoError(t, minioClient.MakeBucket(ctx, sourceBucket, minio.MakeBucketOptions{}))
	require.NoError(t, minioClient.MakeBucket(ctx, targetBucket, minio.MakeBucketOptions{}))

	objects := map[string][]byte{
		"dataValue/uid1":          []byte("data-value-content"),
		"userAvatar/uid2":         []byte("avatar-content"),
		"apps/app1/manifest.json": []byte(`{"name":"app1"}`),
	}
	for key, content := range objects {
		_, err := minioClient.PutObject(ctx, sourceBucket, key, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{})
		require.NoError(t, err)
	}

	s3Dir := t.TempDir()
	s3Bucket := "database-bucket"
	require.NoError(t, os.Mkdir(s3Dir+"/"+s3Bucket, 0o755))
	s3Test := inttest.SetupS3(t, s3Dir)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	backupService := NewBackupService(logger, storage.NewS3Client(logger, s3Test.Client, nil))
	source := NewMinioBackupSource(logger, minioClient, sourceBucket)

	s3Key := "group/round-trip-fs.tar.gz"
	_, err := backupService.PerformBackup(ctx, s3APISource{source}, s3Bucket, s3Key)
	require.NoError(t, err)

	tarball := s3Test.GetObject(t, s3Bucket, s3Key)
	require.NoError(t, restoreTarGzToBucket(ctx, minioClient, targetBucket, bytes.NewReader(tarball)))

	for key, content := range objects {
		object, err := minioClient.GetObject(ctx, targetBucket, key, minio.GetObjectOptions{})
		require.NoErrorf(t, err, "restored object %q", key)
		restored, err := io.ReadAll(object)
		require.NoErrorf(t, err, "restored object %q", key)
		assert.Equalf(t, content, restored, "restored object %q", key)
	}
}

// TestPerformBackupWithoutS3Client asserts a service built without an S3 client reports it rather
// than panicking in the upload goroutine, where the error reaches no caller.
func TestPerformBackupWithoutS3Client(t *testing.T) {
	backupService := NewBackupService(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	_, err := backupService.PerformBackup(context.Background(), s3APISource{}, "bucket", "key")

	require.ErrorContains(t, err, "no S3 client")
}

// TestFilestoreRestoreMarker checks the guard that makes the external-S3 restore a
// one-time operation: the marker is absent on a fresh bucket and present once written,
// so a redeploy skips the restore instead of re-clobbering live filestore data.
func TestFilestoreRestoreMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	container, minioClient := setupMinio(t, ctx)
	defer func() {
		require.NoError(t, testcontainers.TerminateContainer(container))
	}()

	bucket := "restore-marker"
	require.NoError(t, minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}))

	restored, err := filestoreRestored(ctx, minioClient, bucket)
	require.NoError(t, err)
	assert.False(t, restored, "a fresh bucket has not been restored")

	require.NoError(t, markFilestoreRestored(ctx, minioClient, bucket))

	restored, err = filestoreRestored(ctx, minioClient, bucket)
	require.NoError(t, err)
	assert.True(t, restored, "the marker makes a subsequent restore a no-op")
}

func setupMinio(t *testing.T, ctx context.Context) (*minioContainer.MinioContainer, *minio.Client) {
	container, err := minioContainer.Run(ctx, "minio/minio:RELEASE.2025-01-20T14-49-07Z")
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
