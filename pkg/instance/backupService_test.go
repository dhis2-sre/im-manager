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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	minioClient, minioBucket := inttest.SetupMinIO(t)

	testFiles := map[string][]byte{
		"apps/app1/manifest.json": []byte(`{"name":"app1"}`),
		"userAvatar/uid1":         []byte("avatar-content"),
	}
	for name, content := range testFiles {
		_, err := minioClient.PutObject(ctx, minioBucket, name, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{})
		require.NoError(t, err)
	}

	s3Test := inttest.SetupS3(t)
	s3Bucket := s3Test.Bucket

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	source := NewMinioBackupSource(logger, minioClient, minioBucket)
	// nil uploader: PerformBackup uses StreamUpload, which only needs the multipart client methods.
	backupService := NewBackupService(logger, storage.NewS3Client(logger, s3Test.Client, nil))

	s3Key := "group/save-name-fs.tar.gz"
	require.NoError(t, backupService.PerformBackup(ctx, s3APISource{source}, s3Bucket, s3Key))

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

// TestFilestoreRestoreMarker checks the guard that makes the external-S3 restore a
// one-time operation: the marker is absent on a fresh bucket and present once written,
// so a redeploy skips the restore instead of re-clobbering live filestore data.
func TestFilestoreRestoreMarker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	minioClient, bucket := inttest.SetupMinIO(t)

	restored, err := filestoreRestored(ctx, minioClient, bucket)
	require.NoError(t, err)
	assert.False(t, restored, "a fresh bucket has not been restored")

	require.NoError(t, markFilestoreRestored(ctx, minioClient, bucket))

	restored, err = filestoreRestored(ctx, minioClient, bucket)
	require.NoError(t, err)
	assert.True(t, restored, "the marker makes a subsequent restore a no-op")
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
