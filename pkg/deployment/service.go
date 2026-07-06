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

func NewService(logger *slog.Logger, instanceService instanceService, databaseService databaseService, tokenService *token.TokenService) *Service {
	return &Service{
		logger:          logger,
		instanceService: instanceService,
		databaseService: databaseService,
		tokenService:    tokenService,
	}
}

type instanceService interface {
	DeploymentOrder(deployment *model.Deployment) ([]*model.DeploymentInstance, error)
	DeployInstance(ctx context.Context, token string, instance *model.DeploymentInstance, ttl uint, extraEnv map[string]string, filestoreBackup *model.Database) error
	DestroyInstance(ctx context.Context, instance *model.DeploymentInstance) error
	FindDeploymentById(ctx context.Context, id uint) (*model.Deployment, error)
	FindDecryptedDeploymentById(ctx context.Context, id uint) (*model.Deployment, error)
	FindDecryptedDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error)
	SaveDeployment(ctx context.Context, deployment *model.Deployment) error
	UpdateInstanceParameters(ctx context.Context, deploymentId, instanceId uint, parameters instance.Parameters, public *bool) (*model.DeploymentInstance, error)
}

type databaseService interface {
	FindById(ctx context.Context, id uint) (*model.Database, error)
	CreateExternalDownload(ctx context.Context, databaseID uint, expiration uint) (*model.ExternalDownload, error)
}

type Service struct {
	logger          *slog.Logger
	instanceService instanceService
	databaseService databaseService
	tokenService    *token.TokenService
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
		err = s.deployInstance(ctx, token, instance, deployment.TTL)
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

	return s.deployInstance(ctx, token, instance, ttl)
}

func (s Service) UpdateInstance(ctx context.Context, token string, deploymentId, instanceId uint, parameters instance.Parameters, public *bool) (*model.DeploymentInstance, error) {
	updated, err := s.instanceService.UpdateInstanceParameters(ctx, deploymentId, instanceId, parameters, public)
	if err != nil {
		return nil, err
	}

	deployment, err := s.instanceService.FindDeploymentById(ctx, deploymentId)
	if err != nil {
		return nil, err
	}

	decryptedInstance, err := s.instanceService.FindDecryptedDeploymentInstanceById(ctx, instanceId)
	if err != nil {
		return nil, err
	}

	refreshedToken, err := s.tokenService.RefreshAccessToken(token)
	if err != nil {
		return nil, err
	}

	err = s.deployInstance(ctx, refreshedToken, decryptedInstance, deployment.TTL)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy updated instance: %v", err)
	}

	return updated, nil
}

func (s Service) deployInstance(ctx context.Context, token string, instance *model.DeploymentInstance, ttl uint) error {
	extraEnv, filestoreBackup, err := s.buildSeed(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to build seed environment: %w", err)
	}

	return s.instanceService.DeployInstance(ctx, token, instance, ttl, extraEnv, filestoreBackup)
}

const seedDownloadTTLSeconds uint = 1800

// buildSeed resolves the database referenced by the instance's DATABASE_ID parameter into the
// environment variables and filestore backup record needed to seed the instance at deploy time.
func (s Service) buildSeed(ctx context.Context, instance *model.DeploymentInstance) (map[string]string, *model.Database, error) {
	param, ok := instance.Parameters["DATABASE_ID"]
	if !ok {
		return nil, nil, nil
	}

	databaseID, err := strconv.ParseUint(param.Value, 10, strconv.IntSize)
	if err != nil || databaseID == 0 {
		return nil, nil, nil
	}

	hostname := os.Getenv("HOSTNAME")
	extraEnv := make(map[string]string)

	db, err := s.databaseService.FindById(ctx, uint(databaseID))
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
