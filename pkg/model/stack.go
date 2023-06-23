package model

import (
	"fmt"
	"time"
)

// swagger:model Stack
type Stack struct {
	CreatedAt          time.Time                `json:"createdAt"`
	UpdatedAt          time.Time                `json:"updatedAt"`
	Name               string                   `json:"name" gorm:"primaryKey"`
	RequiredParameters []StackRequiredParameter `json:"requiredParameters" gorm:"foreignKey:StackName; references: Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	OptionalParameters []StackOptionalParameter `json:"optionalParameters" gorm:"foreignKey:StackName; references: Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Instances          []Instance               `json:"instances" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	HostnamePattern    string                   `json:"hostnamePattern"`
	HostnameVariable   string                   `json:"hostnameVariable"`
}

func (s Stack) GetHostname(name, namespace string) string {
	return fmt.Sprintf(s.HostnamePattern, name, namespace)
}

func (s Stack) FindOptionalParameter(name string) (StackOptionalParameter, error) {
	for _, parameter := range s.OptionalParameters {
		if parameter.Name == name {
			return parameter, nil
		}
	}
	return StackOptionalParameter{}, fmt.Errorf("optional parameter not found: %s", name)
}

type StackRequiredParameter struct {
	Name      string `json:"name" gorm:"primaryKey"`
	StackName string `json:"stackName" gorm:"primaryKey"`
	Consumed  bool   `json:"consumed"`
}

type StackOptionalParameter struct {
	Name         string `json:"name" gorm:"primaryKey"`
	StackName    string `json:"stackName" gorm:"primaryKey"`
	DefaultValue string `json:"defaultValue"`
	Consumed     bool   `json:"consumed"`
}
