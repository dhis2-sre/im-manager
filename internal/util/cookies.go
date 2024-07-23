package util

import (
	"net/http"

	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/gin-gonic/gin"
)

func SetCookies(c *gin.Context, tokens *token.Tokens, rememberMe bool, sameSiteMode http.SameSite, hostname string, accessTokenExpirationSeconds int, refreshTokenExpirationSeconds int, refreshTokenRememberMeExpirationSeconds int) {
	c.SetSameSite(sameSiteMode)
	c.SetCookie("accessToken", tokens.AccessToken, accessTokenExpirationSeconds, "/", hostname, true, true)
	if rememberMe {
		c.SetCookie("refreshToken", tokens.RefreshToken, refreshTokenRememberMeExpirationSeconds, "/refresh", hostname, true, true)
		c.SetCookie("rememberMe", "true", refreshTokenRememberMeExpirationSeconds, "/refresh", hostname, true, true)
	} else {
		c.SetCookie("refreshToken", tokens.RefreshToken, refreshTokenExpirationSeconds, "/refresh", hostname, true, true)
	}
}
