package instance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"testing"

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
		"apps/app1/manifest.webapp": []byte(`{"name":"app1"}`),
		"userAvatar/uid1":           []byte("avatar-content"),
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
	source := NewMinioBackupSource(logger, minioClient, minioBucket, "apps/")
	backupService := NewBackupService(logger, source, s3Test.Client)

	s3Key := "group/save-name-fs.tar.gz"
	require.NoError(t, backupService.PerformBackup(ctx, s3Bucket, s3Key))

	tarContent := s3Test.GetObject(t, s3Bucket, s3Key)
	entries := extractTarGz(t, tarContent)

	assert.Equal(t, []string{"userAvatar/uid1"}, sortedKeys(entries), "apps/ should be excluded from backup")
	assert.Equal(t, testFiles["userAvatar/uid1"], entries["userAvatar/uid1"])
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

func TestPerformBackupAbortsOnSourceError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	source := &errorAfterFirstObjectSource{content: []byte("hello")}
	client := &fakeS3BackupClient{}
	svc := NewBackupService(logger, source, client)

	err := svc.PerformBackup(context.Background(), "bucket", "key")

	require.Error(t, err)
	assert.False(t, client.completed, "CompleteMultipartUpload should not be called after a source error")
	assert.True(t, client.aborted, "AbortMultipartUpload should be called after a source error")
}

// errorAfterFirstObjectSource returns content for the first object and an error for all subsequent ones.
type errorAfterFirstObjectSource struct {
	content []byte
	seen    int
}

func (s *errorAfterFirstObjectSource) List(_ context.Context) (<-chan BackupObject, error) {
	ch := make(chan BackupObject, 2)
	ch <- BackupObject{Path: "file1.txt", Size: int64(len(s.content))}
	ch <- BackupObject{Path: "file2.txt", Size: int64(len(s.content))}
	close(ch)
	return ch, nil
}

func (s *errorAfterFirstObjectSource) Get(_ context.Context, path string) (io.ReadCloser, error) {
	s.seen++
	if s.seen > 1 {
		return nil, fmt.Errorf("simulated error reading %q", path)
	}
	return io.NopCloser(bytes.NewReader(s.content)), nil
}

// fakeS3BackupClient tracks whether CompleteMultipartUpload or AbortMultipartUpload was called.
type fakeS3BackupClient struct {
	completed bool
	aborted   bool
}

func (f *fakeS3BackupClient) CreateMultipartUpload(_ context.Context, _ *s3.CreateMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	return &s3.CreateMultipartUploadOutput{UploadId: aws.String("upload-id")}, nil
}

func (f *fakeS3BackupClient) UploadPart(_ context.Context, _ *s3.UploadPartInput, _ ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	return &s3.UploadPartOutput{ETag: aws.String("etag")}, nil
}

func (f *fakeS3BackupClient) CompleteMultipartUpload(_ context.Context, _ *s3.CompleteMultipartUploadInput, _ ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	f.completed = true
	return &s3.CompleteMultipartUploadOutput{}, nil
}

func (f *fakeS3BackupClient) AbortMultipartUpload(_ context.Context, _ *s3.AbortMultipartUploadInput, _ ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	f.aborted = true
	return &s3.AbortMultipartUploadOutput{}, nil
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
