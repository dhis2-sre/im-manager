package user

// Test helpers used across packages. Kept in the main package so they can be
// reused by external tests that depend on user models.

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func CreateUserWithGroup(t *testing.T, db *gorm.DB, groupName, hostname, namespace, email string) (*model.User, *model.Group) {
	t.Helper()

	group := &model.Group{
		Name:      groupName,
		Hostname:  hostname,
		Namespace: namespace,
	}

	user := &model.User{
		Email:  email,
		Groups: []model.Group{*group},
	}

	err := db.Create(user).Error
	require.NoError(t, err, "failed to create test user with group")

	return user, group
}
