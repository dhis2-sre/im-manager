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

func TestUpdate(t *testing.T) {
	t.Parallel()

	type fixture struct {
		service    *Service
		repository *repository
		s3         *inttest.S3Client
		create     func(name, kind string, filestoreID uint) *model.Database
	}

	setup := func(t *testing.T) fixture {
		t.Helper()

		db := inttest.SetupDB(t)
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		s3 := inttest.SetupS3(t)
		repository := NewRepository(db)
		service := NewService(logger, s3.Bucket, storage.NewS3Client(logger, s3.Client, nil), nil, repository, nil, nil)
		ctx := context.Background()

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

		return fixture{service: service, repository: repository, s3: s3, create: create}
	}

	t.Run("RenamesTheDatabaseAndItsFilestore", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		filestore := f.create("before-fs.tar.gz", "fs", 0)
		database := f.create("before.sql.gz", "database", filestore.ID)

		database.Name = "after.sql.gz"
		require.NoError(t, f.service.Update(context.Background(), database))

		assert.Equal(t, "content of before.sql.gz", string(f.s3.GetObject(t, "group-name/after.sql.gz")))
		assert.Equal(t, "content of before-fs.tar.gz", string(f.s3.GetObject(t, "group-name/after-fs.tar.gz")))
		renamed, err := f.repository.FindById(context.Background(), filestore.ID)
		require.NoError(t, err)
		assert.Equal(t, "after-fs.tar.gz", renamed.Name)
		assert.Equal(t, fmt.Sprintf("s3://%s/group-name/after-fs.tar.gz", f.s3.Bucket), renamed.Url)
	})

	t.Run("KeepsTheFileWhenOnlyTheDescriptionChanges", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		database := f.create("described.sql.gz", "database", 0)

		database.Description = "only the description changes"
		require.NoError(t, f.service.Update(context.Background(), database))

		assert.Equal(t, "content of described.sql.gz", string(f.s3.GetObject(t, "group-name/described.sql.gz")))
		stored, err := f.repository.FindById(context.Background(), database.ID)
		require.NoError(t, err)
		assert.Equal(t, "only the description changes", stored.Description)
	})

	t.Run("RefusesTheNameOfAnotherDatabaseWithoutOverwritingItsFile", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		f.create("theirs.sql.gz", "database", 0)
		database := f.create("ours.sql.gz", "database", 0)

		database.Name = "theirs.sql.gz"
		err := f.service.Update(context.Background(), database)

		require.Error(t, err)
		assert.True(t, errdef.IsDuplicated(err), "got: %v", err)
		assert.Equal(t, "content of theirs.sql.gz", string(f.s3.GetObject(t, "group-name/theirs.sql.gz")))
		assert.Equal(t, "content of ours.sql.gz", string(f.s3.GetObject(t, "group-name/ours.sql.gz")))
	})

	// Two databases whose names differ only in their extension map to the same file store name.
	t.Run("RefusesTheFilestoreNameOfAnotherDatabaseWithoutMovingAnything", func(t *testing.T) {
		t.Parallel()
		f := setup(t)
		theirFilestore := f.create("training-fs.tar.gz", "fs", 0)
		f.create("training.pgc", "database", theirFilestore.ID)
		ourFilestore := f.create("ours-fs.tar.gz", "fs", 0)
		database := f.create("ours.sql.gz", "database", ourFilestore.ID)

		database.Name = "training.sql.gz"
		err := f.service.Update(context.Background(), database)

		require.Error(t, err)
		assert.True(t, errdef.IsDuplicated(err), "got: %v", err)
		assert.Equal(t, "content of training-fs.tar.gz", string(f.s3.GetObject(t, "group-name/training-fs.tar.gz")))
		assert.Equal(t, "content of ours-fs.tar.gz", string(f.s3.GetObject(t, "group-name/ours-fs.tar.gz")))
		assert.Equal(t, "content of ours.sql.gz", string(f.s3.GetObject(t, "group-name/ours.sql.gz")))
		stored, err := f.repository.FindById(context.Background(), database.ID)
		require.NoError(t, err)
		assert.Equal(t, "ours.sql.gz", stored.Name)
	})
}
