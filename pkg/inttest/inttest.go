// Package inttest enables writing of integration tests. Setup Docker containers for dependencies
// like PostgreSQL, RabbitMQ and AWS S3 (using localstack). Every setup function ensures the
// container is ready before returning, ensures resources are cleaned up after the tests are
// finished and return a client ready to interact with the container.
package inttest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-redis/redis"
	gnomockRedis "github.com/orlangure/gnomock/preset/redis"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/internal/server"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq" // postgres driver
	"github.com/orlangure/gnomock"
	"github.com/orlangure/gnomock/preset/localstack"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/orlangure/gnomock/preset/postgres"
	"github.com/orlangure/gnomock/preset/rabbitmq"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func SetupRedis(t *testing.T) *redis.Client {
	container, err := gnomock.Start(gnomockRedis.Preset())
	require.NoError(t, err, "failed to start Redis")
	t.Cleanup(func() { require.NoError(t, gnomock.Stop(container), "failed to stop Redis") })

	client := &redis.Options{
		Addr:     container.DefaultAddress(),
		Password: "",
		DB:       0,
	}
	return redis.NewClient(client)
}

// SetupDB creates a PostgreSQL container. Gorm is connected to the DB and runs the migrations.
func SetupDB(t *testing.T) *gorm.DB {
	t.Helper()

	container, err := gnomock.Start(
		postgres.Preset(
			postgres.WithUser("im", "im"),
			postgres.WithDatabase("test_im"),
		),
	)
	require.NoError(t, err, "failed to start DB")
	t.Cleanup(func() { require.NoError(t, gnomock.Stop(container), "failed to stop DB") })

	db, err := storage.NewDatabase(config.Postgresql{
		Host:         container.Host,
		Port:         container.DefaultPort(),
		Username:     "im",
		Password:     "im",
		DatabaseName: "test_im",
	})
	require.NoError(t, err, "failed to setup DB")
	return db
}

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

// SetupHTTPServer creates an HTTP server using Gin. An HTTP client is returned to interact with the
// created server.
func SetupHTTPServer(t *testing.T, f func(engine *gin.Engine)) *HTTPClient {
	t.Helper()

	err := handler.RegisterValidation()
	require.NoError(t, err, "failed to register validation")
	gin.SetMode(gin.TestMode)

	engine := server.GetEngine("")
	f(engine)

	server := httptest.NewServer(engine.Handler())
	client := server.Client()
	t.Cleanup(func() {
		client.CloseIdleConnections()
		server.Close()
	})

	return &HTTPClient{Client: client, ServerURL: server.URL}
}

// HTTPClient allows making requests in a way most of our handlers would expect them. It does so by
// wrapping an http.Client. Access the actual http.Client for specific use cases where our defaults don't
// work.
type HTTPClient struct {
	Client    *http.Client
	ServerURL string
}

// WithHeader adds a header with the given key and value to HTTP request headers.
func WithHeader(key string, value string) func(http.Header) {
	return func(header http.Header) {
		header.Add(key, value)
	}
}

// WithBasicAuth adds a basic authorization header with the given user and password to HTTP request headers.
func WithBasicAuth(user string, password string) func(http.Header) {
	return func(header http.Header) {
		header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+password)))
	}
}

// WithAuthToken adds an authorization header with the given bearer token to HTTP request headers.
func WithAuthToken(token string) func(http.Header) {
	return func(header http.Header) {
		header.Add("Authorization", "Bearer "+token)
	}
}

// Get sends an HTTP GET request to given path. Optional headers are applied to the request. The
// response body is read in full and returned as is. Failure to read or close the HTTP response body
// and HTTP status other than 200 will fail the test associated with t.
func (hc *HTTPClient) Get(t *testing.T, path string, headers ...func(http.Header)) []byte {
	t.Helper()
	return hc.Do(t, http.MethodGet, path, nil, http.StatusOK, headers...)
}

// Post sends an HTTP POST request to given path. Optional headers are applied to the request. The
// response body is read in full and returned as is. Failure to read or close the HTTP response body
// and HTTP status other than 201 will fail the test associated with t.
func (hc *HTTPClient) Post(t *testing.T, path string, requestBody io.Reader, headers ...func(http.Header)) []byte {
	t.Helper()
	return hc.Do(t, http.MethodPost, path, requestBody, http.StatusCreated, headers...)
}

// Delete sends an HTTP DELETE request to given path. Optional headers are applied to the request. The
// response body is read in full and returned as is. Failure to read or close the HTTP response body
// and HTTP status other than 202 will fail the test associated with t.
func (hc *HTTPClient) Delete(t *testing.T, path string, headers ...func(http.Header)) []byte {
	t.Helper()
	return hc.Do(t, http.MethodDelete, path, nil, http.StatusAccepted, headers...)
}

