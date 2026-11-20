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
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFinalizeSaveKeepsWhatTheDumpDoesNotReplace covers the record swap at the end of a save: the
// fresh dump takes over the original's identity, and everything a dump does not produce has to
// survive it. Losing the file store link made every later file store backup of that database fail
// with "already exists", and its seeds restore no files.
func TestFinalizeSaveKeepsWhatTheDumpDoesNotReplace(t *testing.T) {
	t.Parallel()

	db := inttest.SetupDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	s3 := inttest.SetupS3(t)
	s3Client := storage.NewS3Client(logger, s3.Client, nil)
	repository := NewRepository(db)
	service := NewService(logger, s3.Bucket, s3Client, nil, repository, nil, nil)
	ctx := context.Background()

	user := model.User{Email: "user@dhis2.org"}
	require.NoError(t, db.Create(&user).Error)

	filestore := &model.Database{Name: "original-fs.tar.gz", GroupName: "group-name", Type: "fs", Slug: "group-name-original-fs-tar-gz-fs", UserID: user.ID}
	require.NoError(t, db.Create(filestore).Error)

	original, err := service.CreateDatabase(ctx, user.ID, "group-name", "original.sql.gz")
	require.NoError(t, err)
	original.Url = fmt.Sprintf("s3://%s/group-name/original.sql.gz", s3.Bucket)
	original.Description = "kept up to date by the nightly save"
	original.FilestoreID = filestore.ID
	require.NoError(t, repository.Save(ctx, original))
	original, err = repository.FindById(ctx, original.ID)
	require.NoError(t, err)

	dumped, err := service.CreateDatabase(ctx, user.ID, "group-name", "temporary-name")
	require.NoError(t, err)
	_, err = s3.Client.PutObject(ctx, &awss3.PutObjectInput{Bucket: aws.String(s3.Bucket), Key: aws.String("group-name/temporary-name"), Body: strings.NewReader("fresh dump")})
	require.NoError(t, err)
	dumped.Url = fmt.Sprintf("s3://%s/group-name/temporary-name", s3.Bucket)
	require.NoError(t, repository.Save(ctx, dumped))

	saved, err := service.finalizeSave(ctx, original, dumped)
	require.NoError(t, err)

	assert.Equal(t, original.ID, saved.ID)
	assert.Equal(t, filestore.ID, saved.FilestoreID, "the file store backup that follows needs the link to overwrite the same file store")

	reloaded, err := repository.FindById(ctx, original.ID)
	require.NoError(t, err)
	assert.Equal(t, filestore.ID, reloaded.FilestoreID)
	assert.Equal(t, "kept up to date by the nightly save", reloaded.Description)
	assert.Equal(t, "original.sql.gz", reloaded.Name)
	assert.Equal(t, "fresh dump", string(s3.GetObject(t, "group-name/original.sql.gz")))
}
