package handler

import (
	"context"
	"crypto/rsa"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/dhis2-sre/im-user/swagger/sdk/models"

	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

type AuthenticationMiddleware struct {
	c              config.Config
	jwkAutoRefresh *jwk.AutoRefresh
}

func NewAuthentication(c config.Config) AuthenticationMiddleware {
	jwksHost := c.Authentication.Jwks.Host
	minimumRefreshInterval := time.Duration(c.Authentication.Jwks.MinimumRefreshInterval) * time.Second
	autoRefresh, err := provideJwkAutoRefresh(jwksHost, minimumRefreshInterval)
	if err != nil {
		log.Fatal(err)
	}

	return AuthenticationMiddleware{
		c,
		autoRefresh,
	}
}

func provideJwkAutoRefresh(host string, minRefreshInterval time.Duration) (*jwk.AutoRefresh, error) {
	if host != "" {
		ctx := context.TODO()
		ar := jwk.NewAutoRefresh(ctx)
		ar.Configure(host, jwk.WithMinRefreshInterval(minRefreshInterval))

		_, err := ar.Refresh(ctx, host)
		if err != nil {
			return nil, err
		}

		return ar, nil
	}
	return nil, nil
}

func (m AuthenticationMiddleware) TokenAuthentication(c *gin.Context) {
	keySet, err := m.jwkAutoRefresh.Fetch(context.TODO(), m.c.Authentication.Jwks.Host)
	if err != nil {
		internal := apperror.NewInternal("failed to refresh key set")
		_ = c.Error(internal)
		c.Abort()
		return
	}

	publicKey := &rsa.PublicKey{}
	if key, ok := keySet.Get(m.c.Authentication.Jwks.Index); ok {
		err := key.Raw(publicKey)
		if err != nil {
			internal := apperror.NewInternal("failed to extract public key")
			_ = c.Error(internal)
			c.Abort()
			return
		}
	}

	u, err := parseRequest(c.Request, publicKey)
	if err != nil {
		// TODO: token could be not valid for lots of reasons, return err or at least log it
		unauthorized := apperror.NewUnauthorized("token not valid")
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

func parseRequest(request *http.Request, key *rsa.PublicKey) (*models.User, error) {
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

func extractUser(userData interface{}) (*models.User, error) {
	userMap, ok := userData.(map[string]interface{})
	if !ok {
		return nil, errors.New("failed to parse user data")
	}

	id := userMap["ID"].(float64)
	email := userMap["Email"].(string)

	user := &models.User{
		ID:          uint64(id),
		Email:       email,
		Groups:      extractGroups("Groups", userMap),
		AdminGroups: extractGroups("AdminGroups", userMap),
	}
	return user, nil
}

func extractGroups(key string, userMap map[string]interface{}) []*models.Group {
	groups, ok := userMap[key].([]interface{})
	if ok {
		gs := make([]*models.Group, len(groups))
		for i := 0; i < len(groups); i++ {
			group := groups[i].(map[string]interface{})
			gs[i] = &models.Group{
				Name:     group["Name"].(string),
				Hostname: group["Hostname"].(string),
			}
		}
		return gs
	}
	return nil
}
