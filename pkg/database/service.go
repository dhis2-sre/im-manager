package database

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/config"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/storage"

	"github.com/anthhub/forwarder"

	pg "github.com/habx/pg-commands"

	"github.com/google/uuid"
)

func NewService(c config.Config, instanceService instance.Service, groupService groupService, s3Client S3Client, repository Repository) *service {
	return &service{c, instanceService, groupService, s3Client, repository}
}

type groupService interface {
	Find(name string) (*model.Group, error)
}

type service struct {
	c               config.Config
	instanceService instance.Service
	groupService    groupService
	s3Client        S3Client
	repository      Repository
}

type Repository interface {
	Create(d *model.Database) error
	Save(d *model.Database) error
	FindById(id uint) (*model.Database, error)
	Lock(id, instanceId, userId uint) (*model.Lock, error)
	Unlock(id uint) error
	Delete(id uint) error
	FindByGroupNames(names []string) ([]model.Database, error)
	Update(d *model.Database) error
	CreateExternalDownload(databaseID uint, expiration time.Time) (*model.ExternalDownload, error)
	FindExternalDownload(uuid uuid.UUID) (*model.ExternalDownload, error)
	PurgeExternalDownload() error
	FindBySlug(slug string) (*model.Database, error)
	UpdateId(old, new uint) error
}

type S3Client interface {
	Copy(bucket string, source string, destination string) error
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
	err = s.s3Client.Copy(s.c.Bucket, sourceKey, destinationKey)
	if err != nil {
		return err
	}

	d.Url = fmt.Sprintf("s3://%s/%s", s.c.Bucket, destinationKey)

	return s.repository.Create(d)
}

func (s service) FindById(id uint) (*model.Database, error) {
	return s.repository.FindById(id)
}

func (s service) FindBySlug(slug string) (*model.Database, error) {
	return s.repository.FindBySlug(slug)
}

func (s service) Lock(id uint, instanceId uint, userId uint) (*model.Lock, error) {
	return s.repository.Lock(id, instanceId, userId)
}

func (s service) Unlock(id uint) error {
	return s.repository.Unlock(id)
}

type ReadAtSeeker interface {
	io.ReaderAt
	io.ReadSeeker
}

func (s service) Upload(d *model.Database, group *model.Group, reader ReadAtSeeker, size int64) (*model.Database, error) {
	key := fmt.Sprintf("%s/%s", group.Name, d.Name)
	err := s.s3Client.Upload(s.c.Bucket, key, reader, size)
	if err != nil {
		return nil, err
	}

	d.Url = fmt.Sprintf("s3://%s/%s", s.c.Bucket, key)

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
	return s.s3Client.Download(s.c.Bucket, key, dst, cb)
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
		err = s.s3Client.Delete(s.c.Bucket, key)
		if err != nil {
			return err
		}
	}

	return s.repository.Delete(id)
}

func (s service) List(groups []model.Group) ([]model.Database, error) {
	groupNames := make([]string, len(groups))
	for i, group := range groups {
		groupNames[i] = group.Name
	}

	instances, err := s.repository.FindByGroupNames(groupNames)
	if err != nil {
		return nil, err
	}
	return instances, nil
}

func (s service) Update(d *model.Database) error {
	return s.repository.Update(d)
}

