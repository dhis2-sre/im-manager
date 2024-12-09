package model_test

import (
	"context"
	"testing"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/stretchr/testify/assert"
)

func TestUserContext(t *testing.T) {
	id := uint(1000)
	email := "some@thing.dk"
	groupName := "whoami"
	groupHostname := "whoami.org"
	groups := []model.Group{
		{
			Name:     groupName,
			Hostname: groupHostname,
		},
		{
			Name:     "play",
			Hostname: "play.org",
		},
	}
	adminGroups := []model.Group{
		{
			Name:     groupName,
			Hostname: groupHostname,
		},
	}
	user := &model.User{
		ID:          id,
		Email:       email,
		Groups:      groups,
		AdminGroups: adminGroups,
	}

	ctx := context.Background()

	got, ok := model.GetUserFromContext(ctx)
	assert.Nil(t, got, "want nil when no user is in the context")
	assert.False(t, ok, "want an error when no user is in the context")

	ctx = model.NewContextWithUser(ctx, user)

	got, ok = model.GetUserFromContext(ctx)
	assert.True(t, ok)

	assert.Equal(t, id, got.ID)
	assert.Equal(t, email, got.Email)
	assert.Equal(t, 2, len(got.Groups))
	assert.Equal(t, 1, len(got.AdminGroups))
	assert.Equal(t, groupName, got.AdminGroups[0].Name)
	assert.Equal(t, groupHostname, got.AdminGroups[0].Hostname)
}
