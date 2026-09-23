package deployment

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/dhis2-sre/im-manager/internal/errdef"

	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-manager/pkg/stack"
)

func NewService(logger *slog.Logger, instanceService instanceService, databaseService databaseService, tokenService tokenService, publisher Publisher) *Service {
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
	EditDeployment(ctx context.Context, deploymentId uint, edit instance.Edit) (*instance.DeploymentChanges, error)
	DeleteDestroyedInstance(ctx context.Context, deploymentInstance *model.DeploymentInstance) error
	FilestoreBackup(ctx context.Context, instance *model.DeploymentInstance, name string, database *model.Database) error
	AcquireDeployLock(ctx context.Context, deploymentId uint) (bool, error)
	ReleaseDeployLock(ctx context.Context, deploymentId uint) error
	SetDeployStatus(ctx context.Context, instance *model.DeploymentInstance, status model.DeployStatus) error
}

type databaseService interface {
	FindById(ctx context.Context, id uint) (*model.Database, error)
	FindByIdentifier(ctx context.Context, identifier string) (*model.Database, error)
	CreateExternalDownload(ctx context.Context, databaseID uint, expiration uint) (*model.ExternalDownload, error)
	CreateDatabase(ctx context.Context, userId uint, groupName, name string) (*model.Database, error)
	Dump(ctx context.Context, userId uint, database *model.Database, instance *model.DeploymentInstance, stack *stack.Stack, format string) (*model.Database, error)
	EnsureLocked(ctx context.Context, database *model.Database, instanceId, userId uint) (*model.Database, bool, error)
	SaveLocked(ctx context.Context, database *model.Database, instance *model.DeploymentInstance, stack *stack.Stack, wasLocked bool) (*model.Database, error)
}

// tokenService mints the access token a deploy carries into the cluster. A deploy refreshes it
// between instances so it outlives the request that started it.
type tokenService interface {
	RefreshAccessToken(accessToken string) (string, error)
}

// Publisher publishes notifications for async cross-service operations.
type Publisher interface {
	Publish(ctx context.Context, userID uint, groupName, kind string, payload any)
	PublishTransient(ctx context.Context, groupName, kind string, payload any)
}

type Service struct {
	logger          *slog.Logger
	instanceService instanceService
	databaseService databaseService
	tokenService    tokenService
	publisher       Publisher
}

// StartDeployment accepts a deploy and runs it in the background. The deployment's deploy lock is
// taken before returning, so a caller that gets no error knows the work is theirs and a second
// caller is refused rather than racing helm. Progress arrives as events.
//
// It takes an id rather than a deployment because the background run outlives the request: loading
// its own copy is what keeps the caller free to serialise, and strip the sensitive values from, the
// deployment it holds while the deploy is reading parameters out of its own.
func (s Service) StartDeployment(ctx context.Context, token string, deploymentId uint, userID uint) error {
	acquired, err := s.instanceService.AcquireDeployLock(ctx, deploymentId)
	if err != nil {
		return err
	}
	if !acquired {
		return errdef.NewConflict("deployment %d is already being deployed", deploymentId)
	}

	deployment, err := s.instanceService.FindDecryptedDeploymentById(ctx, deploymentId)
	if err != nil {
		s.releaseDeployLock(ctx, deploymentId)
		return err
	}

	instances, err := s.instanceService.DeploymentOrder(deployment)
	if err != nil {
		s.releaseDeployLock(ctx, deployment.ID)
		return err
	}
	deployment.Instances = instances

	// The deploy outlives the request, so the token has to be minted while the caller's is still
	// valid. Each instance then refreshes from the previous one, as a synchronous deploy did.
	token, err = s.tokenService.RefreshAccessToken(token)
	if err != nil {
		s.releaseDeployLock(ctx, deployment.ID)
		return err
	}

	for _, deploymentInstance := range instances {
		if err := s.instanceService.SetDeployStatus(ctx, deploymentInstance, model.DeployStatusPending); err != nil {
			s.releaseDeployLock(ctx, deployment.ID)
			return err
		}
	}

	ctx = context.WithoutCancel(ctx)
	go func() {
		defer s.releaseDeployLock(ctx, deployment.ID)

		if err := s.deployDeployment(ctx, token, deployment, userID); err != nil {
			s.logger.ErrorContext(ctx, "deploy failed", "deploymentId", deployment.ID, "deploymentName", deployment.Name, "error", err)
			s.publisher.Publish(ctx, userID, deployment.GroupName, kindDeployment, newDeploymentEvent(deployment, "error", err.Error()))
			return
		}
		s.publisher.Publish(ctx, userID, deployment.GroupName, kindDeployment, newDeploymentEvent(deployment, "success", ""))
	}()

	return nil
}

