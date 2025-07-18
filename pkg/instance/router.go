package instance

import (
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine, authenticator gin.HandlerFunc, handler Handler) {
	r.GET("/instances/public", handler.FindPublicInstances)

	tokenAuthenticationRouter := r.Group("")
	tokenAuthenticationRouter.Use(authenticator)

	tokenAuthenticationRouter.PUT("/instances/:id/reset", handler.Reset)
	tokenAuthenticationRouter.PUT("/instances/:id/pause", handler.Pause)
	tokenAuthenticationRouter.PUT("/instances/:id/resume", handler.Resume)
	tokenAuthenticationRouter.PUT("/instances/:id/restart", handler.Restart)
	tokenAuthenticationRouter.GET("/instances/:id/logs", handler.Logs)
	tokenAuthenticationRouter.GET("/instances/:id/status", handler.Status)
	tokenAuthenticationRouter.GET("/instances/:id/details", handler.InstanceWithDetails)

	tokenAuthenticationRouter.POST("/deployments", handler.SaveDeployment)
	tokenAuthenticationRouter.GET("/deployments", handler.FindDeployments)
	tokenAuthenticationRouter.GET("/deployments/:id", handler.FindDeploymentById)
	tokenAuthenticationRouter.DELETE("/deployments/:id", handler.DeleteDeployment)
	tokenAuthenticationRouter.POST("/deployments/:id/instance", handler.SaveInstance)
	tokenAuthenticationRouter.PUT("/deployments/:id/instance/:instanceId", handler.UpdateInstance)
	tokenAuthenticationRouter.DELETE("/deployments/:id/instance/:instanceId", handler.DeleteDeploymentInstance)
	tokenAuthenticationRouter.POST("/deployments/:id/deploy", handler.DeployDeployment)
	tokenAuthenticationRouter.PUT("/deployments/:id", handler.UpdateDeployment)
}
