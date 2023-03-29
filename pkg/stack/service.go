package stack

import (
	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/dhis2-sre/im-manager/pkg/model"
)

type Service interface {
	Create(stack *model.Stack) (*model.Stack, error)
	Delete(name string) error
	CreateParameter(stack *model.Stack, parameterName string, consumed bool) (*model.Parameter, error)
	Find(name string) (*model.Stack, error)
	FindAll() (*[]model.Stack, error)
	Save(stack *model.Stack) error
}

type service struct {
	repository Repository
}

func NewService(repository Repository) *service {
	return &service{repository}
}

func (s service) Create(stack *model.Stack) (*model.Stack, error) {
	err := s.repository.Create(stack)
	if err != nil {
		return nil, apperror.NewBadRequest(err.Error())
	}

	return stack, err
}

func (s service) Delete(name string) error {
	return s.repository.Delete(name)
}

func (s service) CreateParameter(stack *model.Stack, parameterName string, consumed bool) (*model.Parameter, error) {
	parameter := &model.Parameter{Name: parameterName, StackName: stack.Name, Consumed: consumed}

	err := s.repository.CreateParameter(parameter)

	return parameter, err
}

func (s service) Find(name string) (*model.Stack, error) {
	return s.repository.Find(name)
}

func (s service) FindAll() (*[]model.Stack, error) {
	return s.repository.FindAll()
}

func (s service) Save(stack *model.Stack) error {
	return s.repository.Save(stack)
}
