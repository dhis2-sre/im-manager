package handler

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetTokenFromRequest(c *gin.Context) (string, error) {
	token := c.GetHeader("Authorization")
	if token != "" {
		return strings.TrimPrefix(token, "Bearer "), nil
	}

	cookie, err := c.Cookie("accessToken")
	if err != nil {
		return "", errors.New("no token found on request")
	}
	return cookie, nil
}
