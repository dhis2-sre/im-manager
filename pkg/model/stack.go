package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// swagger:model Stack
type Stack struct {
	Name      string    `json:"name" gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// GormParameters are only used by Gorm to persist parameters as it cannot persist a
	// StackParameters. Only use GormParameters within the repository. Otherwise use
	// Parameters.
	GormParameters []StackParameter `json:"parameters" gorm:"foreignKey:StackName; references: Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	// Parameters used by the stacks helmfile template.
	Parameters       StackParameters `json:"-" gorm:"-"`
	Instances        []Instance      `json:"instances" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	HostnamePattern  string          `json:"hostnamePattern"`
	HostnameVariable string          `json:"hostnameVariable"`
	// Providers provide parameters to other stacks.
	Providers Providers `json:"-" gorm:"-"`
	// Requires these stacks to deploy an instance of this stack.
	Requires []Stack `json:"-" gorm:"-"`
}

// BeforeSave translates Parameters from a map to a slice before persisting the stack in the DB.
func (s *Stack) BeforeSave(_ *gorm.DB) error {
	s.GormParameters = make([]StackParameter, 0, len(s.Parameters))
	for n, parameter := range s.Parameters {
		parameter.Name = n
		s.GormParameters = append(s.GormParameters, parameter)
	}
	return nil
}

// AfterFind translates GormParameters from a slice to a map in Parameters after fetching the stack
// from the DB.
func (s *Stack) AfterFind(_ *gorm.DB) error {
	s.Parameters = make(StackParameters, len(s.GormParameters))
	for _, parameter := range s.GormParameters {
		s.Parameters[parameter.Name] = parameter
	}
	return nil
}

func (s *Stack) GetHostname(name, namespace string) string {
	return fmt.Sprintf(s.HostnamePattern, name, namespace)
}

type StackParameters map[string]StackParameter

type StackParameter struct {
	Name         string  `json:"name" gorm:"primaryKey"`
	StackName    string  `json:"-" gorm:"primaryKey"`
	DefaultValue *string `json:"defaultValue"`
	// Consumed signals that this parameter is provided by another stack i.e. one of the stacks required stacks.
	Consumed bool `json:"consumed"`
	// Validator ensure that actual stack parameters are valid according to its rules.
	Validator Validator `json:"-" gorm:"-"`
}

// Validator validates given value returning an error for invalid ones.
type Validator interface {
	Validate(value string) error
}

type ValidatorFunc func(value string) error

func (v ValidatorFunc) Validate(value string) error {
	return v(value)
}

type Providers map[string]Provider

// Provides a value that can be consumed by a stack as a stack parameter.
type Provider interface {
	Provide(instance Instance) (value string, err error)
}

type ProviderFunc func(instance Instance) (string, error)

func (p ProviderFunc) Provide(instance Instance) (string, error) {
	return p(instance)
}
