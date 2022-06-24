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
	CreateRequiredParameter(name string, parameter *model.StackRequiredParameter, consumed bool) error
	CreateOptionalParameter(name string, parameter *model.StackOptionalParameter, consumed bool, defaultValue string) error
	Save(stack *model.Stack) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(DB *gorm.DB) *repository {
	return &repository{db: DB}
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
		Preload("RequiredParameters", "consumed <> ?", true).
		Preload("OptionalParameters", "consumed <> ?", true).
		First(&stack, "name = ?", name).Error
	return stack, err
}

func (r repository) FindAll() (*[]model.Stack, error) {
	var stacks []model.Stack
	err := r.db.Find(&stacks).Error
	return &stacks, err
}

func (r repository) CreateOptionalParameter(name string, parameter *model.StackOptionalParameter, consumed bool, defaultValue string) error {
	err := r.db.FirstOrCreate(&parameter).Error
	if err != nil {
		return err
	}
	return err

	//	joinModel := &model.OptionalStackParametersJoin{StackName: name, ParameterID: parameter.Name, Consumed: consumed, DefaultValue: defaultValue}

	//	return r.db.Create(&joinModel).Error
}

func (r repository) CreateRequiredParameter(name string, parameter *model.StackRequiredParameter, consumed bool) error {
	err := r.db.FirstOrCreate(&parameter).Error
	if err != nil {
		return err
	}
	return err

	//	joinModel := &model.RequiredStackParametersJoin{StackName: name, ParameterID: parameter.Name, Consumed: consumed}

	//	return r.db.Create(&joinModel).Error
}

func (r repository) Save(stack *model.Stack) error {
	return r.db.Save(stack).Error
}
