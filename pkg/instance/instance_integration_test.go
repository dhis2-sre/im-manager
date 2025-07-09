package instance_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

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
		"whoami-go":  stack.WhoamiGo,
		"dhis2-core": stack.WhoamiGo, // Used to test public instance view - stack.WhoamiGo because it has no dependencies
		"dhis2":      stack.DHIS2,
	}
	stackService := stack.NewService(stacks)
	// classification 'test' does not actually exist, this is used to decrypt the stack parameters
	helmfileService := instance.NewHelmfileService(logger, stackService, "../../stacks", "test")
	instanceService := instance.NewService(logger, instanceRepo, groupService, stackService, helmfileService, nil, "")

	s3Dir := t.TempDir()
	s3Bucket := "database-bucket"
	err = os.Mkdir(s3Dir+"/"+s3Bucket, 0o755)
	require.NoError(t, err, "failed to create S3 output bucket")
	s3 := inttest.SetupS3(t, s3Dir)
	uploader := manager.NewUploader(s3.Client)
	s3Client := storage.NewS3Client(logger, s3.Client, uploader)
	databaseRepository := database.NewRepository(db)
	databaseService := database.NewService(logger, s3Bucket, s3Client, groupService, databaseRepository)

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

	// TODO: Convert below test to use deployments
	/*
		//	var databaseID string
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

				body := client.Post(t, "/databases", &b, inttest.WithHeader("Content-Type", w.FormDataContentType()))

				var actualDB model.Database
				err = json.Unmarshal(body, &actualDB)
				require.NoError(t, err, "POST /databases: failed to unmarshal HTTP response body")
				require.Equal(t, "path/name.extension", actualDB.Name)
				require.Equal(t, "group-name", actualDB.GroupName)

				databaseID = strconv.FormatUint(uint64(actualDB.ID), 10)
			}
			   	t.Run("DeployDHIS2", func(t *testing.T) {
			   		hostname := client.GetHostname(t)
			   		t.Log("hostname:", hostname)
			   		t.Setenv("HOSTNAME", hostname)

			   		var instance model.Instance
			   		body := strings.NewReader(`{
			   			"name": "test-dhis2",
			   			"groupName": "group-name",
			   			"stackName": "dhis2",
			   			"parameters": [
			                   {
			       		        "name": "DATABASE_ID",
			   			        "value": "` + databaseID + `"
			   			    }
			   			]
			   		}`)
			   		client.PostJSON(t, "/instances", body, &instance, inttest.WithAuthToken("sometoken"))

			   		k8sClient.AssertPodIsReady(t, group.Name, instance.Name+"-database", 3*60)
			   		k8sClient.AssertPodIsReady(t, group.Name, instance.Name, 5*60)
			   	})
	*/
	t.Run("DeployDeploymentWithoutInstances", func(t *testing.T) {
		t.Parallel()
		t.Log("Create deployment")
		var deployment model.Deployment
		body := strings.NewReader(`{
			"name": "test-deployment",
			"group": "group-name",
			"description": "some description"
		}`)

		client.PostJSON(t, "/deployments", body, &deployment, inttest.WithAuthToken("sometoken"))

		assert.Equal(t, "test-deployment", deployment.Name)
		assert.Equal(t, "group-name", deployment.GroupName)
		assert.Equal(t, "some description", deployment.Description)

		t.Log("Deploy deployment")
		path := fmt.Sprintf("/deployments/%d/deploy", deployment.ID)
		response := client.Do(t, http.MethodPost, path, nil, http.StatusBadRequest, inttest.WithAuthToken("sometoken"))

		assert.Contains(t, "deployment contains no instances", string(response))
	})

	t.Run("Deployment", func(t *testing.T) {
		t.Parallel()
		t.Log("Create deployment")
		var deployment model.Deployment
		body := strings.NewReader(`{
			"name": "test-deployment-whoami",
			"group": "group-name",
			"description": "some description"
		}`)

		client.PostJSON(t, "/deployments", body, &deployment, inttest.WithAuthToken("sometoken"))

		assert.Equal(t, "test-deployment-whoami", deployment.Name)
		assert.Equal(t, "group-name", deployment.GroupName)
		assert.Equal(t, "some description", deployment.Description)

		t.Log("Create deployment instance")
		var deploymentInstance model.DeploymentInstance
		body = strings.NewReader(`{
			"stackName": "whoami-go"
		}`)

		path := fmt.Sprintf("/deployments/%d/instance", deployment.ID)
		client.PostJSON(t, path, body, &deploymentInstance, inttest.WithAuthToken("sometoken"))
		assert.Equal(t, deployment.ID, deploymentInstance.DeploymentID)
		assert.Equal(t, "group-name", deploymentInstance.GroupName)
		assert.Equal(t, "whoami-go", deploymentInstance.StackName)

		t.Log("Get deployment instance with details")
		path = fmt.Sprintf("/instances/%d/details", deploymentInstance.ID)
		var instance model.DeploymentInstance
		client.GetJSON(t, path, &instance, inttest.WithAuthToken("sometoken"))
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

		t.Log("Deploy deployment")
		path = fmt.Sprintf("/deployments/%d/deploy", deployment.ID)
		client.Do(t, http.MethodPost, path, nil, http.StatusOK, inttest.WithAuthToken("sometoken"))
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 60)

		t.Log("Destroy deployment")
		path = fmt.Sprintf("/deployments/%d", deployment.ID)
		client.Do(t, http.MethodDelete, path, nil, http.StatusAccepted, inttest.WithAuthToken("sometoken"))
		// TODO: Ideally we shouldn't use sleep here but rather watch the pod until it disappears or a timeout is reached
		time.Sleep(3 * time.Second)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, deploymentInstance.Name)
	})

	t.Run("GetPublicDeployments", func(t *testing.T) {
		t.Parallel()
		t.Log("Create deployment")
		var deployment model.Deployment
		body := strings.NewReader(`{
			"name": "private-deployment",
			"group": "group-name",
			"description": "some description"
		}`)

		client.PostJSON(t, "/deployments", body, &deployment, inttest.WithAuthToken("sometoken"))

		assert.Equal(t, "private-deployment", deployment.Name)
		assert.Equal(t, "group-name", deployment.GroupName)
		assert.Equal(t, "some description", deployment.Description)

		t.Log("Create deployment instance")
		var deploymentInstance model.DeploymentInstance
		body = strings.NewReader(`{
			"stackName": "dhis2-core"
		}`)

		path := fmt.Sprintf("/deployments/%d/instance", deployment.ID)
		client.PostJSON(t, path, body, &deploymentInstance, inttest.WithAuthToken("sometoken"))
		assert.Equal(t, deployment.ID, deploymentInstance.DeploymentID)
		assert.Equal(t, "group-name", deploymentInstance.GroupName)
		assert.Equal(t, "dhis2-core", deploymentInstance.StackName)

		t.Log("Create public deployment")
		body = strings.NewReader(`{
			"name": "dev-public-deployment",
			"group": "group-name",
			"description": "some description"
		}`)

		client.PostJSON(t, "/deployments", body, &deployment, inttest.WithAuthToken("sometoken"))

		assert.Equal(t, "dev-public-deployment", deployment.Name)
		assert.Equal(t, "group-name", deployment.GroupName)
		assert.Equal(t, "some description", deployment.Description)

		t.Log("Create public deployment instance")
		var publicDeploymentInstance model.DeploymentInstance
		body = strings.NewReader(`{
			"stackName": "dhis2-core",
			"public": true
		}`)

		path = fmt.Sprintf("/deployments/%d/instance", deployment.ID)
		client.PostJSON(t, path, body, &publicDeploymentInstance, inttest.WithAuthToken("sometoken"))
		assert.Equal(t, deployment.ID, publicDeploymentInstance.DeploymentID)
		assert.Equal(t, "group-name", publicDeploymentInstance.GroupName)
		assert.Equal(t, "dhis2-core", publicDeploymentInstance.StackName)

		t.Log("Get public instances")
		var groupsWithInstances []instance.GroupWithPublicInstances

		client.GetJSON(t, "/instances/public", &groupsWithInstances)

		require.Len(t, groupsWithInstances, 1)
		assert.Equal(t, "group-name", groupsWithInstances[0].Name)
		instances := groupsWithInstances[0].Categories[0].Instances
		assert.Len(t, instances, 1)
		assert.Equal(t, "dev-public-deployment", instances[0].Name)
		assert.Equal(t, "some description", instances[0].Description)
		assert.Equal(t, "https://some/dev-public-deployment", instances[0].Hostname)
	})

	t.Run("UpdateDeployment", func(t *testing.T) {
		t.Parallel()
		t.Log("Create deployment")
		var deployment model.Deployment
		body := strings.NewReader(`{
			"name": "test-deployment-update",
			"group": "group-name",
			"description": "initial description",
			"ttl": 86400
		}`)

		client.PostJSON(t, "/deployments", body, &deployment, inttest.WithAuthToken("sometoken"))

		t.Log("Update deployment")
		body = strings.NewReader(`{
			"description": "updated description",
			"ttl": 172800
		}`)

		path := fmt.Sprintf("/deployments/%d", deployment.ID)
		var updatedDeployment model.Deployment
		client.PutJSON(t, path, body, &updatedDeployment, inttest.WithAuthToken("sometoken"))

		assert.Equal(t, uint(172800), updatedDeployment.TTL)
		assert.Equal(t, "updated description", updatedDeployment.Description)
		assert.Equal(t, "group-name", updatedDeployment.GroupName)
		assert.Equal(t, "test-deployment-update", updatedDeployment.Name)
	})

	t.Run("UpdateDeploymentInstance", func(t *testing.T) {
		t.Parallel()
		t.Log("Create deployment")
		var deployment model.Deployment
		body := strings.NewReader(`{
			"name": "test-deployment-instance-update",
			"group": "group-name",
			"description": "some description"
		}`)

		client.PostJSON(t, "/deployments", body, &deployment, inttest.WithAuthToken("sometoken"))

		t.Log("Create deployment instance")
		var deploymentInstance model.DeploymentInstance
		body = strings.NewReader(`{
			"stackName": "whoami-go",
			"parameters": {
				"IMAGE_TAG": {
					"value": "0.6.0"
				}
			},
			"public": false
		}`)

		path := fmt.Sprintf("/deployments/%d/instance", deployment.ID)
		client.PostJSON(t, path, body, &deploymentInstance, inttest.WithAuthToken("sometoken"))

		t.Log("Update deployment instance")
		body = strings.NewReader(`{
			"stackName": "whoami-go",
			"parameters": {
				"IMAGE_TAG": {
					"value": "0.7.0"
				}
			},
			"public": true
		}`)

		path = fmt.Sprintf("/deployments/%d/instance/%d", deployment.ID, deploymentInstance.ID)
		var updatedInstance model.DeploymentInstance
		client.PutJSON(t, path, body, &updatedInstance, inttest.WithAuthToken("sometoken"))

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
