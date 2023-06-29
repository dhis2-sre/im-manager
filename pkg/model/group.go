package model

import "time"

const AdministratorGroupName = "administrators"

// Group domain object defining a group
// swagger:model
type Group struct {
	CreatedAt            time.Time             `json:"createdAt"`
	UpdatedAt            time.Time             `json:"updatedAt"`
	Name                 string                `json:"name" gorm:"primarykey; unique;"`
	Hostname             string                `json:"hostname" gorm:"unique;"`
	Deployable           bool                  `json:"deployable"`
	Users                []User                `json:"users" gorm:"many2many:user_groups;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ClusterConfiguration *ClusterConfiguration `json:"clusterConfiguration"`
}

type ClusterConfiguration struct {
	ID                      uint      `json:"id" gorm:"primarykey"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
	GroupName               string    `json:"groupName"`
	KubernetesConfiguration []byte    `json:"kubernetesConfiguration"`
}
