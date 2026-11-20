package instance

import (
	"context"
	"testing"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRecordBackup(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (Service, *gorm.DB, model.User) {
		t.Helper()

		db := inttest.SetupDB(t)
		user := model.User{Email: "user@dhis2.org"}
		require.NoError(t, db.Create(&user).Error)
		instanceRepository, err := NewRepository(db, "01234567890123456789012345678901")
		require.NoError(t, err)

		return Service{instanceRepository: instanceRepository}, db, user
	}

	// A database can lose the link to its file store while the row stays behind, as every save did
	// before the link was carried over. The backup has just overwritten the tarball that row
	// describes, so the row is taken over instead of colliding with it on every later save.
	t.Run("AdoptsTheFilestoreNoDatabasePointsAt", func(t *testing.T) {
		t.Parallel()
		service, db, user := setup(t)
		ctx := context.Background()

		leftBehind, err := service.recordBackup(ctx, "group-name", "s3://bucket/group-name/training-fs.tar.gz", "training-fs.tar.gz", user.ID, 10, 0)
		require.NoError(t, err)

		adopted, err := service.recordBackup(ctx, "group-name", "s3://bucket/group-name/training-fs.tar.gz", "training-fs.tar.gz", user.ID, 20, 0)
		require.NoError(t, err)

		assert.Equal(t, leftBehind.ID, adopted.ID)
		var stored model.Database
		require.NoError(t, db.First(&stored, leftBehind.ID).Error)
		assert.Equal(t, int64(20), stored.Size)
		var count int64
		require.NoError(t, db.Model(&model.Database{}).Where("type = ?", "fs").Count(&count).Error)
		assert.Equal(t, int64(1), count)
	})

	// Two databases whose names differ only in their extension map to the same file store name.
	// Sharing the row would let deleting one database take the other's file store with it.
	t.Run("RefusesTheFilestoreOfAnotherDatabase", func(t *testing.T) {
		t.Parallel()
		service, db, user := setup(t)
		ctx := context.Background()

		owned, err := service.recordBackup(ctx, "group-name", "s3://bucket/group-name/training-fs.tar.gz", "training-fs.tar.gz", user.ID, 10, 0)
		require.NoError(t, err)
		owner := &model.Database{Name: "training.sql.gz", GroupName: "group-name", Type: "database", Slug: "group-name-training-sql-gz", UserID: user.ID, FilestoreID: owned.ID}
		require.NoError(t, db.Create(owner).Error)

		_, err = service.recordBackup(ctx, "group-name", "s3://bucket/group-name/training-fs.tar.gz", "training-fs.tar.gz", user.ID, 20, 0)

		require.Error(t, err)
		assert.True(t, errdef.IsDuplicated(err), "got: %v", err)
	})
}
