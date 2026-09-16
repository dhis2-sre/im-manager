package inttest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dhis2-sre/im-manager/internal/testenv"
	"github.com/dhis2-sre/im-manager/pkg/inttest"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/go-redis/redis"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

func TestDatabaseFixturesAreIsolated(t *testing.T) {
	var mu sync.Mutex
	names := map[string]bool{}
	t.Run("parallel fixtures", func(t *testing.T) {
		for range 4 {
			t.Run("fixture", func(t *testing.T) {
				t.Parallel()
				db := inttest.SetupDB(t)
				require.NoError(t, db.Create(&model.User{Email: "same@example.org"}).Error)
				var count int64
				require.NoError(t, db.Model(&model.User{}).Count(&count).Error)
				require.EqualValues(t, 1, count)
				var name string
				require.NoError(t, db.Raw("SELECT current_database()").Scan(&name).Error)
				mu.Lock()
				names[name] = true
				mu.Unlock()
				var present bool
				require.NoError(t, db.Raw("SELECT EXISTS (SELECT FROM pg_extension WHERE extname = 'pg_trgm')").Scan(&present).Error)
				require.True(t, present)
				require.NoError(t, db.Raw("SELECT EXISTS (SELECT FROM pg_indexes WHERE indexname = 'idx_databases_description')").Scan(&present).Error)
				require.True(t, present)
			})
		}
	})
	require.Len(t, names, 4)
	config, err := testenv.Shared.Postgres()
	require.NoError(t, err)
	admin, err := testenv.Admin(config)
	require.NoError(t, err)
	defer admin.Close()
	for name := range names {
		var exists bool
		require.NoError(t, admin.QueryRow("SELECT EXISTS (SELECT FROM pg_database WHERE datname = $1)", name).Scan(&exists))
		require.False(t, exists, "fixture database must be dropped after subtests finish")
	}
}

func TestRedisLeasesAcrossProcesses(t *testing.T) {
	if os.Getenv("IM_TEST_REDIS_CHILD") == "1" {
		client := inttest.SetupRedis(t)
		require.ErrorIs(t, client.Get("same-key").Err(), redis.Nil)
		require.NoError(t, client.Set("same-key", "child", 0).Err())
		return
	}
	client := inttest.SetupRedis(t)
	require.NoError(t, client.Set("same-key", "parent", 0).Err())
	address, err := testenv.Shared.Redis()
	require.NoError(t, err)
	encoded, err := json.Marshal(testenv.Config{Redis: address})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRedisLeasesAcrossProcesses$", "-test.count=1") // #nosec G204 -- re-executes this test binary with a fixed test selector.
	cmd.Env = append(os.Environ(), testenv.ConfigEnv+"="+string(encoded), "IM_TEST_REDIS_CHILD=1")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.Equal(t, "parent", client.Get("same-key").Val())

}

func TestS3BucketsAreIsolated(t *testing.T) {
	left := inttest.SetupS3(t)
	ctx := context.Background()
	_, err := left.Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(left.Bucket), Key: aws.String("same-key"), Body: bytes.NewReader([]byte("left"))})
	require.NoError(t, err)
	var removed string
	t.Run("other fixture", func(t *testing.T) {
		right := inttest.SetupS3(t)
		removed = right.Bucket
		require.NotEqual(t, left.Bucket, right.Bucket)
		_, err := right.Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(right.Bucket), Key: aws.String("same-key"), Body: bytes.NewReader([]byte("right"))})
		require.NoError(t, err)
		require.Equal(t, []byte("right"), right.GetObject(t, "same-key"))
		// An interrupted upload must not outlive its fixture either.
		_, err = right.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{Bucket: aws.String(right.Bucket), Key: aws.String("unfinished")})
		require.NoError(t, err)
	})
	require.Equal(t, []byte("left"), left.GetObject(t, "same-key"))
	_, err = left.Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(removed)})
	require.Error(t, err)
}

func TestMinIOBucketsAreIsolated(t *testing.T) {
	left := inttest.SetupMinIO(t)
	ctx := context.Background()
	_, err := left.Client.PutObject(ctx, left.Bucket, "same-key", bytes.NewReader([]byte("left")), 4, minio.PutObjectOptions{})
	require.NoError(t, err)
	var removed string
	t.Run("other fixture", func(t *testing.T) {
		right := inttest.SetupMinIO(t)
		removed = right.Bucket
		require.NotEqual(t, left.Bucket, right.Bucket)
		_, err := right.Client.PutObject(ctx, right.Bucket, "same-key", bytes.NewReader([]byte("right")), 5, minio.PutObjectOptions{})
		require.NoError(t, err)
	})
	exists, err := left.Client.BucketExists(ctx, removed)
	require.NoError(t, err)
	require.False(t, exists)
	info, err := left.Client.StatObject(ctx, left.Bucket, "same-key", minio.StatObjectOptions{})
	require.NoError(t, err)
	require.EqualValues(t, 4, info.Size)
}
