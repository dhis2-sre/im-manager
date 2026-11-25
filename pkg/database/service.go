package database

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/maps"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/kube"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/dhis2-sre/im-manager/pkg/storage"

	pg "github.com/habx/pg-commands"

	"github.com/google/uuid"
)

// Publisher publishes notifications for async database operations.
// The implementation owns JSON marshaling and error handling — callers pass typed
// payloads and never see failures.
type Publisher interface {
	Publish(ctx context.Context, userID uint, groupName, kind string, payload any)
}

//goland:noinspection GoExportedFuncWithUnexportedType
func NewService(logger *slog.Logger, s3Bucket string, s3Client S3Client, groupService groupService, repository *repository, podExecutor podExecutorFunc, publisher Publisher) *Service {
	return &Service{
		logger:       logger,
		s3Bucket:     s3Bucket,
		s3Client:     s3Client,
		groupService: groupService,
		repository:   repository,
		podExecutor:  podExecutor,
		publisher:    publisher,
	}
}

type groupService interface {
	Find(ctx context.Context, name string) (*model.Group, error)
}

type PodExecutor interface {
	Exec(ctx context.Context, namespace, podName, container string, command []string, stdout, stderr io.Writer) error
}

type podExecutorFunc func(cluster model.Cluster) (*kube.Client, error)

type Service struct {
	logger       *slog.Logger
	s3Bucket     string
	s3Client     S3Client
	groupService groupService
	repository   *repository
	podExecutor  podExecutorFunc
	publisher    Publisher
}

type S3Client interface {
	Copy(bucket string, source string, destination string) error
	Move(bucket string, source string, destination string) error
	Upload(ctx context.Context, bucket string, key string, body storage.ReadAtSeeker, size int64) error
	Delete(bucket string, key string) error
	Download(ctx context.Context, bucket string, key string, dst io.Writer, cb func(contentLength int64)) error
	StreamUpload(ctx context.Context, bucket, key, contentType string, r io.Reader) (int64, error)
}

func (s Service) FindByIdentifier(ctx context.Context, identifier string) (*model.Database, error) {
	id, err := strconv.ParseUint(identifier, 10, 32)
	if err != nil {
		database, err := s.FindBySlug(ctx, identifier)
		if err != nil {
			return nil, err
		}
		return database, nil
	}

	database, err := s.FindById(ctx, uint(id))
	if err != nil {
		return nil, err
	}
	return database, nil
}

// Copy copies the database and its file store under a new name. Both names are checked before any
// file is copied, since a copy onto a taken name overwrites the file of the record holding it.
func (s Service) Copy(ctx context.Context, id uint, d *model.Database, group *model.Group) error {
	source, err := s.FindById(ctx, id)
	if err != nil {
		return err
	}

	if strings.HasSuffix(source.Name, ".pgc") && !strings.HasSuffix(d.Name, ".pgc") {
		d.Name += ".pgc"
	}
	if strings.HasSuffix(source.Name, ".sql.gz") && !strings.HasSuffix(d.Name, ".sql.gz") {
		d.Name += ".sql.gz"
	}

	err = s.refuseTakenName(ctx, d)
	if err != nil {
		return err
	}
	if source.FilestoreID != 0 {
		err = s.refuseTakenName(ctx, &model.Database{Name: model.FilestoreName(d.Name), GroupName: group.Name})
		if err != nil {
			return err
		}
	}

	u, err := url.Parse(source.Url)
	if err != nil {
		return err
	}

	sourceKey := strings.TrimPrefix(u.Path, "/")
	destinationKey := fmt.Sprintf("%s/%s", group.Name, d.Name)
	err = s.s3Client.Copy(s.s3Bucket, sourceKey, destinationKey)
	if err != nil {
		return err
	}

	d.Url = fmt.Sprintf("s3://%s/%s", s.s3Bucket, destinationKey)
	d.Size = source.Size

	updateSlug(d)

	err = s.repository.Create(ctx, d)
	if err != nil {
		return err
	}

	return s.copyFS(ctx, d, group, source.FilestoreID)
}

