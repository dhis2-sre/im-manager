package user

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/google/uuid"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-playground/validator/v10"

	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/gin-gonic/gin"
)

func NewHandler(logger *slog.Logger, hostname string, sameSiteMode http.SameSite, accessTokenExpirationSeconds int, refreshTokenExpirationSeconds int, refreshTokenRememberMeExpirationSeconds int, publicKey rsa.PublicKey, userService *Service, tokenService tokenService) Handler {
	return Handler{
		logger:                                  logger,
		hostname:                                hostname,
		sameSiteMode:                            sameSiteMode,
		accessTokenExpirationSeconds:            accessTokenExpirationSeconds,
		refreshTokenExpirationSeconds:           refreshTokenExpirationSeconds,
		refreshTokenRememberMeExpirationSeconds: refreshTokenRememberMeExpirationSeconds,
		publicKey:                               publicKey,
		userService:                             userService,
		tokenService:                            tokenService,
	}
}

type Handler struct {
	logger                                  *slog.Logger
	hostname                                string
	sameSiteMode                            http.SameSite
	accessTokenExpirationSeconds            int
	refreshTokenExpirationSeconds           int
	refreshTokenRememberMeExpirationSeconds int
	publicKey                               rsa.PublicKey
	userService                             *Service
	tokenService                            tokenService
}

type tokenService interface {
	GetTokens(user *model.User, previousTokenId string, rememberMe bool) (*token.Tokens, error)
	ValidateRefreshToken(ctx context.Context, tokenString string) (*token.RefreshTokenData, error)
	SignOut(userId uint) error
}

type signUpRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,gte=24,lte=128"`
}

// SignUp user
func (h Handler) SignUp(c *gin.Context) {
	// swagger:route POST /users signUp
	//
	// SignUp user
	//
	// Sign up a user. This endpoint is publicly accessible and therefore anyone can sign up. However, before being able to perform any actions, users needs to be a member of a group. And only administrators can add users to groups.
	//
	// responses:
	//   201: User
	//   400: Error
	//   415: Error
	var request signUpRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(handleSignUpErrors(err))
		return
	}

	user, err := h.userService.SignUp(c.Request.Context(), request.Email, request.Password)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, user)
}

func handleSignUpErrors(err error) error {
	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	if !ok {
		return errdef.NewBadRequest("Error binding data: %+v", err)
	}

	var errs error
	for _, fieldError := range validationErrors {
		if fieldError.Field() == "Password" && (fieldError.Tag() == "gte" || fieldError.Tag() == "lte") {
			badRequest := errdef.NewBadRequest("password must be between 24 and 128 characters")
			errs = errors.Join(errs, badRequest)
		}

		if fieldError.Field() == "Email" && fieldError.Tag() == "email" {
			badRequest := errdef.NewBadRequest("invalid email provided: %s", fieldError.Value())
			errs = errors.Join(errs, badRequest)
		}
	}
	return errs
}

type validateEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

// ValidateEmail validate users email
func (h Handler) ValidateEmail(c *gin.Context) {
	// swagger:route POST /users/validate validateEmail
	//
	// Validate email
	//
	// Validate users email
	//
	// responses:
	//   200:
	//   400: Error
	//   404: Error
	var request validateEmailRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(errdef.NewBadRequest("validate email error: %v", err))
		return
	}

	//goland:noinspection GoImportUsedAsName
	token, err := uuid.Parse(request.Token)
	if err != nil {
		badRequest := errdef.NewBadRequest("unable to parse token: %v", err.Error())
		_ = c.Error(badRequest)
		return
	}

	err = h.userService.ValidateEmail(c.Request.Context(), token)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusOK)
}

type RequestPasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (h Handler) RequestPasswordReset(c *gin.Context) {
	// swagger:route POST /users/request-reset requestPasswordReset
	//
	// Request password reset
	//
	// Request user's password reset
	//
	// responses:
	//   201:
	//   400: Error
	//   404: Error
	//   415: Error
	var request RequestPasswordResetRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	err := h.userService.RequestPasswordReset(c.Request.Context(), request.Email)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusCreated)
}

type ResetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,gte=24,lte=128"`
}

