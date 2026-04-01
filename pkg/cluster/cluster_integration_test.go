package cluster_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	filippoioage "filippo.io/age"
	"github.com/dhis2-sre/im-manager/pkg/cluster"
	"github.com/dhis2-sre/im-manager/pkg/group"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/user"
	"github.com/gin-gonic/gin"
	"github.com/go-mail/mail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterHandler(t *testing.T) {
	db := inttest.SetupDB(t)

	userRepository := user.NewRepository(db)
	userService := user.NewService("", 900, userRepository, fakeDialer{})

	clusterRepository := cluster.NewRepository(db)
	clusterService := cluster.NewService(clusterRepository)

	identity, err := filippoioage.GenerateX25519Identity()
	require.NoError(t, err, "failed to generate age key pair")

	t.Setenv("SOPS_AGE_KEY", identity.String())

	groupRepository := group.NewRepository(db)
	groupService := group.NewService(groupRepository, userService, clusterService)

	err = user.CreateUser(context.Background(), "admin", "admin", userService, groupService, model.AdministratorGroupName, "", "admin")
	require.NoError(t, err, "failed to create admin user and group")

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		handler := cluster.NewHandler(clusterService)
		authentication := TestAuthenticationMiddleware{}
		authorization := TestAuthorizationMiddleware{}
		cluster.Routes(engine, authentication, authorization, handler)
	})

	var createdClusterID uint

	t.Run("Create", func(t *testing.T) {
		requestBody := strings.NewReader(`{
				"name": "name",
				"description": "description"
			}`)

		var cluster model.Cluster
		client.PostJSON(t, "/clusters", requestBody, &cluster)

		assert.Equal(t, "name", cluster.Name)
		assert.Equal(t, "description", cluster.Description)
		createdClusterID = cluster.ID
	})

	t.Run("Create with configuration", func(t *testing.T) {
		k8sConfig := []byte(`apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://localhost:6443
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user:
    token: test-token`)

		requestBody := strings.NewReader(fmt.Sprintf(`{
			"name": "name-with-config",
			"description": "description-with-config",
			"rawConfig": %q
		}`, base64.StdEncoding.EncodeToString(k8sConfig)))

		var cluster model.Cluster
		client.PostJSON(t, "/clusters", requestBody, &cluster)

		assert.Equal(t, "name-with-config", cluster.Name)
		assert.Equal(t, "description-with-config", cluster.Description)
	})

	t.Run("Read", func(t *testing.T) {
		var cluster model.Cluster
		client.GetJSON(t, "/clusters/"+fmt.Sprint(createdClusterID), &cluster)

		assert.Equal(t, "name", cluster.Name)
		assert.Equal(t, "description", cluster.Description)
	})

	t.Run("Update", func(t *testing.T) {
		requestBody := strings.NewReader(`{
				"name": "new-name",
				"description": "new-description"
			}`)

		var cluster model.Cluster
		client.PutJSON(t, "/clusters/"+fmt.Sprint(createdClusterID), requestBody, &cluster)

		assert.Equal(t, "new-name", cluster.Name)
		assert.Equal(t, "new-description", cluster.Description)
	})

	t.Run("Delete", func(t *testing.T) {
		client.Delete(t, "/clusters/"+fmt.Sprint(createdClusterID))
	})

	t.Run("ReadAll", func(t *testing.T) {
		var clusters []model.Cluster
		client.GetJSON(t, "/clusters", &clusters)

		assert.Len(t, clusters, 1)
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
