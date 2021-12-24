package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"strings"
)

func GetTokenFromHttpAuthHeader(c *gin.Context) (string, error) {
	token := c.GetHeader("Authorization")

	token = strings.TrimPrefix(token, "Bearer ")

	if token == "" {
		return "", errors.New("token not found in Authorization header")
	}

	return token, nil
}
