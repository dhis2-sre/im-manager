package handler

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetTokenFromRequest(c *gin.Context) (string, error) {
	// The authentication middleware accepts the token from the Authorization header or from either
	// cookie, so a request can carry an Authorization header belonging to a different scheme and
	// still be authenticated by its cookie. Handing that header on as a token yields "invalid JWT"
	// on a request that authenticated perfectly well, so anything that is not a bearer token falls
	// through to the cookie. Callers that send the bare token without a scheme still work.
	header := c.GetHeader("Authorization")
	if scheme, credentials, found := strings.Cut(header, " "); !found {
		if header != "" {
			return header, nil
		}
	} else if strings.EqualFold(scheme, "Bearer") {
		return credentials, nil
	}

	cookie, err := c.Cookie("accessToken")
	if err != nil {
		return "", errors.New("no token found on request")
	}
	return cookie, nil
}
