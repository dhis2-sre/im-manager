package migratev3

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gosimple/slug"
	"gorm.io/gorm"
)

// The names the removed stacks' charts gave their resources, all derived from the release name.
const (
	databaseContainer = "postgresql"
	minioContainer    = "minio"
	coreContainer     = "core"
)

func databaseRelease(d LegacyDeployment) string     { return d.ReleaseName() + "-database" }
func databaseStatefulSet(d LegacyDeployment) string { return databaseRelease(d) + "-postgresql" }
func databaseClaim(d LegacyDeployment) string       { return "data-" + databaseStatefulSet(d) + "-0" }
func minioRelease(d LegacyDeployment) string        { return d.ReleaseName() + "-minio" }
func pgAdminRelease(d LegacyDeployment) string      { return d.ReleaseName() + "-pgadmin" }

func instanceSelector(instance *model.DeploymentInstance) string {
	return fmt.Sprintf("im-instance-id=%d", instance.ID)
}

// Uploader streams an object to S3, as pkg/storage's S3Client does.
type Uploader interface {
	StreamUpload(ctx context.Context, bucket, key, contentType string, r io.Reader) (int64, error)
}

// Snapshotter saves a legacy deployment's database and file store as a database record of its
// group, the same shape a save produces, so the dhis2-v2 seed can restore it.
type Snapshotter struct {
	Uploader Uploader
	Bucket   string
	// ExcludeAnalyticsData leaves the rows of the analytics tables out of the dump, as a save does.
	// They can be regenerated, and are often the larger part of a DHIS 2 database.
	ExcludeAnalyticsData bool
}

func snapshotName(d LegacyDeployment) string {
	return "v3-migration-" + d.Deployment.Name
}

// DatabaseSize is the on-disk size of the deployment's database, which orders the migration so the
// longest dumps start first.
func DatabaseSize(ctx context.Context, cluster *Cluster, d LegacyDeployment) (int64, error) {
	pod, err := cluster.readyPod(ctx, instanceSelector(d.Database), time.Minute)
	if err != nil {
		return 0, err
	}
	var out bytes.Buffer
	command := append(databaseEnvironment(d), "psql", "--no-align", "--tuples-only", "--command", "SELECT pg_database_size(current_database())")
	if err := cluster.exec(ctx, pod, databaseContainer, command, &out); err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(out.String()), 10, 64)
}

func databaseEnvironment(d LegacyDeployment) []string {
	return []string{
		"env",
		"PGHOST=127.0.0.1",
		"PGUSER=" + d.Database.Parameters["DATABASE_USERNAME"].Value,
		"PGPASSWORD=" + d.Database.Parameters["DATABASE_PASSWORD"].Value,
		"PGDATABASE=" + d.Database.Parameters["DATABASE_NAME"].Value,
	}
}

// pgDumpPlainTrailer is the last line of a complete plain dump. A pod exec can report success
// having delivered only part of the output, so its absence marks a truncated dump.
const pgDumpPlainTrailer = "-- PostgreSQL database dump complete"

// DumpDatabase streams a gzipped plain pg_dump of the deployment's database to S3 and returns the
// object's size. It runs as the database's own user with the options a save uses, so the snapshot
// restores the way a saved database does.
func (s Snapshotter) DumpDatabase(ctx context.Context, cluster *Cluster, d LegacyDeployment, key string) (int64, error) {
	pod, err := cluster.readyPod(ctx, instanceSelector(d.Database), 10*time.Minute)
	if err != nil {
		return 0, err
	}

	command := append(databaseEnvironment(d), "pg_dump", "--no-owner", "--no-acl", "--blobs", "--format=plain")
	if s.ExcludeAnalyticsData {
		command = append(command, "--exclude-table-data=analytics*", "--exclude-table-data=_*")
	}

	return s.stream(ctx, key, func(w io.Writer) error {
		compressed, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			return err
		}
		tail := &tailWriter{w: compressed, max: 4096}
		if err := cluster.exec(ctx, pod, databaseContainer, command, tail); err != nil {
			return fmt.Errorf("pg_dump failed: %v", err)
		}
		if !bytes.Contains(tail.tail, []byte(pgDumpPlainTrailer)) {
			return fmt.Errorf("pg_dump output is truncated, %q not found at its end", pgDumpPlainTrailer)
		}
		return compressed.Close()
	})
}

// minioHost points mc at the MinIO of the pod it runs in, with the credentials the minio stack set.
const minioHost = "MC_HOST_backup=http://dhisdhis:dhisdhis@127.0.0.1:9000"

