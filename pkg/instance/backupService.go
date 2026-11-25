package instance

import (
	"context"
	"errors"
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

// errArchiveTruncated marks a file store archive whose tail did not arrive. A pod exec can report
// success having delivered only part of the command's stdout.
var errArchiveTruncated = errors.New("file store archive is truncated")

const truncatedArchiveAttempts = 3

// PerformBackup uploads the streamer's output to key in s3Bucket and returns the number of bytes
// written, which the caller records on the file store so its size is known. An archive that
// arrived truncated is not stored, and is streamed again a few times before the backup fails.
func (s *BackupService) PerformBackup(ctx context.Context, streamer filestoreStreamer, s3Bucket, key string) (int64, error) {
	start := time.Now()

	for attempt := 1; ; attempt++ {
		uploaded, err := s.upload(ctx, streamer, s3Bucket, key)
		if err == nil {
			s.logger.InfoContext(ctx, "Filestore backup completed", "key", key, "duration", time.Since(start), "bytesUploaded", uploaded)
			return uploaded, nil
		}
		if !errors.Is(err, errArchiveTruncated) || attempt >= truncatedArchiveAttempts {
			return 0, fmt.Errorf("backup failed: %w", err)
		}

		s.logger.WarnContext(ctx, "the file store archive arrived truncated, retrying", "key", key, "attempt", attempt, "error", err)
	}
}

func (s *BackupService) upload(ctx context.Context, streamer filestoreStreamer, s3Bucket, key string) (int64, error) {
	pr, pw := io.Pipe()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		verifier := storage.NewGzipVerifier(io.Discard)
		err := streamer.stream(ctx, io.MultiWriter(pw, verifier))
		if verifyErr := verifier.Close(); err == nil && verifyErr != nil {
			err = fmt.Errorf("%w: %v", errArchiveTruncated, verifyErr)
		}
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
		return 0, err
	}

	return uploaded, nil
}
