package server

import (
	"github.com/dhis2-sre/im-manager/internal/di"
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/health"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func GetEngine(environment di.Environment) *gin.Engine {
	basePath := environment.Config.BasePath

	r := gin.Default()
	r.Use(cors.Default())
	r.Use(middleware.ErrorHandler())

	router := r.Group(basePath)
	router.GET("/health", health.Health)

	router.GET("/stacks", environment.StackHandler.FindAll)
	router.GET("/stacks/:id", environment.StackHandler.FindById)

	router.GET("/instances-name-to-id/:groupId/:name", environment.InstanceHandler.NameToId)

	tokenAuthenticationRouter := router.Group("")
	tokenAuthenticationRouter.Use(environment.AuthenticationMiddleware.TokenAuthentication)
	tokenAuthenticationRouter.POST("/instances", environment.InstanceHandler.Create)
	tokenAuthenticationRouter.DELETE("/instances/:id", environment.InstanceHandler.Delete)
	tokenAuthenticationRouter.GET("/instances/:id", environment.InstanceHandler.FindById)
	tokenAuthenticationRouter.GET("/instances/:id/logs", environment.InstanceHandler.Logs)

	return r
}
