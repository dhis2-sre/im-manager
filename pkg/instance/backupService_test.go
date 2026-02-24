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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

func TestBackupServiceUnit(t *testing.T) {
	t.Parallel()

	testFiles := map[string][]byte{
		"apps/app1/manifest.json": []byte(`{"name":"app1"}`),
		"userAvatar/uid1":         []byte("avatar-content"),
	}

	source := &memoryBackupSource{objects: testFiles}
	s3Mock := &mockS3BackupClient{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	backupService := NewBackupService(logger, source, s3Mock)

	ctx := context.Background()
	err := backupService.PerformBackup(ctx, "dest-bucket", "group/backup.tar.gz")
	require.NoError(t, err)

	require.True(t, s3Mock.createCalled, "CreateMultipartUpload should be called")
	require.True(t, s3Mock.completeCalled, "CompleteMultipartUpload should be called")
	require.False(t, s3Mock.abortCalled, "AbortMultipartUpload should not be called")
	require.Greater(t, len(s3Mock.uploadedParts), 0, "at least one part should be uploaded")

	var allData []byte
	for _, part := range s3Mock.uploadedParts {
		allData = append(allData, part...)
	}

	entries := extractTarGz(t, allData)

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

// memoryBackupSource implements BackupSource with in-memory objects.
type memoryBackupSource struct {
	objects map[string][]byte
}

func (m *memoryBackupSource) List(ctx context.Context) (<-chan BackupObject, error) {
	ch := make(chan BackupObject)
	go func() {
		defer close(ch)
		for path, data := range m.objects {
			ch <- BackupObject{
				Path:         path,
				Size:         int64(len(data)),
				LastModified: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			}
		}
	}()
	return ch, nil
}

func (m *memoryBackupSource) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(m.objects[path])), nil
}

// mockS3BackupClient records calls to S3 multipart upload methods.
type mockS3BackupClient struct {
	createCalled   bool
	completeCalled bool
	abortCalled    bool
	uploadedParts  [][]byte
	partCounter    int32
}

func (m *mockS3BackupClient) CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	m.createCalled = true
	return &s3.CreateMultipartUploadOutput{
		UploadId: aws.String("test-upload-id"),
		Bucket:   params.Bucket,
		Key:      params.Key,
	}, nil
}

func (m *mockS3BackupClient) UploadPart(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	data, _ := io.ReadAll(params.Body)
	m.uploadedParts = append(m.uploadedParts, data)
	m.partCounter++
	etag := "etag-" + string(rune('0'+m.partCounter))
	return &s3.UploadPartOutput{
		ETag: aws.String(etag),
	}, nil
}

func (m *mockS3BackupClient) CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	m.completeCalled = true
	return &s3.CompleteMultipartUploadOutput{}, nil
}

func (m *mockS3BackupClient) AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	m.abortCalled = true
	return &s3.AbortMultipartUploadOutput{}, nil
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
