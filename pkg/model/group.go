package model

import "time"

const AdministratorGroupName = "administrators"
const DefaultGroupName = "whoami"

// Group domain object defining a group
// swagger:model
type Group struct {
	Name        string    `json:"name" gorm:"primarykey; unique;"`
	Namespace   string    `json:"namespace"`
	Description string    `json:"description" gorm:"type:text"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Hostname    string    `json:"hostname" gorm:"unique;"`
	Deployable  bool      `json:"deployable"`
	Autoscaled  bool      `json:"autoscaled"`
	Users       []User    `json:"users" gorm:"many2many:user_groups;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	AdminUsers  []User    `json:"adminUsers" gorm:"many2many:user_groups_admin;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	// TODO: Shouldn't this be json:"-"?
	ClusterConfiguration *ClusterConfiguration `json:"clusterConfiguration"`
}

type ClusterConfiguration struct {
	ID                      uint      `json:"id" gorm:"primarykey"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
	GroupName               string    `json:"groupName"`
	KubernetesConfiguration []byte    `json:"kubernetesConfiguration"`
}