// EditDeployment applies an edit to a deployment and runs whatever cluster work it implies in the
// background, under the same deploy lock a deploy takes. The database is up to date by the time it
// returns, so the caller answers with the edited deployment; the cluster catches up on the event
// stream. An edit that changes nothing the cluster cares about, a TTL or a description, is finished
// when it returns.
func (s Service) EditDeployment(ctx context.Context, token string, deploymentId uint, edit instance.Edit, userID uint) (*model.Deployment, error) {
	acquired, err := s.instanceService.AcquireDeployLock(ctx, deploymentId)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, errdef.NewConflict("deployment %d is already being deployed", deploymentId)
	}

	changes, err := s.instanceService.EditDeployment(ctx, deploymentId, edit)
	if err != nil {
		s.releaseDeployLock(ctx, deploymentId)
		return nil, err
	}

	if len(changes.Redeploy) == 0 && len(changes.Destroy) == 0 {
		s.releaseDeployLock(ctx, deploymentId)
		return changes.Deployment, nil
	}

	token, err = s.tokenService.RefreshAccessToken(token)
	if err != nil {
		s.releaseDeployLock(ctx, deploymentId)
		return nil, err
	}

	for _, deploymentInstance := range changes.Redeploy {
		if err := s.instanceService.SetDeployStatus(ctx, deploymentInstance, model.DeployStatusPending); err != nil {
			s.releaseDeployLock(ctx, deploymentId)
			return nil, err
		}
	}

	redeploy := instanceIds(changes.Redeploy)
	destroy := instanceIds(changes.Destroy)

	ctx = context.WithoutCancel(ctx)
	go func() {
		defer s.releaseDeployLock(ctx, deploymentId)

		if err := s.applyDeploymentChanges(ctx, token, deploymentId, redeploy, destroy); err != nil {
			s.logger.ErrorContext(ctx, "edit failed", "deploymentId", deploymentId, "error", err)
			s.publisher.Publish(ctx, userID, changes.Deployment.GroupName, kindDeployment, newDeploymentEvent(changes.Deployment, "error", err.Error()))
			return
		}
		s.publisher.Publish(ctx, userID, changes.Deployment.GroupName, kindDeployment, newDeploymentEvent(changes.Deployment, "success", ""))
	}()

	return changes.Deployment, nil
}

// applyDeploymentChanges works from ids on its own copy of the deployment, so the caller stays free
// to serialise, and strip the sensitive values from, the one it answers with.
func (s Service) applyDeploymentChanges(ctx context.Context, token string, deploymentId uint, redeploy, destroy []uint) error {
	deployment, err := s.instanceService.FindDecryptedDeploymentById(ctx, deploymentId)
	if err != nil {
		return err
	}

	for _, instanceId := range destroy {
		deploymentInstance, err := findInstanceById(deployment.Instances, instanceId)
		if err != nil {
			return err
		}

		s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "started", ""))
		if err := s.instanceService.DestroyInstance(ctx, deploymentInstance); err != nil {
			err = fmt.Errorf("failed to destroy instance(%s) %q: %w", deploymentInstance.StackName, deploymentInstance.Name, err)
			s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "error", err.Error()))
			return err
		}
		if err := s.instanceService.DeleteDestroyedInstance(ctx, deploymentInstance); err != nil {
			s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "error", err.Error()))
			return err
		}
		s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "success", ""))
	}

	for _, instanceId := range redeploy {
		deploymentInstance, err := findInstanceById(deployment.Instances, instanceId)
		if err != nil {
			return err
		}

		token, err = s.tokenService.RefreshAccessToken(token)
		if err != nil {
			return err
		}

		s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "started", ""))
		if err := s.deployInstance(ctx, token, deploymentInstance, deployment.TTL, deployment.Instances); err != nil {
			err = fmt.Errorf("failed to deploy instance(%s) %q: %w", deploymentInstance.StackName, deploymentInstance.Name, err)
			s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "error", err.Error()))
			return err
		}
		s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "success", ""))
	}

	return nil
}

