package handler

import (
	"errors"

	"github.com/dhis2-sre/im-user/swagger/sdk/models"

	"github.com/gin-gonic/gin"
)

func GetUserFromContext(c *gin.Context) (*models.User, error) {
	userData, exists := c.Get("user")

	if !exists {
		return nil, errors.New("user not found on context")
	}

	user, ok := userData.(*models.User)
	if !ok {
		return nil, errors.New("failed to parse user data")
	}

	return user, nil
}
