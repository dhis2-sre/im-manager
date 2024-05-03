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
func NewService(logger *slog.Logger, s3Bucket string, s3Client S3Client, groupService groupService, repository Repository) *service {
	return &service{
		logger:       logger,
		s3Bucket:     s3Bucket,
		s3Client:     s3Client,
		groupService: groupService,
		repository:   repository,
	}
}

type groupService interface {
	Find(name string) (*model.Group, error)
}

type service struct {
	logger       *slog.Logger
	s3Bucket     string
	s3Client     S3Client
	groupService groupService
	repository   Repository
}

type Repository interface {
	Create(d *model.Database) error
	Save(d *model.Database) error
	FindById(id uint) (*model.Database, error)
	Lock(databaseId, instanceId, userId uint) (*model.Lock, error)
	Unlock(databaseId uint) error
	Delete(id uint) error
	FindByGroupNames(names []string) ([]model.Database, error)
	Update(d *model.Database) error
	CreateExternalDownload(databaseID uint, expiration uint) (*model.ExternalDownload, error)
	FindExternalDownload(uuid uuid.UUID) (*model.ExternalDownload, error)
	PurgeExternalDownload() error
	FindBySlug(slug string) (*model.Database, error)
	UpdateId(old, new uint) error
}

type S3Client interface {
	Copy(bucket string, source string, destination string) error
	Move(bucket string, source string, destination string) error
	Upload(bucket string, key string, body storage.ReadAtSeeker, size int64) error
	Delete(bucket string, key string) error
	Download(bucket string, key string, dst io.Writer, cb func(contentLength int64)) error
}

func (s service) FindByIdentifier(identifier string) (*model.Database, error) {
	id, err := strconv.ParseUint(identifier, 10, 32)
	if err != nil {
		database, err := s.FindBySlug(identifier)
		if err != nil {
			return nil, err
		}
		return database, nil
	}

	database, err := s.FindById(uint(id))
	if err != nil {
		return nil, err
	}
	return database, nil
}

func (s service) Copy(id uint, d *model.Database, group *model.Group) error {
	source, err := s.FindById(id)
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

	return s.repository.Create(d)
}

func (s service) FindById(id uint) (*model.Database, error) {
	return s.repository.FindById(id)
}

func (s service) FindBySlug(slug string) (*model.Database, error) {
	return s.repository.FindBySlug(slug)
}

func (s service) Lock(databaseId uint, instanceId uint, userId uint) (*model.Lock, error) {
	return s.repository.Lock(databaseId, instanceId, userId)
}

func (s service) Unlock(databaseId uint) error {
	return s.repository.Unlock(databaseId)
}

type ReadAtSeeker interface {
	io.ReaderAt
	io.ReadSeeker
}

