package database_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go/aws"
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
		require.Equal(t, "packages", actualDB.GroupName)

		actualContent := s3.GetObject(t, s3Bucket, "packages/mydb")
		require.Equalf(t, "file contents", string(actualContent), "DB in S3 should have expected content")

		databaseID = strconv.FormatUint(uint64(actualDB.ID), 10)
	}

	t.Run("Get", func(t *testing.T) {
		var actualDB model.Database
		client.GetJSON(t, "/databases/"+databaseID, &actualDB)

		assert.Equal(t, "mydb", actualDB.Name)
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
				"name":  "mydbcopy",
				"group": "packages"
			}`), &actualDB)

			require.Equal(t, "mydbcopy", actualDB.Name)
			require.Equal(t, "packages", actualDB.GroupName)

			actualContent := s3.GetObject(t, s3Bucket, "packages/mydbcopy")
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
			assert.ElementsMatchf(t, []string{"mydb", "mydbcopy"}, names, "GET /databases failed: should return original and copied DB")
		}
	})

	{
		t.Log("Delete")

		client.Delete(t, "/databases/"+databaseID)

		_, err = s3.Client.GetObject(context.TODO(), &awss3.GetObjectInput{
			Bucket: aws.String(s3Bucket),
			Key:    aws.String("packages/mydb"),
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

func (is instanceService) FindById(id uint) (*model.Instance, error) {
	return nil, nil
}

func (is instanceService) FindByIdDecrypted(id uint) (*model.Instance, error) {
	return nil, nil
}

type stackService struct{}

func (ss stackService) Find(name string) (*model.Stack, error) {
	return nil, nil
}
