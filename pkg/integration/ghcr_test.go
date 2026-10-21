package integration

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func ghcrClientRecordingTokenRequest(username, password string, tokenRequest **http.Request) *ghcrClient {
	client := NewGhcrClient(username, password)
	client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		*tokenRequest = req
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"token":"t"}`))}, nil
	})}
	return client
}

func TestGhcrClientGetToken(t *testing.T) {
	t.Run("AnonymousWithoutCredentials", func(t *testing.T) {
		var tokenRequest *http.Request
		client := ghcrClientRecordingTokenRequest("", "", &tokenRequest)

		token, err := client.getToken("dhis2", "example")

		require.NoError(t, err)
		assert.Equal(t, "t", token)
		_, _, ok := tokenRequest.BasicAuth()
		assert.False(t, ok)
	})

	t.Run("BasicAuthWithCredentials", func(t *testing.T) {
		var tokenRequest *http.Request
		client := ghcrClientRecordingTokenRequest("user", "secret", &tokenRequest)

		_, err := client.getToken("dhis2", "example")

		require.NoError(t, err)
		username, password, ok := tokenRequest.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "user", username)
		assert.Equal(t, "secret", password)
		assert.Equal(t, "repository:dhis2/example:pull", tokenRequest.URL.Query().Get("scope"))
	})
}
