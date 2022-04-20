package handler

import (
	"errors"

	"github.com/gin-gonic/gin"
)

func GetUserFromContext(c *gin.Context) (User, error) {
	userData, exists := c.Get("user")

	if !exists {
		return User{}, errors.New("user not found on context")
	}

	user, ok := userData.(User)
	if !ok {
		return User{}, errors.New("failed to parse user data")
	}
	return user, nil
}
