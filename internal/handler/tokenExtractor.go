package handler

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetTokenFromRequest(c *gin.Context) (string, error) {
	token := c.GetHeader("Authorization")
	if token == "" {
		cookie, err := c.Cookie("accessToken")
		if err != nil {
			return "", err
		}
		token = cookie
	}

	token = strings.TrimPrefix(token, "Bearer ")

	if token == "" {
		return "", errors.New("token not found on request")
	}

	return token, nil
}
