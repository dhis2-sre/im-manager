package user

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/markbates/goth/gothic"
)

type oauthUserService interface {
	SignInWithSSO(ctx context.Context, email string) (*model.User, error)
}

type oauthTokenService interface {
	GetTokens(user *model.User, previousTokenId string, rememberMe bool) (*token.Tokens, error)
}

func NewOAuthHandler(
	logger *slog.Logger,
	uiURL string,
	sameSiteMode http.SameSite,
	cookieSecure bool,
	accessTokenExpirationSeconds int,
	refreshTokenExpirationSeconds int,
	userService oauthUserService,
	tokenService oauthTokenService,
) OAuthHandler {
	return OAuthHandler{
		logger:                        logger,
		uiURL:                         uiURL,
		sameSiteMode:                  sameSiteMode,
		cookieSecure:                  cookieSecure,
		accessTokenExpirationSeconds:  accessTokenExpirationSeconds,
		refreshTokenExpirationSeconds: refreshTokenExpirationSeconds,
		userService:                   userService,
		tokenService:                  tokenService,
	}
}

type OAuthHandler struct {
	logger                        *slog.Logger
	uiURL                         string
	sameSiteMode                  http.SameSite
	cookieSecure                  bool
	accessTokenExpirationSeconds  int
	refreshTokenExpirationSeconds int
	userService                   oauthUserService
	tokenService                  oauthTokenService
}

// gothic reads the provider name from a query parameter; gin path params won't do.
func withProviderQuery(c *gin.Context) {
	provider := c.Param("provider")
	q := c.Request.URL.Query()
	q.Set("provider", provider)
	c.Request.URL.RawQuery = q.Encode()
}

// BeginAuth starts the OAuth flow by redirecting to the identity provider.
func (h OAuthHandler) BeginAuth(c *gin.Context) {
	withProviderQuery(c)
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

// Callback handles the OAuth callback, signs the user in, sets cookies and redirects to the UI.
func (h OAuthHandler) Callback(c *gin.Context) {
	withProviderQuery(c)

	gothUser, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "OAuth callback failed", "error", err)
		_ = c.Error(errdef.NewUnauthorized("oauth callback failed: %s", err))
		return
	}

	if gothUser.Email == "" {
		_ = c.Error(errdef.NewUnauthorized("oauth provider did not return an email address"))
		return
	}

	ctx := c.Request.Context()
	user, err := h.userService.SignInWithSSO(ctx, gothUser.Email)
	if err != nil {
		_ = c.Error(err)
		return
	}

	tokens, err := h.tokenService.GetTokens(user, "", false)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.SetSameSite(h.sameSiteMode)
	c.SetCookie("accessToken", tokens.AccessToken, h.accessTokenExpirationSeconds, "/", "", h.cookieSecure, true)
	c.SetCookie("refreshToken", tokens.RefreshToken, h.refreshTokenExpirationSeconds, "/refresh", "", h.cookieSecure, true)

	c.Redirect(http.StatusFound, h.uiURL)
}
