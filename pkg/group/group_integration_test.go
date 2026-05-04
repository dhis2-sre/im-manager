package group_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/cluster"

	"github.com/go-mail/mail"

	"github.com/stretchr/testify/assert"

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
	groupRepository := group.NewRepository(db)

	userRepository := user.NewRepository(db)
	userService := user.NewService("", 900, userRepository, fakeDialer{})

	clusterRepository := cluster.NewRepository(db)
	clusterService := cluster.NewService(clusterRepository, cluster.Encryptor{})

	groupService := group.NewService(groupRepository, userService, clusterService)

	cluster, err := clusterService.FindOrCreate(t.Context(), "default-name", "default-description")
	assert.NoError(t, err)

	err = user.CreateUser(context.Background(), "admin", "admin", userService, groupService, model.AdministratorGroupName, "", "admin")
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
		user, err = userService.FindOrCreate(context.Background(), "user@dhis2.org", "oneoneoneoneoneoneone111")
		require.NoError(t, err)
		userId = strconv.FormatUint(uint64(user.ID), 10)
	}

	var groupName string
	{
		requestBody := strings.NewReader(`{
			"name": "test-group",
			"namespace": "test-group-namespace",
			"description": "test-group-description",
			"hostname": "test-hostname.com"
		}`)

		var group model.Group
		client.PostJSON(t, "/groups", requestBody, &group)

		require.Equal(t, "test-group", group.Name)
		require.Equal(t, "test-group-namespace", group.Namespace)
		require.Equal(t, "test-group-description", group.Description)
		require.Equal(t, "test-hostname.com", group.Hostname)
		require.False(t, group.Deployable)
		groupName = group.Name
	}

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		{
			t.Log("AddUserToGroup")
			path := fmt.Sprintf("/groups/%s/users/%s", groupName, userId)

			client.Do(t, http.MethodPost, path, nil, http.StatusCreated)
		}

		t.Run("FindGroup", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s", groupName)

			var group model.Group
			client.GetJSON(t, path, &group)

			require.Equal(t, groupName, group.Name)
		})

		t.Run("FindGroupWithDetails", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/details", groupName)

			var group model.Group
			client.GetJSON(t, path, &group)

			require.Equal(t, groupName, group.Name)
			require.Equal(t, group.Users[0].ID, user.ID)
		})

		{
			t.Log("RemoveUserFromGroup")
			path := fmt.Sprintf("/groups/%s/users/%s", groupName, userId)

			client.Do(t, http.MethodDelete, path, nil, http.StatusNoContent)
		}

		t.Run("AddAdminUserToGroup", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/admins/%s", groupName, userId)

			client.Do(t, http.MethodPost, path, nil, http.StatusCreated)

			detailsPath := fmt.Sprintf("/groups/%s/details", groupName)
			var g model.Group
			client.GetJSON(t, detailsPath, &g)
			require.Equal(t, groupName, g.Name)
			require.Len(t, g.AdminUsers, 1)
			require.Equal(t, user.ID, g.AdminUsers[0].ID)
		})

		t.Run("RemoveAdminUserFromGroup", func(t *testing.T) {
			addPath := fmt.Sprintf("/groups/%s/admins/%s", groupName, userId)
			client.Do(t, http.MethodPost, addPath, nil, http.StatusCreated)

			removePath := fmt.Sprintf("/groups/%s/admins/%s", groupName, userId)
			client.Do(t, http.MethodDelete, removePath, nil, http.StatusNoContent)

			detailsPath := fmt.Sprintf("/groups/%s/details", groupName)
			var g model.Group
			client.GetJSON(t, detailsPath, &g)
			require.Empty(t, g.AdminUsers)
		})

		t.Run("AddClusterToGroup", func(t *testing.T) {
			group, err := groupService.Find(t.Context(), groupName)
			assert.NoError(t, err)
			assert.Nil(t, group.ClusterID)

			path := fmt.Sprintf("/groups/%s/clusters/%d", groupName, cluster.ID)
			client.Post(t, path, nil)

			group, err = groupService.Find(t.Context(), groupName)
			assert.NoError(t, err)
			assert.Equal(t, cluster.ID, *group.ClusterID)
		})

		t.Run("RemoveClusterFromGroup", func(t *testing.T) {
			group, err := groupService.Find(t.Context(), groupName)
			assert.NoError(t, err)
			assert.NotNil(t, group.ClusterID)
			assert.Equal(t, cluster.ID, *group.ClusterID)

			path := fmt.Sprintf("/groups/%s/clusters/%d", groupName, cluster.ID)
			client.Delete(t, path)

			group, err = groupService.Find(t.Context(), groupName)
			assert.NoError(t, err)
			assert.Nil(t, group.ClusterID)
		})

		t.Run("FindOrCreatePersistsAllFields", func(t *testing.T) {
			t.Parallel()

			g, err := groupService.FindOrCreate(t.Context(), "find-or-create-group", "find-or-create-namespace", "find-or-create-hostname.com", true)
			require.NoError(t, err)
			assert.Equal(t, "find-or-create-group", g.Name)
			assert.Equal(t, "find-or-create-namespace", g.Namespace)
			assert.Equal(t, "find-or-create-hostname.com", g.Hostname)
			assert.True(t, g.Deployable)

			persisted, err := groupService.Find(t.Context(), "find-or-create-group")
			require.NoError(t, err)
			assert.Equal(t, "find-or-create-namespace", persisted.Namespace)
			assert.Equal(t, "find-or-create-hostname.com", persisted.Hostname)
			assert.True(t, persisted.Deployable)
		})

		t.Run("CreateDeployableGroup", func(t *testing.T) {
			t.Parallel()

			requestBody := strings.NewReader(`{
				"name": "deployable-test-group",
				"namespace": "deployable-test-group-namespace",
				"description": "deployable-test-group-description",
				"hostname": "deployable-test-hostname.com",
				"deployable": true
			}`)

			var group model.Group
			client.PostJSON(t, "/groups", requestBody, &group)

			assert.Equal(t, "deployable-test-group", group.Name)
			assert.Equal(t, "deployable-test-group-namespace", group.Namespace)
			assert.Equal(t, "deployable-test-group-description", group.Description)
			assert.Equal(t, "deployable-test-hostname.com", group.Hostname)
			assert.True(t, group.Deployable)
		})
	})

	t.Run("FailedTo", func(t *testing.T) {
		t.Parallel()

		t.Run("AddUserToNonExistingGroup", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/users/%s", "non-existing-group", userId)

			response := client.Do(t, http.MethodPost, path, nil, http.StatusNotFound)

			require.Equal(t, "group \"non-existing-group\" doesn't exist", string(response))
		})

		t.Run("AddNonExistingUserToGroup", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/users/99999", groupName)

			response := client.Do(t, http.MethodPost, path, nil, http.StatusNotFound)

			require.Equal(t, "failed to find user with id 99999", string(response))
		})

		t.Run("RemoveUserFromNonExistingGroup", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/users/%s", "non-existing-group", userId)

			response := client.Do(t, http.MethodDelete, path, nil, http.StatusNotFound)

			require.Equal(t, "group \"non-existing-group\" doesn't exist", string(response))
		})

		t.Run("RemoveNonExistingUserFromGroup", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/users/99999", groupName)

			response := client.Do(t, http.MethodDelete, path, nil, http.StatusNotFound)

			require.Equal(t, "failed to find user with id 99999", string(response))
		})

		t.Run("FindGroup", func(t *testing.T) {
			client.Do(t, http.MethodGet, "/groups/non-existing-group", nil, http.StatusNotFound)
		})

		t.Run("FindNonExistingGroupWithDetails", func(t *testing.T) {
			path := fmt.Sprintf("/groups/%s/details", "non-existing-group")

			response := client.Do(t, http.MethodGet, path, nil, http.StatusNotFound)

			require.Equal(t, "group \"non-existing-group\" doesn't exist", string(response))
		})
	})
}

type fakeDialer struct{}

func (f fakeDialer) DialAndSend(m ...*mail.Message) error {
	panic("not implemented")
}

type TestAuthenticationMiddleware struct{}

func (t TestAuthenticationMiddleware) TokenAuthentication(c *gin.Context) {}

type TestAuthorizationMiddleware struct{}

func (t TestAuthorizationMiddleware) RequireAdministrator(c *gin.Context) {
	c.Next()
}
