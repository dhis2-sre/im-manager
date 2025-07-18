package database_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/dhis2-sre/im-manager/pkg/database"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseHandler(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)

	s3Dir := t.TempDir()
	s3Bucket := "database-bucket"
	err := os.Mkdir(s3Dir+"/"+s3Bucket, 0o755)
	require.NoError(t, err, "failed to create S3 output bucket")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	s3 := inttest.SetupS3(t, s3Dir)
	uploader := manager.NewUploader(s3.Client)
	s3Client := storage.NewS3Client(logger, s3.Client, uploader)

	databaseRepository := database.NewRepository(db)
	databaseService := database.NewService(logger, s3Bucket, s3Client, groupService{}, databaseRepository)

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		databaseHandler := database.NewHandler(logger, databaseService, groupService{groupName: "packages"}, instanceService{}, stackService{})
		authenticator := func(c *gin.Context) {
			ctx := model.NewContextWithUser(c.Request.Context(), &model.User{
				ID:    1,
				Email: "user1@dhis2.org",
				Groups: []model.Group{
					{
						Name: "packages",
					},
				},
			})
			c.Request = c.Request.WithContext(ctx)
		}
		database.Routes(engine, authenticator, databaseHandler)
	})

	var databaseID string
	{
		t.Log("Upload")

		requestBody := strings.NewReader("file contents")
		nameHeader := inttest.WithHeader("X-Upload-Name", "path/name.extension")
		groupHeader := inttest.WithHeader("X-Upload-Group", "packages")
		body := client.Put(t, "/databases", requestBody, http.StatusCreated, nameHeader, groupHeader)

		var database model.Database
		err = json.Unmarshal(body, &database)
		require.NoError(t, err, "POST /databases: failed to unmarshal HTTP response body")
		require.Equal(t, "path/name.extension", database.Name)
		require.Equal(t, "packages", database.GroupName)
		require.Equal(t, "s3://database-bucket/packages/path/name.extension", database.Url)

		actualContent := s3.GetObject(t, s3Bucket, "packages/path/name.extension")
		require.Equalf(t, "file contents", string(actualContent), "DB in S3 should have expected content")

		databaseID = strconv.FormatUint(uint64(database.ID), 10)
	}

	var instanceID string
	{
		group := &model.Group{
			Name:     "group-name",
			Hostname: "some",
		}
		user := &model.User{
			Email: "user1@dhis2.org",
			Groups: []model.Group{
				*group,
			},
		}
		db.Create(user)

		deployment := &model.Deployment{
			UserID:    user.ID,
			Name:      "name",
			GroupName: group.Name,
		}
		db.Create(deployment)

		instance := &model.DeploymentInstance{
			ID:           0,
			Name:         "name",
			GroupName:    group.Name,
			StackName:    "dhis2",
			DeploymentID: deployment.ID,
		}
		db.Create(instance)

		instanceID = strconv.FormatUint(uint64(instance.ID), 10)
	}

	t.Run("Get", func(t *testing.T) {
		var actualDB model.Database
		client.GetJSON(t, "/databases/"+databaseID, &actualDB)

		assert.Equal(t, "path/name.extension", actualDB.Name)
		assert.Equal(t, "packages", actualDB.GroupName)
	})

	t.Run("Download", func(t *testing.T) {
		actualContent := client.Get(t, "/databases/"+databaseID+"/download")

		assert.Equal(t, "file contents", string(actualContent))
	})

	t.Run("Copy", func(t *testing.T) {
		{
			t.Log("Copy")

			var actualDB model.Database
			client.PostJSON(t, "/databases/"+databaseID+"/copy", strings.NewReader(`{
				"name":  "path/copy.extension",
				"group": "packages"
			}`), &actualDB)

			require.Equal(t, "path/copy.extension", actualDB.Name)
			require.Equal(t, "packages", actualDB.GroupName)

			actualContent := s3.GetObject(t, s3Bucket, "packages/path/copy.extension")
			require.Equalf(t, "file contents", string(actualContent), "DB in S3 should have expected content")
		}

		{
			t.Log("GetAll")

			var groupDBs []database.GroupsWithDatabases
			client.GetJSON(t, "/databases", &groupDBs)

			require.Len(t, groupDBs, 1)
			groupDB := groupDBs[0]
			assert.Equal(t, "packages", groupDB.Name, "GET /databases failed")
			var names []string
			for _, d := range groupDB.Databases {
				names = append(names, d.Name)
			}
			assert.ElementsMatchf(t, []string{"path/name.extension", "path/copy.extension"}, names, "GET /databases failed: should return original and copied DB")
		}
	})

	t.Run("Update", func(t *testing.T) {
		{
			t.Log("Update")

			requestBody := strings.NewReader(`{"name": "path/rename.extension"}`)
			response := client.Do(t, http.MethodPut, "/databases/"+databaseID, requestBody, http.StatusOK, inttest.WithHeader("Content-Type", "application/json"))
			var actualDB model.Database
			err := json.Unmarshal(response, &actualDB)
			assert.NoError(t, err)

			require.Equal(t, "path/rename.extension", actualDB.Name)
			require.Equal(t, "packages", actualDB.GroupName)

			actualContent := s3.GetObject(t, s3Bucket, "packages/path/rename.extension")
			require.Equalf(t, "file contents", string(actualContent), "DB in S3 should have expected content")
		}

		{
			t.Log("GetAll")

			var groupDBs []database.GroupsWithDatabases
			client.GetJSON(t, "/databases", &groupDBs)

			require.Len(t, groupDBs, 1)
			groupDB := groupDBs[0]
			assert.Equal(t, "packages", groupDB.Name, "GET /databases failed")
			var names []string
			for _, d := range groupDB.Databases {
				names = append(names, d.Name)
			}
			assert.ElementsMatchf(t, []string{"path/rename.extension", "path/copy.extension"}, names, "GET /databases failed: should return original and copied DB")
		}
	})

	t.Run("Lock/Unlock", func(t *testing.T) {
		{
			t.Log("Lock")

			body := strings.NewReader(`{
				"instanceId":  ` + instanceID + `
			}`)
			var lock model.Lock
			client.PostJSON(t, "/databases/"+databaseID+"/lock", body, &lock)

			require.Equal(t, uint(1), lock.DatabaseID)
			require.Equal(t, uint(1), lock.InstanceID)
			require.Equal(t, uint(1), lock.UserID)
		}

		{
			t.Log("Unlock")

			client.Delete(t, "/databases/"+databaseID+"/lock")

			var database model.Database
			client.GetJSON(t, "/databases/"+databaseID, &database)
			require.Nil(t, database.Lock)
		}
	})

	t.Run("ExternalDownload", func(t *testing.T) {
		{
			t.Log("ExternalDownload")

			body := strings.NewReader(`{
				"expiration": 60
			}`)
			var externalDownload model.ExternalDownload
			client.PostJSON(t, "/databases/"+databaseID+"/external", body, &externalDownload)

			require.Equal(t, uint(1), externalDownload.DatabaseID)
			diff := int64(externalDownload.Expiration) - time.Now().Unix() - 60
			require.Zero(t, diff)
		}
	})

	{
		t.Log("Delete")

		// Lock database
		body := strings.NewReader(`{"instanceId":  ` + instanceID + `}`)
		client.PostJSON(t, "/databases/"+databaseID+"/lock", body, &model.Lock{})
		// Attempt delete but expect a bad request response
		client.Do(t, http.MethodDelete, "/databases/"+databaseID, nil, http.StatusBadRequest, inttest.WithAuthToken("sometoken"))
		// Unlock database
		client.Delete(t, "/databases/"+databaseID+"/lock")

		// Create external download to ensure the database can still be deleted
		client.PostJSON(t, "/databases/"+databaseID+"/external", strings.NewReader(`{
				"expiration": 60
			}`), &model.ExternalDownload{})

		client.Delete(t, "/databases/"+databaseID)

		_, err = s3.Client.GetObject(context.TODO(), &awss3.GetObjectInput{
			Bucket: aws.String(s3Bucket),
			Key:    aws.String("packages/path/name.extension"),
		})
		var e *types.NoSuchKey
		require.ErrorAsf(t, err, &e, "DELETE \"/databases/%s\" failed: DB should be deleted from S3", databaseID)
	}
}

type groupService struct {
	groupName string
}

func (gs groupService) Find(ctx context.Context, name string) (*model.Group, error) {
	return &model.Group{
		Name: gs.groupName,
	}, nil
}

type instanceService struct{}

func (is instanceService) FindDeploymentById(ctx context.Context, id uint) (*model.Deployment, error) {
	panic("implement me")
}

func (is instanceService) FilestoreBackup(ctx context.Context, instance *model.DeploymentInstance, name string, database *model.Database) error {
	panic("implement me")
}

func (is instanceService) FindDecryptedDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error) {
	panic("implement me")
}

type stackService struct{}

func (ss stackService) Find(name string) (*model.Stack, error) {
	return nil, nil
}
