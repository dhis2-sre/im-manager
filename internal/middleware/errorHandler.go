package middleware

import (
	"github.com/dhis2-sre/im-manager/internal/apperror"
	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return errorHandlerT(gin.ErrorTypeAny)
}

func errorHandlerT(errType gin.ErrorType) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		detectedErrors := c.Errors.ByType(errType)

		if len(detectedErrors) > 0 {
			// TODO: Handle more than one error
			err := detectedErrors[0].Err
			c.String(apperror.ToHttpStatusCode(err), err.Error())
			c.Abort()
			return
		}
	}
}
