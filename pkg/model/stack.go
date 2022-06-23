package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Stack struct {
	Name               string                   `gorm:"primaryKey" json:"name"`
	RequiredParameters []StackRequiredParameter `gorm:"foreignKey:StackName; references: Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"requiredParameters"`
	OptionalParameters []StackOptionalParameter `gorm:"foreignKey:StackName; references: Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"optionalParameters"`
	Instances          []Instance               `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"instances"`

	HostnamePattern  string
	HostnameVariable string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
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
	Name      string `gorm:"primaryKey"`
	StackName string `gorm:"primaryKey"`
	Consumed  bool
}

type StackOptionalParameter struct {
	Name         string `gorm:"primaryKey"`
	StackName    string `gorm:"primaryKey"`
	DefaultValue string
	Consumed     bool
}
