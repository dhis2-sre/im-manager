package instance

import (
	"context"
	"fmt"
	"io"
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
	"github.com/gosimple/slug"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type tarballOf string

func (content tarballOf) stream(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, string(content))
	return err
}

type backupFixture struct {
	service Service
	db      *gorm.DB
	s3      *inttest.S3Client
	user    model.User
}

func setupBackupFixture(t *testing.T) backupFixture {
	t.Helper()

	db := inttest.SetupDB(t)
	user := model.User{Email: "user@dhis2.org"}
	require.NoError(t, db.Create(&user).Error)
	instanceRepository, err := NewRepository(db, "01234567890123456789012345678901")
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	s3 := inttest.SetupS3(t)

	return backupFixture{
		service: Service{logger: logger, instanceRepository: instanceRepository, s3Client: storage.NewS3Client(logger, s3.Client, nil), s3Bucket: s3.Bucket},
		db:      db,
		s3:      s3,
		user:    user,
	}
}

func (f backupFixture) createDatabase(t *testing.T, name string, filestoreID uint) *model.Database {
	t.Helper()

	database := &model.Database{Name: name, GroupName: "group-name", Type: "database", Slug: slug.Make("group-name/" + name), UserID: f.user.ID, FilestoreID: filestoreID}
	require.NoError(t, f.db.Create(database).Error)

	return database
}

func (f backupFixture) createFilestore(t *testing.T, name, content string) *model.Database {
	t.Helper()

	key := "group-name/" + name
	_, err := f.s3.Client.PutObject(context.Background(), &awss3.PutObjectInput{Bucket: aws.String(f.s3.Bucket), Key: aws.String(key), Body: strings.NewReader(content)})
	require.NoError(t, err)
	filestore := &model.Database{Name: name, GroupName: "group-name", Type: "fs", Slug: slug.Make(key + "-fs"), Url: fmt.Sprintf("s3://%s/%s", f.s3.Bucket, key), UserID: f.user.ID, Size: int64(len(content))}
	require.NoError(t, f.db.Create(filestore).Error)

	return filestore
}

func (f backupFixture) filestores(t *testing.T) []model.Database {
	t.Helper()

	var filestores []model.Database
	require.NoError(t, f.db.Where("type = ?", "fs").Order("id").Find(&filestores).Error)

	return filestores
}

func (f backupFixture) reload(t *testing.T, id uint) model.Database {
	t.Helper()

	var database model.Database
	require.NoError(t, f.db.First(&database, id).Error)

	return database
}

