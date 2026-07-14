package deployment

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/token"
)

func NewService(logger *slog.Logger, instanceService instanceService, databaseService databaseService, tokenService *token.TokenService, publisher Publisher) *Service {
	return &Service{
		logger:          logger,
		instanceService: instanceService,
		databaseService: databaseService,
		tokenService:    tokenService,
		publisher:       publisher,
	}
}

type instanceService interface {
	DeploymentOrder(deployment *model.Deployment) ([]*model.DeploymentInstance, error)
	DeployInstance(ctx context.Context, token string, instance *model.DeploymentInstance, ttl uint, extraEnv map[string]string, filestoreBackup *model.Database) error
	DestroyInstance(ctx context.Context, instance *model.DeploymentInstance) error
	FindDecryptedDeploymentById(ctx context.Context, id uint) (*model.Deployment, error)
	SaveDeployment(ctx context.Context, deployment *model.Deployment) error
	UpdateInstanceParameters(ctx context.Context, deploymentId, instanceId uint, parameters instance.Parameters, public *bool) (*model.DeploymentInstance, error)
	FilestoreBackup(ctx context.Context, instance *model.DeploymentInstance, name string, database *model.Database) error
}

type databaseService interface {
	FindById(ctx context.Context, id uint) (*model.Database, error)
	CreateExternalDownload(ctx context.Context, databaseID uint, expiration uint) (*model.ExternalDownload, error)
	CreateDatabase(ctx context.Context, userId uint, groupName, name string) (*model.Database, error)
	Dump(ctx context.Context, userId uint, database *model.Database, instance *model.DeploymentInstance, stack *model.Stack, format string) (*model.Database, error)
	EnsureLocked(ctx context.Context, database *model.Database, instanceId, userId uint) (*model.Database, bool, error)
	SaveLocked(ctx context.Context, database *model.Database, instance *model.DeploymentInstance, stack *model.Stack, wasLocked bool) (*model.Database, error)
}

// Publisher publishes notifications for async cross-service operations.
type Publisher interface {
	Publish(ctx context.Context, userID uint, groupName, kind string, payload any)
}

type Service struct {
	logger          *slog.Logger
	instanceService instanceService
	databaseService databaseService
	tokenService    *token.TokenService
	publisher       Publisher
}

func (s Service) DeployDeployment(ctx context.Context, token string, deployment *model.Deployment) error {
	instances, err := s.instanceService.DeploymentOrder(deployment)
	if err != nil {
		return err
	}

	deployment.Instances = instances

	for _, instance := range instances {
		var err error
		token, err = s.tokenService.RefreshAccessToken(token)
		if err != nil {
			return err
		}
		err = s.deployInstance(ctx, token, instance, deployment.TTL, deployment.Instances)
		if err != nil {
			return fmt.Errorf("failed to deploy instance(%s) %q: %w", instance.StackName, instance.Name, err)
		}
	}

	return nil
}

func (s Service) UpdateDeployment(ctx context.Context, token string, deploymentId uint, ttl uint, description string) (*model.Deployment, error) {
	deployment, err := s.instanceService.FindDecryptedDeploymentById(ctx, deploymentId)
	if err != nil {
		return nil, err
	}

	ttlChanged := deployment.TTL != ttl

	deployment.TTL = ttl
	deployment.Description = description

	err = s.instanceService.SaveDeployment(ctx, deployment)
	if err != nil {
		return nil, err
	}

	if ttlChanged {
		err = s.DeployDeployment(ctx, token, deployment)
		if err != nil {
			return nil, fmt.Errorf("failed to redeploy instances: %v", err)
		}
	}

	return deployment, nil
}

func (s Service) Reset(ctx context.Context, token string, instance *model.DeploymentInstance, ttl uint) error {
	err := s.instanceService.DestroyInstance(ctx, instance)
	if err != nil {
		return err
	}

	deployment, err := s.instanceService.FindDecryptedDeploymentById(ctx, instance.DeploymentID)
	if err != nil {
		return err
	}

	return s.deployInstance(ctx, token, instance, ttl, deployment.Instances)
}

func (s Service) UpdateInstance(ctx context.Context, token string, deploymentId, instanceId uint, parameters instance.Parameters, public *bool) (*model.DeploymentInstance, error) {
	updated, err := s.instanceService.UpdateInstanceParameters(ctx, deploymentId, instanceId, parameters, public)
	if err != nil {
		return nil, err
	}

	deployment, err := s.instanceService.FindDecryptedDeploymentById(ctx, deploymentId)
	if err != nil {
		return nil, err
	}

	decryptedInstance, err := findInstanceById(deployment.Instances, instanceId)
	if err != nil {
		return nil, err
	}

	refreshedToken, err := s.tokenService.RefreshAccessToken(token)
	if err != nil {
		return nil, err
	}

	err = s.deployInstance(ctx, refreshedToken, decryptedInstance, deployment.TTL, deployment.Instances)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy updated instance: %v", err)
	}

	return updated, nil
}

func findInstanceById(instances []*model.DeploymentInstance, id uint) (*model.DeploymentInstance, error) {
	for _, instance := range instances {
		if instance.ID == id {
			return instance, nil
		}
	}
	return nil, fmt.Errorf("instance %d not found in deployment", id)
}

