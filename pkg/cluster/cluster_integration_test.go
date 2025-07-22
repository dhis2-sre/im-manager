package cluster_test

import (
	"bytes"
	"context"
	"mime/multipart"
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

func TestClusterHandler(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)

	userRepository := user.NewRepository(db)
	userService := user.NewService("", 900, userRepository, fakeDialer{})

	clusterRepository := cluster.NewRepository(db)
	clusterService := cluster.NewService(clusterRepository)

	groupRepository := group.NewRepository(db)
	groupService := group.NewService(groupRepository, userService, clusterService)

	err := user.CreateUser(context.Background(), "admin", "admin", userService, groupService, model.AdministratorGroupName, "", "admin")
	require.NoError(t, err, "failed to create admin user and group")

	create, err := clusterService.FindOrCreate(t.Context(), "default-name", "default-description")
	assert.NoError(t, err)

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		handler := cluster.NewHandler(clusterService)
		authentication := TestAuthenticationMiddleware{}
		authorization := TestAuthorizationMiddleware{}
		cluster.Routes(engine, authentication, authorization, handler)
	})

	t.Run("Create", func(t *testing.T) {
		requestBody := strings.NewReader(`{
				"name": "name",
				"description": "description"
			}`)

		var cluster model.Cluster
		client.PostJSON(t, "/clusters", requestBody, &cluster)

		assert.Equal(t, "name", cluster.Name)
		assert.Equal(t, "description", cluster.Description)
	})

	t.Run("Create with configuration", func(t *testing.T) {
		var b bytes.Buffer
		w := multipart.NewWriter(&b)

		_ = w.WriteField("name", "name-with-configuration")
		_ = w.WriteField("description", "description")

		part, err := w.CreateFormFile("kubernetesConfiguration", "config.yaml")
		require.NoError(t, err)
		_, err = part.Write([]byte("kubernetesConfiguration"))
		require.NoError(t, err)

		w.Close()

		var cluster model.Cluster
		client.PostForm(t, "/clusters", w, &b, &cluster)

		assert.Equal(t, "name-with-configuration", cluster.Name)
		assert.Equal(t, "description", cluster.Description)
	})

	t.Run("Read", func(t *testing.T) {
		var cluster model.Cluster
		client.GetJSON(t, "/clusters/1", &cluster)

		assert.Equal(t, create.Name, cluster.Name)
		assert.Equal(t, create.Description, cluster.Description)
	})

	t.Run("Update", func(t *testing.T) {
		requestBody := strings.NewReader(`{
				"name": "new-name",
				"description": "new-description"
			}`)

		var cluster model.Cluster
		client.PutJSON(t, "/clusters/1", requestBody, &cluster)

		assert.Equal(t, "new-name", cluster.Name)
		assert.Equal(t, "new-description", cluster.Description)
	})

	t.Run("Delete", func(t *testing.T) {
		client.Delete(t, "/clusters/1")
	})

	t.Run("ReadAll", func(t *testing.T) {
		var clusters []model.Cluster
		client.GetJSON(t, "/clusters", &clusters)

		assert.Len(t, clusters, 2)
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
