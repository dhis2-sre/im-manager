package model

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// User domain object defining a user
// swagger:model
type User struct {
	ID               uint           `json:"id" gorm:"primaryKey"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	Email            string         `json:"email" gorm:"index;unique"`
	EmailToken       uuid.UUID      `json:"-" gorm:"unique;type:uuid"`
	Validated        bool           `json:"validated"`
	Password         string         `json:"-"`
	PasswordToken    sql.NullString `json:"-"`
	PasswordTokenTTL uint           `json:"-"`
	Groups           []Group        `json:"groups" gorm:"many2many:user_groups;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	AdminGroups      []Group        `json:"adminGroups" gorm:"many2many:user_groups_admin;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
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

func (u *User) LogValue() slog.Value {
	return slog.Uint64Value(uint64(u.ID))
}

type ctxKey int

var userKey ctxKey

// NewContextWithUser returns a new [context.Context] that carries value user.
func NewContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// GetUserFromContext returns the User value stored in ctx, if any.
func GetUserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userKey).(*User)
	return user, ok
}
