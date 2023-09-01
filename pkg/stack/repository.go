package stack

import (
	"errors"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

type Repository interface {
	Create(stack *model.Stack) error
	Delete(name string) error
	Find(name string) (*model.Stack, error)
	FindAll() (*[]model.Stack, error)
	CreateParameter(parameter *model.StackParameter) error
	Save(stack *model.Stack) error
}

type repository struct {
	db *gorm.DB
}

//goland:noinspection GoExportedFuncWithUnexportedType
func NewRepository(db *gorm.DB) *repository {
	return &repository{db}
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
		Preload("GormParameters").
		First(&stack, "name = ?", name).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = errdef.NewNotFound("stack not found by name: %s", name)
		}
	}
	return stack, err
}

func (r repository) FindAll() (*[]model.Stack, error) {
	var stacks []model.Stack
	err := r.db.Find(&stacks).Error
	return &stacks, err
}

func (r repository) CreateParameter(parameter *model.StackParameter) error {
	return r.db.FirstOrCreate(&parameter).Error
}

func (r repository) Save(stack *model.Stack) error {
	return r.db.Save(stack).Error
}
