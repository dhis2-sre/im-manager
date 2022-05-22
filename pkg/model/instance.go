package model

import "gorm.io/gorm"

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

type InstanceRequiredParameter struct {
	gorm.Model
	InstanceID               uint                   `gorm:"index:idx_instance_required_parameter,unique"`
	StackRequiredParameterID string                 `gorm:"index:idx_instance_required_parameter,unique"`
	StackRequiredParameter   StackRequiredParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Value                    string
}

type InstanceOptionalParameter struct {
	gorm.Model
	InstanceID               uint                   `gorm:"index:idx_instance_optional_parameter,unique"`
	StackOptionalParameterID string                 `gorm:"index:idx_instance_optional_parameter,unique"`
	StackOptionalParameter   StackOptionalParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Value                    string
}
