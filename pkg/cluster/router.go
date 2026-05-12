package cluster

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

	administratorRestrictedRouter := tokenAuthenticationRouter.Group("")
	administratorRestrictedRouter.Use(authorizationMiddleware.RequireAdministrator)
	administratorRestrictedRouter.POST("/clusters", handler.Create)
	administratorRestrictedRouter.GET("/clusters", handler.FindAll)
	administratorRestrictedRouter.GET("/clusters/:id", handler.Find)
	administratorRestrictedRouter.PUT("/clusters/:id", handler.Update)
	administratorRestrictedRouter.DELETE("/clusters/:id", handler.Delete)
}
