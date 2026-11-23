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
	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyRefusesTakenNames(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	s3 := inttest.SetupS3(t)
	repository := NewRepository(db)
	service := NewService(logger, s3.Bucket, storage.NewS3Client(logger, s3.Client, nil), nil, repository, nil, nil)
	ctx := context.Background()
	group := &model.Group{Name: "group-name"}

	user := model.User{Email: "user@dhis2.org"}
	require.NoError(t, db.Create(&user).Error)

	create := func(name, kind string, filestoreID uint) *model.Database {
		key := "group-name/" + name
		_, err := s3.Client.PutObject(ctx, &awss3.PutObjectInput{Bucket: aws.String(s3.Bucket), Key: aws.String(key), Body: strings.NewReader("content of " + name)})
		require.NoError(t, err)
		database := &model.Database{Name: name, GroupName: "group-name", Type: kind, Url: fmt.Sprintf("s3://%s/%s", s3.Bucket, key), UserID: user.ID, FilestoreID: filestoreID}
		require.NoError(t, repository.Create(ctx, database))
		return database
	}
	copyAs := func(source *model.Database, name string) error {
		return service.Copy(ctx, source.ID, &model.Database{Name: name, GroupName: "group-name", Type: "database", UserID: user.ID}, group)
	}

	sourceFilestore := create("source.sql.gz-fs.tar.gz", "fs", 0)
	source := create("source.sql.gz", "database", sourceFilestore.ID)

	t.Run("OfAnotherDatabaseWithoutOverwritingItsFile", func(t *testing.T) {
		create("taken.sql.gz", "database", 0)

		err := copyAs(source, "taken.sql.gz")

		require.Error(t, err)
		assert.True(t, errdef.IsDuplicated(err), "got: %v", err)
		assert.Equal(t, "content of taken.sql.gz", string(s3.GetObject(t, "group-name/taken.sql.gz")))
	})

	t.Run("OfAnotherFilestoreWithoutCopyingAnything", func(t *testing.T) {
		theirFilestore := create("copy.sql.gz-fs.tar.gz", "fs", 0)
		create("linked-by-hand.sql.gz", "database", theirFilestore.ID)

		err := copyAs(source, "copy.sql.gz")

		require.Error(t, err)
		assert.True(t, errdef.IsDuplicated(err), "got: %v", err)
		assert.Equal(t, "content of copy.sql.gz-fs.tar.gz", string(s3.GetObject(t, "group-name/copy.sql.gz-fs.tar.gz")))
		_, err = s3.TryGetObject("group-name/copy.sql.gz")
		require.Error(t, err, "the database must not be copied when its file store cannot follow")
		_, err = repository.FindBySlug(ctx, "group-name-copy-sql-gz")
		assert.True(t, errdef.IsNotFound(err), "got: %v", err)
	})
}
