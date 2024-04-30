package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/config"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandler_RefreshToken_Cookie(t *testing.T) {
	userService := &mockUserService{}
	user := &model.User{ID: 123}
	userService.
		On("FindById", uint(123)).
		Return(user, nil)
	tokenService := &mockTokenService{}
	id := uuid.New()
	refreshTokenData := &token.RefreshTokenData{
		SignedToken: "signed-token",
		ID:          id,
		UserId:      123,
	}
	tokenService.
		On("ValidateRefreshToken", "token").
		Return(refreshTokenData, nil)
	tokens := &token.Tokens{
		AccessToken:  "accessToken",
		TokenType:    "tokenType",
		RefreshToken: "refreshToken",
		ExpiresIn:    900,
	}
	tokenService.
		On("GetTokens", user, id.String(), false).
		Return(tokens, nil)
	authentication := config.Authentication{AccessTokenExpirationSeconds: 900, RefreshTokenExpirationSeconds: 86400}
	handler := NewHandler("hostname", authentication, userService, tokenService)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	request := newPost(t, "/some-path", nil)
	cookie := &http.Cookie{Name: "refreshToken", Value: "token"}
	require.NoError(t, cookie.Valid())
	request.AddCookie(cookie)
	c.Request = request

	handler.RefreshToken(c)

	require.Len(t, c.Errors.Errors(), 0)
	cookies := recorder.Result().Cookies()
	assert.Len(t, cookies, 2)
	expectedAccessTokenCookie := "accessToken=accessToken; Path=/; Domain=hostname; Max-Age=900; HttpOnly; Secure; SameSite=Strict"
	assert.Equal(t, expectedAccessTokenCookie, cookies[0].Raw)
	expectedRefreshTokenCookie := "refreshToken=refreshToken; Path=/refresh; Domain=hostname; Max-Age=86400; HttpOnly; Secure; SameSite=Strict"
	assert.Equal(t, expectedRefreshTokenCookie, cookies[1].Raw)
	tokenService.AssertExpectations(t)
	userService.AssertExpectations(t)
}

func TestHandler_RefreshToken_Cookie_RememberMe(t *testing.T) {
	userService := &mockUserService{}
	user := &model.User{ID: 123}
	userService.
		On("FindById", uint(123)).
		Return(user, nil)
	tokenService := &mockTokenService{}
	id := uuid.New()
	refreshTokenData := &token.RefreshTokenData{
		SignedToken: "signed-token",
		ID:          id,
		UserId:      123,
	}
	tokenService.
		On("ValidateRefreshToken", "token").
		Return(refreshTokenData, nil)
	tokens := &token.Tokens{
		AccessToken:  "accessToken",
		TokenType:    "tokenType",
		RefreshToken: "refreshToken",
		ExpiresIn:    900,
	}
	tokenService.
		On("GetTokens", user, id.String(), true).
		Return(tokens, nil)
	authentication := config.Authentication{AccessTokenExpirationSeconds: 900, RefreshTokenExpirationSeconds: 2592000, RefreshTokenRememberMeExpirationSeconds: 2592000}
	handler := NewHandler("hostname", authentication, userService, tokenService)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	request := newPost(t, "/some-path", nil)
	refreshCookie := &http.Cookie{Name: "refreshToken", Value: "token"}
	require.NoError(t, refreshCookie.Valid())
	request.AddCookie(refreshCookie)
	rememberMeCookie := &http.Cookie{Name: "rememberMe", Value: "true"}
	require.NoError(t, rememberMeCookie.Valid())
	request.AddCookie(rememberMeCookie)
	c.Request = request

	handler.RefreshToken(c)

	require.Len(t, c.Errors.Errors(), 0)
	cookies := recorder.Result().Cookies()
	assert.Len(t, cookies, 3)
	expectedAccessTokenCookie := "accessToken=accessToken; Path=/; Domain=hostname; Max-Age=900; HttpOnly; Secure; SameSite=Strict"
	assert.Equal(t, expectedAccessTokenCookie, cookies[0].Raw)
	expectedRefreshTokenCookie := "refreshToken=refreshToken; Path=/refresh; Domain=hostname; Max-Age=2592000; HttpOnly; Secure; SameSite=Strict"
	assert.Equal(t, expectedRefreshTokenCookie, cookies[1].Raw)
	expectedRefreshTokenRememberMeCookie := "rememberMe=true; Path=/refresh; Domain=hostname; Max-Age=2592000; HttpOnly; Secure; SameSite=Strict"
	assert.Equal(t, expectedRefreshTokenRememberMeCookie, cookies[2].Raw)
	tokenService.AssertExpectations(t)
	userService.AssertExpectations(t)
}

