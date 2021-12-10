package instance

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

type Repository interface {
	Create(instance *model.Instance) error
	FindWithParametersById(id uint) (*model.Instance, error)
	FindByNameAndGroup(instanceName string, groupId uint) (*model.Instance, error)
	SaveDeployLog(instance *model.Instance, log string) error
	FindById(id uint) (*model.Instance, error)
	Delete(id uint) error
}

func ProvideRepository(DB *gorm.DB) Repository {
	return &repository{db: DB}
}

type repository struct {
	db *gorm.DB
}

func (r repository) Create(instance *model.Instance) error {
	return r.db.Create(&instance).Error
}

func (r repository) FindWithParametersById(id uint) (*model.Instance, error) {
	var instance *model.Instance
	err := r.db.
		Preload("RequiredParameters.StackRequiredParameter").
		Preload("OptionalParameters.StackOptionalParameter").
		First(&instance, id).Error
	return instance, err
}

func (r repository) SaveDeployLog(instance *model.Instance, log string) error {
	return r.db.Model(&instance).Update("DeployLog", log).Error
}

func (r repository) FindById(id uint) (*model.Instance, error) {
	var instance *model.Instance
	err := r.db.First(&instance, id).Error
	return instance, err
}

func (r repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&model.Instance{}, id).Error
}

func (r repository) FindByNameAndGroup(instanceName string, groupId uint) (*model.Instance, error) {
	var instance *model.Instance

	err := r.db.Where("name = ?", instanceName).Where("group_id = ?", groupId).First(&instance).Error
	if err != nil {
		return nil, err
	}

	return instance, err
}
