package user_test

import (
	"crypto/rand"
	"crypto/rsa"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

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

	authorization := middleware.NewAuthorization(slog.New(slog.NewTextHandler(os.Stdout, nil)), userService)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate private key")
	// TODO(DEVOPS-259) we should not use a pointer as we do not mutate and should not mutate the
	// certificate
	authentication := middleware.NewAuthentication(&key.PublicKey, userService)

	redis := inttest.SetupRedis(t)
	tokenRepository := token.NewRepository(redis)
	tokenService, err := token.NewService(tokenRepository, key, &key.PublicKey, 10, "secret", 10)
	require.NoError(t, err)

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		userHandler := user.NewHandler("hostname", config.Authentication{}, userService, tokenService)
		user.Routes(engine, authentication, authorization, userHandler)
	})

	var user1ID string
	{
		t.Log("SignUpUsers")

		var user1 model.User
		client.PostJSON(t, "/users", strings.NewReader(`{
			"email":    "user1@dhis2.org",
			"password": "oneoneoneoneoneoneone111"
		}`), &user1)

		require.Equal(t, "user1@dhis2.org", user1.Email)
		require.Empty(t, user1.Password)
		user1ID = strconv.FormatUint(uint64(user1.ID), 10)

		u1, err := userService.FindById(user1.ID)
		require.NoError(t, err)
		err = userService.ValidateEmail(u1.EmailToken)
		require.NoError(t, err)

		var user2 model.User
		client.PostJSON(t, "/users", strings.NewReader(`{
			"email":    "user2@dhis2.org",
			"password": "oneoneoneoneoneoneone111"
		}`), &user2)

		require.Equal(t, "user2@dhis2.org", user2.Email)
		require.Empty(t, user2.Password)

		u2, err := userService.FindById(user2.ID)
		require.NoError(t, err)
		err = userService.ValidateEmail(u2.EmailToken)
		require.NoError(t, err)

		var user3 model.User
		client.PostJSON(t, "/users", strings.NewReader(`{
			"email":    "user3@dhis2.org",
			"password": "oneoneoneoneoneoneone111"
		}`), &user3)

		require.Equal(t, "user3@dhis2.org", user3.Email)
		require.Empty(t, user3.Password)
	}

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

				client.PostJSON(t, "/tokens", nil, &user1Token, inttest.WithBasicAuth("user1@dhis2.org", "oneoneoneoneoneoneone111"))

				require.NotEmpty(t, user1Token.AccessToken, "should return an access token")
			}

			{
				t.Log("GetMe")

				var me model.User
				client.GetJSON(t, "/me", &me, inttest.WithAuthToken(user1Token.AccessToken))

				assert.Equal(t, "user1@dhis2.org", me.Email)
			}

			{
				t.Log("GetAllIsUnauthorized")

				client.Do(t, http.MethodGet, "/users", nil, http.StatusUnauthorized, inttest.WithAuthToken(user1Token.AccessToken))
			}
		})

		t.Run("SignInFailed", func(t *testing.T) {
			t.Parallel()

			{
				t.Log("WrongPassword")

				client.Do(t, http.MethodPost, "/tokens", nil, http.StatusUnauthorized, inttest.WithBasicAuth("user1@dhis2.org", "wrongpassword"))
			}

			{
				t.Log("EmailNotValidated")

				client.Do(t, http.MethodPost, "/tokens", nil, http.StatusUnauthorized, inttest.WithBasicAuth("user3@dhis2.org", "oneoneoneoneoneoneone111"))
			}
		})

		t.Run("DeleteUserIsUnauthorized", func(t *testing.T) {
			t.Parallel()

			var user2Token *token.Tokens
			{
				t.Log("SignIn")

				client.PostJSON(t, "/tokens", nil, &user2Token, inttest.WithBasicAuth("user2@dhis2.org", "oneoneoneoneoneoneone111"))

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

			client.PostJSON(t, "/tokens", nil, &adminToken, inttest.WithBasicAuth("admin", "admin"))

			require.NotEmpty(t, adminToken.AccessToken, "should return an access token")
		}

		{
			t.Log("GetAllUsers")

			var users []model.User
			client.GetJSON(t, "/users", &users, inttest.WithAuthToken(adminToken.AccessToken))

			assert.Lenf(t, users, 4, "GET /users should return 4 users one of which is an admin")
		}

		{
			t.Log("DeleteUser")

			client.Delete(t, "/users/"+user1ID, inttest.WithAuthToken(adminToken.AccessToken))

			client.Do(t, http.MethodGet, "/users/"+user1ID, nil, http.StatusNotFound, inttest.WithAuthToken(adminToken.AccessToken))
		}
	})
}

type fakeDialer struct {
	t *testing.T
}

func (f fakeDialer) DialAndSend(m ...*mail.Message) error {
	f.t.Log("Fake sending mail...", m[0].GetHeader("To"))
	return nil
}
