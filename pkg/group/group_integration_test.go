package group_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/group"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/user"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupHandler(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	userRepository := user.NewRepository(db)
	userService := user.NewService(userRepository)
	groupRepository := group.NewRepository(db)
	groupService := group.NewService(groupRepository, userService)

	err := user.CreateAdminUser("admin", "admin", userService, groupService)
	require.NoError(t, err, "failed to create admin user and group")

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		handler := group.NewHandler(groupService)
		authentication := TestAuthenticationMiddleware{}
		authorization := TestAuthorizationMiddleware{}
		group.Routes(engine, authentication, authorization, handler)
	})

	var userId string
	var user *model.User
	{
		user, err = userService.FindOrCreate("user@dhis2.org", "oneoneoneoneoneoneone111")
		require.NoError(t, err)
		userId = strconv.FormatUint(uint64(user.ID), 10)
	}

	var groupName string
	{
		group, err := groupService.Create("test-group", "test-hostname.com", true)
		require.NoError(t, err)
		groupName = group.Name
	}

	t.Run("CreateGroup", func(t *testing.T) {
		t.Parallel()

		t.Run("Deployable", func(t *testing.T) {
			requestBody := strings.NewReader(`{
				"name": "deployable-test-group",
				"hostname": "deployable-test-hostname.com",
				"deployable": true
			}`)

			var group model.Group
			client.PostJSON(t, "/groups", requestBody, &group)

			assert.Equal(t, "deployable-test-group", group.Name)
			assert.Equal(t, "deployable-test-hostname.com", group.Hostname)
			assert.True(t, group.Deployable)
		})

		t.Run("NonDeployable", func(t *testing.T) {
			requestBody := strings.NewReader(`{
				"name": "non-deployable-test-group",
				"hostname": "non-deployable-test-hostname.com",
				"deployable": false
			}`)

			var group model.Group
			client.PostJSON(t, "/groups", requestBody, &group)

			assert.Equal(t, "non-deployable-test-group", group.Name)
			assert.Equal(t, "non-deployable-test-hostname.com", group.Hostname)
			assert.False(t, group.Deployable)
		})
	})

	t.Run("AddUserToGroup", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/users/%s", groupName, userId)

			client.Do(t, http.MethodPost, path, nil, http.StatusCreated)
		})

		t.Run("AddUserToGroupNonExistingGroup", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/users/%s", "non-existing-group", userId)

			response := client.Do(t, http.MethodPost, path, nil, http.StatusNotFound)

			require.Equal(t, "group \"non-existing-group\" doesn't exist", string(response))
		})

		t.Run("AddNonExistingUserToGroup", func(t *testing.T) {
			_, err := userService.FindById(99999)
			require.Error(t, err, "user already exists")
			path := fmt.Sprintf("/groups/%s/users/99999", groupName)

			response := client.Do(t, http.MethodPost, path, nil, http.StatusNotFound)

			require.Equal(t, "failed to find user with id 99999", string(response))
		})
	})

	t.Run("RemoveUserFromGroup", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			err := groupService.AddUser(groupName, user.ID)
			require.NoError(t, err)
			path := fmt.Sprintf("/groups/%s/users/%s", groupName, userId)

			client.Do(t, http.MethodDelete, path, nil, http.StatusNoContent)
		})

		t.Run("RemoveUserFromNonExistingGroup", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/users/%s", "non-existing-group", userId)

			response := client.Do(t, http.MethodPost, path, nil, http.StatusNotFound)

			require.Equal(t, "group \"non-existing-group\" doesn't exist", string(response))
		})

		t.Run("RemoveNonExistingUserFromGroup", func(t *testing.T) {
			_, err := userService.FindById(99999)
			require.Error(t, err, "user already exists")
			path := fmt.Sprintf("/groups/%s/users/99999", groupName)

			response := client.Do(t, http.MethodDelete, path, nil, http.StatusNotFound)

			require.Equal(t, "failed to find user with id 99999", string(response))
		})
	})

	t.Run("FindGroup", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s", groupName)

			var group model.Group
			client.GetJSON(t, path, &group)

			require.Equal(t, groupName, group.Name)
		})

		t.Run("FindGroupFailed", func(t *testing.T) {
			client.Do(t, http.MethodGet, "/groups/non-existing-group", nil, http.StatusNotFound)
		})
	})

	t.Run("FindGroupWithDetails", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			err := groupService.AddUser(groupName, user.ID)
			require.NoError(t, err)
			path := fmt.Sprintf("/groups/%s/details", groupName)

			var group model.Group
			client.GetJSON(t, path, &group)

			require.Equal(t, groupName, group.Name)
			require.Equal(t, group.Users[0].ID, user.ID)
		})

		t.Run("FindNonExistingGroupWithDetails", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/details", "non-existing-group")

			response := client.Do(t, http.MethodGet, path, nil, http.StatusNotFound)

			require.Equal(t, "group \"non-existing-group\" doesn't exist", string(response))
		})
	})
}

type TestAuthenticationMiddleware struct{}

func (t TestAuthenticationMiddleware) TokenAuthentication(c *gin.Context) {}

type TestAuthorizationMiddleware struct{}

func (t TestAuthorizationMiddleware) RequireAdministrator(c *gin.Context) {
	c.Next()
}
