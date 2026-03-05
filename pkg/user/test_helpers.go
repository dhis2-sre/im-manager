package user

// Test helpers used across packages. Kept in the main package so they can be
// reused by external tests that depend on user models.

import (
	"github.com/dhis2-sre/im-manager/pkg/model"
	"gorm.io/gorm"
)

func CreateTestUserWithGroup(db *gorm.DB, groupName, hostname, namespace, email string) (*model.User, *model.Group) {
	group := &model.Group{
		Name:      groupName,
		Hostname:  hostname,
		Namespace: namespace,
	}

	user := &model.User{
		Email:  email,
		Groups: []model.Group{*group},
	}

	db.Create(user)

	return user, group
}
