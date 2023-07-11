package inttest

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/localstack"
	"github.com/stretchr/testify/require"
)

// SetupS3 creates an S3 container (using localstack) with all the buckets and files in given path.
func SetupS3(t *testing.T, path string) *S3Client {
	t.Helper()

	container, err := gnomock.Start(
		localstack.Preset(
			localstack.WithServices(localstack.S3),
			localstack.WithS3Files(path),
			localstack.WithVersion("2.1.0"),
		),
	)
	require.NoError(t, err, "failed to start S3")
	t.Cleanup(func() { require.NoError(t, gnomock.Stop(container), "failed to stop S3") })

	region := "eu-west-1"
	return &S3Client{
		Client: s3.NewFromConfig(
			aws.Config{
				Region: region,
				EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{
						URL:           fmt.Sprintf("http://%s/", container.Address(localstack.APIPort)),
						SigningRegion: region,
					}, nil
				}),
			},
			func(o *s3.Options) {
				o.UsePathStyle = true
			},
		),
	}
}

// S3Client allows making requests to S3. It does so by wrapping an S3.Client. Access the actual
// S3.Client for specific use cases where our defaults don't work.
type S3Client struct {
	Client *s3.Client
}

func (sc *S3Client) GetObject(t *testing.T, bucket, key string) []byte {
	t.Helper()

	object, err := sc.Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	errMsg := "failed GET from S3 bucket %q and key %q"
	require.NoErrorf(t, err, errMsg, bucket, key)
	body, err := io.ReadAll(object.Body)
	require.NoErrorf(t, err, errMsg+": failed to read body", bucket, key)
	return body
}
