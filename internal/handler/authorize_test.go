package handler

import (
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestCanWriteDeployment_isOwnerAndMember(t *testing.T) {
	var userId uint = 123
	var group = "321"

	user := &model.User{
		ID: userId,
		Groups: []model.Group{
			{
				Name: group,
			},
		},
	}

	instance := &model.Deployment{
		UserID:    userId,
		GroupName: group,
	}

	isAdmin := CanWriteDeployment(user, instance)

	assert.True(t, isAdmin)
}

func TestCanWriteDeployment_isGroupAdministrator(t *testing.T) {
	var group = "123"

	user := &model.User{
		AdminGroups: []model.Group{
			{
				Name: group,
			},
		},
	}

	instance := &model.Deployment{GroupName: group}

	isAdmin := CanWriteDeployment(user, instance)

	assert.True(t, isAdmin)
}

func TestCanWriteDeployment_isAdministrator(t *testing.T) {
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

	instance := &model.Deployment{}

	isAdmin := CanWriteDeployment(user, instance)

	assert.True(t, isAdmin)
}

func TestCanReadDeployment_isMemberOf(t *testing.T) {
	var group = "123"

	user := &model.User{
		Groups: []model.Group{
			{
				Name: group,
			},
		},
	}

	instance := &model.Deployment{GroupName: group}

	isAdmin := CanReadDeployment(user, instance)

	assert.True(t, isAdmin)
}

func TestCanReadDeployment_isAdministrator(t *testing.T) {
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

	instance := &model.Deployment{}

	isAdmin := CanReadDeployment(user, instance)

	assert.True(t, isAdmin)
}

func TestCanReadDeployment_AccessDenied(t *testing.T) {
	user := &model.User{
		Groups: []model.Group{
			{
				Name: "123",
			},
		},
	}

	instance := &model.Deployment{}

	isAdmin := CanReadDeployment(user, instance)

	assert.False(t, isAdmin)
}
