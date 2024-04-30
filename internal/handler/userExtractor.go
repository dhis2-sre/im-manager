package handler

import (
	"errors"

	"github.com/dhis2-sre/im-manager/pkg/model"
	"github.com/gin-gonic/gin"
)

func GetUserFromContext(c *gin.Context) (*model.User, error) {
	userData, exists := c.Get("user")
	if !exists {
		return nil, errors.New("user not found on context")
	}

	user, ok := userData.(*model.User)
	if !ok {
		return nil, errors.New("failed to parse user data from context")
	}

	return user, nil
}
