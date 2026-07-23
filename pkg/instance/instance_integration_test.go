package instance_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/token"
	"github.com/getsops/sops/v3"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/dhis2-sre/im-manager/pkg/cluster"
	"github.com/dhis2-sre/im-manager/pkg/database"
	"github.com/dhis2-sre/im-manager/pkg/storage"

	"filippo.io/age"
	sops_age "github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/keys"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
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
		"minio":      stack.MINIO,
		"whoami-go":  stack.WhoamiGo,
		"dhis2-db":   stack.DHIS2DB,
		"dhis2-core": stack.DHIS2Core,
		"dhis2":      stack.DHIS2,
	}
	stackService := stack.NewService(stacks)
	// classification 'test' does not actually exist, this is used to decrypt the stack parameters
	helmfileService, err := instance.NewHelmfileService(logger, stackService, "../../stacks", "test")
	require.NoError(t, err, "failed to create helmfile service")
	tokenRepository := token.NewRepository(redis)
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate RSA private key")
	tokenService, err := token.NewService(logger, tokenRepository, privateKey, 100, 60, "secret", 100, 100)
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
	databaseService := database.NewService(logger, s3Bucket, s3Client, groupService, databaseRepository, func(c model.Cluster) (database.PodExecutor, error) {
		return instance.NewKubernetesService(c)
	}, noopPublisher{}, noopFilestoreBackuper{})
	instanceService.SetExternalDownloads(databaseService)

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
		instanceHandler := instance.NewHandler(stackService, groupService, instanceService, twoDayTTL)
		instance.Routes(engine, authenticator, instanceHandler)

		databaseHandler := database.NewHandler(logger, databaseService, groupService, instanceService, stackService)
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
		deployment := createDeployment(t, client, "companion-deployment", tokens.AccessToken, WithDescription("some description"))
		deploymentInstance := createDHIS2DBInstance(t, client, deployment.ID, databaseID, tokens.AccessToken)
		deploymentInstance = createMinioInstance(t, client, deployment.ID, tokens.AccessToken)
		deploymentInstance = createDHIS2CoreInstance(t, client, deployment.ID, tokens.AccessToken, WithParameter("ALLOW_SUSPEND", "false"))
		groupedName := fmt.Sprintf("%s-%d", deploymentInstance.Name, deploymentInstance.Group.ID)

		deployDeployment(t, client, deployment.ID, tokens.AccessToken)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, groupedName+"-database", 30)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, groupedName+"-minio", 30)
		k8sClient.AssertPodIsReady(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 90, deploymentInstance.Group.ID)

		destroyDeployment(t, client, deployment.ID, tokens.AccessToken)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, deploymentInstance.Name, 10, deploymentInstance.Group.ID)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, groupedName+"-minio", 30)
		k8sClient.AssertPodIsNotRunning(t, deploymentInstance.Group.Namespace, groupedName+"-database", 10)
	})

	t.Run("FilestoreBackupMinioViaExec", func(t *testing.T) {
		t.Parallel()

		deployment := createDeployment(t, client, "fs-backup-deployment", tokens.AccessToken)
		createDHIS2DBInstance(t, client, deployment.ID, databaseID, tokens.AccessToken)
		createMinioInstance(t, client, deployment.ID, tokens.AccessToken)
		coreInstance := createDHIS2CoreInstance(t, client, deployment.ID, tokens.AccessToken, WithParameter("ALLOW_SUSPEND", "false"))

		deployDeployment(t, client, deployment.ID, tokens.AccessToken)

		groupedName := fmt.Sprintf("%s-%d", coreInstance.Name, coreInstance.Group.ID)
		k8sClient.AssertPodIsReady(t, coreInstance.Group.Namespace, groupedName+"-minio", 120)

		// seed an object into the minio bucket via exec
		ks, err := instance.NewKubernetesService(group.Cluster)
		require.NoError(t, err)
		minioPod := minioPodName(t, k8sClient, coreInstance.Group.Namespace, deployment.ID)
		// create the bucket; the stack creates it asynchronously and we'd otherwise race it
		seedScript := `mc alias set local http://127.0.0.1:9000 dhisdhis dhisdhis >/dev/null 2>&1; mc mb --ignore-existing local/dhis2 >/dev/null 2>&1; printf 'hello-filestore' > /tmp/marker.txt; mc cp --quiet /tmp/marker.txt local/dhis2/seeded/marker.txt`
		var seedOut, seedErr strings.Builder
		require.NoError(t, ks.Exec(context.Background(), coreInstance.Group.Namespace, minioPod, "minio", []string{"sh", "-c", seedScript}, &seedOut, &seedErr), "seed failed: %s", seedErr.String())

		// FilestoreBackup links the filestore to an existing (SaveAs target) database row.
		target := &model.Database{Name: "fs-backup-target.sql.gz", GroupName: "group-name", Type: "database", Slug: "group-name/fs-backup-target", UserID: user.ID}
		require.NoError(t, db.Create(target).Error)

		// The shared instanceService is wired with a nil S3 client; build one with the real client.
		fsService := instance.NewService(logger, instanceRepo, groupService, stackService, helmfileService, s3Client, s3Bucket, tokenService)
		require.NoError(t, fsService.FilestoreBackup(context.Background(), &coreInstance, target.Name, target))

		content := s3.GetObject(t, s3Bucket, "group-name/fs-backup-target-fs.tar.gz")
		require.NotEmpty(t, content)
		entries := extractTarGzEntries(t, content)
		assert.Equal(t, "hello-filestore", string(entries["seeded/marker.txt"]))
		assert.Contains(t, rawTarGzNames(t, content), "./seeded/marker.txt",
			"backup must tar with ./-relative keys so restore reproduces the original object key")

		var saved model.Database
		require.NoError(t, db.First(&saved, target.ID).Error)
		assert.NotZero(t, saved.FilestoreID)

		destroyDeployment(t, client, deployment.ID, tokens.AccessToken)
	})

	t.Run("FilestoreBackupFilesystemViaExec", func(t *testing.T) {
		// not parallel: a full dhis2-core deploy is heavy; running it outside the parallel batch avoids starving the runner

		// STORAGE_TYPE=filesystem: no minio stack; the filestore lives on a PVC in the core pod
		deployment := createDeployment(t, client, "fsstore-backup-deployment", tokens.AccessToken)
		createDHIS2DBInstance(t, client, deployment.ID, databaseID, tokens.AccessToken)
		coreInstance := createDHIS2CoreInstance(t, client, deployment.ID, tokens.AccessToken,
			WithParameter("STORAGE_TYPE", "filesystem"),
			WithParameter("ALLOW_SUSPEND", "false"))

		deployDeployment(t, client, deployment.ID, tokens.AccessToken)

		// the backup execs into the core pod, so wait for Running, not Ready
		ks, err := instance.NewKubernetesService(group.Cluster)
		require.NoError(t, err)
		corePod, coreContainer := waitForCorePodRunning(t, k8sClient, coreInstance.Group.Namespace, coreInstance.ID, 120*time.Second)
		seedScript := `mkdir -p /opt/dhis2/files/seeded && printf 'hello-filestore' > /opt/dhis2/files/seeded/marker.txt`
		var seedOut, seedErr strings.Builder
		require.NoError(t, ks.Exec(context.Background(), coreInstance.Group.Namespace, corePod, coreContainer, []string{"sh", "-c", seedScript}, &seedOut, &seedErr), "seed failed: %s", seedErr.String())

		// FilestoreBackup links the filestore to an existing (SaveAs target) database row.
		target := &model.Database{Name: "fsstore-backup-target.sql.gz", GroupName: "group-name", Type: "database", Slug: "group-name/fsstore-backup-target", UserID: user.ID}
		require.NoError(t, db.Create(target).Error)

		// The shared instanceService is wired with a nil S3 client; build one with the real client.
		fsService := instance.NewService(logger, instanceRepo, groupService, stackService, helmfileService, s3Client, s3Bucket, tokenService)
		require.NoError(t, fsService.FilestoreBackup(context.Background(), &coreInstance, target.Name, target))

		content := s3.GetObject(t, s3Bucket, "group-name/fsstore-backup-target-fs.tar.gz")
		require.NotEmpty(t, content)
		entries := extractTarGzEntries(t, content)
		assert.Equal(t, "hello-filestore", string(entries["seeded/marker.txt"]))
		assert.Contains(t, rawTarGzNames(t, content), "./seeded/marker.txt",
			"backup must tar with ./-relative keys so restore reproduces the original object key")

		var saved model.Database
		require.NoError(t, db.First(&saved, target.ID).Error)
		assert.NotZero(t, saved.FilestoreID)

		destroyDeployment(t, client, deployment.ID, tokens.AccessToken)
	})

	t.Run("SaveAsDatabase", func(t *testing.T) {
		t.Parallel()
		deployment := createDeployment(t, client, "save-as-deployment", tokens.AccessToken)
		dbInstance := createDHIS2DBInstance(t, client, deployment.ID, databaseID, tokens.AccessToken)

		deployDeployment(t, client, deployment.ID, tokens.AccessToken)
		groupedName := fmt.Sprintf("%s-%d", dbInstance.Name, dbInstance.Group.ID)
		k8sClient.AssertPodIsReady(t, dbInstance.Group.Namespace, groupedName+"-database", 60)

		body := strings.NewReader(`{"name": "saved-copy.sql.gz", "format": "plain"}`)
		var savedDB model.Database
		instanceIDStr := strconv.FormatUint(uint64(dbInstance.ID), 10)
		client.PostJSON(t, "/databases/save-as/"+instanceIDStr, body, &savedDB, inttest.WithAuthToken(tokens.AccessToken))

		assert.Equal(t, "saved-copy.sql.gz", savedDB.Name)
		assert.Equal(t, "group-name", savedDB.GroupName)

		require.Eventually(t, func() bool {
			var d model.Database
			if err := db.First(&d, savedDB.ID).Error; err != nil {
				return false
			}
			return d.Url != ""
		}, 60*time.Second, 500*time.Millisecond, "database URL should be set by async goroutine")

		var finalDB model.Database
		err := db.First(&finalDB, savedDB.ID).Error
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("s3://%s/group-name/saved-copy.sql.gz", s3Bucket), finalDB.Url)
		assert.Greater(t, finalDB.Size, int64(0))

		s3Content := s3.GetObject(t, s3Bucket, "group-name/saved-copy.sql.gz")
		assert.Greater(t, len(s3Content), 0, "S3 object should have content")

		destroyDeployment(t, client, deployment.ID, tokens.AccessToken)
	})

	t.Run("SaveDatabase", func(t *testing.T) {
		t.Parallel()

		dbID := database.UploadTestDatabase(t, client, "save-test.sql.gz", "select now();", "group-name", inttest.WithAuthToken(tokens.AccessToken))

		deployment := createDeployment(t, client, "save-deployment", tokens.AccessToken)
		dbInstance := createDHIS2DBInstance(t, client, deployment.ID, dbID, tokens.AccessToken)

		deployDeployment(t, client, deployment.ID, tokens.AccessToken)
		groupedName := fmt.Sprintf("%s-%d", dbInstance.Name, dbInstance.Group.ID)
		k8sClient.AssertPodIsReady(t, dbInstance.Group.Namespace, groupedName+"-database", 60)

		originalSize := len(s3.GetObject(t, s3Bucket, "group-name/save-test.sql.gz"))

		instanceIDStr := strconv.FormatUint(uint64(dbInstance.ID), 10)
		client.Do(t, http.MethodPost, "/databases/save/"+instanceIDStr, nil, http.StatusAccepted, inttest.WithAuthToken(tokens.AccessToken))

		require.Eventually(t, func() bool {
			content, err := s3.TryGetObject(s3Bucket, "group-name/save-test.sql.gz")
			return err == nil && len(content) > originalSize
		}, 60*time.Second, 500*time.Millisecond, "saved database in S3 should grow beyond the uploaded placeholder")

		destroyDeployment(t, client, deployment.ID, tokens.AccessToken)
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

		createDHIS2DBInstance(t, client, deployment.ID, databaseID, tokens.AccessToken)
		createMinioInstance(t, client, deployment.ID, tokens.AccessToken)
		deploymentInstance := createDHIS2CoreInstance(t, client, deployment.ID, tokens.AccessToken,
			WithParameter("IMAGE_TAG", "2.42.0"),
			WithParameter("ALLOW_SUSPEND", "false"),
			WithPublic(false))

		updatedInstance := updateInstance(t, client, deploymentInstance, tokens.AccessToken,
			WithParameter("IMAGE_TAG", "2.43.0"),
			WithPublic(true))

		assert.Equal(t, deploymentInstance.ID, updatedInstance.ID)
		assert.Equal(t, "2.43.0", updatedInstance.Parameters["IMAGE_TAG"].Value)
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

type noopFilestoreBackuper struct{}

func (noopFilestoreBackuper) FilestoreBackup(context.Context, *model.DeploymentInstance, string, *model.Database) error {
	return nil
}
