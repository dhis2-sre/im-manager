package server

import (
	"github.com/dhis2-sre/im-manager/internal/di"
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/health"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	redocMiddleware "github.com/go-openapi/runtime/middleware"
)

func GetEngine(environment di.Environment) *gin.Engine {
	basePath := environment.Config.BasePath

	r := gin.Default()
	r.Use(cors.Default())
	r.Use(middleware.ErrorHandler())

	router := r.Group(basePath)

	redoc(router, basePath)

	router.GET("/health", health.Health)

	tokenAuthenticationRouter := router.Group("")
	tokenAuthenticationRouter.Use(environment.AuthenticationMiddleware.TokenAuthentication)

	tokenAuthenticationRouter.GET("/stacks", environment.StackHandler.FindAll)
	tokenAuthenticationRouter.GET("/stacks/:name", environment.StackHandler.Find)

	tokenAuthenticationRouter.POST("/instances", environment.InstanceHandler.Create)
	tokenAuthenticationRouter.POST("/instances/:id/deploy", environment.InstanceHandler.Deploy)
	tokenAuthenticationRouter.GET("/instances/:id/parameters", environment.InstanceHandler.FindByIdWithDecryptedParameters)
	tokenAuthenticationRouter.GET("/instances", environment.InstanceHandler.List)
	tokenAuthenticationRouter.DELETE("/instances/:id", environment.InstanceHandler.Delete)
	tokenAuthenticationRouter.GET("/instances/:id", environment.InstanceHandler.FindById)
	tokenAuthenticationRouter.GET("/instances/:id/logs", environment.InstanceHandler.Logs)
	tokenAuthenticationRouter.GET("/instances-name-to-id/:groupName/:instanceName", environment.InstanceHandler.NameToId)

	//tokenAuthenticationRouter.POST("/instances/:id/save", environment.InstanceHandler.Save)
	//tokenAuthenticationRouter.POST("/instances/:id/saveas", health.Health)

	return r
}

func redoc(router *gin.RouterGroup, basePath string) {
	router.StaticFile("/swagger.yaml", "./swagger/swagger.yaml")

	redocOpts := redocMiddleware.RedocOpts{
		BasePath: basePath,
		SpecURL:  "./swagger.yaml",
	}
	router.GET("/docs", func(c *gin.Context) {
		redocHandler := redocMiddleware.Redoc(redocOpts, nil)
		redocHandler.ServeHTTP(c.Writer, c.Request)
	})
}
