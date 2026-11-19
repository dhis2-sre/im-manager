package migrations_test

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage/migrations"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestDropLegacyDeploymentNameIndex(t *testing.T) {
	t.Parallel()

	migrate := func(t *testing.T, db *gorm.DB) {
		for _, migration := range migrations.All() {
			if migration.ID == "20261019000" {
				require.NoError(t, migration.Migrate(db))
				return
			}
		}
		t.Fatal("migration 20261019000 not found")
	}

	indexExists := func(t *testing.T, db *gorm.DB, table string) bool {
		var count int64
		require.NoError(t, db.Raw("SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_name_and_group' AND tablename = ?", table).Scan(&count).Error)
		return count > 0
	}

	t.Run("DropsItFromDeployments", func(t *testing.T) {
		t.Parallel()
		db := inttest.SetupDB(t)
		require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_name_and_group ON deployments (name, group_name)").Error)
		group := model.Group{Name: "group", Hostname: "group.example.org", Deployable: true}
		require.NoError(t, db.Omit(clause.Associations).Create(&group).Error)
		user := model.User{Email: "user@example.org"}
		require.NoError(t, db.Create(&user).Error)

		migrate(t, db)

		require.False(t, indexExists(t, db, "deployments"))
		require.NoError(t, db.Create(&model.Deployment{Name: "shared", GroupName: group.Name, UserID: user.ID, Preset: true}).Error)
		require.NoError(t, db.Create(&model.Deployment{Name: "shared", GroupName: group.Name, UserID: user.ID}).Error)
	})

	t.Run("LeavesTheDatabasesOneAlone", func(t *testing.T) {
		t.Parallel()
		db := inttest.SetupDB(t)
		require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_name_and_group ON databases (name, group_name)").Error)

		migrate(t, db)

		require.True(t, indexExists(t, db, "databases"))
	})
}
