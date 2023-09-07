package stack

import (
	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/dhis2-sre/im-manager/pkg/model"
	"golang.org/x/exp/maps"
)

//goland:noinspection GoExportedFuncWithUnexportedType
func NewService(stacks Stacks) *service {
	return &service{stacks}
}

type service struct {
	stacks Stacks
}

func (s service) Find(name string) (*model.Stack, error) {
	stack, ok := s.stacks[name]
	if !ok {
		return nil, errdef.NewNotFound("stack not found")
	}
	return &stack, nil
}

func (s service) FindAll() ([]model.Stack, error) {
	return maps.Values(s.stacks), nil
}
