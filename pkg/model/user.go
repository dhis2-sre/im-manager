package model

import (
	"time"

	"github.com/google/uuid"
)

// User domain object defining a user
// swagger:model
type User struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Email       string    `json:"email" gorm:"index;unique"`
	EmailToken  uuid.UUID `json:"-" gorm:"unique;type:uuid"`
	Validated   bool      `json:"-"`
	Password    string    `json:"-"`
	Groups      []Group   `json:"groups" gorm:"many2many:user_groups;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	AdminGroups []Group   `json:"adminGroups" gorm:"many2many:user_groups_admin;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
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
