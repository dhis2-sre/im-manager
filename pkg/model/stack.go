package model

import (
	"fmt"
	"time"
)

// swagger:model Stack
type Stack struct {
	CreatedAt          time.Time                `json:"createdAt"`
	UpdatedAt          time.Time                `json:"updatedAt"`
	Name               string                   `gorm:"primaryKey" json:"name"`
	RequiredParameters []StackRequiredParameter `gorm:"foreignKey:StackName; references: Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"requiredParameters"`
	OptionalParameters []StackOptionalParameter `gorm:"foreignKey:StackName; references: Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"optionalParameters"`
	Instances          []Instance               `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"instances"`
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
	Name      string `gorm:"primaryKey" json:"name"`
	StackName string `gorm:"primaryKey" json:"stackName"`
	Consumed  bool   `json:"consumed"`
}

type StackOptionalParameter struct {
	Name         string `gorm:"primaryKey" json:"name"`
	StackName    string `gorm:"primaryKey" json:"stackName"`
	DefaultValue string `json:"defaultValue"`
	Consumed     bool   `json:"consumed"`
}
