package user_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

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
	userService := user.NewService(userRepository)
	groupRepository := group.NewRepository(db)
	groupService := group.NewService(groupRepository, userService)

	err := createAdminUser("admin", "admin", userService, groupService)
	require.NoError(t, err, "failed to create admin user and group")

	authorization := middleware.NewAuthorization(userService)
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate private key")
	// TODO(DEVOPS-259) we should not use a pointer as we do not mutate and should not mutate the
	// certificate
	authentication := middleware.NewAuthentication(&privKey.PublicKey, userService)

	redis := inttest.SetupRedis(t)
	tokenRepository := token.NewRepository(redis)
	tokenService, err := token.NewService(tokenRepository, privKey, &privKey.PublicKey, 10, "secret", 10)
	require.NoError(t, err)

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		userHandler := user.NewHandler(userService, tokenService)
		user.Routes(engine, authentication, authorization, userHandler)
	})

	var user1ID string
	{
		t.Log("SignUpUsers")

		var user1 model.User
		client.PostJSON(t, "/users", strings.NewReader(`{
			"email":    "user1@dhis2.org",
			"password": "oneoneoneoneone1"
		}`), &user1)

		require.Equal(t, "user1@dhis2.org", user1.Email)
		require.Empty(t, user1.Password)
		user1ID = strconv.FormatUint(uint64(user1.ID), 10)

		var user2 model.User
		client.PostJSON(t, "/users", strings.NewReader(`{
			"email":    "user2@dhis2.org",
			"password": "oneoneoneoneone1"
		}`), &user2)

		require.Equal(t, "user2@dhis2.org", user2.Email)
		require.Empty(t, user2.Password)
	}

	t.Run("AsNonAdmin", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			t.Parallel()

			var user1Token *token.Tokens
			{
				t.Log("SignIn")

				client.PostJSON(t, "/tokens", nil, &user1Token, inttest.WithBasicAuth("user1@dhis2.org", "oneoneoneoneone1"))

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

			client.Do(t, http.MethodPost, "/tokens", nil, http.StatusUnauthorized, inttest.WithBasicAuth("user1@dhis2.org", "wrongpassword"))
		})

		t.Run("DeleteUserIsUnauthorized", func(t *testing.T) {
			t.Parallel()

			var user2Token *token.Tokens
			{
				t.Log("SignIn")

				client.PostJSON(t, "/tokens", nil, &user2Token, inttest.WithBasicAuth("user2@dhis2.org", "oneoneoneoneone1"))

				require.NotEmpty(t, user2Token.AccessToken, "should return an access token")
			}

			{
				t.Log("Delete")

				client.Do(t, http.MethodDelete, "/users/"+user1ID, nil, http.StatusUnauthorized, inttest.WithAuthToken(user2Token.AccessToken))
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

			assert.Lenf(t, users, 3, "GET /users should return 3 users one of which is an admin")
		}

		{
			t.Log("DeleteUser")

			client.Delete(t, "/users/"+user1ID, inttest.WithAuthToken(adminToken.AccessToken))

			client.Do(t, http.MethodGet, "/users/"+user1ID, nil, http.StatusNotFound, inttest.WithAuthToken(adminToken.AccessToken))
		}
	})
}

type groupService interface {
	FindOrCreate(name string, hostname string, deployable bool) (*model.Group, error)
	AddUser(groupName string, userId uint) error
}

type userService interface {
	FindOrCreate(email string, password string) (*model.User, error)
}

func createAdminUser(user string, password string, userService userService, groupService groupService) error {
	u, err := userService.FindOrCreate(user, password)
	if err != nil {
		return fmt.Errorf("error creating admin user: %v", err)
	}

	g, err := groupService.FindOrCreate(model.AdministratorGroupName, "", false)
	if err != nil {
		return fmt.Errorf("error creating admin group: %v", err)
	}

	err = groupService.AddUser(g.Name, u.ID)
	if err != nil {
		return fmt.Errorf("error adding admin user to admin group: %v", err)
	}

	return nil
}
