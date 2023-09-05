package model

import (
	"fmt"
	"time"
)

// swagger:model Instance
type Instance struct {
	ID          uint                `json:"id" gorm:"primarykey"`
	User        User                `json:"user"`
	UserID      uint                `json:"userId"`
	Name        string              `json:"name" gorm:"index:idx_name_and_group,unique"`
	Group       Group               `json:"group"`
	GroupName   string              `json:"groupName" gorm:"index:idx_name_and_group,unique"`
	Description string              `json:"description"`
	StackName   string              `json:"stackName"`
	TTL         uint                `json:"ttl"`
	Parameters  []InstanceParameter `json:"parameters" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	DeployLog   string              `json:"deployLog" gorm:"type:text"`
	Preset      bool                `json:"preset"`
	Public      bool                `json:"public"`
	PresetID    uint                `json:"presetId"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

type Linked struct {
	SourceInstanceID      uint     `json:"sourceInstanceId" gorm:"primaryKey"`
	SourceInstance        Instance `json:"sourceInstance"`
	DestinationStackName  string   `json:"destinationStackName" gorm:"primaryKey"`
	DestinationInstanceID uint     `json:"destinationInstanceId" gorm:"index:idx_linked_second_instance,unique"`
	DestinationInstance   Instance `json:"destinationInstance"`
}

func (i Instance) FindParameter(name string) (InstanceParameter, error) {
	for _, parameter := range i.Parameters {
		if parameter.StackParameterName == name {
			return parameter, nil
		}
	}
	return InstanceParameter{}, fmt.Errorf("parameter not found: %s", name)
}

type InstanceParameter struct {
	InstanceID         uint   `json:"-" gorm:"primaryKey"`
	StackParameterName string `json:"name" gorm:"primaryKey"`
	Value              string `json:"value"`
}
