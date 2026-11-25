package migrations_test

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRemoveLeftoversOfFailedSaves(t *testing.T) {
	t.Parallel()

	migrate := func(t *testing.T, db *gorm.DB) {
		for _, migration := range migrations.All() {
			if migration.ID == "20261123001" {
				require.NoError(t, migration.Migrate(db))
				return
			}
		}
		t.Fatal("migration 20261123001 not found")
	}

	setup := func(t *testing.T) (*gorm.DB, func(name, kind, url string, filestoreID uint) *model.Database) {
		t.Helper()

		db := inttest.SetupDB(t)
		user := model.User{Email: "user@example.org"}
		require.NoError(t, db.Create(&user).Error)

		create := func(name, kind, url string, filestoreID uint) *model.Database {
			database := &model.Database{Name: name, GroupName: "emis", Type: kind, Slug: "emis/" + name, Url: url, UserID: user.ID, FilestoreID: filestoreID}
			require.NoError(t, db.Create(database).Error)
			return database
		}

		return db, create
	}

	exists := func(t *testing.T, db *gorm.DB, database *model.Database) bool {
		t.Helper()

		var count int64
		require.NoError(t, db.Model(&model.Database{}).Where("id = ?", database.ID).Count(&count).Error)
		return count == 1
	}

	t.Run("RemovesDatabasesWithoutAFile", func(t *testing.T) {
		t.Parallel()
		db, create := setup(t)
		temporary := create("058984b7-6b67-4c91-8a6b-cd97a07e9cc4", "database", "", 0)
		failedSaveAs := create("lao-demo.pgc", "database", "", 0)
		require.NoError(t, db.Create(&model.ExternalDownload{UUID: uuid.New(), DatabaseID: failedSaveAs.ID}).Error)
		saved := create("training.sql.gz", "database", "s3://bucket/emis/training.sql.gz", 0)

		migrate(t, db)

		assert.False(t, exists(t, db, temporary))
		assert.False(t, exists(t, db, failedSaveAs))
		assert.True(t, exists(t, db, saved))
	})

	t.Run("RemovesFileStoresNoDatabasePointsAt", func(t *testing.T) {
		t.Parallel()
		db, create := setup(t)
		leftBehind := create("deleted-fs.tar.gz", "fs", "s3://bucket/emis/deleted-fs.tar.gz", 0)
		linked := create("training-fs.tar.gz", "fs", "s3://bucket/emis/training-fs.tar.gz", 0)
		database := create("training.sql.gz", "database", "s3://bucket/emis/training.sql.gz", linked.ID)

		migrate(t, db)

		assert.False(t, exists(t, db, leftBehind))
		assert.True(t, exists(t, db, linked))
		assert.True(t, exists(t, db, database))
	})

	t.Run("ResetsALinkToAFileStoreThatIsGone", func(t *testing.T) {
		t.Parallel()
		db, create := setup(t)
		database := create("training.sql.gz", "database", "s3://bucket/emis/training.sql.gz", 604)

		migrate(t, db)

		var stored model.Database
		require.NoError(t, db.First(&stored, database.ID).Error)
		assert.Zero(t, stored.FilestoreID)
	})
}