// Do sends an HTTP request of given method to given path. Optional headers are applied to the
// request. The response body is read in full and returned as is. Failure to read or close the HTTP
// response body and HTTP status other than given expectedStatus will fail the test associated with t.
func (hc *HTTPClient) Do(t *testing.T, method, path string, requestBody io.Reader, expectedStatus int, headers ...func(http.Header)) []byte {
	t.Helper()

	req := hc.newRequest(t, method, path, requestBody, headers...)
	res := hc.do(t, req)

	errMsg := httpClientErrMessage(method, path)
	defer func() {
		require.NoError(t, res.Body.Close(), errMsg+": failed to close HTTP response body")
	}()
	require.Equal(t, expectedStatus, res.StatusCode, errMsg+": HTTP status mismatch")
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err, errMsg+": failed to read HTTP response body")
	return body
}

// do delegates the request to the underlying HTTP client.
func (hc *HTTPClient) do(t *testing.T, req *http.Request) *http.Response {
	resp, err := hc.Client.Do(req)
	require.NoError(t, err, httpClientErrMessage(req.Method, req.URL.Path)+": HTTP request failed")
	return resp
}

// GetJSON sends an HTTP GET request to given path. Optional headers are applied to the request. The
// response body is unmarshaled as JSON into given responseBody. Failure to read or close the HTTP
// response body and HTTP status other than 200 will fail the test associated with t.
func (hc *HTTPClient) GetJSON(t *testing.T, path string, responseBody any, headers ...func(http.Header)) {
	t.Helper()

	body := hc.Get(t, path, headers...)

	err := json.Unmarshal(body, &responseBody)
	errMsg := httpClientErrMessage(http.MethodGet, path)
	require.NoError(t, err, errMsg+": failed to unmarshal response body")
}

// PostJSON sends an HTTP POST request to given path. Optional headers are applied to the request. The
// optional requestBody is assumed to be JSON. The response body is unmarshaled as JSON into given
// responseBody. Failure to read or close the HTTP response body and HTTP status other than 201
// will fail the test associated with t.
func (hc *HTTPClient) PostJSON(t *testing.T, path string, requestBody io.Reader, responseBody any, headers ...func(http.Header)) {
	t.Helper()

	if requestBody != nil {
		headers = append(headers, WithHeader("Content-Type", "application/json"))
	}
	body := hc.Post(t, path, requestBody, headers...)

	err := json.Unmarshal(body, &responseBody)
	errMsg := httpClientErrMessage(http.MethodPost, path)
	require.NoError(t, err, errMsg+": failed to unmarshal response body")
}

func httpClientErrMessage(method, path string) string {
	return fmt.Sprintf("failed %s %q", method, path)
}

// newRequest creates a new HTTP request to the server at given path after applying any optional
// headers.
func (hc *HTTPClient) newRequest(t *testing.T, method, path string, body io.Reader, headers ...func(http.Header)) *http.Request {
	t.Helper()

	req, err := http.NewRequest(method, hc.ServerURL+path, body)
	require.NoError(t, err, httpClientErrMessage(method, path)+": failed to create request")

	for _, f := range headers {
		f(req.Header)
	}

	return req
}

// SetupRabbitMQ creates a RabbitMQ container returning an AMQP client ready to send messages to it.
func SetupRabbitMQ(t *testing.T) *amqpTestClient {
	t.Helper()

	container, err := gnomock.Start(
		rabbitmq.Preset(
			rabbitmq.WithUser("im", "im"),
		),
	)
	require.NoError(t, err, "failed to start RabbitMQ")
	t.Cleanup(func() { require.NoError(t, gnomock.Stop(container), "failed to stop RabbitMQ") })

	URI := fmt.Sprintf(
		"amqp://%s:%s@%s",
		"im", "im",
		container.DefaultAddress(),
	)
	conn, err := amqp.Dial(URI)
	require.NoErrorf(t, err, "failed to connect to RabbitMQ", URI)
	t.Cleanup(func() {
		require.NoErrorf(t, conn.Close(), "failed to close connection to RabbitMQ")
	})

	ch, err := conn.Channel()
	require.NoErrorf(t, err, "failed to open channel to RabbitMQ")
	t.Cleanup(func() {
		require.NoErrorf(t, ch.Close(), "failed to close channel to RabbitMQ")
	})

	return &amqpTestClient{Channel: ch, URI: URI}
}

type amqpTestClient struct {
	Channel *amqp.Channel
	URI     string
}
