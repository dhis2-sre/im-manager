package instance

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"golang.org/x/sync/errgroup"
)

// BackupSource defines a generic interface for backup sources
type BackupSource interface {
	// List returns a channel of objects to back up
	List(ctx context.Context) (<-chan BackupObject, error)
	// Get returns a reader for a specific object
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}

// BackupObject represents an object to be backed up
type BackupObject struct {
	Path         string
	Size         int64
	LastModified time.Time
	Err          error
}

const (
	bufferSize = 5 * 1024 * 1024 // 5 MB
)

// NewBackupService creates a new backup service instance
func NewBackupService(logger *slog.Logger, s3Client S3BackupClient) *BackupService {
	return &BackupService{logger, s3Client}
}

// BackupService streams a filestoreStreamer's output to S3 via multipart upload.
type BackupService struct {
	logger   *slog.Logger
	s3Client S3BackupClient
}

// S3BackupClient defines the methods we need from AWS S3 client
type S3BackupClient interface {
	CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
}

// PerformBackup runs streamer in a goroutine writing into a pipe whose reader is
// streamed to S3 via multipart upload.
func (s *BackupService) PerformBackup(ctx context.Context, streamer filestoreStreamer, s3Bucket, key string) error {
	g, ctx := errgroup.WithContext(ctx)
	start := time.Now()
	pr, pw := io.Pipe()

	g.Go(func() error {
		defer pw.Close()
		return streamer.stream(ctx, pw)
	})

	g.Go(func() error {
		return s.streamToS3WithMultipart(ctx, s3Bucket, key, pr)
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("backup failed: %v", err)
	}

	s.logger.InfoContext(ctx, "Filestore backup completed", "key", key, "duration", time.Since(start))
	return nil
}

func (s *BackupService) streamToS3WithMultipart(ctx context.Context, bucket, key string, reader io.Reader) error {
	createResponse, err := s.s3Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      &bucket,
		Key:         &key,
		ContentType: aws.String("application/x-gzip"),
	})
	if err != nil {
		return fmt.Errorf("create multipart upload: %v", err)
	}

	uploadID := *createResponse.UploadId
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

	partResponse, err := s.s3Client.UploadPart(ctx, partInput)
	if err != nil {
		return types.CompletedPart{}, fmt.Errorf("upload part %d: %v", partNumber, err)
	}

	return types.CompletedPart{
		PartNumber: aws.Int32(partNumber),
		ETag:       partResponse.ETag,
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
