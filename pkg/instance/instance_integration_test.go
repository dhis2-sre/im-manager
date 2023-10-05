package instance_test

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/dhis2-sre/im-manager/pkg/database"
	"github.com/dhis2-sre/im-manager/pkg/storage"

	"filippo.io/age"
	"go.mozilla.org/sops/v3"
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

	t.Run("DeployWhoAmI", func(t *testing.T) {
		var instance model.Instance
		body := strings.NewReader(`{
			"name": "test-whoami",
			"groupName": "group-name",
			"stackName": "whoami-go"
		}`)
		client.PostJSON(t, "/instances", body, &instance, inttest.WithAuthToken("sometoken"))

		k8sClient.AssertPodIsReady(t, group.Name, instance.Name, 60)
	})

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

		// TODO: Delete instances
		// TODO: Delete deployment
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

func (gs groupService) Find(name string) (*model.Group, error) {
	return gs.group, nil
}
