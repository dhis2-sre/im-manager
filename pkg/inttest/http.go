package inttest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/internal/server"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// SetupHTTPServer creates an HTTP server using Gin. An HTTP client is returned to interact with the
// created server.
func SetupHTTPServer(t *testing.T, f func(engine *gin.Engine)) *HTTPClient {
	t.Helper()

	err := handler.RegisterValidation()
	require.NoError(t, err, "failed to register validation")
	gin.SetMode(gin.TestMode)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := server.GetEngine(logger, "")
	f(engine)

	//goland:noinspection GoImportUsedAsName
	server := httptest.NewUnstartedServer(engine.Handler())
	listen, err := net.Listen("tcp", "0.0.0.0:0") // #nosec
	require.NoError(t, err)
	server.Listener = listen
	server.Start()

	client := server.Client()
	t.Cleanup(func() {
		client.CloseIdleConnections()
		server.Close()
	})

	return &HTTPClient{Client: client, ServerURL: server.URL}
}

// HTTPClient allows making requests in a way most of our handlers would expect them. It does so by
// wrapping a http.Client. Access the actual http.Client for specific use cases where our defaults don't
// work.
type HTTPClient struct {
	Client    *http.Client
	ServerURL string
}

func (hc *HTTPClient) GetHostname(t *testing.T) string {
	parsedUrl, err := url.Parse(hc.ServerURL)
	require.NoError(t, err)

	addrs, err := net.InterfaceAddrs()
	require.NoError(t, err)
	for _, address := range addrs {
		// check the address type and if it is not a loopback
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String() + ":" + parsedUrl.Port()
			}
		}
	}
	t.Error("failed getting ip")
	return ""
}

// WithHeader adds a header with the given key and value to HTTP request headers.
func WithHeader(key string, value string) func(http.Header) {
	return func(header http.Header) {
		header.Add(key, value)
	}
}

// WithBasicAuth adds a basic authorization header with the given user and password to HTTP request headers.
func WithBasicAuth(user string, password string) func(http.Header) {
	return WithHeader("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+password)))
}

// WithAuthToken adds an authorization header with the given bearer token to HTTP request headers.
func WithAuthToken(token string) func(http.Header) {
	return WithHeader("Authorization", "Bearer "+token)
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

// Do send an HTTP request of given method to given path. Optional headers are applied to the
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

// do delegate the request to the underlying HTTP client.
func (hc *HTTPClient) do(t *testing.T, req *http.Request) *http.Response {
	resp, err := hc.Client.Do(req)
	require.NoError(t, err, httpClientErrMessage(req.Method, req.URL.Path)+": HTTP request failed")
	return resp
}

// GetJSON sends an HTTP GET request to given path. Optional headers are applied to the request. The
// response body is unmarshalled as JSON into given responseBody. Failure to read or close the HTTP
// response body and HTTP status other than 200 will fail the test associated with t.
func (hc *HTTPClient) GetJSON(t *testing.T, path string, responseBody any, headers ...func(http.Header)) {
	t.Helper()

	body := hc.Get(t, path, headers...)

	err := json.Unmarshal(body, &responseBody)
	errMsg := httpClientErrMessage(http.MethodGet, path)
	require.NoError(t, err, errMsg+": failed to unmarshal response body")
}

// PostJSON sends an HTTP POST request to given path. Optional headers are applied to the request. The
// optional requestBody is assumed to be JSON. The response body is unmarshalled as JSON into given
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
