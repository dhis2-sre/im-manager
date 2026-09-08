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
	refreshTokenRememberMeExpirationSeconds int,
	userService oauthUserService,
	tokenService oauthTokenService,
) OAuthHandler {
	return OAuthHandler{
		logger: logger,
		uiURL:  uiURL,
		cookies: cookieWriter{
			sameSiteMode:                            sameSiteMode,
			cookieSecure:                            cookieSecure,
			accessTokenExpirationSeconds:            accessTokenExpirationSeconds,
			refreshTokenExpirationSeconds:           refreshTokenExpirationSeconds,
			refreshTokenRememberMeExpirationSeconds: refreshTokenRememberMeExpirationSeconds,
		},
		cookieSecure: cookieSecure,
		userService:  userService,
		tokenService: tokenService,
	}
}

type OAuthHandler struct {
	logger       *slog.Logger
	uiURL        string
	cookies      cookieWriter
	cookieSecure bool
	userService  oauthUserService
	tokenService oauthTokenService
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

	// The provider round trip loses the query string, so the caller's "remember me" choice is
	// stashed in a cookie the callback picks up. SameSite=Lax for the same reason the gothic state
	// cookie uses it: Strict would drop the cookie on the cross-site redirect back from the
	// provider. Its lifetime matches the gothic session, since it is useless once that expires.
	if c.Query("rememberMe") == "true" {
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(rememberMeIntentCookie, "true", oauthIntentTTLSeconds, "/", "", h.cookieSecure, true)
	}

	gothic.BeginAuthHandler(c.Writer, c.Request)
}

const (
	rememberMeIntentCookie = "oauthRememberMe"
	oauthIntentTTLSeconds  = 600
)

// consumeRememberMeIntent reports whether BeginAuth was asked to remember the session and clears
// the cookie either way, so a later sign-in cannot inherit this one's choice.
func (h OAuthHandler) consumeRememberMeIntent(c *gin.Context) bool {
	cookie, err := c.Cookie(rememberMeIntentCookie)

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(rememberMeIntentCookie, "", -1, "/", "", h.cookieSecure, true)

	return err == nil && cookie == "true"
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

	rememberMe := h.consumeRememberMeIntent(c)

	tokens, err := h.tokenService.GetTokens(user, "", rememberMe)
	if err != nil {
		_ = c.Error(err)
		return
	}

	h.cookies.set(c, tokens, rememberMe)

	c.Redirect(http.StatusFound, h.uiURL)
}
