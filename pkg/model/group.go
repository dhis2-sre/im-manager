package model

import "time"

const AdministratorGroupName = "administrators"

// Group domain object defining a group
// swagger:model
type Group struct {
	CreatedAt            time.Time             `json:"createdAt"`
	UpdatedAt            time.Time             `json:"updatedAt"`
	Name                 string                `gorm:"primarykey; unique;" json:"name"`
	Hostname             string                `gorm:"unique;" json:"hostname"`
	Users                []User                `gorm:"many2many:user_groups;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"users"`
	ClusterConfiguration *ClusterConfiguration `json:"clusterConfiguration"`
}

type ClusterConfiguration struct {
	ID                      uint      `gorm:"primarykey" json:"id"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
	GroupName               string    `json:"groupName"`
	KubernetesConfiguration []byte    `json:"kubernetesConfiguration"`
}
