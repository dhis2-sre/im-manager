package server

import (
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/health"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	redocMiddleware "github.com/go-openapi/runtime/middleware"
)

func GetEngine(basePath string, stackHandler stack.Handler, instanceHandler instance.Handler, authMiddleware handler.AuthenticationMiddleware) *gin.Engine {
	r := gin.Default()
	r.Use(cors.Default())
	r.Use(middleware.ErrorHandler())

	router := r.Group(basePath)

	redoc(router, basePath)

	router.GET("/health", health.Health)

	tokenAuthenticationRouter := router.Group("")
	tokenAuthenticationRouter.Use(authMiddleware.TokenAuthentication)

	tokenAuthenticationRouter.GET("/stacks", stackHandler.FindAll)
	tokenAuthenticationRouter.GET("/stacks/:name", stackHandler.Find)

	tokenAuthenticationRouter.POST("/instances", instanceHandler.Create)
	tokenAuthenticationRouter.POST("/instances/:id/link/:newInstanceId", instanceHandler.LinkDeploy)
	tokenAuthenticationRouter.POST("/instances/:id/deploy", instanceHandler.Deploy)
	tokenAuthenticationRouter.GET("/instances/:id/parameters", instanceHandler.FindByIdWithDecryptedParameters)
	tokenAuthenticationRouter.GET("/instances", instanceHandler.List)
	tokenAuthenticationRouter.DELETE("/instances/:id", instanceHandler.Delete)
	tokenAuthenticationRouter.GET("/instances/:id", instanceHandler.FindById)
	tokenAuthenticationRouter.GET("/instances/:id/logs", instanceHandler.Logs)
	tokenAuthenticationRouter.GET("/instances-name-to-id/:groupName/:instanceName", instanceHandler.NameToId)

	// tokenAuthenticationRouter.POST("/instances/:id/save", instanceHandler.Save)
	// tokenAuthenticationRouter.POST("/instances/:id/saveas", health.Health)

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
