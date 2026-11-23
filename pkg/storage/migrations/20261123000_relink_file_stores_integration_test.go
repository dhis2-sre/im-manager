package migrations_test

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRelinkFileStores(t *testing.T) {
	t.Parallel()

	migrate := func(t *testing.T, db *gorm.DB) {
		for _, migration := range migrations.All() {
			if migration.ID == "20261123000" {
				require.NoError(t, migration.Migrate(db))
				return
			}
		}
		t.Fatal("migration 20261123000 not found")
	}

	setup := func(t *testing.T) (*gorm.DB, func(groupName, name, kind string, filestoreID uint) *model.Database) {
		t.Helper()

		db := inttest.SetupDB(t)
		user := model.User{Email: "user@example.org"}
		require.NoError(t, db.Create(&user).Error)

		create := func(groupName, name, kind string, filestoreID uint) *model.Database {
			database := &model.Database{Name: name, GroupName: groupName, Type: kind, Slug: groupName + "/" + name, UserID: user.ID, FilestoreID: filestoreID}
			require.NoError(t, db.Create(database).Error)
			return database
		}

		return db, create
	}

	filestoreOf := func(t *testing.T, db *gorm.DB, database *model.Database) uint {
		t.Helper()

		var stored model.Database
		require.NoError(t, db.First(&stored, database.ID).Error)
		return stored.FilestoreID
	}

	t.Run("RelinksTheFileStoreADatabaseLost", func(t *testing.T) {
		t.Parallel()
		db, create := setup(t)
		plain := create("emis", "training-05/10/2026.sql.gz", "database", 0)
		plainFileStore := create("emis", "training-05/10/2026-fs.tar.gz", "fs", 0)
		custom := create("emis", "archive.pgc", "database", 0)
		customFileStore := create("emis", "archive-fs.tar.gz", "fs", 0)

		migrate(t, db)

		assert.Equal(t, plainFileStore.ID, filestoreOf(t, db, plain))
		assert.Equal(t, customFileStore.ID, filestoreOf(t, db, custom))
	})

	t.Run("LeavesTheFileStoreADatabaseHas", func(t *testing.T) {
		t.Parallel()
		db, create := setup(t)
		own := create("emis", "renamed-fs.tar.gz", "fs", 0)
		database := create("emis", "training.sql.gz", "database", own.ID)
		create("emis", "training-fs.tar.gz", "fs", 0)

		migrate(t, db)

		assert.Equal(t, own.ID, filestoreOf(t, db, database))
	})

	t.Run("LeavesAFileStoreAnotherDatabasePointsAt", func(t *testing.T) {
		t.Parallel()
		db, create := setup(t)
		fileStore := create("emis", "training-fs.tar.gz", "fs", 0)
		create("emis", "renamed.sql.gz", "database", fileStore.ID)
		database := create("emis", "training.sql.gz", "database", 0)

		migrate(t, db)

		assert.Zero(t, filestoreOf(t, db, database))
	})

	// Deleting a database deletes its file store, so two databases must not end up sharing one.
	t.Run("LeavesAFileStoreTwoDatabasesCouldClaim", func(t *testing.T) {
		t.Parallel()
		db, create := setup(t)
		plain := create("emis", "training.sql.gz", "database", 0)
		custom := create("emis", "training.pgc", "database", 0)
		create("emis", "training-fs.tar.gz", "fs", 0)

		migrate(t, db)

		assert.Zero(t, filestoreOf(t, db, plain))
		assert.Zero(t, filestoreOf(t, db, custom))
	})

	t.Run("LeavesAFileStoreOfAnotherGroup", func(t *testing.T) {
		t.Parallel()
		db, create := setup(t)
		database := create("emis", "training.sql.gz", "database", 0)
		create("lmis", "training-fs.tar.gz", "fs", 0)

		migrate(t, db)

		assert.Zero(t, filestoreOf(t, db, database))
	})
}
