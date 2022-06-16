package stack

import (
	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

type Service interface {
	Create(name string) (*model.Stack, error)
	Delete(name string) error
	CreateRequiredParameter(stack *model.Stack, parameterName string) (*model.StackRequiredParameter, error)
	CreateOptionalParameter(stack *model.Stack, parameterName string, defaultValue string) (*model.StackOptionalParameter, error)
	Find(name string) (*model.Stack, error)
	FindAll() (*[]model.Stack, error)
}

type service struct {
	repository Repository
}

func NewService(repository Repository) *service {
	return &service{repository}
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

func (s service) Delete(name string) error {
	return s.repository.Delete(name)
}

func (s service) CreateRequiredParameter(stack *model.Stack, parameterName string) (*model.StackRequiredParameter, error) {
	parameter := &model.StackRequiredParameter{Name: parameterName}

	err := s.repository.CreateRequiredParameter(stack.Name, parameter)

	return parameter, err
}

func (s service) CreateOptionalParameter(stack *model.Stack, parameterName string, defaultValue string) (*model.StackOptionalParameter, error) {
	parameter := &model.StackOptionalParameter{Name: parameterName}

	err := s.repository.CreateOptionalParameter(stack.Name, parameter, defaultValue)

	return parameter, err
}

func (s service) Find(name string) (*model.Stack, error) {
	return s.repository.Find(name)
}

func (s service) FindAll() (*[]model.Stack, error) {
	return s.repository.FindAll()
}
