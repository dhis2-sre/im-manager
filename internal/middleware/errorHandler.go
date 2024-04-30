package middleware

import (
	"fmt"
	"net/http"

	"github.com/dhis2-sre/im-manager/internal/errdef"
	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		err := c.Errors.Last()
		if err == nil {
			return
		}
		if c.Writer.Status() != http.StatusOK {
			_, _ = c.Writer.WriteString(err.Error())
			return
		}

		// nolint:gocritic
		if errdef.IsBadRequest(err) {
			c.String(http.StatusBadRequest, err.Error())
		} else if errdef.IsForbidden(err) {
			c.String(http.StatusForbidden, err.Error())
		} else if errdef.IsDuplicated(err) {
			c.String(http.StatusConflict, err.Error())
		} else if errdef.IsNotFound(err) {
			c.String(http.StatusNotFound, err.Error())
		} else if errdef.IsUnauthorized(err) {
			c.String(http.StatusUnauthorized, err.Error())
		} else if errdef.IsConflict(err) {
			c.String(http.StatusConflict, err.Error())
		} else {
			id := sloggin.GetRequestID(c)
			err := fmt.Errorf("something went wrong. We'll look into it if you send us the id %q :)", id)
			c.String(http.StatusInternalServerError, err.Error())
		}
	}
}
