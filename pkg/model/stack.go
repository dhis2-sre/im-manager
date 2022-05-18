package model

import "gorm.io/gorm"

type Stack struct {
	gorm.Model
	Name               string                   `gorm:"unique"`
	RequiredParameters []StackRequiredParameter `gorm:"many2many:required_stack_parameters_joins; foreignKey:ID; joinForeignKey:StackID; References:ID; joinReferences:ParameterID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"requiredParameters,omitempty"`
	OptionalParameters []StackOptionalParameter `gorm:"many2many:optional_stack_parameters_joins; foreignKey:ID; joinForeignKey:StackID; References:ID; joinReferences:ParameterID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"optionalParameters,omitempty"`
	Instances          []Instance               `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type RequiredStackParametersJoin struct {
	StackID     uint   `gorm:"primaryKey"`
	ParameterID string `gorm:"primaryKey"`
}

type OptionalStackParametersJoin struct {
	StackID      uint   `gorm:"primaryKey"`
	ParameterID  string `gorm:"primaryKey"`
	DefaultValue string
}

type StackRequiredParameter struct {
	ID string `gorm:"primaryKey"`
}

type StackOptionalParameter struct {
	ID string `gorm:"primaryKey"`
}
