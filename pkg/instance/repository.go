package instance

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/gosimple/slug"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB, instanceParameterEncryptionKey string) *repository {
	return &repository{
		db:                             db,
		instanceParameterEncryptionKey: instanceParameterEncryptionKey,
	}
}

type repository struct {
	db                             *gorm.DB
	instanceParameterEncryptionKey string
}

func (r repository) DeleteDeploymentInstance(ctx context.Context, instance *model.DeploymentInstance) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	err := r.db.WithContext(ctx).Unscoped().Delete(&model.DeploymentInstance{}, instance.ID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errdef.NewNotFound("instance not found by id: %d", instance.ID)
		}
		return fmt.Errorf("failed to delete instance %q: %v", instance.Name, err)
	}

	return nil
}

func (r repository) DeleteDeployment(ctx context.Context, deployment *model.Deployment) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	err := r.db.WithContext(ctx).Unscoped().Delete(&model.Deployment{}, deployment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errdef.NewNotFound("deployment not found by id: %d", deployment.ID)
		}
		return fmt.Errorf("failed to delete deployment: %v", err)
	}

	return nil
}

func (r repository) SaveDeployment(ctx context.Context, deployment *model.Deployment) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	err := r.db.WithContext(ctx).Save(&deployment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errdef.NewDuplicated("a deployment named %q already exists", deployment.Name)
		}
		return fmt.Errorf("failed to save deployment: %v", err)
	}

	return nil
}

func (r repository) FindDeploymentById(ctx context.Context, id uint) (*model.Deployment, error) {
	var deployment *model.Deployment
	err := r.db.
		WithContext(ctx).
		Joins("Group").
		Joins("User").
		Preload("Instances.GormParameters").
		Preload("Instances.Group").
		First(&deployment, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errdef.NewNotFound("deployment not found by id: %d", id)
		}
		return nil, fmt.Errorf("failed to find deployment: %v", err)
	}

	return deployment, nil
}

func (r repository) FindDeploymentInstanceById(ctx context.Context, id uint) (*model.DeploymentInstance, error) {
	var instance *model.DeploymentInstance
	err := r.db.
		WithContext(ctx).
		Joins("Group.ClusterConfiguration").
		Preload("GormParameters").
		First(&instance, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errdef.NewNotFound("instance not found by id: %d", id)
		}
		return nil, fmt.Errorf("failed to find instance: %v", err)
	}

	return instance, nil
}

func (r repository) DecryptDeploymentInstance(deploymentInstance *model.DeploymentInstance, stack *model.Stack) (*model.DeploymentInstance, error) {
	err := decryptParameters(r.instanceParameterEncryptionKey, deploymentInstance, stack)
	if err != nil {
		return nil, err
	}

	return deploymentInstance, nil
}

func (r repository) DecryptDeployment(deployment *model.Deployment, stacksByName map[string]*model.Stack) (*model.Deployment, error) {
	for _, instance := range deployment.Instances {
		err := decryptParameters(r.instanceParameterEncryptionKey, instance, stacksByName[instance.StackName])
		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}

func (r repository) SaveInstance(ctx context.Context, instance *model.DeploymentInstance, stack *model.Stack) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	key := r.instanceParameterEncryptionKey

	err := encryptParameters(key, instance, stack)
	if err != nil {
		return err
	}

	err = r.db.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: true}).Save(instance).Error
	if err != nil {
		return fmt.Errorf("failed to save instance: %v", err)
	}
	return nil
}

func (r repository) SaveDeployLog(ctx context.Context, instance *model.DeploymentInstance, log string) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	err := r.db.WithContext(ctx).Model(&instance).Update("DeployLog", log).Error
	if err != nil {
		return fmt.Errorf("failed to save deploy log: %v", err)
	}
	return nil
}

const administratorGroupName = "administrators"

func (r repository) FindDeployments(ctx context.Context, groupNames []string) ([]*model.Deployment, error) {
	db := r.db.WithContext(ctx)

	isAdmin := slices.Contains(groupNames, administratorGroupName)
	if !isAdmin {
		db = db.Where("group_name IN ?", groupNames)
	}

	var deployments []*model.Deployment
	err := db.
		Joins("Group").
		Joins("User").
		Preload("Instances").
		Order("updated_at desc").
		Find(&deployments).Error

	return deployments, err
}

func (r repository) FindPublicInstances(ctx context.Context) ([]*model.DeploymentInstance, error) {
	var instances []*model.DeploymentInstance
	err := r.db.
		WithContext(ctx).
		Joins("Group").
		Joins("Deployment").
		Where("public = true").
		Order("updated_at desc").
		Find(&instances).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find instances: %v", err)
	}
	return instances, nil
}

// TODO: This code is duplicated from https://github.com/dhis2-sre/im-manager/blob/5581b4765fd6878138f4741d4c82607ed4ce0998/pkg/database/repository.go#L41
// A significant difference is that here we include the database.Type in the slug
// Ideally we should rewrite the database service to a storage service and use that in both the database and instance handlers (and/or services)
func (r repository) RecordBackup(ctx context.Context, database *model.Database) error {
	// only use ctx for values (logging) and not cancellation signals on cud operations for now. ctx
	// cancellation can lead to rollbacks which we should decide individually.
	ctx = context.WithoutCancel(ctx)

	s := fmt.Sprintf("%s/%s-%s", database.GroupName, database.Name, database.Type)
	database.Slug = slug.Make(s)

	err := r.db.WithContext(ctx).Save(database).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return errdef.NewDuplicated("database named %q already exists", database.Name)
	}

	return err
}

func (r repository) SaveDatabase(ctx context.Context, database *model.Database) error {
	return r.db.WithContext(ctx).Save(&database).Error
}

func (r repository) FindAllDeployments(ctx context.Context) ([]model.Deployment, error) {
	var deployments []model.Deployment
	err := r.db.WithContext(ctx).
		Find(&deployments).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []model.Deployment{}, nil
		}
		return nil, fmt.Errorf("failed to find instance: %v", err)
	}
	return deployments, err
}

func encryptParameters(key string, instance *model.DeploymentInstance, stack *model.Stack) error {
	for i, parameter := range instance.Parameters {
		if !stack.Parameters[parameter.ParameterName].Sensitive {
			continue
		}
		value, err := encryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		parameter.Value = value
		instance.Parameters[i] = parameter
	}

	return nil
}

func decryptParameters(key string, instance *model.DeploymentInstance, stack *model.Stack) error {
	for i, parameter := range instance.Parameters {
		if !stack.Parameters[parameter.ParameterName].Sensitive {
			continue
		}
		value, err := decryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		parameter.Value = value
		instance.Parameters[i] = parameter
	}

	return nil
}
