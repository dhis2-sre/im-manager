package instance_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/getsops/sops/v3"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/dhis2-sre/im-manager/pkg/database"
	"github.com/dhis2-sre/im-manager/pkg/storage"

	"filippo.io/age"
	"github.com/getsops/sops/v3/aes"
	sops_age "github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/cmd/sops/common"
	"github.com/getsops/sops/v3/keys"
	"github.com/getsops/sops/v3/keyservice"
	"github.com/getsops/sops/v3/stores/yaml"
	"github.com/getsops/sops/v3/version"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstanceHandler(t *testing.T) {
	k8sClient := inttest.SetupK8s(t)
	db := inttest.SetupDB(t)
	redis := inttest.SetupRedis(t)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err, "failed to generate age key pair")
	t.Setenv("SOPS_KMS_ARN", "") // make sure not to use key stored in AWS
	t.Setenv(sops_age.SopsAgeKeyEnv, identity.String())
	k8sConfig := encryptUsingAge(t, identity, k8sClient.Config)

	group := &model.Group{
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
		Email: "user1@dhis2.org",
		Groups: []model.Group{
			*group,
		},
	}
	db.Create(user)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	encryptionKey := strings.Repeat("a", 32)
	instanceRepo := instance.NewRepository(db, encryptionKey)
	groupService := groupService{group: group}
	stacks := stack.Stacks{
		"minio":      stack.MINIO,
		"whoami-go":  stack.WhoamiGo,
		"dhis2-db":   stack.DHIS2DB,
		"dhis2-core": stack.DHIS2Core,
		"dhis2":      stack.DHIS2,
	}
	stackService := stack.NewService(stacks)
	// classification 'test' does not actually exist, this is used to decrypt the stack parameters
	helmfileService := instance.NewHelmfileService(logger, stackService, "../../stacks", "test")
	tokenRepository := token.NewRepository(redis)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate RSA private key")
	tokenService, err := token.NewService(logger, tokenRepository, privateKey, 100, "secret", 100, 100)
	require.NoError(t, err, "failed to create token service")
	instanceService := instance.NewService(logger, instanceRepo, groupService, stackService, helmfileService, nil, "", tokenService)

	s3Dir := t.TempDir()
	s3Bucket := "database-bucket"
	err = os.Mkdir(s3Dir+"/"+s3Bucket, 0o755)
	require.NoError(t, err, "failed to create S3 output bucket")
	s3 := inttest.SetupS3(t, s3Dir)
	uploader := manager.NewUploader(s3.Client)
	s3Client := storage.NewS3Client(logger, s3.Client, uploader)
	databaseRepository := database.NewRepository(db)
	databaseService := database.NewService(logger, s3Bucket, s3Client, groupService, databaseRepository)

	tokens, err := tokenService.GetTokens(user, "", false)
	authenticator := func(c *gin.Context) {
		ctx := model.NewContextWithUser(c.Request.Context(), user)
		c.Request = c.Request.WithContext(ctx)
	}
	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		var twoDayTTL uint = 172800
		instanceHandler := instance.NewHandler(stackService, groupService, instanceService, twoDayTTL)
		instance.Routes(engine, authenticator, instanceHandler)

		databaseHandler := database.NewHandler(logger, databaseService, groupService, instanceService, stackService)
		database.Routes(engine, authenticator, databaseHandler)
	})

	hostname := client.GetHostname(t)
	// This is used when the database init container is downloading its database from IM
	t.Setenv("HOSTNAME", hostname)

	var databaseID string
	{
		t.Log("Upload")
		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		err := w.WriteField("group", "group-name")
		require.NoError(t, err, "failed to write form field")
		err = w.WriteField("name", "path/name.extension")
		require.NoError(t, err, "failed to write form field")
		f, err := w.CreateFormFile("database", "mydb")
		require.NoError(t, err, "failed to create form file")
		_, err = io.WriteString(f, "select now();")
		require.NoError(t, err, "failed to write file")
		_ = w.Close()

		body := client.Put(t, "/databases", &b, http.StatusCreated,
			inttest.WithHeader("X-Upload-Group", "group-name"),
			inttest.WithHeader("X-Upload-Name", "path/name.extension"),
			inttest.WithHeader("X-Upload-Description", "Some database"),
			inttest.WithAuthToken(tokens.AccessToken),
		)

		var actualDB model.Database
		err = json.Unmarshal(body, &actualDB)
		require.NoError(t, err, "POST /databases: failed to unmarshal HTTP response body")
		require.Equal(t, "path/name.extension", actualDB.Name)
		require.Equal(t, "group-name", actualDB.GroupName)

		databaseID = strconv.FormatUint(uint64(actualDB.ID), 10)
	}

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
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 60)

		// TODO:		t.Log("Save as deployment")

		destroyDeployment(t, client, deployment.ID, tokens.AccessToken)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 10)
	})

	t.Run("GetPublicDeployments", func(t *testing.T) {
		t.Parallel()
		privateDeployment := createDeployment(t, client, "private-deployment", tokens.AccessToken)
		createDHIS2DBInstance(t, client, privateDeployment.ID, databaseID, tokens.AccessToken)
		createDHIS2CoreInstance(t, client, privateDeployment.ID, tokens.AccessToken)
		publicDeployment := createDeployment(t, client, "dev-public-deployment", tokens.AccessToken)
		createDHIS2DBInstance(t, client, publicDeployment.ID, databaseID, tokens.AccessToken)
		createDHIS2CoreInstance(t, client, publicDeployment.ID, tokens.AccessToken, WithPublic(true))

		var groupsWithInstances []instance.GroupWithPublicInstances
		client.GetJSON(t, "/instances/public", &groupsWithInstances)

		require.Len(t, groupsWithInstances, 1)
		assert.Equal(t, "group-name", groupsWithInstances[0].Name)
		instances := groupsWithInstances[0].Categories[0].Instances
		assert.Len(t, instances, 1)
		assert.Equal(t, "dev-public-deployment", instances[0].Name)
		assert.Equal(t, "https://some/dev-public-deployment", instances[0].Hostname)
	})

	t.Run("DeploymentWithCompanionStack", func(t *testing.T) {
		t.Parallel()
		t.Log("Create deployment")
		var deployment model.Deployment
		body := strings.NewReader(`{
			"name": "companion-deployment",
			"group": "group-name",
			"description": "some description"
		}`)

		client.PostJSON(t, "/deployments", body, &deployment, inttest.WithAuthToken(tokens.AccessToken))

		assert.Equal(t, "companion-deployment", deployment.Name)
		assert.Equal(t, "group-name", deployment.GroupName)
		assert.Equal(t, "some description", deployment.Description)

		t.Log("Create dhis2-db instance")
		path := fmt.Sprintf("/deployments/%d/instance", deployment.ID)
		body = strings.NewReader(fmt.Sprintf(`{
			"stackName": "dhis2-db",
			"parameters": {
				"DATABASE_ID": {
					"value": "%s"
				}
			}
		}`, databaseID))
		var deploymentInstance model.DeploymentInstance
		client.PostJSON(t, path, body, &deploymentInstance, inttest.WithAuthToken(tokens.AccessToken))
		assert.Equal(t, deployment.ID, deploymentInstance.DeploymentID)
		assert.Equal(t, "group-name", deploymentInstance.GroupName)
		assert.Equal(t, "dhis2-db", deploymentInstance.StackName)

		t.Log("Create minio instance")
		path = fmt.Sprintf("/deployments/%d/instance", deployment.ID)
		body = strings.NewReader(`{"stackName": "minio"}`)
		client.PostJSON(t, path, body, &deploymentInstance, inttest.WithAuthToken(tokens.AccessToken))
		assert.Equal(t, deployment.ID, deploymentInstance.DeploymentID)
		assert.Equal(t, "group-name", deploymentInstance.GroupName)
		assert.Equal(t, "minio", deploymentInstance.StackName)
		t.Log("Create dhis2-core instance")
		body = strings.NewReader(`{"stackName": "dhis2-core"}`)
		body = strings.NewReader(`{
			"stackName": "dhis2-core",
			"parameters": {
				"ALLOW_SUSPEND": {
					"value": "false"
				}
			}
		}`)
		client.PostJSON(t, path, body, &deploymentInstance, inttest.WithAuthToken(tokens.AccessToken))
		assert.Equal(t, deployment.ID, deploymentInstance.DeploymentID)
		assert.Equal(t, "group-name", deploymentInstance.GroupName)
		assert.Equal(t, "dhis2-core", deploymentInstance.StackName)

		t.Log("Deploy deployment")
		path = fmt.Sprintf("/deployments/%d/deploy", deployment.ID)
		client.Do(t, http.MethodPost, path, nil, http.StatusOK, inttest.WithAuthToken(tokens.AccessToken))
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name+"-database", 30)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name+"-minio", 30)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 90)

		t.Log("Destroy deployment")
		path = fmt.Sprintf("/deployments/%d", deployment.ID)
		client.Do(t, http.MethodDelete, path, nil, http.StatusAccepted, inttest.WithAuthToken(tokens.AccessToken))
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 10)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, deploymentInstance.Name+"-minio", 30)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, deploymentInstance.Name+"-database", 10)
	})

	t.Run("UpdateDeployment", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "companion-deployment", tokens.AccessToken, WithDescription("some description"))
		deploymentInstance := createDHIS2DBInstance(t, client, deployment.ID, databaseID, tokens.AccessToken)
		deploymentInstance = createMinioInstance(t, client, deployment.ID, tokens.AccessToken)
		deploymentInstance = createDHIS2CoreInstance(t, client, deployment.ID, tokens.AccessToken, WithParameter("ALLOW_SUSPEND", "false"))

		deployDeployment(t, client, deployment.ID, tokens.AccessToken)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name+"-database", 30)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name+"-minio", 30)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 90)

		destroyDeployment(t, client, deployment.ID, tokens.AccessToken)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 10)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, deploymentInstance.Name+"-minio", 30)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, deploymentInstance.Name+"-database", 10)
	})

	t.Run("UpdateDeployment", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "test-deployment-update", tokens.AccessToken, WithDescription("initial description"), WithTTL(86400))
		updateDeployment(t, client, deployment.ID, tokens.AccessToken, WithDescription("updated description"), WithTTL(172800))
	})

	t.Run("UpdateDeploymentInstance", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "test-deployment-instance-update", tokens.AccessToken, WithDescription("some description"))
		deploymentInstance := createWhoamiInstance(t, client, deployment.ID, tokens.AccessToken,
			WithParameter("IMAGE_TAG", "0.6.0"),
			WithPublic(false))

		t.Log("Update deployment instance")
		path := fmt.Sprintf("/deployments/%d/instance/%d", deployment.ID, deploymentInstance.ID)
		body := strings.NewReader(`{
			"stackName": "whoami-go",
			"parameters": {
				"IMAGE_TAG": {
					"value": "0.7.0"
				}
			},
			"public": true
		}`)
		var updatedInstance model.DeploymentInstance
		client.PutJSON(t, path, body, &updatedInstance, inttest.WithAuthToken(tokens.AccessToken))

		assert.Equal(t, "0.7.0", updatedInstance.Parameters["IMAGE_TAG"].Value)
		assert.True(t, updatedInstance.Public)
		assert.Equal(t, "test-deployment-instance-update", updatedInstance.Name)
		assert.Equal(t, "group-name", updatedInstance.GroupName)
	})
}

