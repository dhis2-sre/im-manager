package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTokenFromRequest(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		cookie        string
		want          string
		wantErr       bool
	}{
		{name: "bearer token", authorization: "Bearer a.b.c", want: "a.b.c"},
		{name: "bearer is case insensitive", authorization: "bearer a.b.c", want: "a.b.c"},
		{name: "bare token without a scheme", authorization: "a.b.c", want: "a.b.c"},
		{name: "another scheme falls through to the cookie", authorization: "Basic dXNlcjpwYXNz", cookie: "a.b.c", want: "a.b.c"},
		{name: "another scheme without a cookie is an error", authorization: "Basic dXNlcjpwYXNz", wantErr: true},
		{name: "cookie only", cookie: "a.b.c", want: "a.b.c"},
		{name: "neither", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authorization != "" {
				c.Request.Header.Set("Authorization", tt.authorization)
			}
			if tt.cookie != "" {
				c.Request.AddCookie(&http.Cookie{Name: "accessToken", Value: tt.cookie})
			}

			token, err := GetTokenFromRequest(c)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, token)
		})
	}
}
