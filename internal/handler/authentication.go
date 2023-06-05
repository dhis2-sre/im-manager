package handler

import (
	"crypto/rsa"
	"errors"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
	"gorm.io/gorm"
)

func NewAuthentication(c config.Config, signInService signInService) AuthenticationMiddleware {
	return AuthenticationMiddleware{c, signInService}
}

type signInService interface {
	SignIn(email string, password string) (*model.User, error)
}

type AuthenticationMiddleware struct {
	c             config.Config
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

func (m AuthenticationMiddleware) TokenAuthentication(c *gin.Context) {
	publicKey, err := m.c.Authentication.Keys.GetPublicKey()
	if err != nil {
		_ = c.Error(errors.New("failed to get public key"))
		c.Abort()
		return
	}

	u, err := parseRequest(c.Request, publicKey)
	if err != nil {
		// TODO: token could be not valid for lots of reasons, return err or at least log it
		unauthorized := errdef.NewUnauthorized("token not valid")
		_ = c.Error(unauthorized)
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

	userData, ok := token.Get("user")
	if !ok {
		return nil, errors.New("user not found in claims")
	}

	return extractUser(userData)
}

func extractUser(userData any) (*model.User, error) {
	userMap, ok := userData.(map[string]any)
	if !ok {
		return nil, errors.New("failed to parse user data")
	}

	id := userMap["ID"].(float64)
	email := userMap["Email"].(string)

	user := &model.User{
		Model:       gorm.Model{ID: uint(id)},
		Email:       email,
		Groups:      extractGroups("Groups", userMap),
		AdminGroups: extractGroups("AdminGroups", userMap),
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
				Name:     group["Name"].(string),
				Hostname: group["Hostname"].(string),
			}
		}
		return groups
	}
	return nil
}
