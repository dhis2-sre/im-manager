package model

import (
	"time"

	"gorm.io/gorm"
)

type Stack struct {
	Name               string                   `gorm:"primaryKey" json:"name"`
	RequiredParameters []StackRequiredParameter `gorm:"many2many:required_stack_parameters_joins; foreignKey:Name; References:Name; joinForeignKey:StackName; joinReferences:ParameterID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"requiredParameters"`
	OptionalParameters []StackOptionalParameter `gorm:"many2many:optional_stack_parameters_joins; foreignKey:Name; References:Name; joinForeignKey:StackName; joinReferences:ParameterID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"optionalParameters"`
	Instances          []Instance               `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"instances"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type RequiredStackParametersJoin struct {
	StackName   string `gorm:"primaryKey"`
	ParameterID string `gorm:"primaryKey"`
}

type OptionalStackParametersJoin struct {
	StackName    string `gorm:"primaryKey"`
	ParameterID  string `gorm:"primaryKey"`
	DefaultValue string
}

type StackRequiredParameter struct {
	Name string `gorm:"primaryKey"`
}

type StackOptionalParameter struct {
	Name string `gorm:"primaryKey"`
}