func TestHandler_RefreshToken_RequestBody(t *testing.T) {
	userService := &mockUserService{}
	user := &model.User{ID: 123}
	userService.
		On("FindById", uint(123)).
		Return(user, nil)
	tokenService := &mockTokenService{}
	id := uuid.New()
	refreshTokenData := &token.RefreshTokenData{
		SignedToken: "signed-token",
		ID:          id,
		UserId:      123,
	}
	tokenService.
		On("ValidateRefreshToken", "token").
		Return(refreshTokenData, nil)
	tokens := &token.Tokens{
		AccessToken:  "accessToken",
		TokenType:    "tokenType",
		RefreshToken: "refreshToken",
		ExpiresIn:    900,
	}
	tokenService.
		On("GetTokens", user, id.String(), false).
		Return(tokens, nil)
	authentication := config.Authentication{AccessTokenExpirationSeconds: 900, RefreshTokenExpirationSeconds: 86400}
	handler := NewHandler("hostname", authentication, userService, tokenService)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = newPost(t, "/some-path", &RefreshTokenRequest{RefreshToken: "token"})

	handler.RefreshToken(c)

	require.Len(t, c.Errors.Errors(), 0)
	cookies := recorder.Result().Cookies()
	assert.Len(t, cookies, 2)
	expectedAccessTokenCookie := "accessToken=accessToken; Path=/; Domain=hostname; Max-Age=900; HttpOnly; Secure; SameSite=Strict"
	assert.Equal(t, expectedAccessTokenCookie, cookies[0].Raw)
	expectedRefreshTokenCookie := "refreshToken=refreshToken; Path=/refresh; Domain=hostname; Max-Age=86400; HttpOnly; Secure; SameSite=Strict"
	assert.Equal(t, expectedRefreshTokenCookie, cookies[1].Raw)
	tokenService.AssertExpectations(t)
	userService.AssertExpectations(t)
}

func TestHandler_SignIn_Cookies(t *testing.T) {
	userService := &mockUserService{}
	user := &model.User{ID: 123}
	tokenService := &mockTokenService{}
	tokens := &token.Tokens{
		AccessToken:  "accessToken",
		TokenType:    "tokenType",
		RefreshToken: "refreshToken",
		ExpiresIn:    900,
	}
	tokenService.
		On("GetTokens", user, "", false).
		Return(tokens, nil)
	authentication := config.Authentication{AccessTokenExpirationSeconds: 900, RefreshTokenExpirationSeconds: 86400}
	handler := NewHandler("hostname", authentication, userService, tokenService)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user", user)
	c.Request = newPost(t, "/some-path", &RefreshTokenRequest{RefreshToken: "token"})

	handler.SignIn(c)

	require.Len(t, c.Errors.Errors(), 0)
	cookies := recorder.Result().Cookies()
	assert.Len(t, cookies, 2)
	expectedAccessTokenCookie := "accessToken=accessToken; Path=/; Domain=hostname; Max-Age=900; HttpOnly; Secure; SameSite=Strict"
	assert.Equal(t, expectedAccessTokenCookie, cookies[0].Raw)
	expectedRefreshTokenCookie := "refreshToken=refreshToken; Path=/refresh; Domain=hostname; Max-Age=86400; HttpOnly; Secure; SameSite=Strict"
	assert.Equal(t, expectedRefreshTokenCookie, cookies[1].Raw)
	tokenService.AssertExpectations(t)
	userService.AssertExpectations(t)
}

func TestHandler_SignOut_Cookies(t *testing.T) {
	userService := &mockUserService{}
	user := &model.User{ID: 123}
	tokenService := &mockTokenService{}
	tokenService.
		On("SignOut", uint(123)).
		Return(nil)
	handler := NewHandler("hostname", config.Authentication{}, userService, tokenService)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("user", user)
	c.Request = newPost(t, "/some-path", &RefreshTokenRequest{RefreshToken: "token"})

	handler.SignOut(c)

	require.Len(t, c.Errors.Errors(), 0)
	cookies := recorder.Result().Cookies()
	assert.Len(t, cookies, 3)
	expectedAccessTokenCookie := "accessToken=; Path=/; Max-Age=0; HttpOnly; Secure"
	assert.Equal(t, expectedAccessTokenCookie, cookies[0].Raw)
	expectedRefreshTokenCookie := "refreshToken=; Path=/; Max-Age=0; HttpOnly; Secure"
	assert.Equal(t, expectedRefreshTokenCookie, cookies[1].Raw)
	expectedRefreshTokenRememberMeCookie := "rememberMe=; Path=/; Max-Age=0; HttpOnly; Secure"
	assert.Equal(t, expectedRefreshTokenRememberMeCookie, cookies[2].Raw)
	tokenService.AssertExpectations(t)
	userService.AssertExpectations(t)
}

type mockUserService struct{ mock.Mock }

func (m *mockUserService) RequestPasswordReset(email string) error {
	panic("implement me")
}

func (m *mockUserService) ResetPassword(token string, password string) error {
	panic("implement me")
}

func (m *mockUserService) SignUp(email string, password string) (*model.User, error) {
	panic("implement me")
}

func (m *mockUserService) SignIn(email string, password string) (*model.User, error) {
	panic("implement me")
}

func (m *mockUserService) FindById(id uint) (*model.User, error) {
	called := m.Called(id)
	return called.Get(0).(*model.User), nil
}

func (m *mockUserService) FindAll() ([]*model.User, error) {
	panic("implement me")
}

func (m *mockUserService) Delete(id uint) error {
	panic("implement me")
}

func (m *mockUserService) Update(id uint, email, password string) (*model.User, error) {
	panic("implement me")
}

func (m *mockUserService) ValidateEmail(token uuid.UUID) error {
	panic("implement me")
}

type mockTokenService struct{ mock.Mock }

func (m *mockTokenService) GetTokens(user *model.User, previousTokenId string, rememberMe bool) (*token.Tokens, error) {
	called := m.Called(user, previousTokenId, rememberMe)
	return called.Get(0).(*token.Tokens), nil
}

func (m *mockTokenService) ValidateRefreshToken(tokenString string) (*token.RefreshTokenData, error) {
	called := m.Called(tokenString)
	return called.Get(0).(*token.RefreshTokenData), nil
}

func (m *mockTokenService) SignOut(userId uint) error {
	called := m.Called(userId)
	return called.Error(0)
}

func newPost(t *testing.T, path string, jsonBody any) *http.Request {
	body, err := json.Marshal(jsonBody)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token")

	return req
}
