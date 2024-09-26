package model

import (
	"time"

	"gorm.io/gorm"
)

type Deployment struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	UserID uint  `json:"userId"`
	User   *User `json:"user,omitempty"`

	Name        string `json:"name" gorm:"index:deployment_name_group_idx,unique"`
	Description string `json:"description"`
	GroupName   string `json:"groupName" gorm:"index:deployment_name_group_idx,unique; references:Name"`
	Group       *Group `json:"group,omitempty"`

	TTL uint `json:"ttl"`

	Instances []*DeploymentInstance `json:"instances" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

type DeploymentInstanceParameters map[string]DeploymentInstanceParameter

type DeploymentInstance struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// TODO: FK to name of Deployment?
	Name      string `json:"name" gorm:"index:deployment_instance_name_group_stack_idx,unique"`
	Group     *Group `json:"group,omitempty"`
	GroupName string `json:"groupName" gorm:"index:deployment_instance_name_group_stack_idx,unique; references:Name"`
	//	Stack     *Stack `json:"stack,omitempty"`
	StackName string `json:"stackName" gorm:"index:deployment_instance_name_group_stack_idx,unique"`

	DeploymentID uint        `json:"deploymentId"`
	Deployment   *Deployment `json:"deployment,omitempty"`

	GormParameters []DeploymentInstanceParameter `json:"-" gorm:"foreignKey:DeploymentInstanceID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Parameters     DeploymentInstanceParameters  `json:"parameters" gorm:"-:all"`

	Public bool `json:"public"`

	DeployLog string `json:"deployLog" gorm:"type:text"`
}

type DeploymentInstanceParameter struct {
	DeploymentInstanceID uint   `json:"-" gorm:"primaryKey"`
	ParameterName        string `json:"-" gorm:"primaryKey"`
	StackName            string `json:"-"`
	Value                string `json:"value"`
}

func (i *DeploymentInstance) BeforeSave(_ *gorm.DB) error {
	i.GormParameters = make([]DeploymentInstanceParameter, 0, len(i.Parameters))
	for name, parameter := range i.Parameters {
		parameter.ParameterName = name
		parameter.DeploymentInstanceID = i.ID
		parameter.StackName = i.StackName
		i.GormParameters = append(i.GormParameters, parameter)
	}
	return nil
}

func (i *DeploymentInstance) AfterFind(_ *gorm.DB) error {
	i.Parameters = make(map[string]DeploymentInstanceParameter, len(i.GormParameters))
	for _, parameter := range i.GormParameters {
		i.Parameters[parameter.ParameterName] = parameter
	}
	return nil
}