func instanceIds(instances []*model.DeploymentInstance) []uint {
	ids := make([]uint, len(instances))
	for i, deploymentInstance := range instances {
		ids[i] = deploymentInstance.ID
	}
	return ids
}

func (s Service) releaseDeployLock(ctx context.Context, deploymentId uint) {
	if err := s.instanceService.ReleaseDeployLock(ctx, deploymentId); err != nil {
		s.logger.ErrorContext(ctx, "failed to release deploy lock", "deploymentId", deploymentId, "error", err)
	}
}

func (s Service) deployDeployment(ctx context.Context, token string, deployment *model.Deployment, userID uint) error {
	for _, deploymentInstance := range deployment.Instances {
		var err error
		token, err = s.tokenService.RefreshAccessToken(token)
		if err != nil {
			return err
		}

		s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "started", ""))
		err = s.deployInstance(ctx, token, deploymentInstance, deployment.TTL, deployment.Instances)
		if err != nil {
			err = fmt.Errorf("failed to deploy instance(%s) %q: %w", deploymentInstance.StackName, deploymentInstance.Name, err)
			s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "error", err.Error()))
			return err
		}
		s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "success", ""))
	}

	return nil
}

// UpdateDeployment is the deployment-level edit restricted to the two fields PUT has always carried.
// It is kept so the scripts that use it keep working, and it redeploys nothing: the inspector reads
// the TTL out of the database, and the im-ttl label it also templates into the pod is read by no one,
// so rolling DHIS 2 core to refresh a label was work done for nobody.
func (s Service) UpdateDeployment(ctx context.Context, token string, deploymentId uint, ttl uint, description string, userID uint) (*model.Deployment, error) {
	return s.EditDeployment(ctx, token, deploymentId, instance.Edit{TTL: &ttl, Description: &description}, userID)
}

// Reset destroys the instance and deploys it again from the same parameters. Like StartDeployment it
// runs in the background under the deployment's deploy lock, on its own copy of the deployment, so
// the caller gets an answer immediately and a reset cannot race a deploy of the same deployment.
func (s Service) Reset(ctx context.Context, token string, deploymentId, instanceId uint, ttl uint, userID uint) error {
	acquired, err := s.instanceService.AcquireDeployLock(ctx, deploymentId)
	if err != nil {
		return err
	}
	if !acquired {
		return errdef.NewConflict("deployment %d is already being deployed", deploymentId)
	}

	deployment, err := s.instanceService.FindDecryptedDeploymentById(ctx, deploymentId)
	if err != nil {
		s.releaseDeployLock(ctx, deploymentId)
		return err
	}

	deploymentInstance, err := findInstanceById(deployment.Instances, instanceId)
	if err != nil {
		s.releaseDeployLock(ctx, deploymentId)
		return errdef.NewNotFound("%v", err)
	}

	token, err = s.tokenService.RefreshAccessToken(token)
	if err != nil {
		s.releaseDeployLock(ctx, deploymentId)
		return err
	}

	if err := s.instanceService.SetDeployStatus(ctx, deploymentInstance, model.DeployStatusPending); err != nil {
		s.releaseDeployLock(ctx, deploymentId)
		return err
	}

	ctx = context.WithoutCancel(ctx)
	go func() {
		defer s.releaseDeployLock(ctx, deployment.ID)

		s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "started", ""))
		err := s.resetInstance(ctx, token, deploymentInstance, ttl, deployment)
		if err != nil {
			s.logger.ErrorContext(ctx, "reset failed", "deploymentId", deployment.ID, "instanceId", deploymentInstance.ID, "error", err)
			s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "error", err.Error()))
			s.publisher.Publish(ctx, userID, deployment.GroupName, kindDeployment, newDeploymentEvent(deployment, "error", err.Error()))
			return
		}
		s.publisher.PublishTransient(ctx, deployment.GroupName, kindDeployment, newInstanceEvent(deployment, deploymentInstance, "success", ""))
		s.publisher.Publish(ctx, userID, deployment.GroupName, kindDeployment, newDeploymentEvent(deployment, "success", ""))
	}()

	return nil
}

