package log

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/internal/server"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogs(t *testing.T) {
	var userID uint = 1
	t.Run("ContainCorrelationIDAndUserID", func(t *testing.T) {
		t.Parallel()

		var b bytes.Buffer
		logger := slog.New(New(slog.NewJSONHandler(&b, nil)))
		r := newGinEngine(t, logger, userID)

		var correlationID string
		r.GET("/test1/:id", func(c *gin.Context) {
			correlationID, _ = middleware.GetCorrelationID(c.Request.Context())

			// our call to InfoContext here and the log line added from [middleware.RequestLogger]
			// should have log attribute id=<requestID> and user=<userID> added by
			// log.ContextHandler
			logger.InfoContext(c.Request.Context(), "logged by handler")
			c.String(http.StatusOK, "success")
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/test1/100", nil)
		require.NoError(t, err)
		req.SetBasicAuth("someUser", "somePassword")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var gotNrLines int
		wantNrLines := 2 // one from our test handler and one from the [middleware.RequestLogger]
		sc := bufio.NewScanner(&b)
		for sc.Scan() {
			line := sc.Text()
			gotNrLines++
			got := make(map[string]any)

			err = json.Unmarshal([]byte(line), &got)

			assert.NoError(t, err)
			t.Log("log line:", line)
			assertLogAttributeEquals(t, got, "correlationId", correlationID)
			assertLogAttributeEquals(t, got, "user", userID)
		}
		assert.Equal(t, wantNrLines, gotNrLines)
		require.NoError(t, sc.Err(), "error reading log lines")
	})

	t.Run("ContainsQueryAndURLParameters", func(t *testing.T) {
		t.Parallel()

		var b bytes.Buffer
		logger := slog.New(New(slog.NewJSONHandler(&b, nil)))
		r := newGinEngine(t, logger, userID)

		r.GET("/test1/:urlParam", func(c *gin.Context) {
			c.String(http.StatusOK, "success")
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/test1/100", nil)
		require.NoError(t, err)
		req.SetBasicAuth("someUser", "somePassword")
		q := req.URL.Query()
		q.Add("query1", "true")
		req.URL.RawQuery = q.Encode()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var gotNrLines int
		wantNrLines := 1
		sc := bufio.NewScanner(&b)
		for sc.Scan() {
			line := sc.Text()
			gotNrLines++
			got := make(map[string]any)

			err = json.Unmarshal([]byte(line), &got)

			require.NoError(t, err)
			t.Log("log line:", line)

			v := assertLogAttributeKey(t, got, "request")
			gotRequest, ok := v.(map[string]any)
			assert.True(t, ok, "want log line to have key `request` of type map[string]any")

			assertLogAttributeEquals(t, gotRequest, "path", "/test1/100")
			assertLogAttributeEquals(t, gotRequest, "route", "/test1/:urlParam")
			assertLogAttributeEquals(t, gotRequest, "query", "query1=true")
			assertLogAttributeEquals(t, gotRequest, "params", map[string]any{"urlParam": "100"})
		}
		assert.Equal(t, wantNrLines, gotNrLines)
		require.NoError(t, sc.Err(), "error reading log lines")
	})

	t.Run("UseLogLevelInfoByDefault", func(t *testing.T) {
		t.Parallel()

		var b bytes.Buffer
		logger := slog.New(New(slog.NewJSONHandler(&b, nil)))
		r := newGinEngine(t, logger, userID)

		r.GET("/test1", func(c *gin.Context) {
			c.String(http.StatusOK, "success")
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/test1", nil)
		require.NoError(t, err)
		req.SetBasicAuth("someUser", "somePassword")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var gotNrLines int
		wantNrLines := 1
		sc := bufio.NewScanner(&b)
		for sc.Scan() {
			line := sc.Text()
			gotNrLines++
			got := make(map[string]any)

			err = json.Unmarshal([]byte(line), &got)

			require.NoError(t, err)
			t.Log("log line:", line)
			assertLogAttributeEquals(t, got, "level", "INFO")
			_, ok := got["error"]
			assert.False(t, ok, "want no key `error` for non warn/error levels")
		}
		assert.Equal(t, wantNrLines, gotNrLines)
		require.NoError(t, sc.Err(), "error reading log lines")
	})

	t.Run("UseLogLevelWarningOnClientError", func(t *testing.T) {
		t.Parallel()

		var b bytes.Buffer
		logger := slog.New(New(slog.NewJSONHandler(&b, nil)))
		r := newGinEngine(t, logger, userID)

		r.GET("/test1", func(c *gin.Context) {
			c.String(http.StatusOK, "success")
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/test1", nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code)

		var gotNrLines int
		wantNrLines := 1
		sc := bufio.NewScanner(&b)
		for sc.Scan() {
			line := sc.Text()
			gotNrLines++
			got := make(map[string]any)

			err = json.Unmarshal([]byte(line), &got)

			require.NoError(t, err)
			t.Log("log line:", line)
			assertLogAttributeEquals(t, got, "level", "WARN")
			assertLogAttributeContains(t, got, "error", "invalid Authorization header")
		}
		assert.Equal(t, wantNrLines, gotNrLines)
		require.NoError(t, sc.Err(), "error reading log lines")
	})

	t.Run("UseLogLevelErrorOnServerError", func(t *testing.T) {
		t.Parallel()

		var b bytes.Buffer
		logger := slog.New(New(slog.NewJSONHandler(&b, nil)))
		r := newGinEngine(t, logger, userID)

		r.GET("/test1", func(c *gin.Context) {
			_ = c.AbortWithError(http.StatusInternalServerError, errors.New("unknown error"))
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/test1", nil)
		require.NoError(t, err)
		req.SetBasicAuth("someUser", "somePassword")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		var gotNrLines int
		wantNrLines := 1
		sc := bufio.NewScanner(&b)
		for sc.Scan() {
			line := sc.Text()
			gotNrLines++
			got := make(map[string]any)

			err = json.Unmarshal([]byte(line), &got)

			require.NoError(t, err)
			t.Log("log line:", line)
			assertLogAttributeEquals(t, got, "level", "ERROR")
			assertLogAttributeContains(t, got, "error", "unknown error")
		}
		assert.Equal(t, wantNrLines, gotNrLines)
		require.NoError(t, sc.Err(), "error reading log lines")
	})
}

func newGinEngine(t *testing.T, logger *slog.Logger, userID uint) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r, err := server.GetEngine(logger, "", []string{"http://localhost"})
	require.NoError(t, err, "failed to set up up Gin")

	auth := middleware.NewAuthentication(rsa.PublicKey{}, SignInService{userID: userID})
	r.Use(auth.BasicAuthentication)

	return r
}

func assertLogAttributeEquals(t *testing.T, got map[string]any, wantKey string, wantValue any) {
	t.Helper()
	v := assertLogAttributeKey(t, got, wantKey)
	assert.EqualValuesf(t, wantValue, v, "want log line to have key %q", wantKey)
}

func assertLogAttributeContains(t *testing.T, got map[string]any, wantKey string, wantValue any) {
	t.Helper()
	v := assertLogAttributeKey(t, got, wantKey)
	assert.Containsf(t, v, wantValue, "want log line to have key %q", wantKey)
}

func assertLogAttributeKey(t *testing.T, got map[string]any, wantKey string) any {
	t.Helper()
	v, ok := got[wantKey]
	assert.Truef(t, ok, "want log line to have key %q", wantKey)
	return v
}

type SignInService struct {
	userID uint
}

func (s SignInService) SignIn(ctx context.Context, email string, password string) (*model.User, error) {
	return &model.User{ID: s.userID, Email: email, Password: password}, nil
}
