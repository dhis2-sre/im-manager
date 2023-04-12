package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dhis2-sre/im-manager/pkg/model"

	"github.com/gin-gonic/gin"
)

func NewAuthentication(userService signInService, tokenService tokenService) AuthenticationMiddleware {
	return AuthenticationMiddleware{
		userService,
		tokenService,
	}
}

type signInService interface {
	SignIn(email string, password string) (*model.User, error)
}

type tokenService interface {
	ValidateAccessToken(tokenString string) (*model.User, error)
}

type AuthenticationMiddleware struct {
	userService  signInService
	tokenService tokenService
}

// BasicAuthentication Inspiration: https://www.pandurang-waghulde.com/custom-http-basic-authentication-using-gin/
func (m AuthenticationMiddleware) BasicAuthentication(c *gin.Context) {
	username, password, ok := c.Request.BasicAuth()
	if !ok {
		m.handleError(c, errors.New("invalid Authorization header format"))
		return
	}

	u, err := m.userService.SignIn(username, password)
	if err != nil {
		m.handleError(c, err)
		return
	}

	c.Set("user", u)
	c.Next()
}

func (m AuthenticationMiddleware) handleError(c *gin.Context, e error) {
	// Trigger username/password prompt
	c.Header("WWW-Authenticate", "Basic realm=\"DHIS2\"")
	_ = c.AbortWithError(http.StatusUnauthorized, e)
}

func (m AuthenticationMiddleware) TokenAuthentication(c *gin.Context) {
	authorizationHeader := c.GetHeader("Authorization")
	authorizationHeader = strings.TrimPrefix(authorizationHeader, "Bearer ")
	authorizationHeader = strings.TrimPrefix(authorizationHeader, "bearer ")

	u, err := m.tokenService.ValidateAccessToken(authorizationHeader)
	if err != nil {
		_ = c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	// Extra precaution to ensure that no errors has occurred, and it's safe to call c.Next()
	if len(c.Errors.Errors()) > 0 {
		c.Abort()
		return
	} else {
		c.Set("user", u)
		c.Next()
	}
}
