package group

import (
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine, authenticationMiddleware middleware.AuthenticationMiddleware, authorizationMiddleware middleware.AuthorizationMiddleware, handler Handler) {
	tokenAuthenticationRouter := r.Group("")
	tokenAuthenticationRouter.Use(authenticationMiddleware.TokenAuthentication)

	tokenAuthenticationRouter.GET("/groups/:name", handler.Find)

	administratorRestrictedRouter := tokenAuthenticationRouter.Group("")
	administratorRestrictedRouter.Use(authorizationMiddleware.RequireAdministrator)
	administratorRestrictedRouter.POST("/groups", handler.Create)
	administratorRestrictedRouter.POST("/groups/:group/users/:userId", handler.AddUserToGroup)
	administratorRestrictedRouter.POST("/groups/:group/cluster-configuration", handler.AddClusterConfiguration)
}
