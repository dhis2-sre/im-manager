package stack

import (
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine, authenticator gin.HandlerFunc, handler Handler) {
	tokenAuthenticationRouter := r.Group("")
	tokenAuthenticationRouter.Use(authenticator)

	tokenAuthenticationRouter.GET("/stacks", handler.FindAll)
	tokenAuthenticationRouter.GET("/stacks/:name", handler.Find)
}