func encryptUsingAge(t *testing.T, identity *age.X25519Identity, yamlData []byte) []byte {
	inputStore := &yaml.Store{}
	branches, err := inputStore.LoadPlainFile(yamlData)
	require.NoError(t, err, "failed to load file")

	ageKeys, err := sops_age.MasterKeysFromRecipients(identity.Recipient().String())
	require.NoError(t, err, "failed to get master keys from age recipient")
	var ageMasterKeys []keys.MasterKey
	for _, k := range ageKeys {
		ageMasterKeys = append(ageMasterKeys, k)
	}
	keyGroups := []sops.KeyGroup{ageMasterKeys}
	keyServices := []keyservice.KeyServiceClient{keyservice.NewLocalClient()}

	tree := sops.Tree{
		Branches: branches,
		Metadata: sops.Metadata{
			KeyGroups:         keyGroups,
			UnencryptedSuffix: "",
			EncryptedSuffix:   "",
			UnencryptedRegex:  "",
			EncryptedRegex:    "",
			Version:           version.Version,
			ShamirThreshold:   0,
		},
		FilePath: "",
	}
	dataKey, errs := tree.GenerateDataKeyWithKeyServices(keyServices)
	require.NoError(t, errors.Join(errs...), "failed to generate data key")

	err = common.EncryptTree(common.EncryptTreeOpts{
		DataKey: dataKey,
		Tree:    &tree,
		Cipher:  aes.NewCipher(),
	})
	require.NoError(t, err, "failed to encrypt")

	outputStore := &yaml.Store{}
	encryptedFile, err := outputStore.EmitEncryptedFile(tree)
	require.NoError(t, err, "failed to emit encrypted yaml file")

	return encryptedFile
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
