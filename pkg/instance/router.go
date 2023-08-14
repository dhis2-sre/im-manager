package instance

import (
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine, authenticationMiddleware middleware.AuthenticationMiddleware, handler Handler) {
	r.GET("/public/instances", handler.ListPublicInstances)

	tokenAuthenticationRouter := r.Group("")
	tokenAuthenticationRouter.Use(authenticationMiddleware.TokenAuthentication)

	tokenAuthenticationRouter.POST("/instances", handler.Deploy)
	tokenAuthenticationRouter.GET("/instances", handler.ListInstances)
	tokenAuthenticationRouter.GET("/presets", handler.ListPresets)
	tokenAuthenticationRouter.GET("/instances/:id", handler.FindById)
	tokenAuthenticationRouter.GET("/instances/:id/parameters", handler.FindByIdDecrypted)
	tokenAuthenticationRouter.DELETE("/instances/:id", handler.Delete)
	tokenAuthenticationRouter.PUT("/instances/:id", handler.Update)
	tokenAuthenticationRouter.PUT("/instances/:id/reset", handler.Reset)
	tokenAuthenticationRouter.PUT("/instances/:id/pause", handler.Pause)
	tokenAuthenticationRouter.PUT("/instances/:id/resume", handler.Resume)
	tokenAuthenticationRouter.PUT("/instances/:id/restart", handler.Restart)
	tokenAuthenticationRouter.GET("/instances/:id/logs", handler.Logs)
	tokenAuthenticationRouter.GET("/instances-name-to-id/:groupName/:instanceName", handler.NameToId)

	tokenAuthenticationRouter.POST("/chains", handler.SaveChain)
	tokenAuthenticationRouter.POST("/chains/:id/link", handler.SaveLink)
}
