package user_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"golang.org/x/exp/slices"

	"github.com/dhis2-sre/im-manager/pkg/config"
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

	authorization := middleware.NewAuthorization(userService)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate private key")
	// TODO(DEVOPS-259) we should not use a pointer as we do not mutate and should not mutate the certificate
	authentication := middleware.NewAuthentication(&key.PublicKey, userService)

	redis := inttest.SetupRedis(t)
	tokenRepository := token.NewRepository(redis)
	authenticationConfig := config.Authentication{
		RefreshTokenSecretKey:                   "secret",
		AccessTokenExpirationSeconds:            10,
		RefreshTokenExpirationSeconds:           20,
		RefreshTokenRememberMeExpirationSeconds: 30,
	}
	tokenService, err := token.NewService(tokenRepository, key, &key.PublicKey, authenticationConfig)
	require.NoError(t, err)

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		userHandler := user.NewHandler("hostname", 10, 20, 30, &key.PublicKey, userService, tokenService)
		user.Routes(engine, authentication, authorization, userHandler)
	})

	var user1ID string
	{
		t.Log("SignUpUsers")

		user1 := createUser(t, client, userService, "user1@dhis2.org", "oneoneoneoneoneoneone111")
		user1ID = strconv.FormatUint(uint64(user1.ID), 10)

		// Create user without validating email
		var user2 model.User
		client.PostJSON(t, "/users", strings.NewReader(`{
			"email":    "user2@dhis2.org",
			"password": "oneoneoneoneoneoneone111"
		}`), &user2)

		require.Equal(t, "user2@dhis2.org", user2.Email)
		require.Empty(t, user2.Password)

		createUser(t, client, userService, "user3@dhis2.org", "oneoneoneoneoneoneone111")
	}

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
			createUser(t, client, userService, "user4@dhis2.org", "oneoneoneoneoneoneone111")

			var tokens *token.Tokens
			client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth("user4@dhis2.org", "oneoneoneoneoneoneone111"))

			request := client.NewRequest(t, http.MethodDelete, "/users", nil, inttest.WithAuthToken(tokens.AccessToken))

			response, err := client.Client.Do(request)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, response.StatusCode)
			assert.EqualValues(t, expectedCookies, response.Cookies())
		})

		t.Run("ExpiredToken", func(t *testing.T) {
			createUser(t, client, userService, "user5@dhis2.org", "oneoneoneoneoneoneone111")

			var tokens *token.Tokens
			client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth("user5@dhis2.org", "oneoneoneoneoneoneone111"))

			time.Sleep(time.Duration(tokens.ExpiresIn) * time.Second)

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

			var user1Token *token.Tokens
			{
				t.Log("SignIn")

				createUser(t, client, userService, "user6@dhis2.org", "oneoneoneoneoneoneone111")
				requestBody := strings.NewReader(`{}`)
				client.PostJSON(t, "/tokens", requestBody, &user1Token, inttest.WithBasicAuth("user6@dhis2.org", "oneoneoneoneoneoneone111"))

				require.NotEmpty(t, user1Token.AccessToken, "should return an access token")
			}

			{
				t.Log("GetMe")

				var me model.User
				client.GetJSON(t, "/me", &me, inttest.WithAuthToken(user1Token.AccessToken))

				assert.Equal(t, "user6@dhis2.org", me.Email)
			}

			{
				t.Log("GetAllIsUnauthorized")

				client.Do(t, http.MethodGet, "/users", nil, http.StatusUnauthorized, inttest.WithAuthToken(user1Token.AccessToken))
			}

			{
				t.Log("SignInCookies")

				createUser(t, client, userService, "user7@dhis2.org", "oneoneoneoneoneoneone111")
				requestBody := strings.NewReader(`{}`)
				request := client.NewRequest(t, http.MethodPost, "/tokens", requestBody, inttest.WithBasicAuth("user7@dhis2.org", "oneoneoneoneoneoneone111"), inttest.WithHeader("Content-Type", "application/json"))

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

				createUser(t, client, userService, "user8@dhis2.org", "oneoneoneoneoneoneone111")
				requestBody := strings.NewReader(`{"rememberMe": true}`)
				request := client.NewRequest(t, http.MethodPost, "/tokens", requestBody, inttest.WithBasicAuth("user8@dhis2.org", "oneoneoneoneoneoneone111"), inttest.WithHeader("Content-Type", "application/json"))

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

				createUser(t, client, userService, "user9@dhis2.org", "oneoneoneoneoneoneone111")
				var tokens *token.Tokens
				client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth("user9@dhis2.org", "oneoneoneoneoneoneone111"))
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

				createUser(t, client, userService, "user10@dhis2.org", "oneoneoneoneoneoneone111")
				var tokens *token.Tokens
				client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth("user10@dhis2.org", "oneoneoneoneoneoneone111"))
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

				createUser(t, client, userService, "user11@dhis2.org", "oneoneoneoneoneoneone111")
				var tokens *token.Tokens
				client.PostJSON(t, "/tokens", strings.NewReader(`{}`), &tokens, inttest.WithBasicAuth("user11@dhis2.org", "oneoneoneoneoneoneone111"))
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
				t.Log("WrongPassword")

				requestBody := strings.NewReader(`{}`)
				client.Do(t, http.MethodPost, "/tokens", requestBody, http.StatusUnauthorized, inttest.WithBasicAuth("user1@dhis2.org", "wrongpassword"))
			}

			{
				t.Log("WrongPasswordNoCookies")

				requestBody := strings.NewReader(`{}`)
				request := client.NewRequest(t, http.MethodPost, "/tokens", requestBody, inttest.WithBasicAuth("user1@dhis2.org", "wrongpassword"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.Len(t, response.Cookies(), 0)
			}

			{
				t.Log("EmailNotValidated")

				requestBody := strings.NewReader(`{}`)
				client.Do(t, http.MethodPost, "/tokens", requestBody, http.StatusUnauthorized, inttest.WithBasicAuth("user2@dhis2.org", "oneoneoneoneoneoneone111"), inttest.WithHeader("Content-Type", "application/json"))
			}

			{
				t.Log("EmailNotValidatedNoCookies")

				requestBody := strings.NewReader(`{}`)
				request := client.NewRequest(t, http.MethodPost, "/tokens", requestBody, inttest.WithBasicAuth("user2@dhis2.org", "oneoneoneoneoneoneone111"))

				response, err := client.Client.Do(request)
				require.NoError(t, err)

				assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
				assert.Len(t, response.Cookies(), 0)
			}
		})

		t.Run("DeleteUserIsUnauthorized", func(t *testing.T) {
			t.Parallel()

			var user2Token *token.Tokens
			{
				t.Log("SignIn")

				requestBody := strings.NewReader(`{}`)
				client.PostJSON(t, "/tokens", requestBody, &user2Token, inttest.WithBasicAuth("user1@dhis2.org", "oneoneoneoneoneoneone111"))

				require.NotEmpty(t, user2Token.AccessToken, "should return an access token")
			}

			{
				t.Log("Delete")

				client.Do(t, http.MethodDelete, "/users/"+user1ID, nil, http.StatusUnauthorized, inttest.WithAuthToken(user2Token.AccessToken))
			}
		})

		t.Run("ResetUserPassword", func(t *testing.T) {
			t.Parallel()

			{
				t.Log("RequestPasswordReset")

				requestResetRequestBody := strings.NewReader(`{
					"email":    "user1@dhis2.org"
				}`)

				client.Do(t, http.MethodPost, "/users/request-reset", requestResetRequestBody, http.StatusCreated, inttest.WithHeader("Content-Type", "application/json"))

				user1ID, err := strconv.ParseUint(user1ID, 10, 0)
				require.NoError(t, err)
				user1IDInt := uint(user1ID)
				user1, err := userService.FindById(user1IDInt)
				require.NoError(t, err)

				require.NotEmpty(t, user1.PasswordToken, "should have a password token")
				require.NotEmpty(t, user1.PasswordTokenTTL, "should have a password token TTL timestamp")

				t.Log("ResetPassword")

				resetRequestBody := strings.NewReader(`{
					"token": "` + user1.PasswordToken.String + `",
					"password": "ResetResetResetResetReset"
				}`)

				oldPassword := user1.Password

				client.Do(t, http.MethodPost, "/users/reset-password", resetRequestBody, http.StatusCreated, inttest.WithHeader("Content-Type", "application/json"))

				newUser1, _ := userService.FindById(user1IDInt)
				newPassword := newUser1.Password

				require.NotEqual(t, oldPassword, newPassword, "old and new password should be different")
			}

			{
				t.Log("PasswordResetTokenExpired")

				newUserService := user.NewService("", 1, userRepository, fakeDialer{t})

				_ = newUserService.RequestPasswordReset("user1@dhis2.org")

				user1ID, err := strconv.ParseUint(user1ID, 10, 0)
				require.NoError(t, err)
				user1IDInt := uint(user1ID)
				user1, _ := newUserService.FindById(user1IDInt)
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

			requestBody := strings.NewReader(`{}`)
			client.PostJSON(t, "/tokens", requestBody, &adminToken, inttest.WithBasicAuth("admin", "admin"))

			require.NotEmpty(t, adminToken.AccessToken, "should return an access token")
		}

		{
			t.Log("GetAllUsers")

			var users []model.User
			client.GetJSON(t, "/users", &users, inttest.WithAuthToken(adminToken.AccessToken))

			assert.Lenf(t, users, 11, "GET /users should return 11 users one of which is an admin")
		}

		{
			t.Log("DeleteUser")

			client.Delete(t, "/users/"+user1ID, inttest.WithAuthToken(adminToken.AccessToken))

			client.Do(t, http.MethodGet, "/users/"+user1ID, nil, http.StatusNotFound, inttest.WithAuthToken(adminToken.AccessToken))
		}
	})
}

type userService interface {
	FindById(id uint) (*model.User, error)
	ValidateEmail(emailToken uuid.UUID) error
}

func createUser(t *testing.T, client *inttest.HTTPClient, userService userService, email string, password string) *model.User {
	requestBody := strings.NewReader(fmt.Sprintf(`{
			"email":    "%s",
			"password": "%s"
		}`, email, password))

	user := model.User{}
	client.PostJSON(t, "/users", requestBody, &user)

	require.Equal(t, email, user.Email)
	require.Empty(t, user.Password)

	u, err := userService.FindById(user.ID)
	require.NoError(t, err)
	err = userService.ValidateEmail(u.EmailToken)
	require.NoError(t, err)
	return u
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
