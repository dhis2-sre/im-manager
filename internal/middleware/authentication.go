package middleware

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
)

func NewAuthentication(publicKey *rsa.PublicKey, signInService signInService) AuthenticationMiddleware {
	return AuthenticationMiddleware{
		publicKey:     publicKey,
		signInService: signInService,
	}
}

type signInService interface {
	SignIn(email string, password string) (*model.User, error)
}

type AuthenticationMiddleware struct {
	publicKey     *rsa.PublicKey
	signInService signInService
}

// BasicAuthentication Inspiration: https://www.pandurang-waghulde.com/custom-http-basic-authentication-using-gin/
func (m AuthenticationMiddleware) BasicAuthentication(c *gin.Context) {
	username, password, ok := c.Request.BasicAuth()
	if !ok {
		m.handleError(c, errors.New("invalid Authorization header format"))
		return
	}

	u, err := m.signInService.SignIn(username, password)
	if err != nil {
		m.handleError(c, err)
		return
	}

	c.Set("user", u)
	c.Next()
}

func (m AuthenticationMiddleware) handleError(c *gin.Context, e error) {
	_ = c.AbortWithError(http.StatusUnauthorized, e)
}

func (m AuthenticationMiddleware) TokenAuthentication(c *gin.Context) {
	user, err := parseRequest(c.Request, m.publicKey)
	if err != nil {
		log.Println("token not valid:", err)
		_ = c.Error(errdef.NewUnauthorized("token not valid"))
		c.Abort()
		return
	}

	// Extra precaution to ensure that no errors has occurred, and it's safe to call c.Next()
	if len(c.Errors.Errors()) > 0 {
		c.Abort()
		return
	} else {
		c.Set("user", user)
		c.Next()
	}
}

func parseRequest(request *http.Request, key *rsa.PublicKey) (*model.User, error) {
	token, err := jwt.ParseRequest(
		request,
		jwt.WithKey(jwa.RS256, key),
		jwt.WithHeaderKey("Authorization"),
		jwt.WithCookieKey("accessToken"),
		jwt.WithCookieKey("refreshToken"),
	)
	if err != nil {
		return nil, err
	}

	return extractUser(token)
}

func extractUser(token jwt.Token) (*model.User, error) {
	userData, ok := token.Get("user")
	if !ok {
		return nil, errors.New("user not found in claims")
	}

	bytes, err := json.Marshal(userData)
	if err != nil {
		return nil, err
	}

	user := &model.User{}
	err = json.Unmarshal(bytes, user)
	return user, err
}
