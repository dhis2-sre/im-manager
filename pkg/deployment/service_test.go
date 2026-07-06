package deployment

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dhis2-sre/im-manager/pkg/model"
)

// fakeDatabaseService resolves database records by id and mints deterministic download links.
type fakeDatabaseService struct {
	byID map[uint]*model.Database
}

func (f fakeDatabaseService) FindById(ctx context.Context, id uint) (*model.Database, error) {
	db, ok := f.byID[id]
	if !ok {
		return nil, fmt.Errorf("database %d not found", id)
	}
	return db, nil
}

func (f fakeDatabaseService) CreateExternalDownload(ctx context.Context, databaseID uint, expiration uint) (*model.ExternalDownload, error) {
	return &model.ExternalDownload{UUID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprint(databaseID))), DatabaseID: databaseID}, nil
}

func TestBuildSeed(t *testing.T) {
	t.Setenv("HOSTNAME", "http://im")
	s := Service{databaseService: fakeDatabaseService{byID: map[uint]*model.Database{
		10: {ID: 10, FilestoreID: 20},
		20: {ID: 20, Url: "s3://im-bucket/group/save-fs.tar.gz"},
	}}}
	instance := &model.DeploymentInstance{Parameters: model.DeploymentInstanceParameters{
		"DATABASE_ID": {Value: "10"},
	}}

	extraEnv, filestore, err := s.buildSeed(context.Background(), instance)
	require.NoError(t, err)
	dbUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("10")).String()
	fsUUID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("20")).String()
	assert.Equal(t, "http://im/databases/external/"+dbUUID, extraEnv["DATABASE_DOWNLOAD_URL"])
	assert.Equal(t, "http://im/databases/external/"+fsUUID, extraEnv["FILESTORE_DOWNLOAD_URL"])
	require.NotNil(t, filestore)
	assert.Equal(t, "s3://im-bucket/group/save-fs.tar.gz", filestore.Url)
}

func TestBuildSeedNoFilestore(t *testing.T) {
	t.Setenv("HOSTNAME", "http://im")
	s := Service{databaseService: fakeDatabaseService{byID: map[uint]*model.Database{
		10: {ID: 10, FilestoreID: 0}, // database saved without a filestore backup
	}}}
	instance := &model.DeploymentInstance{Parameters: model.DeploymentInstanceParameters{
		"DATABASE_ID": {Value: "10"},
	}}

	extraEnv, filestore, err := s.buildSeed(context.Background(), instance)
	require.NoError(t, err)
	assert.Contains(t, extraEnv, "DATABASE_DOWNLOAD_URL")
	assert.NotContains(t, extraEnv, "FILESTORE_DOWNLOAD_URL")
	assert.Nil(t, filestore, "no filestore backup means nothing to restore")
}

func TestBuildSeedNoDatabaseID(t *testing.T) {
	extraEnv, filestore, err := Service{}.buildSeed(context.Background(), &model.DeploymentInstance{})
	require.NoError(t, err)
	assert.Nil(t, extraEnv, "a fresh instance with no DATABASE_ID has nothing to seed")
	assert.Nil(t, filestore)
}