func (h Handler) ResetPassword(c *gin.Context) {
	// swagger:route POST /users/reset-password resetPassword
	//
	// Reset password
	//
	// Reset user's password
	//
	// responses:
	//   201:
	//   400: Error
	//   404: Error
	//   415: Error
	var request ResetPasswordRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	err := h.userService.ResetPassword(c.Request.Context(), request.Token, request.Password)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusCreated)
}

type signInRequest struct {
	RememberMe bool `json:"rememberMe"`
}

// SignIn user
func (h Handler) SignIn(c *gin.Context) {
	// swagger:route POST /tokens signIn
	//
	// Sign in
	//
	// Sign in... And get tokens
	//
	// security:
	//   basicAuth:
	//
	// responses:
	//   201:
	//   400: Error
	//   401: Error
	//   403: Error
	//   404: Error
	//   415: Error
	user, err := handler.GetUserFromContext(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	var request signInRequest
	if err := handler.DataBinder(c, &request); err != nil && err != io.EOF {
		_ = c.Error(err)
		return
	}

	tokens, err := h.tokenService.GetTokens(user, "", request.RememberMe)
	if err != nil {
		_ = c.Error(err)
		return
	}

	h.setCookies(c, tokens, request.RememberMe)

	c.Status(http.StatusCreated)
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// RefreshToken user
func (h Handler) RefreshToken(c *gin.Context) {
	// swagger:route POST /refresh refreshToken
	//
	// Refresh tokens
	//
	// Refresh user tokens
	//
	// responses:
	//   201:
	//   400: Error
	//   415: Error

	var refreshTokenString string

	cookie, err := c.Cookie("refreshToken")
	if err == nil {
		refreshTokenString = cookie
	}

	// as of this writing http.ErrNoCookie is the only error which can be return from c.Cookie
	if errors.Is(err, http.ErrNoCookie) {
		var request RefreshTokenRequest
		if err := handler.DataBinder(c, &request); err != nil {
			_ = c.Error(err)
			return
		}
		refreshTokenString = request.RefreshToken
	}

	if refreshTokenString == "" {
		_ = c.AbortWithError(http.StatusBadRequest, fmt.Errorf("refresh token not found"))
		return
	}

	ctx := c.Request.Context()
	refreshToken, err := h.tokenService.ValidateRefreshToken(ctx, refreshTokenString)
	if err != nil {
		_ = c.AbortWithError(http.StatusUnauthorized, err)
		return
	}

	user, err := h.userService.FindById(ctx, refreshToken.UserId)
	if err != nil {
		if errdef.IsNotFound(err) {
			_ = c.AbortWithError(http.StatusUnauthorized, err)
		} else {
			_ = c.Error(err)
		}
		return
	}

	var rememberMe bool
	rememberMeCookie, _ := c.Cookie("rememberMe")
	if rememberMeCookie == "true" {
		rememberMe = true
	}

	tokens, err := h.tokenService.GetTokens(user, refreshToken.ID.String(), rememberMe)
	if err != nil {
		_ = c.Error(err)
		return
	}

	h.setCookies(c, tokens, rememberMe)

	c.Status(http.StatusCreated)
}

func (h Handler) setCookies(c *gin.Context, tokens *token.Tokens, rememberMe bool) {
	c.SetSameSite(h.sameSiteMode)
	c.SetCookie("accessToken", tokens.AccessToken, h.accessTokenExpirationSeconds, "/", h.hostname, true, true)
	if rememberMe {
		c.SetCookie("refreshToken", tokens.RefreshToken, h.refreshTokenRememberMeExpirationSeconds, "/refresh", h.hostname, true, true)
		c.SetCookie("rememberMe", "true", h.refreshTokenRememberMeExpirationSeconds, "/refresh", h.hostname, true, true)
	} else {
		c.SetCookie("refreshToken", tokens.RefreshToken, h.refreshTokenExpirationSeconds, "/refresh", h.hostname, true, true)
	}
}

// Me user
func (h Handler) Me(c *gin.Context) {
	// swagger:route GET /me me
	//
	// User details
	//
	// Current user details
	//
	// security:
	//   oauth2:
	//
	// responses:
	//   200: User
	//   401: Error
	//   403: Error
	//   404: Error
	//   415: Error
	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	userWithGroups, err := h.userService.FindById(ctx, user.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, userWithGroups)
}

// SignOut user
func (h Handler) SignOut(c *gin.Context) {
	// swagger:route DELETE /users signOut
	//
	// Sign out
	//
	// Sign out user... The authentication is done using oauth and JWT. A JWT can't easily be invalidated so even after calling this endpoint a user can still sign in assuming the JWT isn't expired. However, the token can't be refreshed using the refresh token supplied upon signin
	//
	// responses:
	//	200:
	//	401: Error
	//	415: Error

	// No matter what happens, if the user sends a sign-out request, delete all cookies
	unsetCookie(c)

	user, err := h.parseRequest(c.Request)
	if err != nil {
		_ = c.Error(errdef.NewUnauthorized("token not valid"))
		return
	}

	if err := h.tokenService.SignOut(user.ID); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusOK)
}

func (h Handler) parseRequest(request *http.Request) (*model.User, error) {
	token, err := jwt.ParseRequest(
		request,
		jwt.WithKey(jwa.RS256, h.publicKey),
		jwt.WithHeaderKey("Authorization"),
		jwt.WithCookieKey("accessToken"),
		jwt.WithCookieKey("refreshToken"),
		jwt.WithTypedClaim("user", model.User{}),
		jwt.WithValidate(false),
	)
	if err != nil {
		return nil, fmt.Errorf("error parsing request: %s", err)
	}

	err = jwt.Validate(token)
	if err != nil && !errors.Is(err, jwt.ErrTokenExpired()) {
		return nil, fmt.Errorf("error validating token: %s", err)
	}

	userData, ok := token.Get("user")
	if !ok {
		return nil, errors.New("user not found in claims")
	}

	user, ok := userData.(model.User)
	if !ok {
		return nil, errors.New("unable to convert claim to user")
	}

	return &user, nil
}

func unsetCookie(c *gin.Context) {
	c.SetCookie("accessToken", "", -1, "/", "", true, true)
	c.SetCookie("refreshToken", "", -1, "/", "", true, true)
	c.SetCookie("rememberMe", "", -1, "/", "", true, true)
}

// FindById user
func (h Handler) FindById(c *gin.Context) {
	// swagger:route GET /users/{id} findUserById
	//
	// Find user
	//
	// Find a user by its id
	//
	// security:
	//	oauth2:
	//
	// responses:
	//	200: User
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	userWithGroups, err := h.userService.FindById(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, userWithGroups)
}

// FindAll user
func (h Handler) FindAll(c *gin.Context) {
	// swagger:route GET /users findAllUsers
	//
	// Find users
	//
	// Find all users with the groups they belong to
	//
	// security:
	//	oauth2:
	//
	// responses:
	//	200: UsersResponse
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	users, err := h.userService.FindAll(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, users)
}

// Delete user
func (h Handler) Delete(c *gin.Context) {
	// swagger:route DELETE /users/{id} deleteUser
	//
	// Delete user
	//
	// Delete user by id
	//
	// Security:
	//	oauth2:
	//
	// Responses:
	//	202:
	//	401: Error
	//	403: Error
	//	404: Error
	//	415: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	ctx := c.Request.Context()
	user, err := handler.GetUserFromContext(ctx)
	if err != nil {
		_ = c.Error(err)
		return
	}

	_, err = h.userService.FindById(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if user.ID == id {
		_ = c.Error(errdef.NewBadRequest("cannot delete the current user"))
		return
	}

	err = h.userService.Delete(ctx, id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusAccepted)
}

type updateUserRequest struct {
	Email    string `json:"email" binding:"omitempty,email"`
	Password string `json:"password" binding:"omitempty,gte=16,lte=128"`
}

// Update user
func (h Handler) Update(c *gin.Context) {
	// swagger:route PUT /users/{id} updateUser
	//
	// Update user
	//
	// Update user's email and/or password
	//
	// security:
	//   oauth2:
	//
	// responses:
	//   200: User
	//   401: Error
	//   403: Error
	//   404: Error
	//   415: Error
	id, ok := handler.GetPathParameter(c, "id")
	if !ok {
		return
	}

	var request updateUserRequest
	if err := handler.DataBinder(c, &request); err != nil {
		_ = c.Error(err)
		return
	}

	if request.Email == "" && request.Password == "" {
		_ = c.Error(errdef.NewBadRequest("neither email nor password are specified"))
		return
	}

	user, err := h.userService.Update(c.Request.Context(), id, request.Email, request.Password)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, user)
}
