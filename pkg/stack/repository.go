package stack

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

type Repository interface {
	Create(stack *model.Stack) error
	Delete(name string) error
	Find(name string) (*model.Stack, error)
	FindAll() (*[]model.Stack, error)
	CreateRequiredParameter(name string, parameter *model.StackRequiredParameter) error
	CreateOptionalParameter(name string, parameter *model.StackOptionalParameter, defaultValue string) error
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

func (r repository) Delete(name string) error {
	return r.db.Unscoped().Delete(&model.Stack{}, name).Error
}

func (r repository) Find(name string) (*model.Stack, error) {
	var stack *model.Stack
	err := r.db.
		Preload("RequiredParameters").
		Preload("OptionalParameters").
		First(&stack, "name = ?", name).Error
	return stack, err
}

func (r repository) FindAll() (*[]model.Stack, error) {
	var stacks []model.Stack
	err := r.db.Find(&stacks).Error
	return &stacks, err
}

func (r repository) CreateOptionalParameter(name string, parameter *model.StackOptionalParameter, defaultValue string) error {
	err := r.db.FirstOrCreate(&parameter).Error
	if err != nil {
		return err
	}

	joinModel := &model.OptionalStackParametersJoin{StackName: name, ParameterID: parameter.Name, DefaultValue: defaultValue}

	return r.db.Create(&joinModel).Error
}

func (r repository) CreateRequiredParameter(name string, parameter *model.StackRequiredParameter) error {
	err := r.db.FirstOrCreate(&parameter).Error
	if err != nil {
		return err
	}

	joinModel := &model.RequiredStackParametersJoin{StackName: name, ParameterID: parameter.Name}

	return r.db.Create(&joinModel).Error
}
