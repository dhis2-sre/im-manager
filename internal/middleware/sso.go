package middleware

import (
	"net/http"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/dhis2-sre/im-manager/pkg/user"
	"github.com/gin-gonic/gin"
	"github.com/markbates/goth/gothic"
)

type SSOMiddleware struct {
	signInService                           signInService
	tokenService                            tokenService
	hostname                                string
	sameSiteMode                            http.SameSite
	accessTokenExpirationSeconds            int
	refreshTokenExpirationSeconds           int
	refreshTokenRememberMeExpirationSeconds int
}

type tokenService interface {
	GetTokens(user *model.User, previousTokenId string, rememberMe bool) (*token.Tokens, error)
}

func NewSSOMiddleware(signInService signInService, tokenService tokenService, hostname string, sameSiteMode http.SameSite, accessTokenExpirationSeconds int, refreshTokenExpirationSeconds int, refreshTokenRememberMeExpirationSeconds int) SSOMiddleware {
	return SSOMiddleware{
		signInService:                           signInService,
		tokenService:                            tokenService,
		hostname:                                hostname,
		sameSiteMode:                            sameSiteMode,
		accessTokenExpirationSeconds:            accessTokenExpirationSeconds,
		refreshTokenExpirationSeconds:           refreshTokenExpirationSeconds,
		refreshTokenRememberMeExpirationSeconds: refreshTokenRememberMeExpirationSeconds,
	}
}

// SSOAuthentication handles SSO login callbacks
func (m SSOMiddleware) SSOAuthentication(c *gin.Context) {
	ssoUser, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/auth/login")
		return
	}

	u, err := m.signInService.FindOrCreate(ssoUser.Email, "")
	if err != nil {
		_ = c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	c.Set("user", u)

	tokens, err := m.tokenService.GetTokens(u, "", true)
	if err != nil {
		_ = c.Error(err)
		return
	}

	user.SetCookies(c, tokens, true, m.sameSiteMode, m.hostname, m.accessTokenExpirationSeconds, m.refreshTokenExpirationSeconds, m.refreshTokenRememberMeExpirationSeconds)

	c.Redirect(http.StatusTemporaryRedirect, "/me")
}

// BeginAuthHandler initiates SSO authentication
func (m SSOMiddleware) BeginAuthHandler(c *gin.Context) {
	provider := c.Param("provider")
	q := c.Request.URL.Query()
	q.Add("provider", provider)
	c.Request.URL.RawQuery = q.Encode()
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

// LogoutHandler handles user logout
func (m SSOMiddleware) LogoutHandler(c *gin.Context) {
	err := gothic.Logout(c.Writer, c.Request)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/")
}
