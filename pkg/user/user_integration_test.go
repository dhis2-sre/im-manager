package user_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
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
	const passwordTokenTtl = 10
	userService := user.NewService("", passwordTokenTtl, userRepository, fakeDialer{t})
	groupRepository := group.NewRepository(db)
	groupService := group.NewService(groupRepository, userService)

	userCount.Increment()
	err := user.CreateUser("admin", "admin", userService, groupService, model.AdministratorGroupName, "admin")
	require.NoError(t, err, "failed to create admin user and group")
	userCount.Done()

	authorization := middleware.NewAuthorization(slog.New(slog.NewTextHandler(os.Stdout, nil)), userService)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate private key")
	authentication := middleware.NewAuthentication(key.PublicKey, userService)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	redis := inttest.SetupRedis(t)
	tokenRepository := token.NewRepository(redis)
	tokenService, err := token.NewService(logger, tokenRepository, key, 10, "secret", 20, 30)
	require.NoError(t, err)

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		userHandler := user.NewHandler(logger, "hostname", http.SameSiteStrictMode, 10, 20, 30, key.PublicKey, userService, tokenService)
		user.Routes(engine, authentication, authorization, userHandler)
	})

	t.Run("SignUp", func(t *testing.T) {
		t.Parallel()

		t.Log("SignUpUser")

		var user model.User
		userCount.Increment()
		client.PostJSON(t, "/users", jsonBody(`{
			"email":    "user@dhis2.org",
			"password": "oneoneoneoneoneoneone111"
		}`), &user)
		userCount.Done()

		assert.Equal(t, "user@dhis2.org", user.Email)
		assert.Empty(t, user.Password)
		assert.Empty(t, user.EmailToken)
		assert.False(t, user.Validated)

		t.Log("ValidateEmail")

		u, err := userService.FindById(context.Background(), user.ID)
		require.NoError(t, err)
		requestBody := jsonBody(`{"token": "%s"}`, u.EmailToken.String())
		client.Do(t, http.MethodPost, "/users/validate", requestBody, http.StatusOK, inttest.WithHeader("Content-Type", "application/json"))

		t.Log("SignIn")

		accessToken, _ := client.SignIn(t, "user@dhis2.org", "oneoneoneoneoneoneone111")

		t.Log("GetMe")

		var me model.User
		client.GetJSON(t, "/me", &me, inttest.WithAuthToken(accessToken.Value))
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
			accessToken, _ := client.SignIn(t, email, password)
			request := client.NewRequest(t, http.MethodDelete, "/users", nil, inttest.WithAuthToken(accessToken.Value))

			response, err := client.Client.Do(request)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, response.StatusCode)
			assert.EqualValues(t, expectedCookies, response.Cookies())
		})

		t.Run("ExpiredToken", func(t *testing.T) {
			_, email, password := createUser(t, client, userService)
			accessToken, _ := client.SignIn(t, email, password)
			<-time.After(time.Duration(accessToken.Expires.Unix()) * time.Second)
			request := client.NewRequest(t, http.MethodDelete, "/users", nil, inttest.WithAuthToken(accessToken.Value))

			response, err := client.Client.Do(request)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, response.StatusCode)
			assert.EqualValues(t, expectedCookies, response.Cookies())
		})
	})

	t.Run("SignUpFailed", func(t *testing.T) {
		t.Parallel()

		t.Run("InvalidEmail", func(t *testing.T) {
			requestBody := jsonBody(`{
				"email":    "not-a-valid-email",
				"password": "oneoneoneoneoneoneone111"
			}`)

			response := client.Do(t, http.MethodPost, "/users", requestBody, http.StatusBadRequest, inttest.WithHeader("Content-Type", "application/json"))

			require.Equal(t, "invalid email provided: not-a-valid-email", string(response))
		})

		t.Run("TooShortPassword", func(t *testing.T) {
			requestBody := jsonBody(`{
				"email":    "some@email.com",
				"password": "short-password"
			}`)

			response := client.Do(t, http.MethodPost, "/users", requestBody, http.StatusBadRequest, inttest.WithHeader("Content-Type", "application/json"))

			require.Equal(t, "password must be between 24 and 128 characters", string(response))
		})

		t.Run("TooLongPassword", func(t *testing.T) {
			password := strings.Repeat("a", 129)
			requestBody := jsonBody(`{
				"email":    "some@email.com",
				"password": "%s"
			}`, password)

			response := client.Do(t, http.MethodPost, "/users", requestBody, http.StatusBadRequest, inttest.WithHeader("Content-Type", "application/json"))

			require.Equal(t, "password must be between 24 and 128 characters", string(response))
		})

		t.Run("BothEmailAndPasswordAreInvalid", func(t *testing.T) {
			requestBody := jsonBody(`{
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

			var accessToken *http.Cookie
			{
				t.Log("SignIn")

				_, email, password := createUser(t, client, userService)

				accessToken, _ = client.SignIn(t, email, password)

				t.Log("GetMe")

				var me model.User

				client.GetJSON(t, "/me", &me, inttest.WithAuthToken(accessToken.Value))

				assert.Equal(t, email, me.Email)
			}

			{
				t.Log("GetAllIsUnauthorized")

				client.Do(t, http.MethodGet, "/users", nil, http.StatusUnauthorized, inttest.WithAuthToken(accessToken.Value))
			}

			{
				t.Log("SignInCookies")

				_, email, password := createUser(t, client, userService)
				requestBody := jsonBody(`{}`)
				request := client.NewRequest(t, http.MethodPost, "/tokens", requestBody, inttest.WithBasicAuth(email, password), inttest.WithHeader("Content-Type", "application/json"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, response.StatusCode)
				actualCookies := response.Cookies()
				require.Len(t, actualCookies, 2)

				accessTokenCookie := findCookieByName("accessToken", actualCookies)
				require.NotNil(t, accessTokenCookie)
				assertCookie(t, accessTokenCookie, "/", 10, http.SameSiteStrictMode)

				refreshTokenCookie := findCookieByName("refreshToken", actualCookies)
				require.NotNil(t, refreshTokenCookie)
				assertCookie(t, refreshTokenCookie, "/refresh", 20, http.SameSiteStrictMode)
			}

			{
				t.Log("SignInCookiesWithRememberMe")

				_, email, password := createUser(t, client, userService)
				requestBody := jsonBody(`{"rememberMe": true}`)
				request := client.NewRequest(t, http.MethodPost, "/tokens", requestBody, inttest.WithBasicAuth(email, password), inttest.WithHeader("Content-Type", "application/json"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, response.StatusCode)
				actualCookies := response.Cookies()
				require.Len(t, actualCookies, 3)

				accessTokenCookie := findCookieByName("accessToken", actualCookies)
				require.NotNil(t, accessTokenCookie)
				assertCookie(t, accessTokenCookie, "/", 10, http.SameSiteStrictMode)

				refreshTokenCookie := findCookieByName("refreshToken", actualCookies)
				require.NotNil(t, refreshTokenCookie)
				assertCookie(t, refreshTokenCookie, "/refresh", 30, http.SameSiteStrictMode)

				rememberMeTokenCookie := findCookieByName("rememberMe", actualCookies)
				require.NotNil(t, rememberMeTokenCookie)
				assertCookie(t, rememberMeTokenCookie, "/refresh", 30, http.SameSiteStrictMode)
			}

			{
				t.Log("RefreshTokensUsingCookie")

				_, email, password := createUser(t, client, userService)
				accessToken, refreshToken := client.SignIn(t, email, password)
				require.NotEmpty(t, accessToken.Value, "should return an access token")
				request := client.NewRequest(t, http.MethodPost, "/refresh", jsonBody(`{}`), inttest.WithHeader("Content-Type", "application/json"))
				cookie := &http.Cookie{Name: "refreshToken", Value: refreshToken.Value, Path: "/refresh"}
				require.NoError(t, cookie.Valid())
				request.AddCookie(cookie)

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, response.StatusCode)
				actualCookies := response.Cookies()
				require.Len(t, actualCookies, 2)

				accessTokenCookie := findCookieByName("accessToken", actualCookies)
				require.NotNil(t, accessTokenCookie)
				assertCookie(t, accessTokenCookie, "/", 10, http.SameSiteStrictMode)

				refreshTokenCookie := findCookieByName("refreshToken", actualCookies)
				require.NotNil(t, refreshTokenCookie)
				assertCookie(t, refreshTokenCookie, "/refresh", 20, http.SameSiteStrictMode)
			}

			{
				t.Log("RefreshTokensUsingCookieWithRememberMe")

				_, email, password := createUser(t, client, userService)
				accessToken, refreshToken := client.SignIn(t, email, password)
				require.NotEmpty(t, accessToken.Value, "should return an access token")
				request := client.NewRequest(t, http.MethodPost, "/refresh", jsonBody(`{}`), inttest.WithHeader("Content-Type", "application/json"))
				refreshCookie := &http.Cookie{Name: "refreshToken", Value: refreshToken.Value, Path: "/refresh"}
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
				assertCookie(t, accessTokenCookie, "/", 10, http.SameSiteStrictMode)

				refreshTokenCookie := findCookieByName("refreshToken", actualCookies)
				require.NotNil(t, refreshTokenCookie)
				assertCookie(t, refreshTokenCookie, "/refresh", 30, http.SameSiteStrictMode)

				rememberMeTokenCookie := findCookieByName("rememberMe", actualCookies)
				require.NotNil(t, rememberMeTokenCookie)
				assertCookie(t, rememberMeTokenCookie, "/refresh", 30, http.SameSiteStrictMode)
			}

			{
				t.Log("RefreshTokensRequestBody")

				_, email, password := createUser(t, client, userService)
				accessToken, refreshToken := client.SignIn(t, email, password)
				require.NotEmpty(t, accessToken.Value, "should return an access token")
				requestBody := jsonBody(`{"refreshToken": "%s"}`, refreshToken.Value)
				request := client.NewRequest(t, http.MethodPost, "/refresh", requestBody, inttest.WithHeader("Content-Type", "application/json"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusCreated, response.StatusCode)
				actualCookies := response.Cookies()
				require.Len(t, actualCookies, 2)

				accessTokenCookie := findCookieByName("accessToken", actualCookies)
				require.NotNil(t, accessTokenCookie)
				assertCookie(t, accessTokenCookie, "/", 10, http.SameSiteStrictMode)

				refreshTokenCookie := findCookieByName("refreshToken", actualCookies)
				require.NotNil(t, refreshTokenCookie)
				assertCookie(t, refreshTokenCookie, "/refresh", 20, http.SameSiteStrictMode)
			}

		})

		t.Run("SignInFailed", func(t *testing.T) {
			t.Parallel()

			{
				t.Log("WrongEverything")

				request := client.NewRequest(t, http.MethodPost, "/tokens", jsonBody(`{}`), inttest.WithBasicAuth("some-non-existing-user@dhis2.org", "wrongpassword"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.Len(t, response.Cookies(), 0)
			}

			{
				t.Log("WrongPassword")

				_, email, _ := createUser(t, client, userService)
				request := client.NewRequest(t, http.MethodPost, "/tokens", jsonBody(`{}`), inttest.WithBasicAuth(email, "wrongpassword"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.Len(t, response.Cookies(), 0)
			}

			{
				t.Log("EmailNotValidated")

				var user model.User
				userCount.Increment()
				client.PostJSON(t, "/users", jsonBody(`{
					"email":    "no-email-validation@dhis2.org",
					"password": "oneoneoneoneoneoneone111"
				}`), &user)
				userCount.Done()

				require.Equal(t, "no-email-validation@dhis2.org", user.Email)
				require.Empty(t, user.Password)
				request := client.NewRequest(t, http.MethodPost, "/tokens", jsonBody(`{}`), inttest.WithBasicAuth("no-email-validation@dhis2.org", "oneoneoneoneoneoneone111"))

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
			accessToken, _ := client.SignIn(t, email, password)
			require.NotEmpty(t, accessToken.Value, "should return an access token")

			t.Log("Delete")
			client.Do(t, http.MethodDelete, fmt.Sprintf("/users/%d", id), nil, http.StatusUnauthorized, inttest.WithAuthToken(accessToken.Value))
		})

		t.Run("ResetUserPassword", func(t *testing.T) {
			t.Parallel()

			{
				t.Log("RequestPasswordReset")

				id, email, _ := createUser(t, client, userService)
				requestResetRequestBody := jsonBody(`{"email": "%s"}`, email)

				client.Do(t, http.MethodPost, "/users/request-reset", requestResetRequestBody, http.StatusCreated, inttest.WithHeader("Content-Type", "application/json"))

				user, err := userService.FindById(context.Background(), id)
				require.NoError(t, err)
				require.NotEmpty(t, user.PasswordToken, "should have a password token")
				require.NotEmpty(t, user.PasswordTokenTTL, "should have a password token TTL timestamp")

				t.Log("ResetPassword")

				resetRequestBody := jsonBody(`{
					"token": "%s",
					"password": "ResetResetResetResetReset"
				}`, user.PasswordToken.String)
				oldPassword := user.Password

				client.Do(t, http.MethodPost, "/users/reset-password", resetRequestBody, http.StatusCreated, inttest.WithHeader("Content-Type", "application/json"))

				newUser1, _ := userService.FindById(context.Background(), id)
				newPassword := newUser1.Password

				require.NotEqual(t, oldPassword, newPassword, "old and new password should be different")
			}

			{
				t.Log("PasswordResetTokenExpired")

				id, email, _ := createUser(t, client, userService)

				requestResetRequestBody := jsonBody(`{"email": "%s"}`, email)
				client.Do(t, http.MethodPost, "/users/request-reset", requestResetRequestBody, http.StatusCreated, inttest.WithHeader("Content-Type", "application/json"))

				user, err := userService.FindById(context.Background(), id)
				require.NoError(t, err)

				resetRequestBody := jsonBody(`{
					"token": "%s",
					"password": "ResetResetResetResetReset"
				}`, user.PasswordToken.String)

				<-time.After(time.Duration(passwordTokenTtl) * time.Second)

				client.Do(t, http.MethodPost, "/users/reset-password", resetRequestBody, http.StatusBadRequest, inttest.WithHeader("Content-Type", "application/json"))
			}
		})
	})

	t.Run("AsAdmin", func(t *testing.T) {
		t.Parallel()

		var adminAccessToken *http.Cookie
		{
			t.Log("SignIn")

			adminAccessToken, _ = client.SignIn(t, "admin", "admin")

			require.NotEmpty(t, adminAccessToken.Value, "should return an access token")
		}

		{
			t.Log("GetAllUsers")
			userCount.Wait()

			var users []model.User
			client.GetJSON(t, "/users", &users, inttest.WithAuthToken(adminAccessToken.Value))

			expectedNumberOfUsers := userCount.Value()
			assert.Lenf(t, users, expectedNumberOfUsers, "GET /users should return %d users", expectedNumberOfUsers)
			assert.Truef(t, slices.IndexFunc(users, func(u model.User) bool {
				return slices.IndexFunc(u.Groups, func(g model.Group) bool {
					return g.Name == model.AdministratorGroupName
				}) != -1
			}) != -1, "at least one user should be Administrator")
		}

		{
			t.Log("DeleteUser")

			id, _, _ := createUser(t, client, userService)
			path := fmt.Sprintf("/users/%d", id)
			client.Delete(t, path, inttest.WithAuthToken(adminAccessToken.Value))

			client.Do(t, http.MethodGet, path, nil, http.StatusNotFound, inttest.WithAuthToken(adminAccessToken.Value))
		}
	})
}

func assertCookie(t *testing.T, cookie *http.Cookie, path string, maxAge int, sameSiteMode http.SameSite) {
	assert.Equal(t, path, cookie.Path)
	assert.Equal(t, maxAge, cookie.MaxAge)
	assert.Equal(t, true, cookie.Secure)
	assert.Equal(t, true, cookie.HttpOnly)
	assert.Equal(t, sameSiteMode, cookie.SameSite)
}

type userService interface {
	FindById(context context.Context, id uint) (*model.User, error)
	ValidateEmail(emailToken uuid.UUID) error
}

var userCount userCounter

type userCounter struct {
	wg    sync.WaitGroup
	count atomic.Uint32
}

func (uc *userCounter) Increment() {
	uc.wg.Add(1)
	uc.count.Add(1)
}

func (uc *userCounter) Done() {
	uc.wg.Done()
}

func (uc *userCounter) Wait() {
	uc.wg.Wait()
}

func (uc *userCounter) Value() int {
	return int(uc.count.Load())
}

func createUser(t *testing.T, client *inttest.HTTPClient, userService userService) (uint, string, string) {
	t.Helper()

	userCount.Increment()
	email := fmt.Sprintf("user%d@dhis2.org", userCount.Value())
	password := uuid.NewString()
	requestBody := jsonBody(`{"email": "%s", "password": "%s"}`, email, password)

	var user model.User
	client.PostJSON(t, "/users", requestBody, &user)
	userCount.Done()

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

func jsonBody(format string, args ...any) io.Reader {
	return strings.NewReader(fmt.Sprintf(format, args...))
}
