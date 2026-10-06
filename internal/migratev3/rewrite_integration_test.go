package migratev3

import (
	"fmt"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) { inttest.Main(m) }

func TestRewrite(t *testing.T) {
	db := inttest.SetupDB(t)
	require.NoError(t, migrateStateTable(db))

	user := model.User{Email: "owner@dhis2.org", EmailToken: uuid.New()}
	require.NoError(t, db.Create(&user).Error)
	group := model.Group{Name: "play", Namespace: "play", Hostname: "play.dhis2.org"}
	require.NoError(t, db.Create(&group).Error)
	original := model.Database{Name: "sierra-leone.sql.gz", GroupName: group.Name, Slug: "play-sierra-leone-sql-gz", UserID: user.ID}
	require.NoError(t, db.Create(&original).Error)
	deployment := model.Deployment{Name: "play", GroupName: group.Name, UserID: user.ID, TTL: 3600}
	require.NoError(t, db.Create(&deployment).Error)

	create := func(stackName string, values map[string]string) *model.DeploymentInstance {
		instance := instance(0, stackName, values)
		instance.GroupName, instance.DeploymentID = group.Name, deployment.ID
		seal(t, instance)
		require.NoError(t, db.Create(instance).Error)
		return instance
	}
	databaseValues := legacyDatabaseParameters()
	databaseValues["DATABASE_ID"] = "sierra-leone"
	databaseInstance := create(legacyDatabaseStack, databaseValues)
	minio := create(legacyMinioStack, legacyMinioParameters())
	core := create(legacyCoreStack, legacyCoreParameters())
	pgAdmin := create(pgAdminStack, legacyPgAdminParameters())
	require.NoError(t, db.Create(&model.Lock{DatabaseID: original.ID, InstanceID: databaseInstance.ID, UserID: user.ID}).Error)

	legacy, unsupported, err := FindLegacyDeployments(db, testKey)
	require.NoError(t, err)
	require.Empty(t, unsupported)
	require.Len(t, legacy, 1)
	d := legacy[0]
	assert.Equal(t, core.ID, d.Core.ID)
	assert.Equal(t, pgAdmin.ID, d.PgAdmin.ID)
	assert.Equal(t, fmt.Sprintf("play-%d", group.ID), d.ReleaseName())

	state := &State{
		DeploymentID:       deployment.ID,
		DeploymentName:     deployment.Name,
		GroupName:          group.Name,
		CoreInstanceID:     core.ID,
		PgAdminInstanceID:  pgAdmin.ID,
		HasMinio:           true,
		OriginalDatabaseID: d.Database.Parameters["DATABASE_ID"].Value,
		SnapshotDatabaseID: 99,
		Step:               StepSnapshotted,
	}
	require.NoError(t, saveState(db, state))

	require.NoError(t, Rewrite(db, testKey, d, state))

	t.Run("turns the core into a dhis2-v2 instance seeded from the snapshot", func(t *testing.T) {
		rewritten := load(t, db, core.ID)
		assert.Equal(t, stack.DHIS2V2.Name, rewritten.StackName)
		assert.Equal(t, model.DeployStatusFailed, rewritten.DeployStatus)
		assert.Equal(t, "99", rewritten.Parameters["DATABASE_ID"].Value)
		assert.Equal(t, "2.41.3", rewritten.Parameters["IMAGE_TAG"].Value)
		assert.Equal(t, "500m", rewritten.Parameters["DB_RESOURCES_REQUESTS_CPU"].Value)
		assert.Equal(t, "true", rewritten.Parameters["ENABLE_PGADMIN"].Value)
		assert.Len(t, rewritten.Parameters, len(stack.DHIS2V2.Parameters))
		for _, parameter := range rewritten.Parameters {
			assert.Equal(t, stack.DHIS2V2.Name, parameter.StackName)
		}
	})

	t.Run("encrypts what dhis2-v2 declares sensitive and nothing else", func(t *testing.T) {
		rewritten := load(t, db, core.ID)
		require.NoError(t, decryptInstance(testKey, rewritten, sensitiveOf(stack.DHIS2V2)))
		assert.Equal(t, "secret", rewritten.Parameters["DATABASE_PASSWORD"].Value)
		assert.Equal(t, "dhis", rewritten.Parameters["DATABASE_USERNAME"].Value)

		stored := load(t, db, core.ID)
		assert.Equal(t, "8Gi", stored.Parameters["FILESYSTEM_VOLUME_SIZE"].Value, "dhis2-v2 does not encrypt the volume size")
		assert.NotEqual(t, "secret", stored.Parameters["DATABASE_PASSWORD"].Value)
	})

	t.Run("points pgadmin at the CloudNativePG cluster", func(t *testing.T) {
		rewritten := load(t, db, pgAdmin.ID)
		require.NoError(t, decryptInstance(testKey, rewritten, sensitiveOf(stack.PgAdmin)))
		assert.Equal(t, fmt.Sprintf("play-%d-dhis2-postgresql-rw.play.svc", group.ID), rewritten.Parameters["DATABASE_HOSTNAME"].Value)
		assert.Equal(t, "pgadmin", rewritten.Parameters["PGADMIN_PASSWORD"].Value)
	})

	t.Run("removes the database and minio instances", func(t *testing.T) {
		var count int64
		require.NoError(t, db.Model(&model.DeploymentInstance{}).Where("id IN ?", []uint{databaseInstance.ID, minio.ID}).Count(&count).Error)
		assert.Zero(t, count)
		require.NoError(t, db.Model(&model.DeploymentInstanceParameter{}).Where("deployment_instance_id IN ?", []uint{databaseInstance.ID, minio.ID}).Count(&count).Error)
		assert.Zero(t, count)
	})

	t.Run("moves the database lock onto the dhis2-v2 instance", func(t *testing.T) {
		var lock model.Lock
		require.NoError(t, db.First(&lock, "database_id = ?", original.ID).Error)
		assert.Equal(t, core.ID, lock.InstanceID)
	})

	t.Run("records the step", func(t *testing.T) {
		saved, err := findState(db, deployment.ID)
		require.NoError(t, err)
		assert.Equal(t, StepRewritten, saved.Step)
	})

	t.Run("is no longer a legacy deployment", func(t *testing.T) {
		legacy, unsupported, err := FindLegacyDeployments(db, testKey)
		require.NoError(t, err)
		assert.Empty(t, legacy)
		assert.Empty(t, unsupported)
	})

	t.Run("restores the original DATABASE_ID", func(t *testing.T) {
		require.NoError(t, RestoreDatabaseID(db, state))

		assert.Equal(t, "sierra-leone", load(t, db, core.ID).Parameters["DATABASE_ID"].Value)
	})
}

func load(t *testing.T, db *gorm.DB, id uint) *model.DeploymentInstance {
	t.Helper()
	var instance model.DeploymentInstance
	require.NoError(t, db.Preload("GormParameters").First(&instance, id).Error)
	return &instance
}

func sensitiveOf(s stack.Stack) map[string]bool {
	sensitive := map[string]bool{}
	for name, parameter := range s.Parameters {
		sensitive[name] = parameter.Sensitive
	}
	return sensitive
}
