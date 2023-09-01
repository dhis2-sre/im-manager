package model

import (
	"fmt"
	"time"
)

// swagger:model Stack
type Stack struct {
	Name             string           `json:"name" gorm:"primaryKey"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
	Parameters       []StackParameter `json:"parameters" gorm:"foreignKey:StackName; references: Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Instances        []Instance       `json:"instances" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	HostnamePattern  string           `json:"hostnamePattern"`
	HostnameVariable string           `json:"hostnameVariable"`
	// Providers provide parameters to other stacks.
	Providers map[string]Provider `json:"-" gorm:"-"`
	Requires  []Stack             `json:"-" gorm:"-"`
}

func (s Stack) GetHostname(name, namespace string) string {
	return fmt.Sprintf(s.HostnamePattern, name, namespace)
}

func (s Stack) FindParameter(name string) (StackParameter, error) {
	for _, parameter := range s.Parameters {
		if parameter.Name == name {
			return parameter, nil
		}
	}
	return StackParameter{}, fmt.Errorf("parameter not found: %s", name)
}

type StackParameter struct {
	Name         string  `json:"name" gorm:"primaryKey"`
	StackName    string  `json:"-" gorm:"primaryKey"`
	DefaultValue *string `json:"defaultValue"`
	Consumed     bool    `json:"consumed"`
}

// Provides a value that can be consumed by a stack as a stack parameter.
type Provider interface {
	Provide(instance Instance) (value string, err error)
}

type ProviderFunc func(instance Instance) (string, error)

func (p ProviderFunc) Provide(instance Instance) (string, error) {
	return p(instance)
}
