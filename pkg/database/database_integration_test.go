package database_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
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
	s3 := inttest.SetupS3(t, s3Dir)
	uploader := manager.NewUploader(s3.Client)
	s3Client := storage.NewS3Client(s3.Client, uploader)

	databaseRepository := database.NewRepository(db)
	databaseService := database.NewService(s3Bucket, s3Client, groupService{}, databaseRepository)

	client := inttest.SetupHTTPServer(t, func(engine *gin.Engine) {
		databaseHandler := database.NewHandler(databaseService, groupService{groupName: "packages"}, instanceService{}, stackService{})
		authenticator := func(ctx *gin.Context) {
			ctx.Set("user", &model.User{
				ID:    1,
				Email: "user1@dhis2.org",
				Groups: []model.Group{
					{
						Name: "packages",
					},
				},
			})
		}
		database.Routes(engine, authenticator, databaseHandler)
	})

	var databaseID string
	{
		t.Log("Upload")

		var b bytes.Buffer
		w := multipart.NewWriter(&b)
		err := w.WriteField("group", "packages")
		require.NoError(t, err, "failed to write form field")
		err = w.WriteField("name", "path/name.extension")
		require.NoError(t, err, "failed to write form field")
		f, err := w.CreateFormFile("database", "mydb")
		require.NoError(t, err, "failed to create form file")
		_, err = io.WriteString(f, "file contents")
		require.NoError(t, err, "failed to write file")
		_ = w.Close()

		body := client.Post(t, "/databases", &b, inttest.WithHeader("Content-Type", w.FormDataContentType()))

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

func (gs groupService) Find(name string) (*model.Group, error) {
	return &model.Group{
		Name: gs.groupName,
	}, nil
}

type instanceService struct{}

func (is instanceService) FindDecryptedDeploymentInstanceById(id uint) (*model.DeploymentInstance, error) {
	//TODO implement me
	panic("implement me")
}

type stackService struct{}

func (ss stackService) Find(name string) (*model.Stack, error) {
	return nil, nil
}
