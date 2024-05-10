package log

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(middleware.RequestID())

	var b bytes.Buffer
	logger := slog.New(New(slog.NewJSONHandler(&b, nil)))
	r.Use(middleware.RequestLogger(logger))

	var userID uint = 1
	auth := middleware.NewAuthentication(nil, SignInService{userID: userID})
	r.Use(auth.BasicAuthentication)

	t.Run("ContainRequestIDAndUserID", func(t *testing.T) {
		var requestID string
		r.GET("/test1/:id", func(c *gin.Context) {
			requestID = middleware.GetRequestID(c)
			// middleware.RequestLogger() and our call to InfoContext should add log lines with
			// attribute id=<requestID> and user=<userID>
			logger.InfoContext(c, "info")
			c.String(http.StatusOK, "success")
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/test1/100", nil)
		require.NoError(t, err)
		req.SetBasicAuth("someUser", "somePassword")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		sc := bufio.NewScanner(&b)
		for sc.Scan() {
			line := sc.Text()
			got := make(map[string]any)

			err = json.Unmarshal([]byte(line), &got)

			assert.NoError(t, err)
			t.Log("log line:", line)
			assertLogAttributeEquals(t, got, "id", requestID)
			assertLogAttributeEquals(t, got, "user", userID)
		}
	})

	t.Run("ContainsQueryAndURLParameters", func(t *testing.T) {
		r.GET("/test2/:urlParam", func(c *gin.Context) {
			c.String(http.StatusOK, "success")
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/test2/100", nil)
		require.NoError(t, err)
		req.SetBasicAuth("someUser", "somePassword")
		q := req.URL.Query()
		q.Add("query1", "true")
		req.URL.RawQuery = q.Encode()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		sc := bufio.NewScanner(&b)
		for sc.Scan() {
			line := sc.Text()
			got := make(map[string]any)

			err = json.Unmarshal([]byte(line), &got)

			require.NoError(t, err)
			t.Log("log line:", line)

			v := assertLogAttributeKey(t, got, "request")
			gotRequest, ok := v.(map[string]any)
			assert.True(t, ok, "want log line to have key `request` of type map[string]any")

			assertLogAttributeEquals(t, gotRequest, "path", "/test2/100")
			assertLogAttributeEquals(t, gotRequest, "route", "/test2/:urlParam")
			assertLogAttributeEquals(t, gotRequest, "query", "query1=true")
			assertLogAttributeEquals(t, gotRequest, "params", map[string]any{"urlParam": "100"})
		}
	})

	t.Run("UseLogLevelInfoByDefault", func(t *testing.T) {
		r.GET("/test3", func(c *gin.Context) {
			c.String(http.StatusOK, "success")
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/test3", nil)
		require.NoError(t, err)
		req.SetBasicAuth("someUser", "somePassword")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		sc := bufio.NewScanner(&b)
		for sc.Scan() {
			line := sc.Text()
			got := make(map[string]any)

			err = json.Unmarshal([]byte(line), &got)

			require.NoError(t, err)
			t.Log("log line:", line)
			assertLogAttributeEquals(t, got, "level", "INFO")
			_, ok := got["error"]
			assert.False(t, ok, "want no key `error` for non warn/error levels")
		}
	})

	t.Run("UseLogLevelWarningOnClientError", func(t *testing.T) {
		r.GET("/test4", func(c *gin.Context) {
			c.String(http.StatusOK, "success")
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/test4", nil)
		require.NoError(t, err)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code)

		sc := bufio.NewScanner(&b)
		for sc.Scan() {
			line := sc.Text()
			got := make(map[string]any)

			err = json.Unmarshal([]byte(line), &got)

			require.NoError(t, err)
			t.Log("log line:", line)
			assertLogAttributeEquals(t, got, "level", "WARN")
			assertLogAttributeContains(t, got, "error", "invalid Authorization header")
		}
	})

	t.Run("UseLogLevelErrorOnServerError", func(t *testing.T) {
		r.GET("/test5", func(c *gin.Context) {
			_ = c.AbortWithError(http.StatusInternalServerError, errors.New("unknown error"))
		})

		w := httptest.NewRecorder()
		req, err := http.NewRequest("GET", "/test5", nil)
		require.NoError(t, err)
		req.SetBasicAuth("someUser", "somePassword")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusInternalServerError, w.Code)

		sc := bufio.NewScanner(&b)
		for sc.Scan() {
			line := sc.Text()
			got := make(map[string]any)

			err = json.Unmarshal([]byte(line), &got)

			require.NoError(t, err)
			t.Log("log line:", line)
			assertLogAttributeEquals(t, got, "level", "ERROR")
			assertLogAttributeContains(t, got, "error", "unknown error")
		}
	})
}

func assertLogAttributeEquals(t *testing.T, got map[string]any, wantKey string, wantValue any) {
	v := assertLogAttributeKey(t, got, wantKey)
	assert.EqualValuesf(t, wantValue, v, "want log line to have key %q", wantKey)
}

func assertLogAttributeContains(t *testing.T, got map[string]any, wantKey string, wantValue any) {
	v := assertLogAttributeKey(t, got, wantKey)
	assert.Containsf(t, v, wantValue, "want log line to have key %q", wantKey)
}

func assertLogAttributeKey(t *testing.T, got map[string]any, wantKey string) any {
	v, ok := got[wantKey]
	assert.Truef(t, ok, "want log line to have key %q", wantKey)
	return v
}

type SignInService struct {
	userID uint
}

func (s SignInService) SignIn(email string, password string) (*model.User, error) {
	return &model.User{ID: s.userID, Email: email, Password: password}, nil
}
