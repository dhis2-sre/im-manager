package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetPathParameter(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.AddParam("id", "123")

	id, ok := GetPathParameter(ctx, "id")
	assert.True(t, ok)
	assert.Equal(t, uint(123), id)
}

func TestGetPathParameter_NotFound(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	id, ok := GetPathParameter(ctx, "id")
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, uint(0), id)
}
