// Should we upgrade our scaling to save the ingress, replace it with something generic and then restore it when scaling back up?
//
// Slug needs to include type (fs or sql)

package instance

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/minio/minio-go/v7"
	"golang.org/x/sync/errgroup"
)

func NewRestoreService(logger *slog.Logger, minioClient MinioRestoreClient, s3Client S3RestoreClient) *RestoreService {
	return &RestoreService{
		minioClient: minioClient,
		s3Client:    s3Client,
		logger:      logger,
	}
}

type RestoreService struct {
	minioClient MinioRestoreClient
	s3Client    S3RestoreClient
	logger      *slog.Logger
}

type MinioRestoreClient interface {
	PutObject(ctx context.Context, bucket string, name string, reader io.Reader, size int64, options minio.PutObjectOptions) (minio.UploadInfo, error)
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	RemoveObjects(ctx context.Context, bucketName string, objectsCh <-chan minio.ObjectInfo, opts minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError
}

type S3RestoreClient interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type RestoreStats struct {
	ObjectsRestored int64
	BytesRestored   int64
	StartTime       time.Time
	mu              sync.Mutex
}

func (rs *RestoreStats) increment(size int64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.ObjectsRestored++
	rs.BytesRestored += size
}

func (s *RestoreService) PerformRestore(ctx context.Context, s3Bucket, s3Key, minioBucket string) error {
	stats := &RestoreStats{StartTime: time.Now()}

	output, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s3Bucket,
		Key:    &s3Key,
	})
	if err != nil {
		return fmt.Errorf("get s3 object: %w", err)
	}
	if output == nil || output.Body == nil {
		return errors.New("received nil response or body from S3")
	}
	defer output.Body.Close()

	gzReader, err := gzip.NewReader(output.Body)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	objectCh := make(chan *tar.Header)

	g, ctx := errgroup.WithContext(ctx)

	// Start header reader goroutine
	g.Go(func() error {
		defer close(objectCh)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("read tar header: %w", err)
			}

			select {
			case objectCh <- header:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	// Start restore worker goroutine
	g.Go(func() error {
		for header := range objectCh {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err := s.restoreObject(ctx, minioBucket, header, tarReader, stats); err != nil {
				return fmt.Errorf("restore object %s: %w", header.Name, err)
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	s.logRestoreStats(stats)
	return nil
}

func (s *RestoreService) PerformPurge(ctx context.Context, minioBucket string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	deleteCh := make(chan minio.ObjectInfo, 1000)

	// List objects
	g.Go(func() error {
		defer close(deleteCh)
		for object := range s.minioClient.ListObjects(ctx, minioBucket, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err != nil {
				s.logger.ErrorContext(ctx, "Failed to list object", "error", object.Err, "bucket", minioBucket)
				continue
			}
			select {
			case deleteCh <- object:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	// Delete objects
	g.Go(func() error {
		for err := range s.minioClient.RemoveObjects(ctx, minioBucket, deleteCh, minio.RemoveObjectsOptions{}) {
			s.logger.ErrorContext(ctx, "Failed to delete object", "error", err, "bucket", minioBucket)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error during purge: %w", err)
	}

	s.logger.InfoContext(ctx, "Batch deletion completed", "bucket", minioBucket)
	return nil
}

func (s *RestoreService) restoreObject(ctx context.Context, minioBucket string, header *tar.Header, reader io.Reader, stats *RestoreStats) error {
	if header == nil {
		return errors.New("header is required")
	}

	s.logger.InfoContext(ctx, "Restoring object", "key", header.Name, "bucket", minioBucket)

	pr, pw := io.Pipe()
	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	// Start upload goroutine
	go func() {
		defer wg.Done()
		_, err := s.minioClient.PutObject(ctx, minioBucket, header.Name, pr, header.Size, minio.PutObjectOptions{})
		if err != nil {
			pr.CloseWithError(err)
			errCh <- fmt.Errorf("put object: %w", err)
		}
	}()

	// Start copy goroutine
	go func() {
		defer wg.Done()
		written, err := io.CopyN(pw, reader, header.Size)
		if err != nil {
			pw.CloseWithError(err)
			errCh <- fmt.Errorf("copy object data (wrote %d/%d bytes): %w", written, header.Size, err)
			return
		}

		if written != header.Size {
			err := fmt.Errorf("incomplete copy: wrote %d/%d bytes", written, header.Size)
			pw.CloseWithError(err)
			errCh <- err
			return
		}

		if err := pw.Close(); err != nil {
			errCh <- fmt.Errorf("error closing pipe writer: %w", err)
		}
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}

	stats.increment(header.Size)
	return nil
}

func (s *RestoreService) logRestoreStats(stats *RestoreStats) {
	if stats == nil {
		s.logger.Error("Cannot log nil stats")
		return
	}

	duration := time.Since(stats.StartTime)
	s.logger.Info("Restore completed",
		"objects_restored", stats.ObjectsRestored,
		"bytes_restored", stats.BytesRestored,
		"duration", duration)
}