func (s service) Upload(d *model.Database, group *model.Group, reader ReadAtSeeker, size int64) (*model.Database, error) {
	key := fmt.Sprintf("%s/%s", group.Name, d.Name)
	err := s.s3Client.Upload(s.s3Bucket, key, reader, size)
	if err != nil {
		return nil, err
	}

	d.Url = fmt.Sprintf("s3://%s/%s", s.s3Bucket, key)

	err = s.repository.Save(d)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (s service) Download(id uint, dst io.Writer, cb func(contentLength int64)) error {
	d, err := s.repository.FindById(id)
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
	return s.s3Client.Download(s.s3Bucket, key, dst, cb)
}

func (s service) Delete(id uint) error {
	d, err := s.repository.FindById(id)
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

	return s.repository.Delete(id)
}

func (s service) List(user *model.User) ([]GroupsWithDatabases, error) {
	groups := append(user.Groups, user.AdminGroups...) //nolint:gocritic
	groupsByName := make(map[string]model.Group)
	for _, group := range groups {
		groupsByName[group.Name] = group
	}
	groupNames := maps.Keys(groupsByName)

	databases, err := s.repository.FindByGroupNames(groupNames)
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

func (s service) Update(d *model.Database) error {
	return s.repository.Update(d)
}

func (s service) CreateExternalDownload(databaseID uint, expiration uint) (*model.ExternalDownload, error) {
	err := s.repository.PurgeExternalDownload()
	if err != nil {
		return nil, err
	}

	return s.repository.CreateExternalDownload(databaseID, expiration)
}

func (s service) FindExternalDownload(uuid uuid.UUID) (*model.ExternalDownload, error) {
	err := s.repository.PurgeExternalDownload()
	if err != nil {
		return nil, err
	}
	return s.repository.FindExternalDownload(uuid)
}

func (s service) Save(userId uint, database *model.Database, instance *model.DeploymentInstance, stack *model.Stack) error {
	lock := database.Lock
	isLocked := lock != nil
	if isLocked && (lock.InstanceID != instance.ID || lock.UserID != userId) {
		return errdef.NewUnauthorized("database is locked")
	}

	if !isLocked {
		_, err := s.Lock(database.ID, instance.ID, userId)
		if err != nil {
			return err
		}

		reloaded, err := s.FindById(database.ID)
		if err != nil {
			return err
		}
		database = reloaded
	}

	tmpName := uuid.New().String()
	format := getFormat(database)
	_, err := s.SaveAs(database, instance, stack, tmpName, format, func(saved *model.Database) {
		defer func() {
			if !isLocked {
				err := s.Unlock(database.ID)
				if err != nil {
					s.logError(fmt.Errorf("unlock database failed: %v", err))
				}
			}
		}()

		u, err := url.Parse(saved.Url)
		if err != nil {
			s.logError(err)
			return
		}

		err = s.Delete(database.ID)
		if err != nil {
			s.logError(err)
			return
		}

		sourceKey := strings.TrimPrefix(u.Path, "/")
		destinationKey := fmt.Sprintf("%s/%s", saved.GroupName, database.Name)
		err = s.s3Client.Move(s.s3Bucket, sourceKey, destinationKey)
		if err != nil {
			s.logError(err)
			return
		}

		saved.Name = database.Name
		saved.Url = database.Url
		saved.Slug = database.Slug
		saved.CreatedAt = database.CreatedAt
		err = s.Update(saved)
		if err != nil {
			s.logError(err)
			return
		}

		err = s.repository.UpdateId(saved.ID, database.ID)
		if err != nil {
			s.logError(err)
			return
		}

		if database.Lock != nil {
			_, err := s.repository.Lock(database.ID, database.Lock.InstanceID, database.Lock.UserID)
			if err != nil {
				s.logError(err)
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

func (s service) SaveAs(database *model.Database, instance *model.DeploymentInstance, stack *model.Stack, newName string, format string, done func(saved *model.Database)) (*model.Database, error) {
	// TODO: Add to config
	dumpPath := "/mnt/data/"

	group, err := s.groupService.Find(instance.GroupName)
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

	err = s.repository.Save(newDatabase)
	if err != nil {
		return nil, err
	}

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
				s.logError(err)
				return
			}

			ret, err = forwarder.WithForwardersEmbedConfig(context.Background(), options, kubeConfig)
			if err != nil {
				s.logError(err)
				return
			}
			defer ret.Close()

			ports, err := ret.Ready()
			if err != nil {
				s.logError(err)
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
			s.logger.Error("Failed to dump DB", "error", dumpExec.Error.Err, "dumpOutput", dumpExec.Output)
			return
		}

		dumpFile := path.Join(dumpPath, dumpExec.File)
		file, err := os.Open(dumpFile) // #nosec
		if err != nil {
			s.logError(err)
			return
		}
		defer s.removeTempFile(file)

		if format == "plain" {
			gzFileName := path.Join(dumpPath, fileId+".gz")
			file, err = s.gz(gzFileName, database, file)
			if err != nil {
				s.logError(err)
				return
			}

			defer s.removeTempFile(file)
		}

		stat, err := file.Stat()
		if err != nil {
			s.logError(err)
			return
		}

		// This is added due to the following issue - https://github.com/aws/aws-sdk-go/issues/1962
		_, err = file.Seek(0, 0)
		if err != nil {
			s.logError(err)
			return
		}

		_, err = s.Upload(newDatabase, group, file, stat.Size())
		if err != nil {
			s.logError(err)
			return
		}

		done(newDatabase)
	}()

	return newDatabase, nil
}

func (s service) logError(err error) {
	// TODO: Persist error message
	s.logger.Error("Failed to SaveAs DB", "error", err)
}

func (s service) removeTempFile(fd *os.File) {
	for _, err := range [...]error{fd.Close(), os.Remove(fd.Name())} {
		if err != nil {
			s.logger.Error("Failed to remove temp file", "error", err)
		}
	}
}

func (s service) gz(gzFile string, database *model.Database, src *os.File) (*os.File, error) {
	outFile, err := os.Create(gzFile) // #nosec
	if err != nil {
		return nil, err
	}

	zw := gzip.NewWriter(outFile)
	zw.Name = strings.TrimSuffix(database.Name, ".gz")

	defer func(zw *gzip.Writer) {
		err := zw.Close()
		if err != nil {
			s.logger.Error("Failed to close gzip writer", "error", err)
		}
	}(zw)

	_, err = io.Copy(zw, src)
	if err != nil {
		return nil, err
	}

	defer func(src *os.File) {
		err := src.Close()
		if err != nil {
			s.logger.Error("Failed to close file", "error", err)
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
