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

	router.POST("/instances", environment.InstanceHandler.Create)
	router.DELETE("/instances/:id", environment.InstanceHandler.Delete)
	router.GET("/instances/:id", environment.InstanceHandler.FindById)
	router.GET("/instances/:id/logs", environment.InstanceHandler.Logs)
	router.GET("/instances-name-to-id/:groupId/:name", environment.InstanceHandler.NameToId)

	return r
}
