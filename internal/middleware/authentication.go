package middleware

import (
	"crypto/rsa"
	"errors"
	"log"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
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
	// Trigger username/password prompt
	c.Header("WWW-Authenticate", "Basic realm=\"DHIS2\"")
	_ = c.AbortWithError(http.StatusUnauthorized, e)
}

func (m AuthenticationMiddleware) QueryStringAuthentication(c *gin.Context) {
	token, ok := c.GetQuery("token")
	if !ok {
		_ = c.Error(errdef.NewUnauthorized("token missing"))
		c.Abort()
		return
	}

	user, err := parseToken(token, m.publicKey)
	if err != nil {
		log.Printf("token not valid: %v", err)
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

func parseToken(token string, key *rsa.PublicKey) (*model.User, error) {
	parsedToken, err := jwt.Parse(
		[]byte(token),
		jwt.WithValidate(true),
		jwt.WithVerify(jwa.RS256, key),
	)
	if err != nil {
		return nil, err
	}

	return extractUser(parsedToken)
}

func (m AuthenticationMiddleware) TokenAuthentication(c *gin.Context) {
	u, err := parseRequest(c.Request, m.publicKey)
	if err != nil {
		log.Printf("token not valid: %v", err)
		_ = c.Error(errdef.NewUnauthorized("token not valid"))
		c.Abort()
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

func parseRequest(request *http.Request, key *rsa.PublicKey) (*model.User, error) {
	token, err := jwt.ParseRequest(
		request,
		jwt.WithValidate(true),
		jwt.WithVerify(jwa.RS256, key),
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

	userMap, ok := userData.(map[string]any)
	if !ok {
		return nil, errors.New("failed to parse user data")
	}

	id, ok := userMap["id"].(float64)
	if !ok {
		return nil, errors.New("failed to extract user id")
	}

	email, ok := userMap["email"].(string)
	if !ok {
		return nil, errors.New("failed to extract user email")
	}

	user := &model.User{
		ID:          uint(id),
		Email:       email,
		Groups:      extractGroups("groups", userMap),
		AdminGroups: extractGroups("adminGroups", userMap),
	}
	return user, nil
}

func extractGroups(key string, userMap map[string]any) []model.Group {
	groupsData, ok := userMap[key].([]any)
	if ok {
		groups := make([]model.Group, len(groupsData))
		for i := 0; i < len(groupsData); i++ {
			group := groupsData[i].(map[string]any)
			groups[i] = model.Group{
				Name:       group["name"].(string),
				Hostname:   group["hostname"].(string),
				Deployable: group["deployable"].(bool),
			}
		}
		return groups
	}
	return nil
}
