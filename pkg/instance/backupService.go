package instance

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

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

// s3StreamUploader streams a reader to an object in S3.
type s3StreamUploader interface {
	StreamUpload(ctx context.Context, bucket, key, contentType string, r io.Reader) (int64, error)
}

// NewBackupService creates a new backup service instance
func NewBackupService(logger *slog.Logger, uploader s3StreamUploader) *BackupService {
	return &BackupService{logger: logger, uploader: uploader}
}

// BackupService streams a filestoreStreamer's gzip'd tar to S3.
type BackupService struct {
	logger   *slog.Logger
	uploader s3StreamUploader
}

// PerformBackup runs streamer in a goroutine writing into a pipe whose reader is
// streamed to S3.
func (s *BackupService) PerformBackup(ctx context.Context, streamer filestoreStreamer, s3Bucket, key string) error {
	start := time.Now()
	pr, pw := io.Pipe()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		err := streamer.stream(ctx, pw)
		pw.CloseWithError(err)
		return err
	})
	g.Go(func() error {
		_, err := s.uploader.StreamUpload(ctx, s3Bucket, key, "application/x-gzip", pr)
		pr.CloseWithError(err)
		return err
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("backup failed: %v", err)
	}

	s.logger.InfoContext(ctx, "Filestore backup completed", "key", key, "duration", time.Since(start))
	return nil
}
