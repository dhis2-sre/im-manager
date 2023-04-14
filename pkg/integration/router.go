package integration

import (
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine, authenticationMiddleware handler.AuthenticationMiddleware, handler Handler) {
	tokenAuthenticationRouter := r.Group("")
	tokenAuthenticationRouter.Use(authenticationMiddleware.TokenAuthentication)

	tokenAuthenticationRouter.POST("/integrations", handler.Integrations)
}
