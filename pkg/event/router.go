package event

import (
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine, authenticator gin.HandlerFunc, handler Handler) {
	queryStringAuthenticationRouter := r.Group("")
	queryStringAuthenticationRouter.Use(authenticator)
	queryStringAuthenticationRouter.GET("/subscribe", handler.Subscribe)
}
