package group

import (
	"github.com/gin-gonic/gin"
)

type AuthenticationMiddleware interface {
	TokenAuthentication(context *gin.Context)
}

type AuthorizationMiddleware interface {
	RequireAdministrator(context *gin.Context)
}

func Routes(r *gin.Engine, authenticationMiddleware AuthenticationMiddleware, authorizationMiddleware AuthorizationMiddleware, handler Handler) {
	tokenAuthenticationRouter := r.Group("")
	tokenAuthenticationRouter.Use(authenticationMiddleware.TokenAuthentication)

	tokenAuthenticationRouter.GET("/groups/:name", handler.Find)
	tokenAuthenticationRouter.GET("/groups/:name/details", handler.FindWithDetails)
	tokenAuthenticationRouter.GET("/groups/:name/resources", handler.FindResources)
	tokenAuthenticationRouter.GET("/groups", handler.FindAll)

	administratorRestrictedRouter := tokenAuthenticationRouter.Group("")
	administratorRestrictedRouter.Use(authorizationMiddleware.RequireAdministrator)
	administratorRestrictedRouter.POST("/groups", handler.Create)
	administratorRestrictedRouter.POST("/groups/:group/users/:userId", handler.AddUserToGroup)
	administratorRestrictedRouter.DELETE("/groups/:group/users/:userId", handler.RemoveUserFromGroup)
	administratorRestrictedRouter.POST("/groups/:group/cluster-configuration", handler.AddClusterConfiguration)
}
