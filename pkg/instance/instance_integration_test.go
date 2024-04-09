package instance_test

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"go.mozilla.org/sops/v3"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/dhis2-sre/im-manager/pkg/database"
	"github.com/dhis2-sre/im-manager/pkg/storage"

	"filippo.io/age"
	"go.mozilla.org/sops/v3/aes"
	sops_age "go.mozilla.org/sops/v3/age"
	"go.mozilla.org/sops/v3/cmd/sops/common"
	"go.mozilla.org/sops/v3/keys"
	"go.mozilla.org/sops/v3/keyservice"
	"go.mozilla.org/sops/v3/stores/yaml"
	"go.mozilla.org/sops/v3/version"

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
		Hostname:   "some",
		Deployable: true,
		ClusterConfiguration: &model.ClusterConfiguration{
			GroupName:               "group-name",
			KubernetesConfiguration: k8sConfig,
		},
	}
	user := &model.User{
		Email: "user1@dhis2.org",
		Groups: []model.Group{
			*group,
		},
	}
	db.Create(user)

	encryptionKey := strings.Repeat("a", 32)
	instanceRepo := instance.NewRepository(db, encryptionKey)
	groupService := groupService{group: group}
	stacks := stack.Stacks{
		"whoami-go": stack.WhoamiGo,
		"dhis2":     stack.DHIS2,
	}
	stackService := stack.NewService(stacks)
	// classification 'test' does not actually exist, this is used to decrypt the stack parameters
	helmfileService := instance.NewHelmfileService("../../stacks", stackService, "test")
	instanceService := instance.NewService(instanceRepo, groupService, stackService, helmfileService)

	s3Dir := t.TempDir()
	s3Bucket := "database-bucket"
	err = os.Mkdir(s3Dir+"/"+s3Bucket, 0o755)
	require.NoError(t, err, "failed to create S3 output bucket")
	s3 := inttest.SetupS3(t, s3Dir)
	uploader := manager.NewUploader(s3.Client)
	s3Client := storage.NewS3Client(s3.Client, uploader)
	databaseRepository := database.NewRepository(db)
	databaseService := database.NewService(s3Bucket, s3Client, groupService, databaseRepository)

	authenticator := func(ctx *gin.Context) {
		ctx.Set("user", user)
	}
	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		var twoDayTTL uint = 172800
		instanceHandler := instance.NewHandler(groupService, instanceService, twoDayTTL)
		instance.Routes(engine, authenticator, instanceHandler)

		databaseHandler := database.NewHandler(databaseService, groupService, instanceService, stackService)
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

		t.Log("Deploy deployment")
		path = fmt.Sprintf("/deployments/%d/deploy", deployment.ID)
		client.Do(t, http.MethodPost, path, nil, http.StatusOK, inttest.WithAuthToken("sometoken"))
		k8sClient.AssertPodIsReady(t, deploymentInstance.GroupName, deploymentInstance.Name, 60)

		t.Log("Destroy deployment")
		path = fmt.Sprintf("/deployments/%d", deployment.ID)
		client.Do(t, http.MethodDelete, path, nil, http.StatusAccepted, inttest.WithAuthToken("sometoken"))
		// TODO: Ideally we shouldn't use sleep here but rather watch the pod until it disappears or a timeout is reached
		time.Sleep(3 * time.Second)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.GroupName, deploymentInstance.Name)
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

func (gs groupService) FindByGroupNames(groupNames []string) ([]model.Group, error) {
	panic("implement me")
}

func (gs groupService) Find(name string) (*model.Group, error) {
	return gs.group, nil
}