func (s Service) copyFS(ctx context.Context, d *model.Database, group *model.Group, filestoreID uint) error {
	fs, err := s.repository.FindById(ctx, filestoreID)
	if err != nil {
		if errdef.IsNotFound(err) {
			return nil
		}
		return err
	}

	newName := model.FilestoreName(d.Name)

	fsUrl, err := url.Parse(fs.Url)
	if err != nil {
		return err
	}

	sourceKey := strings.TrimPrefix(fsUrl.Path, "/")
	destinationKey := fmt.Sprintf("%s/%s", group.Name, newName)
	err = s.s3Client.Copy(s.s3Bucket, sourceKey, destinationKey)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("s3://%s/%s", s.s3Bucket, destinationKey)
	fsNewDatabase := &model.Database{
		Name:      newName,
		GroupName: group.Name,
		Url:       url,
		Type:      "fs",
		UserID:    d.UserID,
		Size:      fs.Size,
	}

	updateSlug(fsNewDatabase)

	err = s.repository.Create(ctx, fsNewDatabase)
	if err != nil {
		return err
	}

	d.FilestoreID = fsNewDatabase.ID

	err = s.repository.Save(ctx, d)
	if err != nil {
		return err
	}

	return nil
}

func (s Service) FindById(ctx context.Context, id uint) (*model.Database, error) {
	return s.repository.FindById(ctx, id)
}

func (s Service) FindBySlug(ctx context.Context, slug string) (*model.Database, error) {
	return s.repository.FindBySlug(ctx, slug)
}

func (s Service) Lock(ctx context.Context, databaseId uint, instanceId uint, userId uint) (*model.Lock, error) {
	return s.repository.Lock(ctx, databaseId, instanceId, userId)
}

func (s Service) Unlock(ctx context.Context, databaseId uint) error {
	return s.repository.Unlock(ctx, databaseId)
}

type ReadAtSeeker interface {
	io.ReaderAt
	io.ReadSeeker
}

