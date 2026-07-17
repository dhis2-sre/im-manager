package instance

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/storage"
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

func NewBackupService(logger *slog.Logger, uploader *storage.S3Client) *BackupService {
	return &BackupService{logger: logger, uploader: uploader}
}

// BackupService streams a filestoreStreamer's gzip'd tar to S3.
type BackupService struct {
	logger   *slog.Logger
	uploader *storage.S3Client
}

// PerformBackup uploads the streamer's output to key in s3Bucket.
func (s *BackupService) PerformBackup(ctx context.Context, streamer filestoreStreamer, s3Bucket, key string) error {
	start := time.Now()
	pr, pw := io.Pipe()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		err := streamer.stream(ctx, pw)
		pw.CloseWithError(err)
		return err
	})
	var uploaded int64
	g.Go(func() error {
		n, err := s.uploader.StreamUpload(ctx, s3Bucket, key, "application/x-gzip", pr)
		uploaded = n
		pr.CloseWithError(err)
		return err
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("backup failed: %v", err)
	}

	s.logger.InfoContext(ctx, "Filestore backup completed", "key", key, "duration", time.Since(start))
	s.logger.DebugContext(ctx, "Filestore backup stats", "key", key, "bytesUploaded", uploaded)
	return nil
}
