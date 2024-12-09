package database

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"golang.org/x/exp/maps"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage"

	"github.com/anthhub/forwarder"

	pg "github.com/habx/pg-commands"

	"github.com/google/uuid"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewService(logger *slog.Logger, s3Bucket string, s3Client S3Client, groupService groupService, repository *repository) *service {
	return &service{
		logger:       logger,
		s3Bucket:     s3Bucket,
		s3Client:     s3Client,
		groupService: groupService,
		repository:   repository,
	}
}

type groupService interface {
	Find(ctx context.Context, name string) (*model.Group, error)
}

type service struct {
	logger       *slog.Logger
	s3Bucket     string
	s3Client     S3Client
	groupService groupService
	repository   *repository
}

type S3Client interface {
	Copy(bucket string, source string, destination string) error
	Move(bucket string, source string, destination string) error
	Upload(ctx context.Context, bucket string, key string, body storage.ReadAtSeeker, size int64) error
	Delete(bucket string, key string) error
	Download(ctx context.Context, bucket string, key string, dst io.Writer, cb func(contentLength int64)) error
}

func (s service) FindByIdentifier(ctx context.Context, identifier string) (*model.Database, error) {
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

func (s service) Copy(ctx context.Context, id uint, d *model.Database, group *model.Group) error {
	source, err := s.FindById(ctx, id)
	if err != nil {
		return err
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

	return s.repository.Create(ctx, d)
}

func (s service) FindById(ctx context.Context, id uint) (*model.Database, error) {
	return s.repository.FindById(ctx, id)
}

func (s service) FindBySlug(ctx context.Context, slug string) (*model.Database, error) {
	return s.repository.FindBySlug(ctx, slug)
}

func (s service) Lock(ctx context.Context, databaseId uint, instanceId uint, userId uint) (*model.Lock, error) {
	return s.repository.Lock(ctx, databaseId, instanceId, userId)
}

func (s service) Unlock(ctx context.Context, databaseId uint) error {
	return s.repository.Unlock(ctx, databaseId)
}

type ReadAtSeeker interface {
	io.ReaderAt
	io.ReadSeeker
}

func (s service) Upload(ctx context.Context, d *model.Database, group *model.Group, reader ReadAtSeeker, size int64) (*model.Database, error) {
	key := fmt.Sprintf("%s/%s", group.Name, d.Name)
	err := s.s3Client.Upload(ctx, s.s3Bucket, key, reader, size)
	if err != nil {
		return nil, err
	}

	d.Url = fmt.Sprintf("s3://%s/%s", s.s3Bucket, key)

	err = s.repository.Save(ctx, d)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (s service) Download(ctx context.Context, id uint, dst io.Writer, cb func(contentLength int64)) error {
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

func (s service) Delete(ctx context.Context, id uint) error {
	d, err := s.repository.FindById(ctx, id)
	if err != nil {
		return err
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

	return s.repository.Delete(ctx, id)
}

func (s service) List(ctx context.Context, user *model.User) ([]GroupsWithDatabases, error) {
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
	}
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

func (s service) Update(ctx context.Context, d *model.Database) error {
	return s.repository.Update(ctx, d)
}

func (s service) CreateExternalDownload(ctx context.Context, databaseID uint, expiration uint) (*model.ExternalDownload, error) {
	err := s.repository.PurgeExternalDownload(ctx)
	if err != nil {
		return nil, err
	}

	return s.repository.CreateExternalDownload(ctx, databaseID, expiration)
}

func (s service) FindExternalDownload(ctx context.Context, uuid uuid.UUID) (*model.ExternalDownload, error) {
	err := s.repository.PurgeExternalDownload(ctx)
	if err != nil {
		return nil, err
	}
	return s.repository.FindExternalDownload(ctx, uuid)
}

func (s service) Save(ctx context.Context, userId uint, database *model.Database, instance *model.DeploymentInstance, stack *model.Stack) error {
	lock := database.Lock
	isLocked := lock != nil
	if isLocked && (lock.InstanceID != instance.ID || lock.UserID != userId) {
		return errdef.NewUnauthorized("database is locked")
	}

	if !isLocked {
		_, err := s.Lock(ctx, database.ID, instance.ID, userId)
		if err != nil {
			return err
		}

		reloaded, err := s.FindById(ctx, database.ID)
		if err != nil {
			return err
		}
		database = reloaded
	}

	tmpName := uuid.New().String()
	format := getFormat(database)
	_, err := s.SaveAs(ctx, database, instance, stack, tmpName, format, func(ctx context.Context, saved *model.Database) {
		defer func() {
			if !isLocked {
				err := s.Unlock(ctx, database.ID)
				if err != nil {
					s.logError(ctx, fmt.Errorf("unlock database failed: %v", err))
				}
			}
		}()

		u, err := url.Parse(saved.Url)
		if err != nil {
			s.logError(ctx, err)
			return
		}

		err = s.Delete(ctx, database.ID)
		if err != nil {
			s.logError(ctx, err)
			return
		}

		sourceKey := strings.TrimPrefix(u.Path, "/")
		destinationKey := fmt.Sprintf("%s/%s", saved.GroupName, database.Name)
		err = s.s3Client.Move(s.s3Bucket, sourceKey, destinationKey)
		if err != nil {
			s.logError(ctx, err)
			return
		}

		saved.Name = database.Name
		saved.Url = database.Url
		saved.Slug = database.Slug
		saved.CreatedAt = database.CreatedAt
		err = s.Update(ctx, saved)
		if err != nil {
			s.logError(ctx, err)
			return
		}

		err = s.repository.UpdateId(ctx, saved.ID, database.ID)
		if err != nil {
			s.logError(ctx, err)
			return
		}

		if database.Lock != nil {
			_, err := s.repository.Lock(ctx, database.ID, database.Lock.InstanceID, database.Lock.UserID)
			if err != nil {
				s.logError(ctx, err)
				return
			}
		}
	})

	return err
}

func getFormat(database *model.Database) string {
	if strings.HasSuffix(database.Url, ".pgc") {
		return "custom"
	}
	return "plain"
}

func (s service) SaveAs(ctx context.Context, database *model.Database, instance *model.DeploymentInstance, stack *model.Stack, newName string, format string, done func(ctx context.Context, saved *model.Database)) (*model.Database, error) {
	// TODO: Add to config
	dumpPath := "/mnt/data/"

	group, err := s.groupService.Find(ctx, instance.GroupName)
	if err != nil {
		return nil, err
	}

	dump, err := newPgDumpConfig(instance, stack)
	if err != nil {
		return nil, err
	}

	newDatabase := &model.Database{
		Name: newName,
		// TODO: For now, only saving to the same group is supported
		GroupName: instance.GroupName,
	}

	err = s.repository.Save(ctx, newDatabase)
	if err != nil {
		return nil, err
	}

	// only use ctx for values (logging) and not cancellation signals since we create a go routine
	// that outlives the HTTP request scope
	ctx = context.WithoutCancel(ctx)
	go func() {
		var ret *forwarder.Result
		if group.ClusterConfiguration != nil && len(group.ClusterConfiguration.KubernetesConfiguration) > 0 {
			hostname := fmt.Sprintf(stack.HostnamePattern, instance.Name, instance.GroupName)
			serviceName := strings.Split(hostname, ".")[0]
			options := []*forwarder.Option{
				{
					RemotePort:  5432,
					ServiceName: serviceName,
					Namespace:   instance.GroupName,
				},
			}

			kubeConfig, err := decryptYaml(group.ClusterConfiguration.KubernetesConfiguration)
			if err != nil {
				s.logError(ctx, err)
				return
			}

			ret, err = forwarder.WithForwardersEmbedConfig(context.Background(), options, kubeConfig)
			if err != nil {
				s.logError(ctx, err)
				return
			}
			defer ret.Close()

			ports, err := ret.Ready()
			if err != nil {
				s.logError(ctx, err)
				return
			}

			dump.Host = "localhost"
			dump.Port = int(ports[0][0].Local)
		}

		dump.SetPath(dumpPath)
		fileId := uuid.New().String()
		dump.SetFileName(fileId + ".dump")
		dump.SetupFormat(format)

		// TODO: Remove... Or at least make configurable
		dump.EnableVerbose()

		dumpExec := dump.Exec(pg.ExecOptions{StreamPrint: true, StreamDestination: os.Stdout})
		if dumpExec.Error != nil {
			s.logger.ErrorContext(ctx, "Failed to dump DB", "error", dumpExec.Error.Err, "dumpOutput", dumpExec.Output)
			return
		}

		dumpFile := path.Join(dumpPath, dumpExec.File)
		file, err := os.Open(dumpFile) // #nosec
		if err != nil {
			s.logError(ctx, err)
			return
		}
		defer s.removeTempFile(ctx, file)

		if format == "plain" {
			gzFileName := path.Join(dumpPath, fileId+".gz")
			file, err = s.gz(ctx, gzFileName, database, file)
			if err != nil {
				s.logError(ctx, err)
				return
			}

			defer s.removeTempFile(ctx, file)
		}

		stat, err := file.Stat()
		if err != nil {
			s.logError(ctx, err)
			return
		}

		// This is added due to the following issue - https://github.com/aws/aws-sdk-go/issues/1962
		_, err = file.Seek(0, 0)
		if err != nil {
			s.logError(ctx, err)
			return
		}

		_, err = s.Upload(ctx, newDatabase, group, file, stat.Size())
		if err != nil {
			s.logError(ctx, err)
			return
		}

		done(ctx, newDatabase)
	}()

	return newDatabase, nil
}

func (s service) logError(ctx context.Context, err error) {
	// TODO: Persist error message
	s.logger.ErrorContext(ctx, "Failed to SaveAs DB", "error", err)
}

func (s service) removeTempFile(ctx context.Context, fd *os.File) {
	for _, err := range [...]error{fd.Close(), os.Remove(fd.Name())} {
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to remove temp file", "error", err)
		}
	}
}

func (s service) gz(ctx context.Context, gzFile string, database *model.Database, src *os.File) (*os.File, error) {
	outFile, err := os.Create(gzFile) // #nosec
	if err != nil {
		return nil, err
	}

	zw := gzip.NewWriter(outFile)
	zw.Name = strings.TrimSuffix(database.Name, ".gz")

	defer func(zw *gzip.Writer) {
		err := zw.Close()
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to close gzip writer", "error", err)
		}
	}(zw)

	_, err = io.Copy(zw, src)
	if err != nil {
		return nil, err
	}

	defer func(src *os.File) {
		err := src.Close()
		if err != nil {
			s.logger.ErrorContext(ctx, "Failed to close file", "error", err)
		}
	}(src)

	return outFile, nil
}

func newPgDumpConfig(instance *model.DeploymentInstance, stack *model.Stack) (*pg.Dump, error) {
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

	dump, err := pg.NewDump(&pg.Postgres{
		Host:     fmt.Sprintf(stack.HostnamePattern, instance.Name, instance.GroupName),
		Port:     5432,
		DB:       databaseName.Value,
		Username: databaseUsername.Value,
		Password: databasePassword.Value,
	})
	if err != nil {
		return nil, err
	}

	// TODO: This is very DHIS2 specific... More stack meta data?
	dump.IgnoreTableData = []string{"analytics*", "_*"}

	return dump, nil
}
