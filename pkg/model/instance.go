package model

import (
	"fmt"
	"time"
)

// swagger:model Instance
type Instance struct {
	ID                 uint                        `json:"id" gorm:"primarykey"`
	User               User                        `json:"user"`
	UserID             uint                        `json:"userId"`
	Name               string                      `json:"name" gorm:"index:idx_name_and_group,unique"`
	Group              Group                       `json:"group" gorm:"index:idx_name_and_group,unique"`
	GroupName          string                      `json:"groupName" gorm:"index:idx_name_and_group,unique"`
	Description        string                      `json:"description"`
	StackName          string                      `json:"stackName"`
	TTL                uint                        `json:"ttl"`
	RequiredParameters []InstanceRequiredParameter `json:"requiredParameters" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	OptionalParameters []InstanceOptionalParameter `json:"optionalParameters" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	DeployLog          string                      `json:"deployLog" gorm:"type:text"`
	Preset             bool                        `json:"preset"`
	PresetID           uint                        `json:"presetId"`
	CreatedAt          time.Time                   `json:"createdAt"`
	UpdatedAt          time.Time                   `json:"updatedAt"`
}

type Linked struct {
	SourceInstanceID      uint     `json:"sourceInstanceId" gorm:"primaryKey"`
	SourceInstance        Instance `json:"sourceInstance"`
	DestinationStackName  string   `json:"destinationStackName" gorm:"primaryKey"`
	DestinationInstanceID uint     `json:"destinationInstanceId" gorm:"index:idx_linked_second_instance,unique"`
	DestinationInstance   Instance `json:"destinationInstance"`
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
	InstanceID uint `json:"-" gorm:"primaryKey"`
	// TODO: Rename StackRequiredParameterID to Name
	StackRequiredParameterID string                 `json:"name" gorm:"primaryKey"`
	StackRequiredParameter   StackRequiredParameter `json:"-" gorm:"foreignKey:StackRequiredParameterID,StackName; references:Name,StackName; constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	StackName                string                 `json:"-"`
	Value                    string                 `json:"value"`
}

type InstanceOptionalParameter struct {
	InstanceID uint `json:"-" gorm:"primaryKey"`
	// TODO: Rename StackOptionalParameterID to Name
	StackOptionalParameterID string                 `json:"name" gorm:"primaryKey"`
	StackOptionalParameter   StackOptionalParameter `json:"-" gorm:"foreignKey:StackOptionalParameterID,StackName; references:Name,StackName; constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	StackName                string                 `json:"-"`
	Value                    string                 `json:"value"`
}
