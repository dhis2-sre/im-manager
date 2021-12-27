package handler

import (
	"context"
	"crypto/rsa"
	"errors"
	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
	"log"
	"net/http"
	"time"
)

func ProvideAuthentication(c config.Config) AuthenticationMiddleware {
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

type AuthenticationMiddleware struct {
	c              config.Config
	jwkAutoRefresh *jwk.AutoRefresh
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
	}

	publicKey := &rsa.PublicKey{}
	if key, ok := keySet.Get(m.c.Authentication.Jwks.Index); ok {
		err := key.Raw(publicKey)
		if err != nil {
			internal := apperror.NewInternal("failed to extract public key")
			_ = c.Error(internal)
		}
	}

	u, err := parseRequest(c.Request, publicKey)
	if err != nil {
		unauthorized := apperror.NewUnauthorized("token not valid")
		_ = c.Error(unauthorized)
		return
	}

	c.Set("user", u)

	c.Next()
}

type User struct {
	ID    uint
	Email string
}

func parseRequest(request *http.Request, key *rsa.PublicKey) (User, error) {
	token, err := jwt.ParseRequest(
		request,
		jwt.WithValidate(true),
		jwt.WithVerify(jwa.RS256, key),
	)
	if err != nil {
		return User{}, err
	}

	userData, ok := token.Get("user")
	if !ok {
		return User{}, errors.New("user not found in claims")
	}

	userMap, ok := userData.(map[string]interface{})
	if !ok {
		return User{}, errors.New("failed to parse user data")
	}

	id := userMap["ID"].(float64)
	email := userMap["Email"].(string)

	return User{
		ID:    uint(id),
		Email: email,
	}, nil
}
