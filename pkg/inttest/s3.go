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
// The bucket is cleaned up after t and all its subtests finish.
func SetupS3(t *testing.T) *S3Client {
	t.Helper()
	address, err := testenv.Shared.S3()
	require.NoError(t, err)
	client := newS3(address)
	return &S3Client{Client: client, Bucket: newBucket(t, client)}
}

// SetupMinIO creates an isolated bucket on the shared MinIO service.
// The bucket is cleaned up after t and all its subtests finish.
func SetupMinIO(t *testing.T) *MinIOClient {
	t.Helper()
	address, err := testenv.Shared.MinIO()
	require.NoError(t, err)
	client, err := minio.New(address, &minio.Options{
		Creds:  miniocredentials.NewStaticV4(testenv.AccessKey, testenv.SecretKey, ""),
		Secure: false,
	})
	require.NoError(t, err)
	return &MinIOClient{Client: client, Bucket: newBucket(t, newS3("http://"+address))}
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

// MinIOClient holds a real client and the bucket allocated to this fixture.
type MinIOClient struct {
	Client *minio.Client
	Bucket string
}

// S3Client holds a real client and the bucket allocated to this fixture.
type S3Client struct {
	Client *s3.Client
	Bucket string
}

// GetObject reads a key from this fixture's bucket.
func (sc *S3Client) GetObject(t *testing.T, key string) []byte {
	t.Helper()
	body, err := sc.TryGetObject(key)
	require.NoErrorf(t, err, "GET from S3 bucket %q and key %q", sc.Bucket, key)
	return body
}

// TryGetObject reads from this fixture's bucket, supporting polling for asynchronous uploads.
func (sc *S3Client) TryGetObject(key string) ([]byte, error) {
	object, err := sc.Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(sc.Bucket), Key: aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer object.Body.Close()
	return io.ReadAll(object.Body)
}
