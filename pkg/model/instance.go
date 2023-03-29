package model

import (
	"fmt"

	"gorm.io/gorm"
)

// swagger:model Instance
type Instance struct {
	gorm.Model
	UserID     uint
	Name       string `gorm:"index:idx_name_and_group,unique"`
	GroupName  string `gorm:"index:idx_name_and_group,unique"`
	StackName  string
	Parameters []InstanceParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"parameters"`
	DeployLog  string              `gorm:"type:text"`
	Preset     bool
	PresetID   uint
}

type Linked struct {
	SourceInstanceID      uint `gorm:"primaryKey"`
	SourceInstance        Instance
	DestinationStackName  string `gorm:"primaryKey"`
	DestinationInstanceID uint   `gorm:"index:idx_linked_second_instance,unique"`
	DestinationInstance   Instance
}

func (i Instance) FindParameter(name string) (InstanceParameter, error) {
	for _, parameter := range i.Parameters {
		if parameter.StackParameterName == name {
			return parameter, nil
		}
	}
	return InstanceParameter{}, fmt.Errorf("required parameter not found: %s", name)
}

type InstanceParameter struct {
	InstanceID         uint      `gorm:"primaryKey" json:"-"`
	StackParameterName string    `gorm:"primaryKey" json:"name"`
	StackParameter     Parameter `gorm:"foreignKey:StackParameterName,StackName; references:Name,StackName; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	StackName          string    `json:"-"`
	Value              string    `json:"value"`
}
