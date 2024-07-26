package middleware

import (
	"fmt"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/internal/util"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token"
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

// TODO ensure that gothic or we validate the callback payload from google, so that someone can't impersonate a user through the callback url
// ProviderCallback handles the SSO authentication provider callback.
func (m SSOMiddleware) ProviderCallback(c *gin.Context) {
	fmt.Println("request path at ProviderCallback: ", c.Request.URL.Path)
	fmt.Println("headers at ProviderCallback start")
	for k, value := range c.Request.Header {
		fmt.Println(k, value)
	}
	ssoUser, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	fmt.Println("sso user: ", ssoUser)

	if err != nil {
		_ = c.AbortWithError(http.StatusUnauthorized, errdef.NewUnauthorized("failed to complete authentication with provider: %s", err))
		return
	}

	// TODO the pass shouldn't be an empty string here
	// TODO find what's the best way to deal with this and check with security team
	// TODO should FindOrCreate take a User model, instead of XYZ args?
	u, err := m.signInService.FindOrCreate(ssoUser.Email, "", true)
	if err != nil {
		_ = c.AbortWithError(http.StatusUnauthorized, errdef.NewUnauthorized("failed to find or create user %s: %s", ssoUser.Email, err))
		return
	}

	c.Set("user", u)

	var rememberMe bool
	rememberMeCookie, _ := c.Cookie("rememberMe")
	fmt.Println("rememberMe cookie: ", rememberMeCookie)
	if rememberMeCookie == "true" {
		rememberMe = true
	}

	tokens, err := m.tokenService.GetTokens(u, "", rememberMe)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// TODO why is there no refreshToken set after SSO auth
	util.SetCookies(c, tokens, rememberMe, m.sameSiteMode, m.hostname, m.accessTokenExpirationSeconds, m.refreshTokenExpirationSeconds, m.refreshTokenRememberMeExpirationSeconds)

	// TODO where should we redirect to here?
	// TODO user should be redirected to the page they came from (example: /databases > /databases), header - referrer?
	fmt.Println("headers at ProviderCallback end")
	for k, value := range c.Request.Header {
		fmt.Println(k, value)
	}

	// TODO Should we redirect?
	c.Status(http.StatusOK)
	// c.Redirect(http.StatusTemporaryRedirect, "/")
}

// BeginAuth initiates SSO authentication with the provider.
func (m SSOMiddleware) BeginAuth(c *gin.Context) {
	fmt.Println("request path at BeginAuth: ", c.Request.URL.Path)

	fmt.Println("headers at BeginAuth")
	for k, value := range c.Request.Header {
		fmt.Println(k, value)
	}

	provider := c.Param("provider")
	q := c.Request.URL.Query()
	q.Add("provider", provider)
	c.Request.URL.RawQuery = q.Encode()
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

// Logout handles user logout.
func (m SSOMiddleware) Logout(c *gin.Context) {
	err := gothic.Logout(c.Writer, c.Request)
	if err != nil {
		_ = c.Error(fmt.Errorf("failed to logout: %s", err))
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/")
}
