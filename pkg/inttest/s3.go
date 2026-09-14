package inttest

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/dhis2-sre/im-manager/internal/testenv"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	miniocredentials "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SetupS3 creates an isolated bucket on the shared LocalStack S3 service.
func SetupS3(t *testing.T) *S3Client {
	t.Helper()
	config, err := testenv.Resolve("s3")
	require.NoError(t, err)
	client := newS3(config.S3)
	return &S3Client{Client: client, Bucket: newBucket(t, client)}
}

// SetupMinIO creates an isolated bucket on the shared MinIO service.
func SetupMinIO(t *testing.T) (*minio.Client, string) {
	t.Helper()
	config, err := testenv.Resolve("minio")
	require.NoError(t, err)
	client, err := minio.New(config.MinIO, &minio.Options{
		Creds:  miniocredentials.NewStaticV4(testenv.AccessKey, testenv.SecretKey, ""),
		Secure: false,
	})
	require.NoError(t, err)
	return client, newBucket(t, newS3("http://"+config.MinIO))
}

func newS3(endpoint string) *s3.Client {
	return s3.NewFromConfig(aws.Config{
		Region:      testenv.Region,
		Credentials: credentials.NewStaticCredentialsProvider(testenv.AccessKey, testenv.SecretKey, ""),
	}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
}

func newBucket(t *testing.T, client *s3.Client) string {
	t.Helper()
	bucket := "im-test-" + uuid.NewString()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket:                    aws.String(bucket),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{LocationConstraint: types.BucketLocationConstraint(testenv.Region)},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Re-read from the beginning after each deleted page. No versioning is
		// enabled on fixture buckets, so current objects are the entire contents.
		for {
			objects, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
			if !assert.NoError(t, err) {
				return
			}
			if len(objects.Contents) == 0 {
				break
			}
			ids := make([]types.ObjectIdentifier, 0, len(objects.Contents))
			for _, object := range objects.Contents {
				ids = append(ids, types.ObjectIdentifier{Key: object.Key})
			}
			deleted, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(bucket), Delete: &types.Delete{Objects: ids},
			})
			if !assert.NoError(t, err) || !assert.Empty(t, deleted.Errors) {
				return
			}
		}
		for {
			uploads, err := client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{Bucket: aws.String(bucket)})
			if !assert.NoError(t, err) {
				return
			}
			if len(uploads.Uploads) == 0 {
				break
			}
			for _, upload := range uploads.Uploads {
				_, err := client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{Bucket: aws.String(bucket), Key: upload.Key, UploadId: upload.UploadId})
				if !assert.NoError(t, err) {
					return
				}
			}
		}
		_, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
		assert.NoError(t, err)
	})
	return bucket
}

// S3Client holds a real client and the bucket allocated to this fixture.
type S3Client struct {
	Client *s3.Client
	Bucket string
}

func (sc *S3Client) GetObject(t *testing.T, bucket, key string) []byte {
	t.Helper()
	body, err := sc.TryGetObject(bucket, key)
	require.NoErrorf(t, err, "GET from S3 bucket %q and key %q", bucket, key)
	return body
}

// TryGetObject supports polling for asynchronously uploaded objects.
func (sc *S3Client) TryGetObject(bucket, key string) ([]byte, error) {
	object, err := sc.Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket), Key: aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer object.Body.Close()
	return io.ReadAll(object.Body)
}
