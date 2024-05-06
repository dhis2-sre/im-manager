package log

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestIDHandler(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	gin.SetMode(gin.TestMode)

	var b bytes.Buffer
	var requestID string
	r := gin.New()
	logger := slog.New(New(slog.NewJSONHandler(&b, nil)))
	r.Use(sloggin.New(logger))

	r.GET("/", func(ctx *gin.Context) {
		requestID = sloggin.GetRequestID(ctx)

		// samber/slog-gin and our call to InfoContext should add log lines with attribute
		// id=<requestID>
		logger.InfoContext(ctx, "info")
		ctx.String(http.StatusOK, "success")
	})

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(err)
	r.ServeHTTP(w, req)
	require.Equal(http.StatusOK, w.Code)

	sc := bufio.NewScanner(&b)
	for sc.Scan() {
		line := sc.Text()
		got := make(map[string]any)

		err = json.Unmarshal([]byte(line), &got)

		assert.NoError(err)
		t.Log("log line:", line)
		v, ok := got["id"]
		assert.True(ok, "want log line to have key `id`")
		assert.Equal(requestID, v, "want log line to have request `id` set")
	}
}
