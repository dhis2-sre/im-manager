package stack

import (
	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"golang.org/x/exp/maps"
)

func NewService(stacks Stacks) Service {
	return Service{stacks}
}

type Service struct {
	stacks Stacks
}

func (s Service) Find(name string) (*model.Stack, error) {
	stack, ok := s.stacks[name]
	if !ok {
		return nil, errdef.NewNotFound("stack not found")
	}
	return &stack, nil
}

func (s Service) FindAll() ([]model.Stack, error) {
	return maps.Values(s.stacks), nil
}
