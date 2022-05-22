package handler

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"github.com/stretchr/testify/assert"
)

func TestCanWriteInstance_isOwnerAndMember(t *testing.T) {
	var userId uint64 = 123
	var groupName = "321"

	user := &models.User{
		ID: userId,
		Groups: []*models.Group{
			{
				Name: groupName,
			},
		},
	}

	instance := &model.Instance{
		UserID:    uint(userId),
		GroupName: groupName,
	}

	isAdmin := CanWriteInstance(user, instance)

	assert.True(t, isAdmin)
}

func TestCanWriteInstance_isGroupAdministrator(t *testing.T) {
	var groupName = "123"

	user := &models.User{
		AdminGroups: []*models.Group{
			{
				Name: groupName,
			},
		},
	}

	instance := &model.Instance{GroupName: groupName}

	isAdmin := CanWriteInstance(user, instance)

	assert.True(t, isAdmin)
}

func TestCanWriteInstance_isAdministrator(t *testing.T) {
	user := &models.User{
		Groups: []*models.Group{
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
	var groupName = "123"

	user := &models.User{
		Groups: []*models.Group{
			{
				Name: groupName,
			},
		},
	}

	instance := &model.Instance{GroupName: groupName}

	isAdmin := CanReadInstance(user, instance)

	assert.True(t, isAdmin)
}

func TestCanReadInstance_isAdministrator(t *testing.T) {
	user := &models.User{
		Groups: []*models.Group{
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
	user := &models.User{
		Groups: []*models.Group{
			{
				Name: "123",
			},
		},
	}

	instance := &model.Instance{}

	isAdmin := CanReadInstance(user, instance)

	assert.False(t, isAdmin)
}