func TestBackupFilestore(t *testing.T) {
	t.Parallel()

	t.Run("CreatesAFilestoreForADatabaseWithoutOne", func(t *testing.T) {
		t.Parallel()
		f := setupBackupFixture(t)
		database := f.createDatabase(t, "fresh.sql.gz", 0)

		require.NoError(t, f.service.backupFilestore(context.Background(), tarballOf("files"), "fresh-fs.tar.gz", database))

		assert.Equal(t, "files", string(f.s3.GetObject(t, "group-name/fresh-fs.tar.gz")))
		filestores := f.filestores(t)
		require.Len(t, filestores, 1)
		assert.Equal(t, "fresh-fs.tar.gz", filestores[0].Name)
		assert.Equal(t, fmt.Sprintf("s3://%s/group-name/fresh-fs.tar.gz", f.s3.Bucket), filestores[0].Url)
		assert.Equal(t, int64(len("files")), filestores[0].Size)
		assert.Equal(t, f.user.ID, filestores[0].UserID)
		assert.Equal(t, filestores[0].ID, f.reload(t, database.ID).FilestoreID)
		assert.Equal(t, filestores[0].ID, database.FilestoreID)
	})

	// An instance of one group can be deployed from, and saved over, a database of another.
	t.Run("FilesANewFilestoreInTheGroupOfTheDatabase", func(t *testing.T) {
		t.Parallel()
		f := setupBackupFixture(t)
		database := f.createDatabase(t, "elsewhere.sql.gz", 0)
		require.NoError(t, f.db.Model(database).Update("group_name", "other-group").Error)

		require.NoError(t, f.service.backupFilestore(context.Background(), tarballOf("files"), "elsewhere-fs.tar.gz", database))

		assert.Equal(t, "files", string(f.s3.GetObject(t, "other-group/elsewhere-fs.tar.gz")))
		filestores := f.filestores(t)
		require.Len(t, filestores, 1)
		assert.Equal(t, "other-group", filestores[0].GroupName)
	})

	// A copied database has a file store that is not named the way a backup names one. The backup
	// still belongs in that file store, not in a second tarball next to it.
	t.Run("OverwritesTheFilestoreTheDatabaseHasWhereItIs", func(t *testing.T) {
		t.Parallel()
		f := setupBackupFixture(t)
		filestore := f.createFilestore(t, "copy.fs.tar.gz", "before")
		database := f.createDatabase(t, "copy.sql.gz", filestore.ID)

		require.NoError(t, f.service.backupFilestore(context.Background(), tarballOf("after the save"), "copy-fs.tar.gz", database))

		assert.Equal(t, "after the save", string(f.s3.GetObject(t, "group-name/copy.fs.tar.gz")))
		_, err := f.s3.TryGetObject("group-name/copy-fs.tar.gz")
		assert.Error(t, err, "nothing is written under the name a backup would pick for a new file store")
		filestores := f.filestores(t)
		require.Len(t, filestores, 1)
		assert.Equal(t, filestore.ID, filestores[0].ID)
		assert.Equal(t, "copy.fs.tar.gz", filestores[0].Name)
		assert.Equal(t, int64(len("after the save")), filestores[0].Size)
		assert.Equal(t, filestore.ID, f.reload(t, database.ID).FilestoreID)
	})

	// Older file stores are named without the extension of their database, so a name can be taken.
	t.Run("RefusesTheFilestoreOfAnotherDatabaseWithoutOverwritingIt", func(t *testing.T) {
		t.Parallel()
		f := setupBackupFixture(t)
		theirs := f.createFilestore(t, "training-fs.tar.gz", "their files")
		f.createDatabase(t, "training.sql.gz", theirs.ID)
		database := f.createDatabase(t, "training", 0)

		err := f.service.backupFilestore(context.Background(), tarballOf("our files"), "training-fs.tar.gz", database)

		require.Error(t, err)
		assert.True(t, errdef.IsDuplicated(err), "got: %v", err)
		assert.Equal(t, "their files", string(f.s3.GetObject(t, "group-name/training-fs.tar.gz")))
		assert.Zero(t, f.reload(t, database.ID).FilestoreID)
		assert.Len(t, f.filestores(t), 1)
	})

	// Databases can share one file store. A save writes on top of it, as it does for the database.
	t.Run("SavesOnTopOfAFilestoreOtherDatabasesShare", func(t *testing.T) {
		t.Parallel()
		f := setupBackupFixture(t)
		shared := f.createFilestore(t, "shared-fs.tar.gz", "shared files")
		other := f.createDatabase(t, "other.sql.gz", shared.ID)
		database := f.createDatabase(t, "saved.sql.gz", shared.ID)

		require.NoError(t, f.service.backupFilestore(context.Background(), tarballOf("files after the save"), "saved-fs.tar.gz", database))

		assert.Equal(t, "files after the save", string(f.s3.GetObject(t, "group-name/shared-fs.tar.gz")))
		require.Len(t, f.filestores(t), 1)
		assert.Equal(t, shared.ID, f.reload(t, database.ID).FilestoreID)
		assert.Equal(t, shared.ID, f.reload(t, other.ID).FilestoreID)
	})

	// A backup can run for minutes, and the database it was started with is a copy from before.
	t.Run("LeavesWhatChangedDuringTheBackupAlone", func(t *testing.T) {
		t.Parallel()
		f := setupBackupFixture(t)
		database := f.createDatabase(t, "edited.sql.gz", 0)
		require.NoError(t, f.db.Model(&model.Database{}).Where("id = ?", database.ID).Update("description", "edited during the backup").Error)

		require.NoError(t, f.service.backupFilestore(context.Background(), tarballOf("files"), "edited-fs.tar.gz", database))

		stored := f.reload(t, database.ID)
		assert.NotZero(t, stored.FilestoreID)
		assert.Equal(t, "edited during the backup", stored.Description)
	})
}
