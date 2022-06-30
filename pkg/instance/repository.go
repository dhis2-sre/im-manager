package instance

import (
	"errors"
	"fmt"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/jackc/pgconn"
	"gorm.io/gorm"
)

type Repository interface {
	Link(firstInstance, secondInstance *model.Instance) error
	Unlink(instance *model.Instance) error
	Create(instance *model.Instance) error
	Save(instance *model.Instance) error
	FindWithParametersById(id uint) (*model.Instance, error)
	FindByNameAndGroup(instance string, group string) (*model.Instance, error)
	SaveDeployLog(instance *model.Instance, log string) error
	FindById(id uint) (*model.Instance, error)
	Delete(id uint) error
	FindByGroupNames(names []string) ([]*model.Instance, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(DB *gorm.DB) *repository {
	return &repository{db: DB}
}

func (r repository) Link(firstInstance *model.Instance, secondInstance *model.Instance) error {
	link := &model.Linked{
		FirstInstanceID:  firstInstance.ID,
		StackName:        secondInstance.StackName,
		SecondInstanceID: secondInstance.ID,
	}
	err := r.db.Create(&link).Error
	if err != nil {
		var perr *pgconn.PgError
		if errors.As(err, &perr) && perr.Code == "23505" {
			return fmt.Errorf("instance (%d) already linked with a stack of type \"%s\"", firstInstance.ID, secondInstance.StackName)
		}
		return err
	}
	return nil
}

func (r repository) Unlink(instance *model.Instance) error {
	link := &model.Linked{}

	// Does another instance depends on the instance we're trying to unlink
	err := r.db.First(link, "first_instance_id = ?", instance.ID).Error
	if err == nil {
		return fmt.Errorf("instance %d depends on %d", link.SecondInstanceID, instance.ID)
	}

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// Find instance to delete, return nil if not found
	err = r.db.Find(link, "second_instance_id = ?", instance.ID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	err = r.db.Unscoped().Delete(link, "first_instance_id = ? and second_instance_id = ?", link.FirstInstanceID, link.SecondInstanceID).Error
	if err != nil {
		return err
	}
	return nil
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
