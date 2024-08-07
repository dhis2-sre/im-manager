package integration

import (
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine, authenticationMiddleware middleware.AuthenticationMiddleware, handler Handler) {
	tokenAuthenticationRouter := r.Group("")
	tokenAuthenticationRouter.Use(authenticationMiddleware.TokenAuthentication)

	tokenAuthenticationRouter.POST("/integrations", handler.Integrations)
	tokenAuthenticationRouter.GET("/integrations/image-exists/:repository/:tag", handler.ImageExists)
}
