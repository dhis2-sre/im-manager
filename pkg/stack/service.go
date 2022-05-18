package stack

import (
	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

type Service interface {
	Create(name string) (*model.Stack, error)
	Delete(id uint) error
	CreateRequiredParameter(stack *model.Stack, parameterName string) (*model.StackRequiredParameter, error)
	CreateOptionalParameter(stack *model.Stack, parameterName string, defaultValue string) (*model.StackOptionalParameter, error)
	FindByName(name string) (*model.Stack, error)
	FindAll() (*[]model.Stack, error)
	FindById(id uint) (*model.Stack, error)
}

func ProvideService(repository Repository) Service {
	return &service{repository}
}

type service struct {
	repository Repository
}

func (s service) Create(name string) (*model.Stack, error) {
	stack := &model.Stack{
		Name: name,
	}

	err := s.repository.Create(stack)
	if err != nil {
		return nil, apperror.NewBadRequest(err.Error())
	}

	return stack, err
}

func (s service) Delete(id uint) error {
	return s.repository.Delete(id)
}

func (s service) CreateRequiredParameter(stack *model.Stack, parameterName string) (*model.StackRequiredParameter, error) {
	parameter := &model.StackRequiredParameter{Name: parameterName}

	err := s.repository.CreateRequiredParameter(stack.ID, parameter)

	return parameter, err
}

func (s service) CreateOptionalParameter(stack *model.Stack, parameterName string, defaultValue string) (*model.StackOptionalParameter, error) {
	parameter := &model.StackOptionalParameter{Name: parameterName}

	err := s.repository.CreateOptionalParameter(stack.ID, parameter, defaultValue)

	return parameter, err
}

func (s service) FindByName(name string) (*model.Stack, error) {
	return s.repository.FindByName(name)
}

func (s service) FindAll() (*[]model.Stack, error) {
	return s.repository.FindAll()
}

func (s service) FindById(id uint) (*model.Stack, error) {
	return s.repository.FindById(id)
}
