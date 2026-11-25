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

// TestDeleteKeepsAFilestoreAnotherDatabasePointsAt covers databases that share one file store. The
// file store goes with the last database pointing at it, not with the first one deleted.
func TestDeleteKeepsAFilestoreAnotherDatabasePointsAt(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	s3 := inttest.SetupS3(t)
	repository := NewRepository(db)
	service := NewService(logger, s3.Bucket, storage.NewS3Client(logger, s3.Client, nil), nil, repository, nil, nil)
	ctx := context.Background()

	user := model.User{Email: "user@dhis2.org"}
	require.NoError(t, db.Create(&user).Error)

	create := func(name, databaseType string, filestoreID uint) *model.Database {
		key := "group-name/" + name
		_, err := s3.Client.PutObject(ctx, &awss3.PutObjectInput{Bucket: aws.String(s3.Bucket), Key: aws.String(key), Body: strings.NewReader(name)})
		require.NoError(t, err)
		database := &model.Database{Name: name, GroupName: "group-name", Type: databaseType, Url: fmt.Sprintf("s3://%s/%s", s3.Bucket, key), UserID: user.ID, FilestoreID: filestoreID}
		require.NoError(t, repository.Create(ctx, database))
		return database
	}
	filestore := create("shared-fs.tar.gz", "fs", 0)
	first := create("first.sql.gz", "database", filestore.ID)
	second := create("second.sql.gz", "database", filestore.ID)

	require.NoError(t, service.Delete(ctx, first.ID))

	_, err := repository.FindById(ctx, filestore.ID)
	require.NoError(t, err, "the second database still restores its files from the file store")
	assert.Equal(t, "shared-fs.tar.gz", string(s3.GetObject(t, "group-name/shared-fs.tar.gz")))

	require.NoError(t, service.Delete(ctx, second.ID))

	_, err = repository.FindById(ctx, filestore.ID)
	assert.True(t, errdef.IsNotFound(err), "got: %v", err)
	_, err = s3.TryGetObject("group-name/shared-fs.tar.gz")
	assert.Error(t, err)
}
