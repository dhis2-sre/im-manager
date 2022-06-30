package model

import (
	"fmt"

	"gorm.io/gorm"
)

// swagger:model Instance
type Instance struct {
	gorm.Model
	Name               string `gorm:"index:idx_name_and_group,unique"`
	UserID             uint
	GroupName          string `gorm:"index:idx_name_and_group,unique"`
	StackName          string
	RequiredParameters []InstanceRequiredParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"requiredParameters,omitempty"`
	OptionalParameters []InstanceOptionalParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"optionalParameters,omitempty"`
	DeployLog          string                      `gorm:"type:text"`
}

// TODO: https://gorm.io/docs/has_one.html#Override-Foreign-Key
type Linked struct {
	FirstInstanceID  uint   `gorm:"primaryKey"`
	StackName        string `gorm:"primaryKey"`
	SecondInstanceID uint   `gorm:"index:idx_linked_second_instance,unique"`
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
	gorm.Model
	InstanceID               uint   `gorm:"index:idx_instance_required_parameter,unique"`
	StackRequiredParameterID string `gorm:"index:idx_instance_required_parameter,unique"`
	StackName                string
	StackRequiredParameter   StackRequiredParameter `gorm:"foreignKey:StackName,StackRequiredParameterID; references:StackName,Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Value                    string
}

type InstanceOptionalParameter struct {
	gorm.Model
	InstanceID               uint   `gorm:"index:idx_instance_optional_parameter,unique"`
	StackOptionalParameterID string `gorm:"index:idx_instance_optional_parameter,unique"`
	StackName                string
	StackOptionalParameter   StackOptionalParameter `gorm:"foreignKey:StackName,StackOptionalParameterID; references:StackName,Name; constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Value                    string
}
