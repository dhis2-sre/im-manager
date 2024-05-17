package user_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/go-mail/mail"

	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/group"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/dhis2-sre/im-manager/pkg/user"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserHandler(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	userRepository := user.NewRepository(db)
	userService := user.NewService("", 900, userRepository, fakeDialer{t})
	groupRepository := group.NewRepository(db)
	groupService := group.NewService(groupRepository, userService)

	err := user.CreateUser("admin", "admin", userService, groupService, model.AdministratorGroupName, "admin")
	require.NoError(t, err, "failed to create admin user and group")

	authorization := middleware.NewAuthorization(slog.New(slog.NewTextHandler(os.Stdout, nil)), userService)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate private key")
	// TODO(DEVOPS-259) we should not use a pointer as we do not mutate and should not mutate the certificate
	authentication := middleware.NewAuthentication(&key.PublicKey, userService)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	redis := inttest.SetupRedis(t)
	tokenRepository := token.NewRepository(redis)
	tokenService, err := token.NewService(logger, tokenRepository, key, &key.PublicKey, 10, "secret", 20, 30)
	require.NoError(t, err)

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		userHandler := user.NewHandler("hostname", 10, 20, 30, &key.PublicKey, userService, tokenService)
		user.Routes(engine, authentication, authorization, userHandler)
	})

	t.Run("SignUp", func(t *testing.T) {
		t.Parallel()

		t.Log("SignUpUser")

		var user model.User
		client.PostJSON(t, "/users", strings.NewReader(`{
			"email":    "user@dhis2.org",
			"password": "oneoneoneoneoneoneone111"
		}`), &user)

		assert.Equal(t, "user@dhis2.org", user.Email)
		assert.Empty(t, user.Password)
		assert.Empty(t, user.EmailToken)
		assert.False(t, user.Validated)

		t.Log("ValidateEmail")

		u, err := userService.FindById(context.Background(), user.ID)
		require.NoError(t, err)
		requestBody := strings.NewReader(`{"token": "` + u.EmailToken.String() + `"}`)
		client.Do(t, http.MethodPost, "/users/validate", requestBody, http.StatusOK, inttest.WithHeader("Content-Type", "application/json"))

		t.Log("SignIn")

		var tokens *token.Tokens
		client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth("user@dhis2.org", "oneoneoneoneoneoneone111"))
		require.NotEmpty(t, tokens.AccessToken, "should return an access token")

		t.Log("GetMe")

		var me model.User
		client.GetJSON(t, "/me", &me, inttest.WithAuthToken(tokens.AccessToken))
		assert.Equal(t, "user@dhis2.org", me.Email)
		assert.True(t, me.Validated)
	})

	t.Run("SignOut", func(t *testing.T) {
		t.Parallel()

		expires, err := time.Parse(time.RFC3339, "0001-01-01T00:00:00Z")
		require.NoError(t, err)

		expectedCookies := []*http.Cookie{
			{
				Name:       "accessToken",
				Value:      "",
				Path:       "/",
				Domain:     "",
				Expires:    expires,
				RawExpires: "",
				MaxAge:     -1,
				Secure:     true,
				HttpOnly:   true,
				SameSite:   0,
				Raw:        "accessToken=; Path=/; Max-Age=0; HttpOnly; Secure",
				Unparsed:   nil,
			},
			{
				Name:       "refreshToken",
				Value:      "",
				Path:       "/",
				Domain:     "",
				Expires:    expires,
				RawExpires: "",
				MaxAge:     -1,
				Secure:     true,
				HttpOnly:   true,
				SameSite:   0,
				Raw:        "refreshToken=; Path=/; Max-Age=0; HttpOnly; Secure",
				Unparsed:   nil,
			},
			{
				Name:       "rememberMe",
				Value:      "",
				Path:       "/",
				Domain:     "",
				Expires:    expires,
				RawExpires: "",
				MaxAge:     -1,
				Secure:     true,
				HttpOnly:   true,
				SameSite:   0,
				Raw:        "rememberMe=; Path=/; Max-Age=0; HttpOnly; Secure",
				Unparsed:   nil,
			},
		}

		t.Run("NoToken", func(t *testing.T) {
			request := client.NewRequest(t, http.MethodDelete, "/users", nil)

			response, err := client.Client.Do(request)
			require.NoError(t, err)

			require.Equal(t, http.StatusUnauthorized, response.StatusCode)
			assert.EqualValues(t, expectedCookies, response.Cookies())
		})

		t.Run("ValidToken", func(t *testing.T) {
			_, email, password := createUser(t, client, userService)
			var tokens *token.Tokens
			client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth(email, password))
			request := client.NewRequest(t, http.MethodDelete, "/users", nil, inttest.WithAuthToken(tokens.AccessToken))

			response, err := client.Client.Do(request)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, response.StatusCode)
			assert.EqualValues(t, expectedCookies, response.Cookies())
		})

		t.Run("ExpiredToken", func(t *testing.T) {
			_, email, password := createUser(t, client, userService)
			var tokens *token.Tokens
			client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth(email, password))
			<-time.After(time.Duration(tokens.ExpiresIn) * time.Second)
			request := client.NewRequest(t, http.MethodDelete, "/users", nil, inttest.WithAuthToken(tokens.AccessToken))

			response, err := client.Client.Do(request)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, response.StatusCode)
			assert.EqualValues(t, expectedCookies, response.Cookies())
		})
	})

	t.Run("SignUpFailed", func(t *testing.T) {
		t.Parallel()

		t.Run("InvalidEmail", func(t *testing.T) {
			requestBody := strings.NewReader(`{
				"email":    "not-a-valid-email",
				"password": "oneoneoneoneoneoneone111"
			}`)

			response := client.Do(t, http.MethodPost, "/users", requestBody, http.StatusBadRequest, inttest.WithHeader("Content-Type", "application/json"))

			require.Equal(t, "invalid email provided: not-a-valid-email", string(response))
		})

		t.Run("TooShortPassword", func(t *testing.T) {
			requestBody := strings.NewReader(`{
				"email":    "some@email.com",
				"password": "short-password"
			}`)

			response := client.Do(t, http.MethodPost, "/users", requestBody, http.StatusBadRequest, inttest.WithHeader("Content-Type", "application/json"))

			require.Equal(t, "password must be between 24 and 128 characters", string(response))
		})

		t.Run("TooLongPassword", func(t *testing.T) {
			password := strings.Repeat("a", 129)
			requestBody := strings.NewReader(`{
				"email":    "some@email.com",
				"password": "` + password + `"
			}`)

			response := client.Do(t, http.MethodPost, "/users", requestBody, http.StatusBadRequest, inttest.WithHeader("Content-Type", "application/json"))

			require.Equal(t, "password must be between 24 and 128 characters", string(response))
		})

		t.Run("BothEmailAndPasswordAreInvalid", func(t *testing.T) {
			requestBody := strings.NewReader(`{
				"email":    "not-a-valid-email",
				"password": "short-password"
			}`)

			response := client.Do(t, http.MethodPost, "/users", requestBody, http.StatusBadRequest, inttest.WithHeader("Content-Type", "application/json"))

			require.Equal(t, "invalid email provided: not-a-valid-email\npassword must be between 24 and 128 characters", string(response))
		})
	})

	t.Run("AsNonAdmin", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			t.Parallel()

			var tokens *token.Tokens
			{
				t.Log("SignIn")

				_, email, password := createUser(t, client, userService)

				client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth(email, password))

				require.NotEmpty(t, tokens.AccessToken, "should return an access token")

				t.Log("GetMe")

				var me model.User

				client.GetJSON(t, "/me", &me, inttest.WithAuthToken(tokens.AccessToken))

				assert.Equal(t, email, me.Email)
			}

			{
				t.Log("GetAllIsUnauthorized")

				client.Do(t, http.MethodGet, "/users", nil, http.StatusUnauthorized, inttest.WithAuthToken(tokens.AccessToken))
			}

			{
				t.Log("SignInCookies")

				_, email, password := createUser(t, client, userService)
				requestBody := strings.NewReader(`{}`)
				request := client.NewRequest(t, http.MethodPost, "/tokens", requestBody, inttest.WithBasicAuth(email, password), inttest.WithHeader("Content-Type", "application/json"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, response.StatusCode)
				actualCookies := response.Cookies()
				require.Len(t, actualCookies, 2)
				accessTokenCookie := findCookieByName("accessToken", actualCookies)
				require.NotNil(t, accessTokenCookie)
				assert.Equal(t, accessTokenCookie.Path, "/")
				assert.Equal(t, accessTokenCookie.MaxAge, 10)
				assert.Equal(t, accessTokenCookie.Secure, true)
				assert.Equal(t, accessTokenCookie.HttpOnly, true)
				assert.Equal(t, accessTokenCookie.SameSite, http.SameSiteStrictMode)
				refreshTokenCookie := findCookieByName("refreshToken", actualCookies)
				require.NotNil(t, refreshTokenCookie)
				assert.Equal(t, refreshTokenCookie.Path, "/refresh")
				assert.Equal(t, refreshTokenCookie.MaxAge, 20)
				assert.Equal(t, refreshTokenCookie.Secure, true)
				assert.Equal(t, refreshTokenCookie.HttpOnly, true)
				assert.Equal(t, refreshTokenCookie.SameSite, http.SameSiteStrictMode)
			}

			{
				t.Log("SignInCookiesWithRememberMe")

				_, email, password := createUser(t, client, userService)
				requestBody := strings.NewReader(`{"rememberMe": true}`)
				request := client.NewRequest(t, http.MethodPost, "/tokens", requestBody, inttest.WithBasicAuth(email, password), inttest.WithHeader("Content-Type", "application/json"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, response.StatusCode)
				actualCookies := response.Cookies()
				require.Len(t, actualCookies, 3)
				accessTokenCookie := findCookieByName("accessToken", actualCookies)
				require.NotNil(t, accessTokenCookie)
				assert.Equal(t, accessTokenCookie.Path, "/")
				assert.Equal(t, accessTokenCookie.MaxAge, 10)
				assert.Equal(t, accessTokenCookie.Secure, true)
				assert.Equal(t, accessTokenCookie.HttpOnly, true)
				assert.Equal(t, accessTokenCookie.SameSite, http.SameSiteStrictMode)
				refreshTokenCookie := findCookieByName("refreshToken", actualCookies)
				require.NotNil(t, refreshTokenCookie)
				assert.Equal(t, refreshTokenCookie.Path, "/refresh")
				assert.Equal(t, refreshTokenCookie.MaxAge, 30)
				assert.Equal(t, refreshTokenCookie.Secure, true)
				assert.Equal(t, refreshTokenCookie.HttpOnly, true)
				assert.Equal(t, refreshTokenCookie.SameSite, http.SameSiteStrictMode)
				rememberMeTokenCookie := findCookieByName("rememberMe", actualCookies)
				require.NotNil(t, rememberMeTokenCookie)
				assert.Equal(t, rememberMeTokenCookie.Path, "/refresh")
				assert.Equal(t, rememberMeTokenCookie.MaxAge, 30)
				assert.Equal(t, rememberMeTokenCookie.Secure, true)
				assert.Equal(t, rememberMeTokenCookie.HttpOnly, true)
				assert.Equal(t, rememberMeTokenCookie.SameSite, http.SameSiteStrictMode)
			}

			{
				t.Log("RefreshTokensUsingCookie")

				_, email, password := createUser(t, client, userService)
				var tokens *token.Tokens
				client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth(email, password))
				require.NotEmpty(t, tokens.AccessToken, "should return an access token")
				request := client.NewRequest(t, http.MethodPost, "/refresh", strings.NewReader(`{}`), inttest.WithHeader("Content-Type", "application/json"))
				cookie := &http.Cookie{Name: "refreshToken", Value: tokens.RefreshToken, Path: "/refresh"}
				require.NoError(t, cookie.Valid())
				request.AddCookie(cookie)

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, response.StatusCode)
				actualCookies := response.Cookies()
				require.Len(t, actualCookies, 2)
				accessTokenCookie := findCookieByName("accessToken", actualCookies)
				require.NotNil(t, accessTokenCookie)
				assert.Equal(t, accessTokenCookie.Path, "/")
				assert.Equal(t, accessTokenCookie.MaxAge, 10)
				assert.Equal(t, accessTokenCookie.Secure, true)
				assert.Equal(t, accessTokenCookie.HttpOnly, true)
				assert.Equal(t, accessTokenCookie.SameSite, http.SameSiteStrictMode)
				refreshTokenCookie := findCookieByName("refreshToken", actualCookies)
				require.NotNil(t, refreshTokenCookie)
				assert.Equal(t, refreshTokenCookie.Path, "/refresh")
				assert.Equal(t, refreshTokenCookie.MaxAge, 20)
				assert.Equal(t, refreshTokenCookie.Secure, true)
				assert.Equal(t, refreshTokenCookie.HttpOnly, true)
				assert.Equal(t, refreshTokenCookie.SameSite, http.SameSiteStrictMode)

				// TODO: Assert response body?
			}

			{
				t.Log("RefreshTokensUsingCookieWithRememberMe")

				_, email, password := createUser(t, client, userService)
				var tokens *token.Tokens
				client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth(email, password))
				require.NotEmpty(t, tokens.AccessToken, "should return an access token")
				request := client.NewRequest(t, http.MethodPost, "/refresh", strings.NewReader(`{}`), inttest.WithHeader("Content-Type", "application/json"))
				refreshCookie := &http.Cookie{Name: "refreshToken", Value: tokens.RefreshToken, Path: "/refresh"}
				require.NoError(t, refreshCookie.Valid())
				request.AddCookie(refreshCookie)
				rememberMeCookie := &http.Cookie{Name: "rememberMe", Value: "true", Path: "/refresh"}
				require.NoError(t, rememberMeCookie.Valid())
				request.AddCookie(rememberMeCookie)

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, response.StatusCode)
				actualCookies := response.Cookies()
				require.Len(t, actualCookies, 3)
				accessTokenCookie := findCookieByName("accessToken", actualCookies)
				require.NotNil(t, accessTokenCookie)
				assert.Equal(t, accessTokenCookie.Path, "/")
				assert.Equal(t, accessTokenCookie.MaxAge, 10)
				assert.Equal(t, accessTokenCookie.Secure, true)
				assert.Equal(t, accessTokenCookie.HttpOnly, true)
				assert.Equal(t, accessTokenCookie.SameSite, http.SameSiteStrictMode)
				refreshTokenCookie := findCookieByName("refreshToken", actualCookies)
				require.NotNil(t, refreshTokenCookie)
				assert.Equal(t, refreshTokenCookie.Path, "/refresh")
				assert.Equal(t, refreshTokenCookie.MaxAge, 30)
				assert.Equal(t, refreshTokenCookie.Secure, true)
				assert.Equal(t, refreshTokenCookie.HttpOnly, true)
				assert.Equal(t, refreshTokenCookie.SameSite, http.SameSiteStrictMode)
				rememberMeTokenCookie := findCookieByName("rememberMe", actualCookies)
				require.NotNil(t, rememberMeTokenCookie)
				assert.Equal(t, rememberMeTokenCookie.Path, "/refresh")
				assert.Equal(t, rememberMeTokenCookie.MaxAge, 30)
				assert.Equal(t, rememberMeTokenCookie.Secure, true)
				assert.Equal(t, rememberMeTokenCookie.HttpOnly, true)
				assert.Equal(t, rememberMeTokenCookie.SameSite, http.SameSiteStrictMode)

				// TODO: Assert response body?
			}

			{
				t.Log("RefreshTokensRequestBody")

				_, email, password := createUser(t, client, userService)
				var tokens *token.Tokens
				client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth(email, password))
				require.NotEmpty(t, tokens.AccessToken, "should return an access token")
				requestBody := strings.NewReader(fmt.Sprintf(`{"refreshToken": "%s"}`, tokens.RefreshToken))
				request := client.NewRequest(t, http.MethodPost, "/refresh", requestBody, inttest.WithHeader("Content-Type", "application/json"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, response.StatusCode)
				actualCookies := response.Cookies()
				require.Len(t, actualCookies, 2)
				accessTokenCookie := findCookieByName("accessToken", actualCookies)
				require.NotNil(t, accessTokenCookie)
				assert.Equal(t, accessTokenCookie.Path, "/")
				assert.Equal(t, accessTokenCookie.MaxAge, 10)
				assert.Equal(t, accessTokenCookie.Secure, true)
				assert.Equal(t, accessTokenCookie.HttpOnly, true)
				assert.Equal(t, accessTokenCookie.SameSite, http.SameSiteStrictMode)
				refreshTokenCookie := findCookieByName("refreshToken", actualCookies)
				require.NotNil(t, refreshTokenCookie)
				assert.Equal(t, refreshTokenCookie.Path, "/refresh")
				assert.Equal(t, refreshTokenCookie.MaxAge, 20)
				assert.Equal(t, refreshTokenCookie.Secure, true)
				assert.Equal(t, refreshTokenCookie.HttpOnly, true)
				assert.Equal(t, refreshTokenCookie.SameSite, http.SameSiteStrictMode)

				// TODO: Assert response body?
			}

		})

		t.Run("SignInFailed", func(t *testing.T) {
			t.Parallel()

			{
				t.Log("WrongEverything")

				request := client.NewRequest(t, http.MethodPost, "/tokens", strings.NewReader(`{}`), inttest.WithBasicAuth("some-non-existing-user@dhis2.org", "wrongpassword"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.Len(t, response.Cookies(), 0)
			}

			{
				t.Log("WrongPassword")

				_, email, _ := createUser(t, client, userService)
				request := client.NewRequest(t, http.MethodPost, "/tokens", strings.NewReader(`{}`), inttest.WithBasicAuth(email, "wrongpassword"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.Len(t, response.Cookies(), 0)
			}

			{
				t.Log("EmailNotValidated")
				var user model.User
				client.PostJSON(t, "/users", strings.NewReader(`{
					"email":    "no-email-validation@dhis2.org",
					"password": "oneoneoneoneoneoneone111"
				}`), &user)
				require.Equal(t, "no-email-validation@dhis2.org", user.Email)
				require.Empty(t, user.Password)
				request := client.NewRequest(t, http.MethodPost, "/tokens", strings.NewReader(`{}`), inttest.WithBasicAuth("no-email-validation@dhis2.org", "oneoneoneoneoneoneone111"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.Len(t, response.Cookies(), 0)
			}
		})

		t.Run("DeleteOwnUserIsUnauthorized", func(t *testing.T) {
			t.Parallel()

			t.Log("SignIn")
			id, email, password := createUser(t, client, userService)
			requestBody := strings.NewReader(`{}`)
			var tokens *token.Tokens
			client.PostJSON(t, "/tokens", requestBody, &tokens, inttest.WithBasicAuth(email, password))
			require.NotEmpty(t, tokens.AccessToken, "should return an access token")

			t.Log("Delete")
			client.Do(t, http.MethodDelete, fmt.Sprintf("/users/%d", id), nil, http.StatusUnauthorized, inttest.WithAuthToken(tokens.AccessToken))
		})

		t.Run("ResetUserPassword", func(t *testing.T) {
			t.Parallel()

			{
				t.Log("RequestPasswordReset")

				id, email, _ := createUser(t, client, userService)
				requestResetRequestBody := strings.NewReader(`{
					"email":    "` + email + `"
				}`)

				client.Do(t, http.MethodPost, "/users/request-reset", requestResetRequestBody, http.StatusCreated, inttest.WithHeader("Content-Type", "application/json"))

				user, err := userService.FindById(context.Background(), id)
				require.NoError(t, err)
				require.NotEmpty(t, user.PasswordToken, "should have a password token")
				require.NotEmpty(t, user.PasswordTokenTTL, "should have a password token TTL timestamp")

				t.Log("ResetPassword")

				resetRequestBody := strings.NewReader(`{
					"token": "` + user.PasswordToken.String + `",
					"password": "ResetResetResetResetReset"
				}`)
				oldPassword := user.Password

				client.Do(t, http.MethodPost, "/users/reset-password", resetRequestBody, http.StatusCreated, inttest.WithHeader("Content-Type", "application/json"))

				newUser1, _ := userService.FindById(context.Background(), id)
				newPassword := newUser1.Password

				require.NotEqual(t, oldPassword, newPassword, "old and new password should be different")
			}

			{
				t.Log("PasswordResetTokenExpired")

				newUserService := user.NewService("", 1, userRepository, fakeDialer{t})
				id, email, _ := createUser(t, client, userService)
				_ = newUserService.RequestPasswordReset(email)
				user1, err := newUserService.FindById(context.Background(), id)
				require.NoError(t, err)
				// Wait for token to expire
				time.Sleep(5 * time.Second)

				err = newUserService.ResetPassword(user1.PasswordToken.String, "ResetResetResetResetReset")
				require.Error(t, err, "reset token has expired")
			}
		})
	})

	t.Run("AsAdmin", func(t *testing.T) {
		t.Parallel()

		var adminToken token.Tokens
		{
			t.Log("SignIn")

			client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &adminToken, inttest.WithBasicAuth("admin", "admin"))

			require.NotEmpty(t, adminToken.AccessToken, "should return an access token")
		}

		{
			t.Log("GetAllUsers")

			var users []model.User
			client.GetJSON(t, "/users", &users, inttest.WithAuthToken(adminToken.AccessToken))

			assert.Lenf(t, users, 13, "GET /users should return 13 users one of which is an admin")
		}

		{
			t.Log("DeleteUser")

			id, _, _ := createUser(t, client, userService)
			path := fmt.Sprintf("/users/%d", id)
			client.Delete(t, path, inttest.WithAuthToken(adminToken.AccessToken))

			client.Do(t, http.MethodGet, path, nil, http.StatusNotFound, inttest.WithAuthToken(adminToken.AccessToken))
		}
	})
}

type userService interface {
	FindById(context context.Context, id uint) (*model.User, error)
	ValidateEmail(emailToken uuid.UUID) error
}

var userCounter atomic.Uint32

func createUser(t *testing.T, client *inttest.HTTPClient, userService userService) (uint, string, string) {
	t.Helper()

	userCounter.Add(1)
	currentUserCount := userCounter.Load()

	email := fmt.Sprintf("user%d@dhis2.org", currentUserCount)
	password := uuid.NewString()

	requestBody := strings.NewReader(fmt.Sprintf(`{
			"email":    "%s",
			"password": "%s"
		}`, email, password))

	var user model.User
	client.PostJSON(t, "/users", requestBody, &user)

	require.Equal(t, email, user.Email)
	require.Empty(t, user.Password)

	u, err := userService.FindById(context.Background(), user.ID)
	require.NoError(t, err)
	err = userService.ValidateEmail(u.EmailToken)
	require.NoError(t, err)

	return user.ID, email, password
}

type fakeDialer struct {
	t *testing.T
}

func (f fakeDialer) DialAndSend(m ...*mail.Message) error {
	f.t.Log("Fake sending mail...", m[0].GetHeader("To"))
	return nil
}

func findCookieByName(name string, cookies []*http.Cookie) *http.Cookie {
	index := slices.IndexFunc(cookies, func(cookie *http.Cookie) bool {
		return cookie.Name == name
	})
	if index == -1 {
		return nil
	}

	return cookies[index]
}
