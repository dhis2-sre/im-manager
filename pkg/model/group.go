package model

import (
	"time"

	"gorm.io/gorm"
)

const AdministratorGroupName = "administrators"

// Group domain object defining a group
// swagger:model
type Group struct {
	Name                 string                `gorm:"primarykey; unique;"`
	Hostname             string                `gorm:"unique;"`
	Users                []User                `gorm:"many2many:user_groups;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ClusterConfiguration *ClusterConfiguration `json:"clusterConfiguration"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type ClusterConfiguration struct {
	gorm.Model
	GroupName               string
	KubernetesConfiguration []byte
}
