package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Payload struct {
	Field string `binding:"required,oneOf=one two"`
}

func TestRegisterValidation(t *testing.T) {
	err := RegisterValidation()
	require.NoError(t, err)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	request, err := http.NewRequest("GET", "/", nil)
	assert.NoError(t, err)
	ctx.Request = request

	err = ctx.ShouldBind(&Payload{Field: "one"})
	assert.NoError(t, err)

	err = ctx.ShouldBind(&Payload{Field: "two"})
	assert.NoError(t, err)

	err = ctx.ShouldBind(&Payload{Field: "oh no"})
	assert.Error(t, err)
	assert.Equal(t, "Key: 'Payload.Field' Error:Field validation for 'Field' failed on the 'oneOf' tag", err.Error())
}