// SaveAs dumps the instance's database into a new record. The record is returned right away
// while the dump and the filestore backup of the dhis2-core sibling run in the background.
func (s Service) SaveAs(ctx context.Context, userId uint, instance *model.DeploymentInstance, stack *model.Stack, coreInstance *model.DeploymentInstance, name string, format string) (*model.Database, error) {
	created, err := s.databaseService.CreateDatabase(ctx, userId, instance.GroupName, name)
	if err != nil {
		return nil, err
	}

	// Detach from the request context so the dump and backup aren't cancelled when the
	// HTTP response is sent.
	ctx = context.WithoutCancel(ctx)
	go func() {
		dumped, err := s.databaseService.Dump(ctx, userId, created, instance, stack, format)
		if err != nil {
			return
		}
		s.saveFilestore(ctx, userId, coreInstance, dumped)
	}()

	return created, nil
}

// Save overwrites the instance's source database with a fresh dump. The lock check runs before
// returning; the dump, finalization and filestore backup run in the background.
func (s Service) Save(ctx context.Context, userId uint, database *model.Database, instance *model.DeploymentInstance, stack *model.Stack, coreInstance *model.DeploymentInstance) error {
	locked, wasLocked, err := s.databaseService.EnsureLocked(ctx, database, instance.ID, userId)
	if err != nil {
		return err
	}

	// Detach from the request context so the dump and backup aren't cancelled when the
	// HTTP response is sent.
	ctx = context.WithoutCancel(ctx)
	go func() {
		saved, err := s.databaseService.SaveLocked(ctx, locked, instance, stack, wasLocked)
		if err != nil {
			s.logger.ErrorContext(ctx, "save database failed", "databaseName", locked.Name, "error", err)
			return
		}
		s.saveFilestore(ctx, locked.UserID, coreInstance, saved)
	}()

	return nil
}

func (s Service) saveFilestore(ctx context.Context, userId uint, coreInstance *model.DeploymentInstance, database *model.Database) {
	if coreInstance == nil {
		return
	}

	s.publisher.Publish(ctx, userId, database.GroupName, kindFilestoreBackup, newFilestoreEvent(database, "started", ""))
	if err := s.instanceService.FilestoreBackup(ctx, coreInstance, database.Name, database); err != nil {
		s.logger.ErrorContext(ctx, "filestore backup failed", "groupName", database.GroupName, "databaseName", database.Name, "error", err)
		s.publisher.Publish(ctx, userId, database.GroupName, kindFilestoreBackup, newFilestoreEvent(database, "error", err.Error()))
		return
	}
	s.publisher.Publish(ctx, userId, database.GroupName, kindFilestoreBackup, newFilestoreEvent(database, "success", ""))
}

func (s Service) deployInstance(ctx context.Context, token string, instance *model.DeploymentInstance, ttl uint, instances []*model.DeploymentInstance) error {
	extraEnv, filestoreBackup, err := s.buildSeed(ctx, instances)
	if err != nil {
		return fmt.Errorf("failed to build seed environment: %w", err)
	}

	return s.instanceService.DeployInstance(ctx, token, instance, ttl, extraEnv, filestoreBackup)
}

const seedDownloadTTLSeconds uint = 1800

// databaseIDFromInstances resolves the DATABASE_ID parameter from whichever instance in the
// deployment carries it. DATABASE_ID lives on the db instance, while storage parameters live on
// the core instance, so callers operating on the core must look across siblings to find it.
func databaseIDFromInstances(instances []*model.DeploymentInstance) (uint, bool) {
	for _, instance := range instances {
		param, ok := instance.Parameters["DATABASE_ID"]
		if !ok {
			continue
		}

		databaseID, err := strconv.ParseUint(param.Value, 10, strconv.IntSize)
		if err != nil || databaseID == 0 {
			continue
		}

		return uint(databaseID), true
	}
	return 0, false
}

// buildSeed resolves the database referenced by the deployment's DATABASE_ID parameter into the
// environment variables and filestore backup record needed to seed an instance at deploy time.
func (s Service) buildSeed(ctx context.Context, instances []*model.DeploymentInstance) (map[string]string, *model.Database, error) {
	databaseID, ok := databaseIDFromInstances(instances)
	if !ok {
		return nil, nil, nil
	}

	hostname := os.Getenv("HOSTNAME")
	extraEnv := make(map[string]string)

	db, err := s.databaseService.FindById(ctx, databaseID)
	if err != nil {
		return nil, nil, fmt.Errorf("database %d not found: %w", databaseID, err)
	}

	dbDownload, err := s.databaseService.CreateExternalDownload(ctx, db.ID, seedDownloadTTLSeconds)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create seed download link for database %d: %w", db.ID, err)
	}
	extraEnv["DATABASE_DOWNLOAD_URL"] = hostname + "/databases/external/" + dbDownload.UUID.String()

	var filestore *model.Database
	if db.FilestoreID != 0 {
		fsDownload, err := s.databaseService.CreateExternalDownload(ctx, db.FilestoreID, seedDownloadTTLSeconds)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create seed download link for filestore %d: %w", db.FilestoreID, err)
		}
		extraEnv["FILESTORE_DOWNLOAD_URL"] = hostname + "/databases/external/" + fsDownload.UUID.String()

		filestore, err = s.databaseService.FindById(ctx, db.FilestoreID)
		if err != nil {
			return nil, nil, fmt.Errorf("filestore %d not found: %w", db.FilestoreID, err)
		}
	}

	return extraEnv, filestore, nil
}
