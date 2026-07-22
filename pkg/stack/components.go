package stack

import (
	"github.com/dhis2-sre/im-manager/pkg/kube"
)

// Components returns the components of the named stack.
func (s Service) Components(stackName string) ([]kube.Component, error) {
	stack, err := s.Find(stackName)
	if err != nil {
		return nil, err
	}
	return stack.Components, nil
}
