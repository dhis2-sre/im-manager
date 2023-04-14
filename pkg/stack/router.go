package stack

import (
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine, authenticationMiddleware handler.AuthenticationMiddleware, handler Handler) {
	tokenAuthenticationRouter := r.Group("")
	tokenAuthenticationRouter.Use(authenticationMiddleware.TokenAuthentication)

	tokenAuthenticationRouter.GET("/stacks", handler.FindAll)
	tokenAuthenticationRouter.GET("/stacks/:name", handler.Find)
}
