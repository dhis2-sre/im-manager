package handler

import (
	"errors"
	"github.com/dhis2-sre/im-users/swagger/sdk/models"
	"github.com/gin-gonic/gin"
	"github.com/lestrrat-go/jwx/jwt"
	"strings"
)

func GetUserFromHttpAuthHeader(c *gin.Context) (*models.User, error) {
	tokenString := c.GetHeader("Authorization")
	if strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	token, err := jwt.Parse(
		[]byte(tokenString),
		//		jwt.WithValidate(true),
		// TODO: Should I verify here as well?
		//		jwt.WithVerify(jwa.RS256, key),
	)
	if err != nil {
		return nil, err
	}

	userData, ok := token.Get("user")
	if !ok {
		return nil, errors.New("user not found in claims")
	}

	userMap, ok := userData.(map[string]interface{})
	if !ok {
		return nil, errors.New("failed to parse user data")
	}

	id := userMap["ID"].(float64)
	email := userMap["Email"].(string)

	user := &models.User{
		ID:    uint64(id),
		Email: email,
	}

	return user, nil
}
