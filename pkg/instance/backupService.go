package instance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/minio/minio-go/v7"
	"golang.org/x/sync/errgroup"
)

const (
	bufferSize = 5 * 1024 * 1024 // 5 MB
)

// NewBackupService creates a new backup service instance
func NewBackupService(logger *slog.Logger, minioClient MinioClient, s3Client S3BackupClient) *BackupService {
	return &BackupService{
		minioClient: minioClient,
		s3Client:    s3Client,
		logger:      logger,
	}
}

// BackupService handles the backup operation
type BackupService struct {
	minioClient MinioClient
	s3Client    S3BackupClient
	logger      *slog.Logger
}

// MinioClient defines the methods we need from MinIO client
type MinioClient interface {
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
}

// S3BackupClient defines the methods we need from AWS S3 client
type S3BackupClient interface {
	CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
}

// BackupStats tracks backup operation statistics
type BackupStats struct {
	ObjectsProcessed int64
	BytesProcessed   int64
	StartTime        time.Time
	mu               sync.Mutex
}

func (bs *BackupStats) increment(size int64) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	bs.ObjectsProcessed++
	bs.BytesProcessed += size
}

// PerformBackup executes the backup operation
func (s *BackupService) PerformBackup(ctx context.Context, minioBucket, s3Bucket, key string) error {
	g, ctx := errgroup.WithContext(ctx)
	stats := &BackupStats{StartTime: time.Now()}
	pr, pw := io.Pipe()

	g.Go(func() error {
		defer pw.Close()
		return s.createTarGzStream(ctx, pw, minioBucket, stats)
	})

	g.Go(func() error {
		return s.streamToS3WithMultipart(ctx, s3Bucket, key, pr)
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("backup failed: %v", err)
	}

	s.logBackupStats(stats)
	return nil
}
func (s *BackupService) createTarGzStream(ctx context.Context, w io.Writer, minioBucket string, stats *BackupStats) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	objectCh := s.minioClient.ListObjects(ctx, minioBucket, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return fmt.Errorf("list objects: %v", object.Err)
		}

		if err := s.processSingleObject(ctx, tw, minioBucket, object, stats); err != nil {
			return err
		}
	}

	return nil
}

func (s *BackupService) processSingleObject(ctx context.Context, tw *tar.Writer, minioBucket string, object minio.ObjectInfo, stats *BackupStats) error {
	s.logger.InfoContext(ctx, "Processing object", "key", object.Key, "bucket", minioBucket)
	obj, err := s.minioClient.GetObject(ctx, minioBucket, object.Key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("get object %s: %v", object.Key, err)
	}
	defer obj.Close()

	header := &tar.Header{
		Name:    object.Key,
		Size:    object.Size,
		Mode:    0644,
		ModTime: object.LastModified,
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header for %s: %v", object.Key, err)
	}

	if _, err := io.Copy(tw, obj); err != nil {
		return fmt.Errorf("copy object %s to tar: %v", object.Key, err)
	}

	stats.increment(object.Size)
	return nil
}

func (s *BackupService) streamToS3WithMultipart(ctx context.Context, bucket, key string, reader io.Reader) error {
	createResp, err := s.s3Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      &bucket,
		Key:         &key,
		ContentType: aws.String("application/x-gzip"),
	})
	if err != nil {
		return fmt.Errorf("create multipart upload: %v", err)
	}

	uploadID := *createResp.UploadId
	var completedParts []types.CompletedPart
	defer s.cleanupFailedUpload(ctx, bucket, key, uploadID, &err)

	partNumber := int32(1)
	buffer := make([]byte, bufferSize)

	for {
		n, err := io.ReadFull(reader, buffer)
		if err != nil && err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("read from pipe: %v", err)
		}
		if n == 0 {
			break
		}

		part, err := s.uploadPart(ctx, bucket, key, uploadID, partNumber, buffer[:n])
		if err != nil {
			return err
		}

		completedParts = append(completedParts, part)
		partNumber++
	}

	return s.completeMultipartUpload(ctx, bucket, key, uploadID, completedParts)
}

func (s *BackupService) uploadPart(ctx context.Context, bucket, key string, uploadID string, partNumber int32, data []byte) (types.CompletedPart, error) {
	partInput := &s3.UploadPartInput{
		Bucket:     &bucket,
		Key:        &key,
		PartNumber: aws.Int32(partNumber),
		UploadId:   &uploadID,
		Body:       bytes.NewReader(data),
	}

	partResp, err := s.s3Client.UploadPart(ctx, partInput)
	if err != nil {
		return types.CompletedPart{}, fmt.Errorf("upload part %d: %v", partNumber, err)
	}

	return types.CompletedPart{
		PartNumber: aws.Int32(partNumber),
		ETag:       partResp.ETag,
	}, nil
}

func (s *BackupService) completeMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []types.CompletedPart) error {
	_, err := s.s3Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   &bucket,
		Key:      &key,
		UploadId: &uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
	if err != nil {
		return fmt.Errorf("complete multipart upload: %v", err)
	}
	return nil
}

func (s *BackupService) cleanupFailedUpload(ctx context.Context, bucket, key, uploadID string, err *error) {
	if *err != nil {
		if abortErr := s.abortMultipartUpload(ctx, bucket, key, uploadID); abortErr != nil {
			s.logger.ErrorContext(ctx, "Failed to abort multipart upload", "error", abortErr, "bucket", bucket, "key", key)
		}
	}
}

func (s *BackupService) abortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	_, err := s.s3Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   &bucket,
		Key:      &key,
		UploadId: &uploadID,
	})
	return err
}

func (s *BackupService) logBackupStats(stats *BackupStats) {
	duration := time.Since(stats.StartTime)
	s.logger.Info("Backup completed",
		"objects_processed", stats.ObjectsProcessed,
		"bytes_processed", stats.BytesProcessed,
		"duration", duration)
}
