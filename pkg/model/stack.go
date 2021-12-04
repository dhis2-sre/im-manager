package model

import "gorm.io/gorm"

type Stack struct {
	gorm.Model
	Name               string                   `gorm:"unique;"`
	RequiredParameters []StackRequiredParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"requiredParameters,omitempty"`
	OptionalParameters []StackOptionalParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"optionalParameters,omitempty"`
	Instances          []Instance               `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type StackRequiredParameter struct {
	gorm.Model
	StackID uint   `gorm:"index:idx_name_required_parameter,unique"`
	Name    string `gorm:"index:idx_name_required_parameter,unique"`
}

type StackOptionalParameter struct {
	gorm.Model
	StackID      uint   `gorm:"index:idx_name_optional_parameter,unique"`
	Name         string `gorm:"index:idx_name_optional_parameter,unique"`
	DefaultValue string `gorm:"index:idx_name_optional_parameter,unique"`
}
