package model

import (
	"gorm.io/gorm"
)

// User domain object defining a user
// swagger:model
type User struct {
	gorm.Model
	Email       string  `gorm:"index;unique"`
	Password    string  `json:"-"`
	Groups      []Group `gorm:"many2many:user_groups;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	AdminGroups []Group `gorm:"many2many:user_groups_admin;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (u *User) IsMemberOf(group string) bool {
	return u.contains(group, u.Groups)
}

func (u *User) IsAdminOf(group string) bool {
	return u.contains(group, u.AdminGroups)
}

func (u *User) contains(group string, groups []Group) bool {
	for _, g := range groups {
		if group == g.Name {
			return true
		}
	}
	return false
}

func (u *User) IsAdministrator() bool {
	return u.IsMemberOf(AdministratorGroupName)
}
