package instance

import (
	"errors"
	"fmt"
	"slices"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB, instanceParameterEncryptionKey string) *repository {
	return &repository{db: db, instanceParameterEncryptionKey: instanceParameterEncryptionKey}
}

type repository struct {
	db                             *gorm.DB
	instanceParameterEncryptionKey string
}

func (r repository) DeleteDeploymentInstance(instance *model.DeploymentInstance) error {
	err := r.db.Unscoped().Delete(&model.DeploymentInstance{}, instance.ID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errdef.NewNotFound("instance not found by id: %d", instance.ID)
		}
		return fmt.Errorf("failed to delete instance %q: %v", instance.Name, err)
	}

	return nil
}

func (r repository) DeleteDeployment(deployment *model.Deployment) error {
	err := r.db.Unscoped().Delete(&model.Deployment{}, deployment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errdef.NewNotFound("deployment not found by id: %d", deployment.ID)
		}
		return fmt.Errorf("failed to delete deployment: %v", err)
	}

	return nil
}

func (r repository) SaveDeployment(deployment *model.Deployment) error {
	err := r.db.Create(&deployment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errdef.NewDuplicated("a deployment named %q already exists", deployment.Name)
		}
		return fmt.Errorf("failed to save deployment: %v", err)
	}

	return nil
}

func (r repository) FindDeploymentById(id uint) (*model.Deployment, error) {
	var deployment *model.Deployment
	err := r.db.
		Joins("Group").
		Joins("User").
		Preload("Instances.GormParameters").
		First(&deployment, id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errdef.NewNotFound("deployment not found by id: %d", id)
		}
		return nil, fmt.Errorf("failed to find deployment: %v", err)
	}

	return deployment, nil
}

func (r repository) FindDeploymentInstanceById(id uint) (*model.DeploymentInstance, error) {
	var instance *model.DeploymentInstance
	err := r.db.
		Joins("Group").
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

func (r repository) FindDecryptedDeploymentInstanceById(id uint) (*model.DeploymentInstance, error) {
	instance, err := r.FindDeploymentInstanceById(id)
	if err != nil {
		return nil, err
	}

	err = decryptParameters(r.instanceParameterEncryptionKey, instance)
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func (r repository) FindDecryptedDeploymentById(id uint) (*model.Deployment, error) {
	deployment, err := r.FindDeploymentById(id)
	if err != nil {
		return nil, err
	}

	for _, instance := range deployment.Instances {
		err := decryptParameters(r.instanceParameterEncryptionKey, instance)
		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}

func (r repository) SaveInstance(instance *model.DeploymentInstance) error {
	key := r.instanceParameterEncryptionKey

	err := encryptParameters(key, instance)
	if err != nil {
		return err
	}

	err = r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(instance).Error
	if err != nil {
		return fmt.Errorf("failed to save instance: %v", err)
	}
	return nil
}

func (r repository) SaveDeployLog(instance *model.DeploymentInstance, log string) error {
	err := r.db.Model(&instance).Update("DeployLog", log).Error
	if err != nil {
		return fmt.Errorf("failed to save deploy log: %v", err)
	}
	return nil
}

const administratorGroupName = "administrators"

func (r repository) FindDeployments(groupNames []string) ([]*model.Deployment, error) {
	db := r.db

	isAdmin := slices.Contains(groupNames, administratorGroupName)
	if !isAdmin {
		db = db.Where("group_name IN ?", groupNames)
	}

	var deployments []*model.Deployment
	err := db.
		Joins("User").
		Preload("Instances").
		Order("updated_at desc").
		Find(&deployments).Error

	return deployments, err
}

func encryptParameters(key string, instance *model.DeploymentInstance) error {
	for i, parameter := range instance.Parameters {
		value, err := encryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		parameter.Value = value
		instance.Parameters[i] = parameter
	}

	return nil
}

func decryptParameters(key string, instance *model.DeploymentInstance) error {
	for i, parameter := range instance.Parameters {
		value, err := decryptText(key, parameter.Value)
		if err != nil {
			return err
		}
		parameter.Value = value
		instance.Parameters[i] = parameter
	}

	return nil
}
