package user

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dhis2-sre/im-manager/pkg/token"
)

const refreshCookiePath = "/refresh"

// cookieWriter owns the auth cookies. Both the password and the SSO handler write the same set, and
// a cookie is identified by name, domain and path, so the two have to agree on all three or the
// cookies one writes cannot be replaced or cleared by the other.
type cookieWriter struct {
	sameSiteMode                            http.SameSite
	cookieSecure                            bool
	accessTokenExpirationSeconds            int
	refreshTokenExpirationSeconds           int
	refreshTokenRememberMeExpirationSeconds int
}

func (w cookieWriter) set(c *gin.Context, tokens *token.Tokens, rememberMe bool) {
	c.SetSameSite(w.sameSiteMode)
	c.SetCookie("accessToken", tokens.AccessToken, w.accessTokenExpirationSeconds, "/", "", w.cookieSecure, true)
	if rememberMe {
		c.SetCookie("refreshToken", tokens.RefreshToken, w.refreshTokenRememberMeExpirationSeconds, refreshCookiePath, "", w.cookieSecure, true)
		c.SetCookie("rememberMe", "true", w.refreshTokenRememberMeExpirationSeconds, refreshCookiePath, "", w.cookieSecure, true)
	} else {
		c.SetCookie("refreshToken", tokens.RefreshToken, w.refreshTokenExpirationSeconds, refreshCookiePath, "", w.cookieSecure, true)
	}
}

func (w cookieWriter) unset(c *gin.Context) {
	c.SetSameSite(w.sameSiteMode)
	c.SetCookie("accessToken", "", -1, "/", "", w.cookieSecure, true)
	c.SetCookie("refreshToken", "", -1, refreshCookiePath, "", w.cookieSecure, true)
	c.SetCookie("rememberMe", "", -1, refreshCookiePath, "", w.cookieSecure, true)
}
