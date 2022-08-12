package handler

import (
	"testing"

	"github.com/dhis2-sre/im-user/swagger/sdk/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetUserFromContext(t *testing.T) {
	id := uint64(1000)
	email := "some@thing.dk"
	groupName := "whoami"
	groupHostname := "whoami.org"
	groups := []*models.Group{
		{
			Name:     groupName,
			Hostname: groupHostname,
		},
		{
			Name:     "play",
			Hostname: "play.org",
		},
	}
	adminGroups := []*models.Group{
		{
			Name:     groupName,
			Hostname: groupHostname,
		},
	}
	user := &models.User{
		ID:          id,
		Email:       email,
		Groups:      groups,
		AdminGroups: adminGroups,
	}

	c := &gin.Context{}

	c.Set("user", user)

	u, err := GetUserFromContext(c)
	assert.NoError(t, err)

	assert.Equal(t, id, u.ID)
	assert.Equal(t, email, u.Email)
	assert.Equal(t, 2, len(u.Groups))
	assert.Equal(t, 1, len(u.AdminGroups))
	assert.Equal(t, groupName, u.AdminGroups[0].Name)
	assert.Equal(t, groupHostname, u.AdminGroups[0].Hostname)
}
