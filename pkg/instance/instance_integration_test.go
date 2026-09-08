package instance_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/getsops/sops/v3"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/dhis2-sre/im-manager/pkg/cluster"
	"github.com/dhis2-sre/im-manager/pkg/database"
	"github.com/dhis2-sre/im-manager/pkg/deployment"
	"github.com/dhis2-sre/im-manager/pkg/storage"

	"filippo.io/age"
	sops_age "github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/keys"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/kube"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInstanceHandler(t *testing.T) {
	k8sClient := inttest.SetupK8s(t)

	err := createNamespace(t, k8sClient, "group-name")
	require.NoError(t, err, "failed to create test namespace")

	db := inttest.SetupDB(t)
	redis := inttest.SetupRedis(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err, "failed to generate age key pair")

	t.Setenv("SOPS_AGE_KEY", identity.String())

	ageKeys, err := sops_age.MasterKeysFromRecipients(identity.Recipient().String())
	require.NoError(t, err, "failed to get master keys from age recipient")
	var ageMasterKeys []keys.MasterKey
	for _, k := range ageKeys {
		ageMasterKeys = append(ageMasterKeys, k)
	}
	keyGroups := []sops.KeyGroup{ageMasterKeys}

	var k8sConfig []byte
	k8sConfig, err = cluster.EncryptYaml(k8sClient.Config, keyGroups)
	require.NoError(t, err, "failed to encrypt k8s config")

	group := &model.Group{
		ID:         1,
		Name:       "group-name",
		Namespace:  "group-name",
		Hostname:   "some",
		Deployable: true,
		Cluster: model.Cluster{
			Name:          "cluster-name",
			Configuration: k8sConfig,
		},
	}
	user := &model.User{
		Email:      "user1@dhis2.org",
		EmailToken: uuid.New(),
		Groups: []model.Group{
			*group,
		},
	}
	err = db.Create(user).Error
	require.NoError(t, err, "failed to save user")

	nonMember := &model.User{
		Email:      "user2@dhis2.org",
		EmailToken: uuid.New(),
	}
	err = db.Create(nonMember).Error
	require.NoError(t, err, "failed to save non-member user")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	encryptionKey := strings.Repeat("a", 32)
	instanceRepo, err := instance.NewRepository(db, encryptionKey)
	require.NoError(t, err)
	groupService := groupService{group: group}
	stacks := stack.Stacks{
		"whoami-go": stack.WhoamiGo,
		"dhis2-v2":  stack.DHIS2V2,
	}
	stackService := stack.NewService(stacks)
	// classification 'test' does not actually exist, this is used to decrypt the stack parameters
	helmfileService, err := instance.NewHelmfileService(logger, stackService, "../../stacks", "test")
	require.NoError(t, err, "failed to create helmfile service")
	tokenRepository := token.NewRepository(redis)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate RSA private key")
	// The access token must outlive the whole suite: it is minted once up front and the deploy
	// paths refresh it, which fails with "exp" not satisfied once it expires. Slow CI runners
	// blew through the previous 100 seconds.
	tokenService, err := token.NewService(logger, tokenRepository, privateKey, 3600, 60, "secret", 3600, 3600)
	require.NoError(t, err, "failed to create token service")
	instanceService := instance.NewService(logger, instanceRepo, groupService, stackService, helmfileService, nil, "", kube.NewClients(slog.Default()))

	s3Dir := t.TempDir()
	s3Bucket := "database-bucket"
	err = os.Mkdir(s3Dir+"/"+s3Bucket, 0o755)
	require.NoError(t, err, "failed to create S3 output bucket")
	s3 := inttest.SetupS3(t, s3Dir)
	uploader := manager.NewUploader(s3.Client)
	s3Client := storage.NewS3Client(logger, s3.Client, uploader)
	databaseRepository := database.NewRepository(db)
	databaseService := database.NewService(logger, s3Bucket, s3Client, groupService, databaseRepository, kube.NewClient, noopPublisher{})
	deploymentService := deployment.NewService(logger, instanceService, databaseService, tokenService, noopPublisher{})

	// this is only to allow testing using multiple users without bringing in all our auth stack
	authenticator := func(c *gin.Context) {
		authenticatedUser := user
		if c.Query("user") == "non-member" {
			authenticatedUser = nonMember
		}
		ctx := model.NewContextWithUser(c.Request.Context(), authenticatedUser)
		c.Request = c.Request.WithContext(ctx)
	}
	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		var twoDayTTL uint = 172800
		instanceHandler := instance.NewHandler(stackService, groupService, instanceService, deploymentService, twoDayTTL)
		instance.Routes(engine, authenticator, instanceHandler)

		databaseHandler := database.NewHandler(logger, databaseService, groupService, instanceService, stackService, deploymentService)
		database.Routes(engine, authenticator, databaseHandler)
	})

	hostname := client.GetHostname(t)
	// This is used when the database init container is downloading its database from IM
	t.Setenv("HOSTNAME", hostname)

	tokens, err := tokenService.GetTokens(user, "", false)
	require.NoError(t, err, "failed to get tokens")

	databaseID := database.UploadTestDatabase(t, client, "path/name.extension", "select now();", "group-name", inttest.WithAuthToken(tokens.AccessToken))

	t.Run("DeployDeploymentWithoutInstances", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "test-deployment", tokens.AccessToken, WithDescription("some description"))

		path := fmt.Sprintf("/deployments/%d/deploy", deployment.ID)
		response := client.Do(t, http.MethodPost, path, nil, http.StatusBadRequest, inttest.WithAuthToken(tokens.AccessToken))

		assert.Contains(t, "deployment contains no instances", string(response))
	})

	t.Run("Deployment", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "test-deployment-whoami", tokens.AccessToken, WithDescription("some description"))

		deploymentInstance := createWhoamiInstance(t, client, deployment.ID, tokens.AccessToken)

		path := fmt.Sprintf("/instances/%d/details", deploymentInstance.ID)
		var instance model.DeploymentInstance
		client.GetJSON(t, path, &instance, inttest.WithAuthToken(tokens.AccessToken))
		assert.Equal(t, deploymentInstance.ID, instance.ID)
		assert.Equal(t, "group-name", instance.GroupName)
		assert.Equal(t, "whoami-go", instance.StackName)
		{
			parameters := instance.Parameters
			assert.Len(t, parameters, 5)
			assert.NotEqual(t, parameters["CHART_VERSION"], "0.9.0")
			assert.NotEqual(t, parameters["IMAGE_PULL_POLICY"], "IfNotPresent")
			assert.NotEqual(t, parameters["IMAGE_REPOSITORY"], "whoami-go")
			assert.NotEqual(t, parameters["IMAGE_TAG"], "0.6.0")
			assert.NotEqual(t, parameters["REPLICA_COUNT"], "1")
		}

		deployDeployment(t, client, deployment.ID, tokens.AccessToken)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 60, deploymentInstance.Group.ID)

		destroyDeployment(t, client, deployment.ID, tokens.AccessToken)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 10, deploymentInstance.Group.ID)
	})

	t.Run("ComponentsAndReplicaRestart", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "components-deployment", tokens.AccessToken)
		deploymentInstance := createWhoamiInstance(t, client, deployment.ID, tokens.AccessToken)

		deployDeployment(t, client, deployment.ID, tokens.AccessToken)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 60, deploymentInstance.Group.ID)

		path := fmt.Sprintf("/instances/%d/components", deploymentInstance.ID)
		var components []instance.ComponentStatus
		client.GetJSON(t, path, &components, inttest.WithAuthToken(tokens.AccessToken))

		require.Len(t, components, 1)
		assert.Equal(t, "whoami", components[0].Name)
		assert.Equal(t, []kube.Operation{kube.OperationRestart, kube.OperationRestartReplica}, components[0].SupportedOperations)
		require.Len(t, components[0].Replicas, 1)
		replica := components[0].Replicas[0]
		assert.Equal(t, "Running", replica.Phase)
		assert.True(t, replica.Ready)

		path = fmt.Sprintf("/deployments/%d/components", deployment.ID)
		var deploymentComponents []instance.InstanceComponents
		client.GetJSON(t, path, &deploymentComponents, inttest.WithAuthToken(tokens.AccessToken))

		require.Len(t, deploymentComponents, 1)
		assert.Equal(t, deploymentInstance.ID, deploymentComponents[0].InstanceID)
		assert.Equal(t, "whoami-go", deploymentComponents[0].StackName)
		require.Len(t, deploymentComponents[0].Components, 1)
		assert.Equal(t, "whoami", deploymentComponents[0].Components[0].Name)
		require.Len(t, deploymentComponents[0].Components[0].Replicas, 1)

		path = fmt.Sprintf("/instances/%d/restart?replica=%s", deploymentInstance.ID, replica.Name)
		response := client.Do(t, http.MethodPut, path, nil, http.StatusBadRequest, inttest.WithAuthToken(tokens.AccessToken))
		assert.Contains(t, string(response), "replica requires a component selector")

		path = fmt.Sprintf("/instances/%d/restart?selector=whoami&replica=no-such-pod", deploymentInstance.ID)
		client.Do(t, http.MethodPut, path, nil, http.StatusNotFound, inttest.WithAuthToken(tokens.AccessToken))

		path = fmt.Sprintf("/instances/%d/restart?selector=whoami&replica=%s", deploymentInstance.ID, replica.Name)
		client.Do(t, http.MethodPut, path, nil, http.StatusAccepted, inttest.WithAuthToken(tokens.AccessToken))

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			_, err := k8sClient.Client.CoreV1().Pods(deploymentInstance.Group.Namespace).Get(context.Background(), replica.Name, metav1.GetOptions{})
			assert.Truef(c, k8serrors.IsNotFound(err), "pod %q should be replaced after replica restart, err: %v", replica.Name, err)
		}, 60*time.Second, 2*time.Second)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 60, deploymentInstance.Group.ID)

		destroyDeployment(t, client, deployment.ID, tokens.AccessToken)
	})

	t.Run("InstanceWithDetailsDeniedForNonMember", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "details-auth-deployment", tokens.AccessToken)
		deploymentInstance := createWhoamiInstance(t, client, deployment.ID, tokens.AccessToken)

		path := fmt.Sprintf("/instances/%d/details?user=non-member", deploymentInstance.ID)
		response := client.Do(t, http.MethodGet, path, nil, http.StatusUnauthorized, inttest.WithAuthToken(tokens.AccessToken))

		assert.Contains(t, string(response), "read access denied")
	})

	t.Run("GetPublicDeployments", func(t *testing.T) {
		t.Parallel()
		privateDeployment := createDeployment(t, client, "private-deployment", tokens.AccessToken)
		createDHIS2V2Instance(t, client, privateDeployment.ID, databaseID, tokens.AccessToken)
		publicDeployment := createDeployment(t, client, "dev-public-deployment", tokens.AccessToken)
		createDHIS2V2Instance(t, client, publicDeployment.ID, databaseID, tokens.AccessToken, WithPublic(true))

		var groupsWithInstances []instance.GroupWithPublicInstances
		client.GetJSON(t, "/instances/public", &groupsWithInstances)

		require.Len(t, groupsWithInstances, 1)
		assert.Equal(t, "group-name", groupsWithInstances[0].Name)
		instances := groupsWithInstances[0].Categories[0].Instances
		assert.Len(t, instances, 1)
		assert.Equal(t, "dev-public-deployment", instances[0].Name)
		assert.Equal(t, "https://some/dev-public-deployment", instances[0].Hostname)
	})

	t.Run("UpdateDeployment", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "test-deployment-update", tokens.AccessToken, WithDescription("initial description"), WithTTL(86400))
		updatedDeployment := updateDeployment(t, client, deployment.ID, tokens.AccessToken, WithDescription("updated description"), WithTTL(172800))

		assert.Equal(t, deployment.ID, updatedDeployment.ID)
		assert.Equal(t, "updated description", updatedDeployment.Description)
		assert.Equal(t, uint(172800), updatedDeployment.TTL)
	})

	t.Run("UpdateDeploymentInstance", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "test-deployment-instance-update", tokens.AccessToken, WithDescription("some description"))

		deploymentInstance := createWhoamiInstance(t, client, deployment.ID, tokens.AccessToken,
			WithParameter("IMAGE_TAG", "0.6.0"),
			WithPublic(false))

		updatedInstance := updateInstance(t, client, deploymentInstance, tokens.AccessToken,
			WithParameter("IMAGE_TAG", "0.7.0"),
			WithPublic(true))

		assert.Equal(t, deploymentInstance.ID, updatedInstance.ID)
		assert.Equal(t, "0.7.0", updatedInstance.Parameters["IMAGE_TAG"].Value)
		assert.True(t, updatedInstance.Public)
	})

	t.Run("UpdateDeploymentInstancePreservesOtherParameters", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "test-deployment-instance-preserve", tokens.AccessToken, WithDescription("some description"))

		deploymentInstance := createWhoamiInstance(t, client, deployment.ID, tokens.AccessToken,
			WithParameter("IMAGE_TAG", "0.6.0"),
			WithParameter("IMAGE_PULL_POLICY", "IfNotPresent"))

		updatedInstance := updateInstance(t, client, deploymentInstance, tokens.AccessToken,
			WithParameter("IMAGE_PULL_POLICY", "Always"))

		assert.Equal(t, "Always", updatedInstance.Parameters["IMAGE_PULL_POLICY"].Value)
		assert.Equal(t, "0.6.0", updatedInstance.Parameters["IMAGE_TAG"].Value,
			"IMAGE_TAG should be preserved when omitted from the patch body")
	})

	t.Run("UpdateDeploymentInstancePublicOnly", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "test-deployment-instance-public-only", tokens.AccessToken, WithDescription("some description"))

		deploymentInstance := createWhoamiInstance(t, client, deployment.ID, tokens.AccessToken,
			WithParameter("IMAGE_TAG", "0.6.0"),
			WithPublic(false))

		updatedInstance := updateInstance(t, client, deploymentInstance, tokens.AccessToken,
			WithPublic(true))

		assert.True(t, updatedInstance.Public)
		assert.Equal(t, "0.6.0", updatedInstance.Parameters["IMAGE_TAG"].Value,
			"parameters should be preserved when the patch body only changes public")
	})
}

func createNamespace(t *testing.T, k8sClient *inttest.K8sClient, namespace string) error {
	t.Helper()
	_, err := k8sClient.Client.CoreV1().Namespaces().Create(
		t.Context(),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
		metav1.CreateOptions{},
	)
	return err
}

type groupService struct {
	group *model.Group
}

func (gs groupService) FindByGroupNames(ctx context.Context, groupNames []string) ([]model.Group, error) {
	return []model.Group{*gs.group}, nil
}

func (gs groupService) Find(ctx context.Context, name string) (*model.Group, error) {
	return gs.group, nil
}

type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, uint, string, string, any) {}
