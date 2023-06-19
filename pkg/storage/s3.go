package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func NewS3Client(client AWSS3Client, uploader AWSS3Uploader) *S3Client {
	return &S3Client{client, uploader}
}

type S3Client struct {
	client   AWSS3Client
	uploader AWSS3Uploader
}

type AWSS3Client interface {
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
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

func (s S3Client) Upload(bucket string, key string, body ReadAtSeeker, size int64) error {
	target := path.Join(bucket, key)
	log.Printf("Uploading: " + target)
	reader, err := newProgressReader(body, size, func(read int64, size int64) {
		log.Printf("%s - total read:%d\tprogress:%d%%", target, read, int(float32(read*100)/float32(size)))
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

func (s S3Client) Download(bucket string, key string, dst io.Writer, cb func(contentLength int64)) error {
	object, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("error downloading object from bucket %q using key %q: %s", bucket, key, err)
	}

	cb(object.ContentLength)

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