// ArchiveFilestore streams a gzipped tar of the deployment's file store to S3, in the layout the
// dhis2-v2 seeds expect, and reports false for storage types whose files are not in the cluster.
// MinIO keeps no plain objects on disk, so its bucket is mirrored to a temporary directory in the
// pod first, which needs that much free ephemeral storage.
func (s Snapshotter) ArchiveFilestore(ctx context.Context, cluster *Cluster, d LegacyDeployment, key string) (int64, bool, error) {
	switch d.StorageType() {
	case "minio":
		pod, err := cluster.readyPod(ctx, instanceSelector(d.Minio), 10*time.Minute)
		if err != nil {
			return 0, false, err
		}
		directory := "/tmp/v3-migration-filestore"
		defer func() {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
			defer cancel()
			_ = cluster.exec(cleanup, pod, minioContainer, []string{"rm", "-rf", directory}, io.Discard)
		}()
		mirror := []string{"env", minioHost, "MC_CONFIG_DIR=/tmp/.mc", "mc", "mirror", "--quiet", "--overwrite", "backup/dhis2", directory}
		if err := cluster.exec(ctx, pod, minioContainer, mirror, io.Discard); err != nil {
			return 0, false, fmt.Errorf("mc mirror failed: %v", err)
		}
		size, err := s.stream(ctx, key, func(w io.Writer) error {
			return cluster.exec(ctx, pod, minioContainer, []string{"tar", "-C", directory, "-czf", "-", "."}, w)
		})
		return size, true, err
	case "filesystem":
		pod, err := cluster.readyPod(ctx, instanceSelector(d.Core), 10*time.Minute)
		if err != nil {
			return 0, false, err
		}
		files := strings.TrimRight(d.Core.Parameters["DHIS2_HOME"].Value, "/") + "/files"
		size, err := s.stream(ctx, key, func(w io.Writer) error {
			return cluster.exec(ctx, pod, coreContainer, []string{"tar", "-C", files, "-czf", "-", "."}, w)
		})
		return size, true, err
	default:
		return 0, false, nil
	}
}

// stream uploads what write produces. A failed write reaches the upload through the pipe, so a
// partial object is never stored as if it were complete.
func (s Snapshotter) stream(ctx context.Context, key string, write func(io.Writer) error) (int64, error) {
	reader, writer := io.Pipe()
	type result struct {
		size int64
		err  error
	}
	uploaded := make(chan result, 1)
	go func() {
		size, err := s.Uploader.StreamUpload(ctx, s.Bucket, key, "application/octet-stream", reader)
		_ = reader.CloseWithError(err)
		uploaded <- result{size, err}
	}()

	if err := write(writer); err != nil {
		_ = writer.CloseWithError(err)
		<-uploaded
		return 0, err
	}
	_ = writer.Close()
	upload := <-uploaded
	return upload.size, upload.err
}

// tailWriter passes writes through while keeping the last max bytes.
type tailWriter struct {
	w    io.Writer
	max  int
	tail []byte
}

func (t *tailWriter) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	t.tail = append(t.tail, p[:n]...)
	if len(t.tail) > t.max {
		t.tail = t.tail[len(t.tail)-t.max:]
	}
	return n, err
}

// RecordSnapshot creates the database records of a snapshot: the dump, and the file store linked
// to it the way a save links them, so the deploy builds both seed links from DATABASE_ID alone.
func RecordSnapshot(db *gorm.DB, bucket string, d LegacyDeployment, databaseKey string, databaseSize int64, filestoreKey string, filestoreSize int64) (uint, error) {
	description := fmt.Sprintf("Snapshot of deployment %d taken by the version 3.0 migration, deleted by its cleanup", d.Deployment.ID)
	snapshot := model.Database{
		Name:        snapshotName(d) + ".sql.gz",
		GroupName:   d.Deployment.GroupName,
		Description: description,
		Url:         fmt.Sprintf("s3://%s/%s", bucket, databaseKey),
		Type:        "database",
		UserID:      d.Deployment.UserID,
		Size:        databaseSize,
	}
	snapshot.Slug = slug.Make(snapshot.GroupName + "/" + snapshot.Name)

	err := db.Transaction(func(tx *gorm.DB) error {
		// A snapshot recorded by an attempt that failed before saving its state is replaced.
		names := []string{snapshotName(d) + ".sql.gz", snapshotName(d) + ".fs.tar.gz"}
		if err := tx.Where("group_name = ? AND name IN ?", d.Deployment.GroupName, names).Delete(&model.Database{}).Error; err != nil {
			return fmt.Errorf("failed to replace an earlier snapshot record: %v", err)
		}
		if filestoreKey != "" {
			filestore := model.Database{
				Name:        snapshotName(d) + ".fs.tar.gz",
				GroupName:   d.Deployment.GroupName,
				Description: description,
				Url:         fmt.Sprintf("s3://%s/%s", bucket, filestoreKey),
				Type:        "fs",
				UserID:      d.Deployment.UserID,
				Size:        filestoreSize,
			}
			filestore.Slug = slug.Make(filestore.GroupName + "/" + filestore.Name)
			if err := tx.Create(&filestore).Error; err != nil {
				return fmt.Errorf("failed to record the file store snapshot: %v", err)
			}
			snapshot.FilestoreID = filestore.ID
		}
		if err := tx.Create(&snapshot).Error; err != nil {
			return fmt.Errorf("failed to record the database snapshot: %v", err)
		}
		return nil
	})
	return snapshot.ID, err
}
