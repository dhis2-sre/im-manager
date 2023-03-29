package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// swagger:model Stack
type Stack struct {
	Name       string      `gorm:"primaryKey" json:"name"`
	Parameters []Parameter `gorm:"foreignKey:StackName; references: Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"parameters"`
	Instances  []Instance  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"instances"`

	HostnamePattern  string
	HostnameVariable string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (s Stack) GetHostname(name, namespace string) string {
	return fmt.Sprintf(s.HostnamePattern, name, namespace)
}

func (s Stack) FindParameter(name string) (Parameter, error) {
	for _, parameter := range s.Parameters {
		if parameter.Name == name {
			return parameter, nil
		}
	}
	return Parameter{}, fmt.Errorf("parameter not found: %q", name)
}

type Parameter struct {
	Name      string `gorm:"primaryKey"`
	StackName string `gorm:"primaryKey"`
	Value     string
	Consumed  bool
}