func (s Service) resetInstance(ctx context.Context, token string, instance *model.DeploymentInstance, ttl uint, deployment *model.Deployment) error {
	if err := s.instanceService.DestroyInstance(ctx, instance); err != nil {
		return err
	}

	return s.deployInstance(ctx, token, instance, ttl, deployment.Instances)
}

func (s Service) UpdateInstance(ctx context.Context, token string, deploymentId, instanceId uint, parameters instance.Parameters, public *bool) (*model.DeploymentInstance, error) {
	acquired, err := s.instanceService.AcquireDeployLock(ctx, deploymentId)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, errdef.NewConflict("deployment %d is already being deployed", deploymentId)
	}
	defer s.releaseDeployLock(ctx, deploymentId)

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
// while the dump and the filestore backup run in the background against whichever instance
// advertises the filestoreBackup capability.
func (s Service) SaveAs(ctx context.Context, userId uint, instance *model.DeploymentInstance, stack *stack.Stack, filestoreInstance *model.DeploymentInstance, name string, format string) (*model.Database, error) {
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
		s.saveFilestore(ctx, userId, filestoreInstance, dumped)
	}()

	return created, nil
}

// Save overwrites the instance's source database with a fresh dump. The lock check runs before
// returning; the dump, finalization and filestore backup run in the background.
func (s Service) Save(ctx context.Context, userId uint, database *model.Database, instance *model.DeploymentInstance, stack *stack.Stack, filestoreInstance *model.DeploymentInstance) error {
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
		s.saveFilestore(ctx, locked.UserID, filestoreInstance, saved)
	}()

	return nil
}

func (s Service) saveFilestore(ctx context.Context, userId uint, filestoreInstance *model.DeploymentInstance, database *model.Database) {
	if filestoreInstance == nil {
		return
	}

	s.publisher.Publish(ctx, userId, database.GroupName, kindFilestoreBackup, newFilestoreEvent(database, "started", ""))
	if err := s.instanceService.FilestoreBackup(ctx, filestoreInstance, database.Name, database); err != nil {
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

// databaseIdentifierFromInstances resolves the DATABASE_ID parameter from whichever instance in
// the deployment carries it. DATABASE_ID lives on the db instance, while storage parameters live
// on the core instance, so callers operating on the core must look across siblings to find it.
// The value is either a numeric id or a slug, so it is returned unparsed for the database service
// to resolve; "0" is the sentinel for a deployment that has no database to seed from.
func databaseIdentifierFromInstances(instances []*model.DeploymentInstance) (string, bool) {
	for _, instance := range instances {
		param, ok := instance.Parameters["DATABASE_ID"]
		if !ok {
			continue
		}

		if param.Value == "" || param.Value == "0" {
			continue
		}

		return param.Value, true
	}
	return "", false
}

// buildSeed resolves the database referenced by the deployment's DATABASE_ID parameter into the
// environment variables and filestore backup record needed to seed an instance at deploy time.
func (s Service) buildSeed(ctx context.Context, instances []*model.DeploymentInstance) (map[string]string, *model.Database, error) {
	identifier, ok := databaseIdentifierFromInstances(instances)
	if !ok {
		return nil, nil, nil
	}

	hostname := os.Getenv("HOSTNAME")
	extraEnv := make(map[string]string)

	db, err := s.databaseService.FindByIdentifier(ctx, identifier)
	if err != nil {
		return nil, nil, fmt.Errorf("database %q not found: %w", identifier, err)
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
