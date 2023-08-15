package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Chain struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	UserID uint  `json:"userId"`
	User   *User `json:"user,omitempty"`

	Name        string `json:"name" gorm:"index:idx_name_and_group,unique"`
	Description string `json:"description"`
	GroupName   string `json:"groupName" gorm:"index:idx_name_and_group,unique; references:Name"`
	Group       *Group `json:"group,omitempty"`
	TTL         uint   `json:"ttl"`

	Links []*Link `json:"links"`
}

type Parameters map[string]LinkParameter

type Link struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	ChainID uint   `json:"chainId"`
	Chain   *Chain `json:"chain,omitempty"`

	StackName string `json:"stackName" gorm:"references:Name"`
	Stack     *Stack `json:"stack,omitempty"`

	ParameterList []LinkParameter `json:"-" gorm:"foreignKey:LinkID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Parameters    Parameters      `json:"parameters" gorm:"-:all"`

	Preset   bool `json:"preset"`   // Whether this link is a preset
	PresetID uint `json:"presetId"` // The preset id this link is created from
	Public   bool `json:"public"`

	DeployLog string `json:"deployLog" gorm:"type:text"`
}

func (l *Link) BeforeSave(_ *gorm.DB) error {
	l.ParameterList = make([]LinkParameter, 0, len(l.Parameters))
	for _, parameter := range l.Parameters {
		parameter.LinkID = l.ID
		parameter.StackName = l.StackName
		l.ParameterList = append(l.ParameterList, parameter)
	}
	return nil
}

func (l *Link) AfterFind(_ *gorm.DB) error {
	l.Parameters = make(map[string]LinkParameter, len(l.ParameterList))
	for _, parameter := range l.ParameterList {
		l.Parameters[parameter.ParameterName] = parameter
	}
	return nil
}

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
	// The preset which this instance is created from
	PresetID  uint      `json:"presetId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
		if parameter.StackParameterID == name {
			return parameter, nil
		}
	}
	return InstanceParameter{}, fmt.Errorf("parameter not found: %s", name)
}

type LinkParameter struct {
	LinkID         uint           `json:"-" gorm:"primaryKey"`
	ParameterName  string         `json:"-" gorm:"primaryKey"`
	StackParameter StackParameter `json:"-" gorm:"foreignKey:ParameterName,StackName; references:Name,StackName; constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	StackName      string         `json:"-"`
	Value          string         `json:"value"`
}

type InstanceParameter struct {
	InstanceID uint `json:"-" gorm:"primaryKey"`
	// TODO: Rename StackParameterID to Name
	StackParameterID string         `json:"name" gorm:"primaryKey"`
	StackParameter   StackParameter `json:"-" gorm:"foreignKey:StackParameterID,StackName; references:Name,StackName; constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	StackName        string         `json:"-"`
	Value            string         `json:"value"`
}
