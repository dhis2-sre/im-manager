package model

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// swagger:model Instance
type Instance struct {
	gorm.Model
	UserID             uint
	Name               string `gorm:"index:idx_name_and_group,unique"`
	GroupName          string `gorm:"index:idx_name_and_group,unique"`
	StackName          string
	RequiredParameters []InstanceRequiredParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"requiredParameters"`
	OptionalParameters []InstanceOptionalParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"optionalParameters"`
	DeployLog          string                      `gorm:"type:text"`
}

// TODO: https://gorm.io/docs/has_one.html#Override-Foreign-Key
type Linked struct {
	FirstInstanceID  uint `gorm:"primaryKey"`
	FirstInstance    Instance
	StackName        string `gorm:"primaryKey"`
	SecondInstanceID uint   `gorm:"index:idx_linked_second_instance,unique"`
	SecondInstance   Instance
}

func (i Instance) FindRequiredParameter(name string) (InstanceRequiredParameter, error) {
	for _, parameter := range i.RequiredParameters {
		if parameter.StackRequiredParameterID == name {
			return parameter, nil
		}
	}
	return InstanceRequiredParameter{}, fmt.Errorf("required parameter not found: %s", name)
}

func (i Instance) FindOptionalParameter(name string) (InstanceOptionalParameter, error) {
	for _, parameter := range i.OptionalParameters {
		if parameter.StackOptionalParameterID == name {
			return parameter, nil
		}
	}
	return InstanceOptionalParameter{}, fmt.Errorf("optional parameter not found: %s", name)
}

type InstanceRequiredParameter struct {
	InstanceID uint `gorm:"primaryKey" json:"-"`
	// TODO: Rename StackRequiredParameterID to Name
	StackRequiredParameterID string                 `gorm:"primaryKey" json:"name"`
	StackRequiredParameter   StackRequiredParameter `gorm:"foreignKey:StackRequiredParameterID,StackName; references:Name,StackName; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	StackName                string                 `json:"-"`
	Value                    string                 `json:"value"`
}

func (irp InstanceRequiredParameter) GoString() string {
	bytes, _ := json.MarshalIndent(irp, "", "  ")
	return string(bytes)
}

type InstanceOptionalParameter struct {
	InstanceID uint `gorm:"primaryKey" json:"-"`
	// TODO: Rename StackOptionalParameterID to Name
	StackOptionalParameterID string                 `gorm:"primaryKey" json:"name"`
	StackOptionalParameter   StackOptionalParameter `gorm:"foreignKey:StackOptionalParameterID,StackName; references:Name,StackName; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	StackName                string                 `json:"-"`
	Value                    string                 `json:"value"`
}