func (s Service) Upload(ctx context.Context, d *model.Database, group *model.Group, reader ReadAtSeeker, size int64) (*model.Database, error) {
	key := fmt.Sprintf("%s/%s", group.Name, d.Name)
	err := s.s3Client.Upload(ctx, s.s3Bucket, key, reader, size)
	if err != nil {
		return nil, err
	}

	d.Url = fmt.Sprintf("s3://%s/%s", s.s3Bucket, key)
	d.Size = size

	err = s.repository.Save(ctx, d)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (s Service) Download(ctx context.Context, id uint, dst io.Writer, cb func(contentLength int64)) error {
	d, err := s.repository.FindById(ctx, id)
	if err != nil {
		return err
	}

	if d.Url == "" {
		return errdef.NewBadRequest("database with id %d doesn't reference any url", id)
	}

	u, err := url.Parse(d.Url)
	if err != nil {
		return err
	}

	key := strings.TrimPrefix(u.Path, "/")
	return s.s3Client.Download(ctx, s.s3Bucket, key, dst, cb)
}

func (s Service) Delete(ctx context.Context, id uint) error {
	d, err := s.repository.FindById(ctx, id)
	if err != nil {
		return err
	}

	if d.Lock != nil {
		return errdef.NewBadRequest("database is locked")
	}

	u, err := url.Parse(d.Url)
	if err != nil {
		return err
	}

	key := strings.TrimPrefix(u.Path, "/")
	if key != "" {
		err = s.s3Client.Delete(s.s3Bucket, key)
		if err != nil {
			return err
		}
	}

	err = s.repository.Delete(ctx, id)
	if err != nil {
		return err
	}

	return s.deleteFS(ctx, d)
}

// deleteFS removes the file store of a deleted database, unless another database still points at it.
func (s Service) deleteFS(ctx context.Context, d *model.Database) error {
	fs, err := s.repository.FindById(ctx, d.FilestoreID)
	if err != nil {
		if errdef.IsNotFound(err) {
			return nil
		}
		return err
	}

	inUse, err := s.repository.FilestoreInUse(ctx, fs.ID)
	if err != nil {
		return err
	}
	if inUse {
		return nil
	}

	fsUrl, err := url.Parse(fs.Url)
	if err != nil {
		return err
	}

	fsKey := strings.TrimPrefix(fsUrl.Path, "/")
	if fsKey != "" {
		err = s.s3Client.Delete(s.s3Bucket, fsKey)
		if err != nil {
			return err
		}
	}

	err = s.repository.Delete(ctx, fs.ID)
	if err != nil {
		return err
	}

	return nil
}

func (s Service) List(ctx context.Context, user *model.User) ([]GroupsWithDatabases, error) {
	groups := append(user.Groups, user.AdminGroups...) //nolint:gocritic
	groupsByName := make(map[string]model.Group)
	for _, group := range groups {
		groupsByName[group.Name] = group
	}
	groupNames := maps.Keys(groupsByName)

	databases, err := s.repository.FindByGroupNames(ctx, groupNames)
	if err != nil {
		return nil, err
	}

	if len(databases) < 1 {
		return []GroupsWithDatabases{}, nil
	}

	return groupsWithDatabases(databases), nil
}

func groupsWithDatabases(databases []model.Database) []GroupsWithDatabases {
	groupNamesMap := map[string]struct{}{}
	for _, database := range databases {
		groupNamesMap[database.GroupName] = struct{}{}
	}

	groupNames := maps.Keys(groupNamesMap)
	groupsWithDatabases := make([]GroupsWithDatabases, len(groupNames))
	for i, groupName := range groupNames {
		groupsWithDatabases[i].Name = groupName
		groupsWithDatabases[i].Databases = filterDatabases(databases, func(database *model.Database) bool {
			return database.GroupName == groupName
		})

		slices.SortFunc(groupsWithDatabases[i].Databases, func(a, b model.Database) int {
			return cmp.Compare(a.Name, b.Name)
		})
	}

	slices.SortFunc(groupsWithDatabases, func(a, b GroupsWithDatabases) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return groupsWithDatabases
}

func filterDatabases(databases []model.Database, test func(database *model.Database) bool) (ret []model.Database) {
	for i := range databases {
		if test(&databases[i]) {
			ret = append(ret, databases[i])
		}
	}
	return
}

// Update renames the database, and its file store with it, and stores its description. Both names
// are checked before any file is moved, since a move onto a taken name overwrites the file of the
// record holding it. A database that keeps its name keeps its file store as it is.
func (s Service) Update(ctx context.Context, d *model.Database) error {
	key, err := fileKey(d)
	if err != nil {
		return err
	}
	if key == fmt.Sprintf("%s/%s", d.GroupName, d.Name) {
		return s.repository.Update(ctx, d)
	}

	fs, err := s.renamedFilestore(ctx, d)
	if err != nil {
		return err
	}

	err = s.refuseTakenName(ctx, d)
	if err != nil {
		return err
	}
	if fs != nil {
		err = s.refuseTakenName(ctx, fs)
		if err != nil {
			return err
		}
	}

	err = s.moveFile(d)
	if err != nil {
		return err
	}

	err = s.repository.Update(ctx, d)
	if err != nil {
		return err
	}

	if fs == nil {
		return nil
	}

	err = s.moveFile(fs)
	if err != nil {
		return err
	}

	return s.repository.Update(ctx, fs)
}

// renamedFilestore returns the file store of the database under the name it gets from the
// database, or nil when the database has none.
func (s Service) renamedFilestore(ctx context.Context, d *model.Database) (*model.Database, error) {
	if d.FilestoreID == 0 {
		return nil, nil
	}

	fs, err := s.repository.FindById(ctx, d.FilestoreID)
	if err != nil {
		if errdef.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	fs.Name = model.FilestoreName(d.Name)

	return fs, nil
}

func (s Service) refuseTakenName(ctx context.Context, d *model.Database) error {
	taken, err := s.repository.NameTaken(ctx, d)
	if err != nil {
		return err
	}
	if taken {
		return errdef.NewDuplicated("database named %q already exists", d.Name)
	}

	return nil
}

// moveFile moves the file of the record to the key its group and name give it. A file already
// there stays untouched, since S3 refuses to copy an object onto itself.
func (s Service) moveFile(d *model.Database) error {
	sourceUrl, err := url.Parse(d.Url)
	if err != nil {
		return err
	}

	sourceKey := strings.TrimPrefix(sourceUrl.Path, "/")
	destination := fmt.Sprintf("%s/%s", d.GroupName, d.Name)
	if sourceKey == destination {
		return nil
	}

	err = s.s3Client.Move(s.s3Bucket, sourceKey, destination)
	if err != nil {
		return err
	}

	d.Url = fmt.Sprintf("s3://%s/%s", s.s3Bucket, destination)

	return nil
}

func (s Service) CreateExternalDownload(ctx context.Context, databaseID uint, expiration uint) (*model.ExternalDownload, error) {
	err := s.repository.PurgeExternalDownload(ctx)
	if err != nil {
		return nil, err
	}

	return s.repository.CreateExternalDownload(ctx, databaseID, expiration)
}

func (s Service) FindExternalDownload(ctx context.Context, uuid uuid.UUID) (*model.ExternalDownload, error) {
	err := s.repository.PurgeExternalDownload(ctx)
	if err != nil {
		return nil, err
	}
	return s.repository.FindExternalDownload(ctx, uuid)
}

// EnsureLocked verifies the database may be saved from the given instance, locking it when it
// isn't already. It returns the reloaded database and whether it was locked beforehand.
func (s Service) EnsureLocked(ctx context.Context, database *model.Database, instanceId, userId uint) (*model.Database, bool, error) {
	lock := database.Lock
	isLocked := lock != nil
	if isLocked && (lock.InstanceID != instanceId || lock.UserID != userId) {
		return nil, false, errdef.NewUnauthorized("database is locked")
	}

	if !isLocked {
		_, err := s.Lock(ctx, database.ID, instanceId, userId)
		if err != nil {
			return nil, false, err
		}

		reloaded, err := s.FindById(ctx, database.ID)
		if err != nil {
			return nil, false, err
		}
		database = reloaded
	}

	return database, isLocked, nil
}

// SaveLocked overwrites the file of the given locked database with a fresh dump from the instance.
// The dump is written next to the file and moved over it once complete, so a failed save leaves the
// database as it was. It blocks until done and returns the saved record.
func (s Service) SaveLocked(ctx context.Context, database *model.Database, instance *model.DeploymentInstance, stack *stack.Stack, wasLocked bool) (*model.Database, error) {
	if !wasLocked {
		defer func() {
			err := s.repository.Unlock(ctx, database.ID)
			if err != nil {
				s.logError(ctx, fmt.Errorf("unlock database failed: %v", err))
			}
		}()
	}

	s.publishSave(ctx, database.UserID, database, "started", "", 0)

	temporaryKey := fmt.Sprintf("%s/%s", database.GroupName, uuid.New().String())
	size, err := s.dumpToKey(ctx, instance, stack, getFormat(database), temporaryKey)
	if err != nil {
		return nil, s.failSave(ctx, database.UserID, database, err)
	}

	saved, err := s.finalizeSave(ctx, database, temporaryKey, size)
	if err != nil {
		return nil, s.failSave(ctx, database.UserID, database, err)
	}

	s.publishSave(ctx, database.UserID, database, "success", "", size)

	return saved, nil
}

// finalizeSave moves the fresh dump over the file of the database and records its size. The
// database keeps its record, and with it everything a dump does not produce. The record is read
// again first, since the database can be renamed while the dump runs.
func (s Service) finalizeSave(ctx context.Context, database *model.Database, temporaryKey string, size int64) (*model.Database, error) {
	key, err := s.currentFileKey(ctx, database.ID)
	if err == nil {
		err = s.s3Client.Move(s.s3Bucket, temporaryKey, key)
	}
	if err != nil {
		if deleteErr := s.s3Client.Delete(s.s3Bucket, temporaryKey); deleteErr != nil {
			s.logger.ErrorContext(ctx, "failed to delete the dump that could not be moved", "key", temporaryKey, "error", deleteErr)
		}
		return nil, fmt.Errorf("moving database failed: %w", err)
	}

	err = s.repository.UpdateFile(ctx, database.ID, fmt.Sprintf("s3://%s/%s", s.s3Bucket, key), size)
	if err != nil {
		return nil, err
	}

	return s.repository.FindById(ctx, database.ID)
}

func (s Service) currentFileKey(ctx context.Context, id uint) (string, error) {
	current, err := s.repository.FindById(ctx, id)
	if err != nil {
		return "", err
	}

	return fileKey(current)
}

// fileKey returns the S3 key of the file of the database. The url is what downloads and seeds
// read, so a save has to write there even when the key does not match the group and name.
func fileKey(database *model.Database) (string, error) {
	if database.Url == "" {
		return fmt.Sprintf("%s/%s", database.GroupName, database.Name), nil
	}

	u, err := url.Parse(database.Url)
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(u.Path, "/"), nil
}

func getFormat(database *model.Database) string {
	if strings.HasSuffix(database.Url, ".pgc") {
		return "custom"
	}
	return "plain"
}

// CreateDatabase creates the database record a subsequent Dump streams into.
func (s Service) CreateDatabase(ctx context.Context, userId uint, groupName, name string) (*model.Database, error) {
	newDatabase := &model.Database{
		Name: name,
		// TODO: For now, only saving to the same group is supported
		GroupName: groupName,
		Type:      "database",
		UserID:    userId,
	}

	err := s.repository.Save(ctx, newDatabase)
	if err != nil {
		return nil, err
	}

	return newDatabase, nil
}

// Dump streams a pg_dump of the instance's database into the file of the given record, which
// CreateDatabase made for it, and records the url and size. A record whose dump fails has no file
// and is removed. It blocks until the dump completes and publishes database-save events.
func (s Service) Dump(ctx context.Context, userId uint, database *model.Database, instance *model.DeploymentInstance, stack *stack.Stack, format string) (*model.Database, error) {
	s.publishSave(ctx, userId, database, "started", "", 0)

	saved, err := s.dumpIntoRecord(ctx, database, instance, stack, format)
	if err != nil {
		if deleteErr := s.repository.Delete(ctx, database.ID); deleteErr != nil {
			s.logger.ErrorContext(ctx, "failed to remove the record of a failed dump", "databaseId", database.ID, "error", deleteErr)
		}
		return nil, s.failSave(ctx, userId, database, err)
	}

	s.publishSave(ctx, userId, database, "success", "", saved.Size)

	return saved, nil
}

func (s Service) dumpIntoRecord(ctx context.Context, database *model.Database, instance *model.DeploymentInstance, stack *stack.Stack, format string) (*model.Database, error) {
	key := fmt.Sprintf("%s/%s", database.GroupName, database.Name)
	size, err := s.dumpToKey(ctx, instance, stack, format, key)
	if err != nil {
		return nil, err
	}

	err = s.repository.UpdateFile(ctx, database.ID, fmt.Sprintf("s3://%s/%s", s.s3Bucket, key), size)
	if err != nil {
		return nil, err
	}

	return s.repository.FindById(ctx, database.ID)
}

func (s Service) publishSave(ctx context.Context, userId uint, database *model.Database, status, errorMessage string, size int64) {
	s.publisher.Publish(ctx, userId, database.GroupName, kindDatabaseSave, newDatabaseEvent(database, status, errorMessage, size))
}

func (s Service) failSave(ctx context.Context, userId uint, database *model.Database, err error) error {
	s.logError(ctx, err)
	s.publishSave(ctx, userId, database, "error", err.Error(), 0)
	return err
}

// dumpToKey streams a pg_dump of the instance's database to the given S3 key and returns the
// number of bytes stored. A failed dump aborts its upload, so it leaves no object behind.
func (s Service) dumpToKey(ctx context.Context, instance *model.DeploymentInstance, stack *stack.Stack, format, key string) (int64, error) {
	group, err := s.groupService.Find(ctx, instance.GroupName)
	if err != nil {
		return 0, err
	}

	dump, err := newPgDumpConfig(instance, stack)
	if err != nil {
		return 0, err
	}
	dump.Host = "localhost"
	command := buildPgDumpCommand(dump, format)

	client, err := s.podExecutor(group.Cluster)
	if err != nil {
		return 0, err
	}

	access, err := kube.FindPostgresAccess(stack.Components)
	if err != nil {
		return 0, errdef.NewBadRequest("stack %q does not support database save", stack.Name)
	}
	podName, container, err := access.PostgresPod(ctx, client, instance)
	if err != nil {
		return 0, err
	}
	namespace := instance.Group.Namespace

	s.logger.InfoContext(ctx, "starting pg_dump", "pod", podName, "container", container, "namespace", namespace, "command", strings.Join(redactPgPassword(command), " "))

	// One attempt of the whole pipeline: pg_dump in the pod writing into a pipe that is streamed to
	// S3. Retried as a unit, since a failed attempt leaves a partial object behind that the next
	// upload to the same key replaces.
	attempt := func() (int64, error) {
		pr, pw := io.Pipe()

		type uploadResult struct {
			size int64
			err  error
		}
		uploadDone := make(chan uploadResult, 1)
		go func() {
			defer pr.Close()
			size, err := s.s3Client.StreamUpload(ctx, s.s3Bucket, key, "application/octet-stream", pr)
			uploadDone <- uploadResult{size, err}
		}()

		if err := execPgDump(ctx, client, namespace, podName, container, command, pw, format); err != nil {
			<-uploadDone
			return 0, err
		}

		upload := <-uploadDone
		return upload.size, upload.err
	}

	size, err := s.dumpWithRetries(ctx, attempt)
	if err != nil {
		return 0, fmt.Errorf("failed to exec pg_dump: %w", err)
	}

	s.logger.InfoContext(ctx, "pg_dump completed successfully", "key", key, "size", size)

	return size, nil
}

const (
	pgDumpPlainTrailer = "-- PostgreSQL database dump complete"
	pgDumpTailSize     = 4096
)

// errDumpTruncated marks a dump whose tail did not arrive. A pod exec can report success having
// delivered only part of the command's stdout: when the command exits, whatever the cluster still
// buffers on the way to the client can be dropped.
var errDumpTruncated = errors.New("pg_dump output is truncated")

func execPgDump(ctx context.Context, executor PodExecutor, namespace, podName, container string, command []string, pw *io.PipeWriter, format string) error {
	// Errors have to reach the reader through the pipe, otherwise the upload sees a clean EOF and
	// stores the partial dump as if it were complete.
	fail := func(err error) error {
		pw.CloseWithError(err)
		return err
	}

	var execWriter io.Writer = pw
	var verifier *plainDumpVerifier
	if format == "plain" {
		verifier = newPlainDumpVerifier()
		execWriter = io.MultiWriter(pw, verifier)
	}

	var stderrBuf strings.Builder
	err := executor.Exec(ctx, namespace, podName, container, command, execWriter, &stderrBuf)
	if verifier != nil {
		verifyErr := verifier.close()
		if err == nil && verifyErr != nil {
			return fail(fmt.Errorf("%w, %v: %s", errDumpTruncated, verifyErr, stderrBuf.String()))
		}
	}
	if err != nil {
		return fail(fmt.Errorf("%w: %s", err, stderrBuf.String()))
	}

	return pw.Close()
}

// plainDumpVerifier reads the gzip stream pg_dump produces for a plain dump while it is uploaded
// and tells whether it is whole: the stream has to end as gzip does and the text has to end with
// the line pg_dump writes last.
type plainDumpVerifier struct {
	*storage.GzipVerifier
	tail *tailWriter
}

func newPlainDumpVerifier() *plainDumpVerifier {
	tail := &tailWriter{w: io.Discard, max: pgDumpTailSize}

	return &plainDumpVerifier{GzipVerifier: storage.NewGzipVerifier(tail), tail: tail}
}

func (v *plainDumpVerifier) close() error {
	err := v.Close()
	if err != nil {
		return err
	}
	if !v.tail.contains(pgDumpPlainTrailer) {
		return fmt.Errorf("%q not found in the last %d bytes", pgDumpPlainTrailer, pgDumpTailSize)
	}

	return nil
}

// tailWriter passes writes through while retaining up to max trailing bytes.
type tailWriter struct {
	w    io.Writer
	max  int
	tail []byte
}

func (t *tailWriter) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	if n > 0 {
		t.tail = append(t.tail, p[:n]...)
		if len(t.tail) > t.max {
			t.tail = t.tail[len(t.tail)-t.max:]
		}
	}
	return n, err
}

func (t *tailWriter) contains(s string) bool {
	return bytes.Contains(t.tail, []byte(s))
}

// transientPostgresStates are the messages postgres returns while it is not accepting connections
// yet or no longer is. Bitnami's chart runs a temporary server to execute the initdb scripts, which
// seeding uses, and stops it once they finish; a dump that lands in that transition sees one of
// these. They say nothing about the database being unusable, only that it is mid-transition, so a
// backup should wait rather than fail.
var transientPostgresStates = []string{
	"the database system is shutting down",
	"the database system is starting up",
	"the database system is in recovery mode",
	"connection refused",
}

func isTransientPostgresState(err error) bool {
	message := strings.ToLower(err.Error())
	return slices.ContainsFunc(transientPostgresStates, func(state string) bool {
		return strings.Contains(message, state)
	})
}

const (
	dumpRetryBudget       = 2 * time.Minute
	dumpRetryInterval     = 5 * time.Second
	truncatedDumpAttempts = 3
)

// dumpWithRetries retries the dump while postgres reports a transient state, so a save issued
// moments after a deploy waits for seeding to finish instead of failing with the server's
// mid-restart message. A dump that arrived truncated is retried a few times as well, since the next
// attempt usually arrives whole. Any other error fails immediately: retrying a genuine dump error
// would only delay the report.
func (s Service) dumpWithRetries(ctx context.Context, attempt func() (int64, error)) (int64, error) {
	deadline := time.Now().Add(dumpRetryBudget)
	truncated := 0
	for {
		size, err := attempt()
		if err == nil {
			return size, nil
		}

		if errors.Is(err, errDumpTruncated) {
			truncated++
			if truncated >= truncatedDumpAttempts {
				return 0, err
			}
			s.logger.WarnContext(ctx, "the dump arrived truncated, retrying", "error", err, "attempt", truncated)
			continue
		}

		if !isTransientPostgresState(err) || time.Now().After(deadline) {
			return 0, err
		}

		s.logger.InfoContext(ctx, "postgres is mid-transition, retrying the dump", "error", err, "retryIn", dumpRetryInterval)
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(dumpRetryInterval):
		}
	}
}

func (s Service) logError(ctx context.Context, err error) {
	// TODO: Persist error message
	s.logger.ErrorContext(ctx, "Failed to SaveAs DB", "error", err)
}

func newPgDumpConfig(instance *model.DeploymentInstance, stack *stack.Stack) (*pg.Dump, error) {
	errorMessage := "can't find parameter: %s"

	databaseName, exists := instance.Parameters["DATABASE_NAME"]
	if !exists {
		return nil, fmt.Errorf(errorMessage, "DATABASE_NAME")
	}

	databaseUsername, exists := instance.Parameters["DATABASE_USERNAME"]
	if !exists {
		return nil, fmt.Errorf(errorMessage, "DATABASE_USERNAME")
	}

	databasePassword, exists := instance.Parameters["DATABASE_PASSWORD"]
	if !exists {
		return nil, fmt.Errorf(errorMessage, "DATABASE_PASSWORD")
	}

	// Only build arguments here: pg_dump runs in the database pod. pg.NewDump
	// checks the host PATH, which unnecessarily requires a local PostgreSQL client.
	dump := &pg.Dump{Postgres: &pg.Postgres{
		Host:     fmt.Sprintf(stack.HostnamePattern, instance.Name, instance.Group.Namespace),
		Port:     5432,
		DB:       databaseName.Value,
		Username: databaseUsername.Value,
		Password: databasePassword.Value,
	}}

	// Restores target a freshly created, empty database, so replace the
	// default arguments without the --clean option. The dhis2 chart's seed marker table is chart
	// bookkeeping owned by the superuser: excluding it keeps it out of the catalog artifact and
	// keeps pg_dump from failing to lock it when running as the app user.
	dump.Options = []string{"--no-owner", "--no-acl", "--blob", "--exclude-table=dhis2_chart_seed_complete"}

	// TODO: This is very DHIS2 specific... More stack meta data?
	dump.IgnoreTableData = []string{"analytics*", "_*"}

	return dump, nil
}

func redactPgPassword(command []string) []string {
	redacted := slices.Clone(command)
	for i, arg := range redacted {
		if strings.HasPrefix(arg, "PGPASSWORD=") {
			redacted[i] = "PGPASSWORD=*****"
		}
	}
	return redacted
}

// buildPgDumpCommand builds the pg_dump command run in the database pod. A plain dump is compressed
// there rather than here: the text is many times larger, and the more of it is still on its way
// when pg_dump exits, the more likely the exec loses its tail.
func buildPgDumpCommand(dump *pg.Dump, format string) []string {
	options := dump.Options
	options = append(options, dump.Postgres.Parse()...)
	options = append(options, fmt.Sprintf("-F%s", format))
	if format == "plain" {
		options = append(options, "--compress=6")
	}
	options = append(options, dump.IgnoreTableDataToString()...)

	return append([]string{"env", "PGPASSWORD=" + dump.Password}, kube.Lingering(append([]string{"pg_dump"}, options...)...)...)
}

func (s Service) StreamUpload(ctx context.Context, database model.Database, group *model.Group, body io.Reader, contentType string, contentLength int64) (model.Database, error) {
	ctx = context.WithoutCancel(ctx)

	key := fmt.Sprintf("%s/%s", group.Name, database.Name)
	s.logger.InfoContext(ctx, "Uploading started", "database", database.Name, "group", group.Name)

	start := time.Now()
	size, err := s.s3Client.StreamUpload(ctx, s.s3Bucket, key, contentType, body)
	if err != nil {
		return model.Database{}, err
	}

	s.logger.InfoContext(ctx, "Upload completed", "contentLength", contentLength, "bytesProcessed", size, "duration", time.Since(start))

	database.Url = fmt.Sprintf("s3://%s/%s", s.s3Bucket, key)
	database.Size = size
	err = s.repository.Save(ctx, &database)
	if err != nil {
		return model.Database{}, fmt.Errorf("failed to save database record: %w", err)
	}

	return database, nil
}
