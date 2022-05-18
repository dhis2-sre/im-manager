package stack

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

type Repository interface {
	Create(stack *model.Stack) error
	Delete(id uint) error
	FindById(id uint) (*model.Stack, error)
	FindByName(name string) (*model.Stack, error)
	FindAll() (*[]model.Stack, error)
	CreateRequiredParameter(stackID uint, parameter *model.StackRequiredParameter) error
	CreateOptionalParameter(stackID uint, parameter *model.StackOptionalParameter, defaultValue string) error
}

func ProvideRepository(DB *gorm.DB) Repository {
	return &repository{db: DB}
}

type repository struct {
	db *gorm.DB
}

func (r repository) Create(stack *model.Stack) error {
	return r.db.Create(&stack).Error
}

func (r repository) Delete(id uint) error {
	return r.db.Unscoped().Delete(&model.Stack{}, id).Error
}

func (r repository) FindById(id uint) (*model.Stack, error) {
	var stack *model.Stack
	err := r.db.
		Preload("RequiredParameters").
		Preload("OptionalParameters").
		First(&stack, id).Error
	return stack, err
}

func (r repository) FindByName(name string) (*model.Stack, error) {
	var stack *model.Stack
	err := r.db.Where("name = ?", name).First(&stack).Error
	return stack, err
}

func (r repository) FindAll() (*[]model.Stack, error) {
	var stacks []model.Stack
	err := r.db.Find(&stacks).Error
	return &stacks, err
}

func (r repository) CreateOptionalParameter(stackID uint, parameter *model.StackOptionalParameter, defaultValue string) error {
	err := r.db.FirstOrCreate(&parameter).Error
	if err != nil {
		return err
	}

	joinModel := &model.OptionalStackParametersJoin{StackID: stackID, ParameterID: parameter.ID, DefaultValue: defaultValue}

	return r.db.Create(&joinModel).Error
}

func (r repository) CreateRequiredParameter(stackID uint, parameter *model.StackRequiredParameter) error {
	err := r.db.FirstOrCreate(&parameter).Error
	if err != nil {
		return err
	}

	joinModel := &model.RequiredStackParametersJoin{StackID: stackID, ParameterID: parameter.ID}

	return r.db.Create(&joinModel).Error
}