func (s service) CreateExternalDownload(databaseID uint, expiration time.Time) (*model.ExternalDownload, error) {
	err := s.repository.PurgeExternalDownload()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if expiration.Before(now) {
		return nil, fmt.Errorf("expiration %s needs to be in the future (current %s)", expiration, now)
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

func (s service) Save(userId uint, database *model.Database, instance *model.Instance, stack *model.Stack) error {
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
					logError(fmt.Errorf("unlock database failed: %v", err))
				}
			}
		}()

		u, err := url.Parse(saved.Url)
		if err != nil {
			logError(err)
			return
		}

		err = s.Delete(database.ID)
		if err != nil {
			logError(err)
			return
		}

		sourceKey := strings.TrimPrefix(u.Path, "/")
		destinationKey := fmt.Sprintf("%s/%s", saved.GroupName, database.Name)
		err = s.s3Client.Copy(s.c.Bucket, sourceKey, destinationKey)
		if err != nil {
			logError(err)
			return
		}

		err = s.s3Client.Delete(s.c.Bucket, saved.Url)
		if err != nil {
			logError(err)
			return
		}

		saved.Name = database.Name
		saved.Url = database.Url
		saved.Slug = database.Slug
		err = s.Update(saved)
		if err != nil {
			logError(err)
			return
		}

		err = s.repository.UpdateId(saved.ID, database.ID)
		if err != nil {
			logError(err)
			return
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

func (s service) SaveAs(database *model.Database, instance *model.Instance, stack *model.Stack, newName string, format string, done func(saved *model.Database)) (*model.Database, error) {
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
				logError(err)
				return
			}

			ret, err = forwarder.WithForwardersEmbedConfig(context.Background(), options, kubeConfig)
			if err != nil {
				logError(err)
				return
			}
			defer ret.Close()

			ports, err := ret.Ready()
			if err != nil {
				logError(err)
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
			log.Println(dumpExec.Error.Err)
			log.Println(dumpExec.Output)
			logError(dumpExec.Error.Err)
			return
		}

		dumpFile := path.Join(dumpPath, dumpExec.File)
		file, err := os.Open(dumpFile) // #nosec
		if err != nil {
			logError(err)
			return
		}
		defer removeTempFile(file)

		if format == "plain" {
			gzFileName := path.Join(dumpPath, fileId+".gz")
			file, err = gz(gzFileName, database, file)
			if err != nil {
				logError(err)
				return
			}

			defer removeTempFile(file)
		}

		stat, err := file.Stat()
		if err != nil {
			logError(err)
			return
		}

		// This is added due to the following issue - https://github.com/aws/aws-sdk-go/issues/1962
		_, err = file.Seek(0, 0)
		if err != nil {
			logError(err)
			return
		}

		_, err = s.Upload(newDatabase, group, file, stat.Size())
		if err != nil {
			logError(err)
			return
		}

		done(newDatabase)
	}()

	return newDatabase, nil
}

func logError(err error) {
	// TODO: Persist error message
	log.Println(err)
}

func removeTempFile(fd *os.File) {
	for _, err := range [...]error{fd.Close(), os.Remove(fd.Name())} {
		if err != nil {
			log.Println("failed to remove temp file:", err)
		}
	}
}

func gz(gzFile string, database *model.Database, src *os.File) (*os.File, error) {
	outFile, err := os.Create(gzFile) // #nosec
	if err != nil {
		return nil, err
	}

	zw := gzip.NewWriter(outFile)
	zw.Name = strings.TrimSuffix(database.Name, ".gz")

	defer func(zw *gzip.Writer) {
		err := zw.Close()
		if err != nil {
			log.Println(err)
		}
	}(zw)

	_, err = io.Copy(zw, src)
	if err != nil {
		return nil, err
	}

	defer func(src *os.File) {
		err := src.Close()
		if err != nil {
			log.Println(err)
		}
	}(src)

	return outFile, nil
}

func newPgDumpConfig(instance *model.Instance, stack *model.Stack) (*pg.Dump, error) {
	databaseName, err := findParameter("DATABASE_NAME", instance, stack)
	if err != nil {
		return nil, err
	}

	databaseUsername, err := findParameter("DATABASE_USERNAME", instance, stack)
	if err != nil {
		return nil, err
	}

	databasePassword, err := findParameter("DATABASE_PASSWORD", instance, stack)
	if err != nil {
		return nil, err
	}

	dump, err := pg.NewDump(&pg.Postgres{
		Host:     fmt.Sprintf(stack.HostnamePattern, instance.Name, instance.GroupName),
		Port:     5432,
		DB:       databaseName,
		Username: databaseUsername,
		Password: databasePassword,
	})
	if err != nil {
		return nil, err
	}

	// TODO: This is very DHIS2 specific... More stack meta data?
	dump.IgnoreTableData = []string{"analytics*", "_*"}

	return dump, nil
}

func findParameter(parameter string, instance *model.Instance, stack *model.Stack) (string, error) {
	for _, p := range instance.RequiredParameters {
		if p.StackRequiredParameterID == parameter {
			return p.Value, nil
		}
	}

	for _, p := range instance.OptionalParameters {
		if p.StackOptionalParameterID == parameter {
			return p.Value, nil
		}
	}

	for _, p := range stack.OptionalParameters {
		if p.Name == parameter {
			return p.DefaultValue, nil
		}
	}

	return "", errors.New("parameter not found: " + parameter)
}
