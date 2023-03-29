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
	Save(stack *model.Stack) error
}

type repository struct {
	db *gorm.DB
}

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
		Preload("Parameters").
		First(&stack, "name = ?", name).Error
	return stack, err
}

func (r repository) FindAll() (*[]model.Stack, error) {
	var stacks []model.Stack
	err := r.db.Find(&stacks).Error
	return &stacks, err
}

func (r repository) Save(stack *model.Stack) error {
	return r.db.Save(stack).Error
}
