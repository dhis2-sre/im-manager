package server

import (
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/database"
	"github.com/dhis2-sre/im-manager/pkg/health"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/integration"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	redocMiddleware "github.com/go-openapi/runtime/middleware"
)

func GetEngine(basePath string, stackHandler stack.Handler, instanceHandler instance.Handler, integrationHandler integration.Handler, databaseHandler database.Handler, authMiddleware *handler.AuthenticationMiddleware) *gin.Engine {
	r := gin.Default()

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowCredentials = true
	corsConfig.AddAllowHeaders("authorization")
	r.Use(cors.New(corsConfig))

	r.Use(middleware.ErrorHandler())

	router := r.Group(basePath)

	redoc(router, basePath)

	router.GET("/health", health.Health)

	tokenAuthenticationRouter := router.Group("")
	tokenAuthenticationRouter.Use(authMiddleware.TokenAuthentication)

	tokenAuthenticationRouter.GET("/stacks", stackHandler.FindAll)
	tokenAuthenticationRouter.GET("/stacks/:name", stackHandler.Find)

	tokenAuthenticationRouter.POST("/integrations", integrationHandler.Integrations)

	tokenAuthenticationRouter.POST("/instances", instanceHandler.Deploy)
	tokenAuthenticationRouter.GET("/instances", instanceHandler.ListInstances)
	tokenAuthenticationRouter.GET("/presets", instanceHandler.ListPresets)
	tokenAuthenticationRouter.GET("/instances/:id", instanceHandler.FindById)
	tokenAuthenticationRouter.GET("/instances/:id/parameters", instanceHandler.FindByIdDecrypted)
	tokenAuthenticationRouter.DELETE("/instances/:id", instanceHandler.Delete)
	tokenAuthenticationRouter.PUT("/instances/:id", instanceHandler.Update)
	tokenAuthenticationRouter.PUT("/instances/:id/pause", instanceHandler.Pause)
	tokenAuthenticationRouter.PUT("/instances/:id/restart", instanceHandler.Restart)
	tokenAuthenticationRouter.GET("/instances/:id/logs", instanceHandler.Logs)
	tokenAuthenticationRouter.GET("/instances-name-to-id/:groupName/:instanceName", instanceHandler.NameToId)

	// /databases
	router.GET("/databases/external/:uuid", databaseHandler.ExternalDownload)

	// tokenAuthenticationRouter := router.Group("")
	tokenAuthenticationRouter.Use(authMiddleware.TokenAuthentication)
	tokenAuthenticationRouter.POST("/databases", databaseHandler.Upload)
	tokenAuthenticationRouter.POST("/databases/:id/copy", databaseHandler.Copy)
	tokenAuthenticationRouter.GET("/databases/:id/download", databaseHandler.Download)
	tokenAuthenticationRouter.GET("/databases", databaseHandler.List)
	tokenAuthenticationRouter.GET("/databases/:id", databaseHandler.FindByIdentifier)
	tokenAuthenticationRouter.PUT("/databases/:id", databaseHandler.Update)
	tokenAuthenticationRouter.DELETE("/databases/:id", databaseHandler.Delete)
	tokenAuthenticationRouter.POST("/databases/:id/lock", databaseHandler.Lock)
	tokenAuthenticationRouter.DELETE("/databases/:id/unlock", databaseHandler.Unlock)
	tokenAuthenticationRouter.POST("/databases/save-as/:instanceId", databaseHandler.SaveAs)
	tokenAuthenticationRouter.POST("/databases/:id/external", databaseHandler.CreateExternalDownload)

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
