package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func NewS3Client(logger *slog.Logger, client AWSS3Client, uploader AWSS3Uploader) *S3Client {
	return &S3Client{
		logger:   logger,
		client:   client,
		uploader: uploader,
	}
}

type S3Client struct {
	logger   *slog.Logger
	client   AWSS3Client
	uploader AWSS3Uploader
}

type AWSS3Client interface {
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)

	CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	UploadPart(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
}

type AWSS3Uploader interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

func (s S3Client) Copy(bucket string, source string, destination string) error {
	_, err := s.client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(bucket + "/" + source),
		Key:        aws.String(destination),
		ACL:        types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return fmt.Errorf("error copying object from %q to %q: %s", source, destination, err)
	}
	return nil
}

func (s S3Client) Move(bucket string, source string, destination string) error {
	err := s.Copy(bucket, source, destination)
	if err != nil {
		return fmt.Errorf("error moving object from %q to %q during copy operation: %s", source, destination, err)
	}

	err = s.Delete(bucket, source)
	if err != nil {
		return fmt.Errorf("error moving object from bucket %q using key %q during delete operation: %s", bucket, source, err)
	}

	return nil
}

func (s S3Client) Upload(ctx context.Context, bucket string, key string, body ReadAtSeeker, size int64) error {
	// only use ctx for values (logging) and not cancellation signals for now. lets discuss if we
	// want to cancel the upload if the client cancels the request first and make sure we align that
	// with how we handle the DB context after the upload.
	ctx = context.WithoutCancel(ctx)

	s.logger.InfoContext(ctx, "Uploading", "bucket", bucket, "key", key)
	reader, err := newProgressReader(body, size, func(read int64, size int64) {
		// TODO(DEVOPS-390) this is meant to be read by users but this implementation does not work
		fmt.Fprintf(os.Stdout, "%s/%s - total read:%d\tprogress:%d%%", bucket, key, read, int(float32(read*100)/float32(size)))
	})
	if err != nil {
		return err
	}

	_, err = s.uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   reader,
		ACL:    types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return fmt.Errorf("error uploading object to bucket %q using key %q: %s", bucket, key, err)
	}
	return nil
}

func (s S3Client) Delete(bucket string, key string) error {
	_, err := s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("error deleting object from bucket %q using key %q: %s", bucket, key, err)
	}
	return nil
}

func (s S3Client) Download(ctx context.Context, bucket string, key string, dst io.Writer, cb func(contentLength int64)) error {
	object, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("error downloading object from bucket %q using key %q: %s", bucket, key, err)
	}

	cb(*object.ContentLength)

	_, err = io.Copy(dst, object.Body)

	return err
}

type ReadAtSeeker interface {
	io.ReaderAt
	io.ReadSeeker
}

func newProgressReader(fp ReadAtSeeker, size int64, progress func(read int64, size int64)) (*progressReader, error) {
	return &progressReader{fp, size, 0, progress}, nil
}

// be aware that this reader is not safe for concurrent use
type progressReader struct {
	fp       ReadAtSeeker
	size     int64
	read     int64
	progress func(read int64, size int64)
}

func (r *progressReader) Read(p []byte) (int, error) {
	return r.fp.Read(p)
}

func (r *progressReader) ReadAt(p []byte, off int64) (int, error) {
	n, err := r.fp.ReadAt(p, off)
	if err != nil {
		return n, err
	}

	r.read += int64(n)
	r.progress(r.read, r.size)

	return n, err
}

func (r *progressReader) Seek(offset int64, whence int) (int64, error) {
	return r.fp.Seek(offset, whence)
}

func (s S3Client) InitiateMultipartUpload(ctx context.Context, bucket, key string) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket: &bucket,
		Key:    &key,
	}
	resp, err := s.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", err
	}
	return *resp.UploadId, nil
}

func (s S3Client) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int, data []byte) (*types.CompletedPart, error) {
	resp, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     &bucket,
		Key:        &key,
		UploadId:   &uploadID,
		PartNumber: aws.Int32(int32(partNumber)),
		Body:       bytes.NewReader(data),
	})
	if err != nil {
		return nil, err
	}

	completedPart := &types.CompletedPart{
		ETag:       resp.ETag,
		PartNumber: aws.Int32(int32(partNumber)),
	}
	return completedPart, nil
}

func (s S3Client) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, completedParts []types.CompletedPart) error {
	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   &bucket,
		Key:      &key,
		UploadId: &uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	return err
}

func (s S3Client) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   &bucket,
		Key:      &key,
		UploadId: &uploadID,
	})
	return err
}
