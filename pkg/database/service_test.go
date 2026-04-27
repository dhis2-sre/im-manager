package database

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDatabaseService(t *testing.T) {
	t.Parallel()

	t.Run("SaveAs", func(t *testing.T) {
		t.Parallel()

		fx := setupSaveAsServiceFixture(t)

		doneCh := make(chan *model.Database, 1)
		done := func(_ context.Context, saved *model.Database) {
			doneCh <- saved
		}

		newDB, err := fx.Svc.SaveAs(context.Background(), fx.User.ID, fx.SourceDB, fx.Instance, fx.Stack, "new-save.sql.gz", "custom", done)
		require.NoError(t, err)

		assert.Equal(t, "new-save.sql.gz", newDB.Name)
		assert.Equal(t, "packages", newDB.GroupName)
		assert.Equal(t, "database", newDB.Type)
		assert.Equal(t, fx.User.ID, newDB.UserID)
		assert.NotZero(t, newDB.ID)

		select {
		case savedDB := <-doneCh:
			assert.Equal(t, "s3://database-bucket/packages/new-save.sql.gz", savedDB.Url)
			assert.Greater(t, savedDB.Size, int64(0))
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for async SaveAs to complete")
		}

		fx.S3Spy.mu.Lock()
		assert.Equal(t, "database-bucket", fx.S3Spy.lastBucket)
		assert.Equal(t, "packages/new-save.sql.gz", fx.S3Spy.lastKey)
		fx.S3Spy.mu.Unlock()

		var finalDB model.Database
		err = fx.DB.First(&finalDB, newDB.ID).Error
		require.NoError(t, err)
		assert.Equal(t, "s3://database-bucket/packages/new-save.sql.gz", finalDB.Url)
		assert.Greater(t, finalDB.Size, int64(0))
	})
}

type saveAsServiceFixture struct {
	DB       *gorm.DB
	Svc      *service
	User     *model.User
	SourceDB *model.Database
	Instance *model.DeploymentInstance
	Stack    *model.Stack
	S3Spy    *s3ClientSpy
}

func setupSaveAsServiceFixture(t *testing.T) saveAsServiceFixture {
	t.Helper()

	db := inttest.SetupDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	s3Spy := &s3ClientSpy{}

	group := &model.Group{
		Name:      "packages",
		Hostname:  "some",
		Namespace: "ns",
	}
	user := &model.User{
		Email:  "user1@dhis2.org",
		Groups: []model.Group{*group},
	}
	db.Create(user)

	sourceDB := &model.Database{
		Name:      "source.sql.gz",
		GroupName: "packages",
		Type:      "database",
		Url:       "s3://database-bucket/packages/source.sql.gz",
		UserID:    user.ID,
	}
	db.Create(sourceDB)

	repo := NewRepository(db)

	fakeGroup := &mockGroupService{group: group}
	fakePod := &fakePodExec{output: []byte("pg_dump output")}
	podExecFunc := podExecutorFunc(func(_ model.Cluster) (PodExecutor, error) {
		return fakePod, nil
	})

	svc := NewService(logger, "database-bucket", s3Spy, fakeGroup, repo, podExecFunc)

	instance := &model.DeploymentInstance{
		Name:      "name",
		GroupName: "packages",
		StackName: "dhis2-db",
		Group:     group,
		Parameters: model.DeploymentInstanceParameters{
			"DATABASE_ID":       {Value: "1"},
			"DATABASE_NAME":     {Value: "dhis2"},
			"DATABASE_USERNAME": {Value: "dhis"},
			"DATABASE_PASSWORD": {Value: "dhis"},
		},
	}

	stack := &model.Stack{
		Name:            "dhis2-db",
		HostnamePattern: "%s-database-postgresql.%s.svc",
		ParameterProviders: model.ParameterProviders{
			"DATABASE_HOSTNAME": model.ParameterProviderFunc(func(instance model.DeploymentInstance) (string, error) {
				return instance.Name + "-database-postgresql." + instance.Group.Namespace + ".svc", nil
			}),
		},
	}

	return saveAsServiceFixture{
		DB:       db,
		Svc:      svc,
		User:     user,
		SourceDB: sourceDB,
		Instance: instance,
		Stack:    stack,
		S3Spy:    s3Spy,
	}
}

type mockGroupService struct {
	group *model.Group
}

func (m *mockGroupService) Find(ctx context.Context, name string) (*model.Group, error) {
	return m.group, nil
}

type fakePodExec struct {
	output []byte
}

func (f *fakePodExec) Exec(ctx context.Context, namespace, pod, container string, command []string, stdout, stderr io.Writer) error {
	_, err := stdout.Write(f.output)
	return err
}

type s3ClientSpy struct {
	mu         sync.Mutex
	lastBucket string
	lastKey    string
	lastData   []byte
}

func (s *s3ClientSpy) Copy(bucket, source, destination string) error { return nil }

func (s *s3ClientSpy) Move(bucket, source, destination string) error { return nil }

func (s *s3ClientSpy) Upload(ctx context.Context, bucket string, key string, body storage.ReadAtSeeker, size int64) error {
	return nil
}

func (s *s3ClientSpy) Delete(bucket, key string) error { return nil }

func (s *s3ClientSpy) Download(ctx context.Context, bucket, key string, dst io.Writer, cb func(int64)) error {
	return nil
}

func (s *s3ClientSpy) StreamUpload(ctx context.Context, bucket, key, contentType string, r io.Reader) (int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastBucket = bucket
	s.lastKey = key
	s.lastData = data
	return int64(len(data)), nil
}
