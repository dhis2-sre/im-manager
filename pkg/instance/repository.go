package instance

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

type Repository interface {
	Create(instance *model.Instance) error
	Save(instance *model.Instance) error
	FindWithParametersById(id uint) (*model.Instance, error)
	FindByNameAndGroup(instance string, group string) (*model.Instance, error)
	SaveDeployLog(instance *model.Instance, log string) error
	FindById(id uint) (*model.Instance, error)
	Delete(id uint) error
	FindByGroupNames(names []string) ([]*model.Instance, error)
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

func (r repository) Save(instance *model.Instance) error {
	err := r.db.Save(instance).Error
	if err != nil {
		return err
	}

	return nil
}

func (r repository) FindWithParametersById(id uint) (*model.Instance, error) {
	var instance *model.Instance
	err := r.db.
		Preload("RequiredParameters.StackRequiredParameter").
		Preload("OptionalParameters.StackOptionalParameter").
		First(&instance, id).Error
	return instance, err
}

func (r repository) FindByNameAndGroup(instance string, group string) (*model.Instance, error) {
	var i *model.Instance

	err := r.db.
		Where("name = ?", instance).
		Where("group_name = ?", group).
		First(&i).Error
	if err != nil {
		return nil, err
	}

	return i, err
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

func (r repository) FindByGroupNames(names []string) ([]*model.Instance, error) {
	var instances []*model.Instance

	err := r.db.
		Preload("RequiredParameters.StackRequiredParameter").
		Preload("OptionalParameters.StackOptionalParameter").
		Where("group_name IN ?", names).
		Find(&instances).Error

	return instances, err
}
