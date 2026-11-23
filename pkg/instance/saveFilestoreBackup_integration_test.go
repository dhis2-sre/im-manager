package instance

import (
	"context"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSaveFilestoreBackup(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (*repository, *gorm.DB, *model.Database) {
		t.Helper()

		db := inttest.SetupDB(t)
		user := model.User{Email: "user@dhis2.org"}
		require.NoError(t, db.Create(&user).Error)
		instanceRepository, err := NewRepository(db, "01234567890123456789012345678901")
		require.NoError(t, err)
		database := &model.Database{Name: "training.sql.gz", GroupName: "group-name", Type: "database", Slug: "group-name-training-sql-gz", UserID: user.ID}
		require.NoError(t, db.Create(database).Error)

		return instanceRepository, db, database
	}

	newFilestore := func(database *model.Database) *model.Database {
		return &model.Database{Name: "training-fs.tar.gz", GroupName: database.GroupName, Type: "fs", Url: "s3://bucket/group-name/training-fs.tar.gz", UserID: database.UserID, Size: 10}
	}

	t.Run("StoresTheFilestoreAndPointsTheDatabaseAtIt", func(t *testing.T) {
		t.Parallel()
		instanceRepository, db, database := setup(t)

		filestore := newFilestore(database)
		require.NoError(t, instanceRepository.SaveFilestoreBackup(context.Background(), database.ID, filestore))

		var stored model.Database
		require.NoError(t, db.First(&stored, database.ID).Error)
		assert.NotZero(t, filestore.ID)
		assert.Equal(t, filestore.ID, stored.FilestoreID)
	})

	// A file store no database points at holds its name against every later save. Nothing may be
	// left behind when the database cannot be pointed at it, here because it was deleted meanwhile.
	t.Run("LeavesNoFilestoreBehindWhenTheDatabaseCannotPointAtIt", func(t *testing.T) {
		t.Parallel()
		instanceRepository, db, database := setup(t)
		require.NoError(t, db.Delete(&model.Database{}, database.ID).Error)

		err := instanceRepository.SaveFilestoreBackup(context.Background(), database.ID, newFilestore(database))

		require.Error(t, err)
		var filestores int64
		require.NoError(t, db.Model(&model.Database{}).Where("type = ?", "fs").Count(&filestores).Error)
		assert.Zero(t, filestores)
	})
}
