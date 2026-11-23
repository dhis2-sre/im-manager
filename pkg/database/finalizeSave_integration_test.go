package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinalizeSave(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	s3 := inttest.SetupS3(t)
	s3Client := storage.NewS3Client(logger, s3.Client, nil)
	repository := NewRepository(db)
	service := NewService(logger, s3.Bucket, s3Client, nil, repository, nil, nil)
	ctx := context.Background()

	user := model.User{Email: "user@dhis2.org", Groups: []model.Group{{Name: "group-name", Hostname: "some"}}}
	require.NoError(t, db.Create(&user).Error)
	deployment := model.Deployment{UserID: user.ID, Name: "name", GroupName: "group-name"}
	require.NoError(t, db.Create(&deployment).Error)
	instance := model.DeploymentInstance{Name: "name", GroupName: "group-name", StackName: "dhis2-v2", DeploymentID: deployment.ID}
	require.NoError(t, db.Create(&instance).Error)

	putObject := func(t *testing.T, key, content string) {
		t.Helper()
		_, err := s3.Client.PutObject(ctx, &awss3.PutObjectInput{Bucket: aws.String(s3.Bucket), Key: aws.String(key), Body: strings.NewReader(content)})
		require.NoError(t, err)
	}

	// The dump replaces the file and nothing else: the record keeps its identity and everything
	// attached to it, since instances, seeds in flight and the file store all refer to it.
	t.Run("ReplacesTheFileAndKeepsTheRecord", func(t *testing.T) {
		filestore := &model.Database{Name: "original-fs.tar.gz", GroupName: "group-name", Type: "fs", Slug: "group-name-original-fs-tar-gz-fs", UserID: user.ID}
		require.NoError(t, db.Create(filestore).Error)

		original, err := service.CreateDatabase(ctx, user.ID, "group-name", "original.sql.gz")
		require.NoError(t, err)
		original.Url = fmt.Sprintf("s3://%s/group-name/original.sql.gz", s3.Bucket)
		original.Description = "kept up to date by the nightly save"
		original.FilestoreID = filestore.ID
		require.NoError(t, repository.Save(ctx, original))
		putObject(t, "group-name/original.sql.gz", "stale dump")
		_, err = repository.Lock(ctx, original.ID, instance.ID, user.ID)
		require.NoError(t, err)
		download, err := repository.CreateExternalDownload(ctx, original.ID, 1800)
		require.NoError(t, err)
		original, err = repository.FindById(ctx, original.ID)
		require.NoError(t, err)

		putObject(t, "group-name/temporary-key", "fresh dump")

		saved, err := service.finalizeSave(ctx, original, "group-name/temporary-key", 10)
		require.NoError(t, err)

		assert.Equal(t, original.ID, saved.ID)
		assert.Equal(t, int64(10), saved.Size)
		assert.Equal(t, filestore.ID, saved.FilestoreID, "the file store backup that follows overwrites the file store the database has")
		assert.Equal(t, "kept up to date by the nightly save", saved.Description)
		assert.Equal(t, "original.sql.gz", saved.Name)
		assert.Equal(t, original.CreatedAt, saved.CreatedAt)
		require.NotNil(t, saved.Lock, "a lock held before the save is still held after it")
		assert.Equal(t, instance.ID, saved.Lock.InstanceID)

		_, err = repository.FindExternalDownload(ctx, download.UUID)
		require.NoError(t, err, "an instance being seeded from the database keeps its download link")

		assert.Equal(t, "fresh dump", string(s3.GetObject(t, "group-name/original.sql.gz")))
		_, err = s3.TryGetObject("group-name/temporary-key")
		require.Error(t, err, "the temporary dump is gone once it is moved")

		var databases int64
		require.NoError(t, db.Model(&model.Database{}).Where("type = ?", "database").Count(&databases).Error)
		assert.Equal(t, int64(1), databases, "a save leaves no second record behind")
	})

	t.Run("WritesWhereTheUrlPoints", func(t *testing.T) {
		elsewhere := &model.Database{Name: "renamed.sql.gz", GroupName: "group-name", Type: "database", Slug: "group-name-renamed-sql-gz", UserID: user.ID, Url: fmt.Sprintf("s3://%s/other-prefix/first-name.sql.gz", s3.Bucket)}
		require.NoError(t, db.Create(elsewhere).Error)
		putObject(t, "other-prefix/first-name.sql.gz", "stale dump")
		putObject(t, "group-name/another-temporary-key", "fresh dump")

		saved, err := service.finalizeSave(ctx, elsewhere, "group-name/another-temporary-key", 10)
		require.NoError(t, err)

		assert.Equal(t, elsewhere.Url, saved.Url)
		assert.Equal(t, "fresh dump", string(s3.GetObject(t, "other-prefix/first-name.sql.gz")))
		_, err = s3.TryGetObject("group-name/renamed.sql.gz")
		require.Error(t, err, "the dump must not land under a key the record does not point at")
	})
}
