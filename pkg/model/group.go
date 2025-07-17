package model

import "time"

const AdministratorGroupName = "administrators"
const DefaultGroupName = "whoami"

// Group domain object defining a group
// swagger:model
type Group struct {
	Name        string    `json:"name" gorm:"primaryKey; unique;"`
	Namespace   string    `json:"namespace"`
	Description string    `json:"description" gorm:"type:text"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Hostname    string    `json:"hostname" gorm:"unique;"`
	Deployable  bool      `json:"deployable"`
	Autoscaled  bool      `json:"autoscaled"`
	Users       []User    `json:"users" gorm:"many2many:user_groups;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	AdminUsers  []User    `json:"adminUsers" gorm:"many2many:user_groups_admin;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ClusterID   *uint     `json:"clusterId"`
	Cluster     Cluster   `json:"cluster" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
}
