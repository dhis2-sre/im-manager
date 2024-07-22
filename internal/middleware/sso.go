package middleware

import (
	"fmt"
	"net/http"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/markbates/goth/gothic"
)

type SSOMiddleware struct {
	signInService signInService
	tokenService  tokenService
}

type tokenService interface {
	GetTokens(user *model.User, previousTokenId string, rememberMe bool) (*token.Tokens, error)
}

func NewSSOMiddleware(signInService signInService, tokenService tokenService) SSOMiddleware {
	return SSOMiddleware{
		signInService: signInService,
		tokenService:  tokenService,
	}
}

// SSOAuthentication handles SSO login callbacks
func (m SSOMiddleware) SSOAuthentication(c *gin.Context) {
	user, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	fmt.Println("SSOAuthentication error is:")
	fmt.Println(err)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/auth/login")
		return
	}

	// Find or create the user based on email from SSO auth with empty password
	u, err := m.signInService.FindOrCreate(user.Email, "")
	if err != nil {
		_ = c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	c.Set("user", u)
	fmt.Println("user is:")
	fmt.Println(u)

	tokens, err := m.tokenService.GetTokens(u, "", true)
	if err != nil {
		_ = c.Error(err)
		return
	}

	m.setCookies(c, tokens, true)

	fmt.Println("accessToken cookie: ")
	fmt.Println(c.Cookie("accessToken"))
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

// BeginAuthHandler initiates SSO authentication
func (m SSOMiddleware) BeginAuthHandler(c *gin.Context) {
	fmt.Println("calling BeginAuthHandler")
	provider := c.Param("provider")
	fmt.Println("provider:")
	fmt.Println(provider)
	q := c.Request.URL.Query()
	q.Add("provider", provider)
	c.Request.URL.RawQuery = q.Encode()
	fmt.Println("query")
	fmt.Println(c.Request)
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

// LogoutHandler handles user logout
func (m SSOMiddleware) LogoutHandler(c *gin.Context) {
	gothic.Logout(c.Writer, c.Request)
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func (m SSOMiddleware) setCookies(c *gin.Context, tokens *token.Tokens, rememberMe bool) {
	cfg := config.New()

	fmt.Println(cfg)

	c.SetSameSite(1)
	c.SetCookie("accessToken", tokens.AccessToken, 360, "/", cfg.Hostname, true, true)
	if rememberMe {
		c.SetCookie("refreshToken", tokens.RefreshToken, 360, "/refresh", cfg.Hostname, true, true)
		c.SetCookie("rememberMe", "true", 360, "/refresh", cfg.Hostname, true, true)
	} else {
		c.SetCookie("refreshToken", tokens.RefreshToken, 360, "/refresh", cfg.Hostname, true, true)
	}
}
