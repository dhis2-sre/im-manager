package handler

import (
	"testing"

	"gorm.io/gorm"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestCanWriteInstance_isOwnerAndMember(t *testing.T) {
	var userId uint = 123
	var group = "321"

	user := &model.User{
		Model: gorm.Model{ID: userId},
		Groups: []model.Group{
			{
				Name: group,
			},
		},
	}

	instance := &model.Instance{
		UserID:    userId,
		GroupName: group,
	}

	isAdmin := CanWriteInstance(user, instance)

	assert.True(t, isAdmin)
}

func TestCanWriteInstance_isGroupAdministrator(t *testing.T) {
	var group = "123"

	user := &model.User{
		AdminGroups: []model.Group{
			{
				Name: group,
			},
		},
	}

	instance := &model.Instance{GroupName: group}

	isAdmin := CanWriteInstance(user, instance)

	assert.True(t, isAdmin)
}

func TestCanWriteInstance_isAdministrator(t *testing.T) {
	user := &model.User{
		Groups: []model.Group{
			{
				Name: AdministratorGroupName,
			},
			{
				Name: "other group",
			},
		},
	}

	instance := &model.Instance{}

	isAdmin := CanWriteInstance(user, instance)

	assert.True(t, isAdmin)
}

func TestCanReadInstance_isMemberOf(t *testing.T) {
	var group = "123"

	user := &model.User{
		Groups: []model.Group{
			{
				Name: group,
			},
		},
	}

	instance := &model.Instance{GroupName: group}

	isAdmin := CanReadInstance(user, instance)

	assert.True(t, isAdmin)
}

func TestCanReadInstance_isAdministrator(t *testing.T) {
	user := &model.User{
		Groups: []model.Group{
			{
				Name: AdministratorGroupName,
			},
			{
				Name: "other group",
			},
		},
	}

	instance := &model.Instance{}

	isAdmin := CanReadInstance(user, instance)

	assert.True(t, isAdmin)
}

func TestCanReadInstance_AccessDenied(t *testing.T) {
	user := &model.User{
		Groups: []model.Group{
			{
				Name: "123",
			},
		},
	}

	instance := &model.Instance{}

	isAdmin := CanReadInstance(user, instance)

	assert.False(t, isAdmin)
}
