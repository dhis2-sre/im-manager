package model

import (
	"fmt"
	"time"
)

// swagger:model Instance
type Instance struct {
	ID                 uint                        `gorm:"primarykey" json:"id"`
	CreatedAt          time.Time                   `json:"createdAt"`
	UpdatedAt          time.Time                   `json:"updatedAt"`
	UserID             uint                        `json:"userId"`
	Name               string                      `gorm:"index:idx_name_and_group,unique" json:"name"`
	GroupName          string                      `gorm:"index:idx_name_and_group,unique" json:"groupName"`
	StackName          string                      `json:"stackName"`
	RequiredParameters []InstanceRequiredParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"requiredParameters"`
	OptionalParameters []InstanceOptionalParameter `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"optionalParameters"`
	DeployLog          string                      `gorm:"type:text" json:"deployLog"`
	Preset             bool                        `json:"preset"`
	PresetID           uint                        `json:"presetId"`
}

type Linked struct {
	SourceInstanceID      uint `gorm:"primaryKey"`
	SourceInstance        Instance
	DestinationStackName  string `gorm:"primaryKey"`
	DestinationInstanceID uint   `gorm:"index:idx_linked_second_instance,unique"`
	DestinationInstance   Instance
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

type InstanceOptionalParameter struct {
	InstanceID uint `gorm:"primaryKey" json:"-"`
	// TODO: Rename StackOptionalParameterID to Name
	StackOptionalParameterID string                 `gorm:"primaryKey" json:"name"`
	StackOptionalParameter   StackOptionalParameter `gorm:"foreignKey:StackOptionalParameterID,StackName; references:Name,StackName; constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	StackName                string                 `json:"-"`
	Value                    string                 `json:"value"`
}
