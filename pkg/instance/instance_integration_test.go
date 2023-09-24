package instance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/dhis2-sre/im-manager/pkg/database"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"io"
	"mime/multipart"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"go.mozilla.org/sops/v3"
	"go.mozilla.org/sops/v3/aes"
	"go.mozilla.org/sops/v3/cmd/sops/common"
	"go.mozilla.org/sops/v3/keys"
	"go.mozilla.org/sops/v3/keyservice"
	"go.mozilla.org/sops/v3/stores/yaml"
	"go.mozilla.org/sops/v3/version"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sops_age "go.mozilla.org/sops/v3/age"

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
		ctx.Set("user", &model.User{
			ID:    user.ID, // TODO: Why is this necessary all of a sudden?
			Email: "user1@dhis2.org",
			Groups: []model.Group{
				{
					Name: "group-name",
				},
			},
		})
	}
	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		var twoDayTTL uint = 172800
		instanceHandler := instance.NewHandler(groupService, instanceService, twoDayTTL)
		instance.Routes(engine, authenticator, instanceHandler)

		databaseHandler := database.NewHandler(databaseService, groupService, instanceService, stackService)
		database.Routes(engine, authenticator, databaseHandler)
	})

	t.Run("Deploy", func(t *testing.T) {
		var instance model.Instance
		client.PostJSON(t, "/instances", strings.NewReader(`{
			"name": "test-whoami",
			"groupName": "group-name",
			"stackName": "whoami-go"
		}`), &instance, inttest.WithAuthToken("sometoken"))

		ctx, cancel := context.WithCancel(context.Background())
		watch, err := k8sClient.Client.CoreV1().Pods(group.Name).Watch(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/instance=" + instance.Name,
		})
		require.NoErrorf(t, err, "failed to find pod for instance %q", instance.Name)

		timeout := 20 * time.Second
		tm := time.NewTimer(timeout)
		defer tm.Stop()
		for {
			select {
			case <-tm.C:
				assert.Fail(t, "timed out waiting on pod")
				cancel()
				return
			case event := <-watch.ResultChan():
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					assert.Failf(t, "failed to get pod event", "want pod event instead got %T", event.Object)
					if !tm.Stop() {
						<-tm.C
					}
					cancel()
					return
				}

				t.Logf("watching pod conditions: %#v\n", pod.Status.Conditions)
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady {
						t.Logf("pod for instance %q is ready", instance.Name)
						if !tm.Stop() {
							<-tm.C
						}
						cancel()
						return
					}
				}
			}
		}
	})

	var databaseID string
	{
		t.Log("Upload")
		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		err := w.WriteField("group", "group-name")
		require.NoError(t, err, "failed to write form field")
		f, err := w.CreateFormFile("database", "mydb")
		require.NoError(t, err, "failed to create form file")
		_, err = io.WriteString(f, "file contents")
		require.NoError(t, err, "failed to write file")
		_ = w.Close()

		body := client.Post(t, "/databases", &b, inttest.WithHeader("Content-Type", w.FormDataContentType()))

		var actualDB model.Database
		err = json.Unmarshal(body, &actualDB)
		require.NoError(t, err, "POST /databases: failed to unmarshal HTTP response body")
		require.Equal(t, "mydb", actualDB.Name)
		require.Equal(t, "group-name", actualDB.GroupName)

		actualContent := s3.GetObject(t, s3Bucket, "group-name/mydb")
		require.Equalf(t, "file contents", string(actualContent), "DB in S3 should have expected content")

		databaseID = strconv.FormatUint(uint64(actualDB.ID), 10)
	}

	t.Run("DeployDHIS2", func(t *testing.T) {
		t.Setenv("HOSTNAME", "test-dhis2.group-name.svc")

		var instance model.Instance
		client.PostJSON(t, "/instances", strings.NewReader(`{
			"name": "test-dhis2",
			"groupName": "group-name",
			"stackName": "dhis2",
			"parameters": [
                {
    		        "name": "DATABASE_ID",
			        "value": "`+databaseID+`"
			    }
			]
		}`), &instance, inttest.WithAuthToken("sometoken"))

		ctx, cancel := context.WithCancel(context.Background())
		watch, err := k8sClient.Client.CoreV1().Pods(group.Name).Watch(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/instance=" + instance.Name,
		})
		require.NoErrorf(t, err, "failed to find pod for instance %q", instance.Name)

		timeout := 20 * time.Second
		tm := time.NewTimer(timeout)
		defer tm.Stop()
		for {
			select {
			case <-tm.C:
				assert.Fail(t, "timed out waiting on pod")
				cancel()
				return
			case event := <-watch.ResultChan():
				pod, ok := event.Object.(*v1.Pod)
				if !ok {
					assert.Failf(t, "failed to get pod event", "want pod event instead got %T", event.Object)
					if !tm.Stop() {
						<-tm.C
					}
					cancel()
					return
				}

				t.Logf("watching pod conditions: %#v\n", pod.Status.Conditions)
				for _, condition := range pod.Status.Conditions {
					if condition.Type == v1.PodReady {
						t.Logf("pod for instance %q is ready", instance.Name)
						if !tm.Stop() {
							<-tm.C
						}
						cancel()
						return
					}
				}
			}
		}
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
