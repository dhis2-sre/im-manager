package server

import (
	"fmt"
	"log/slog"

	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/health"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	redocMiddleware "github.com/go-openapi/runtime/middleware"
)

func GetEngine(logger *slog.Logger, basePath string, allowedOrigins []string) (*gin.Engine, error) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CorrelationID())
	r.Use(middleware.RequestLogger(logger))

	corsConfig := cors.DefaultConfig()
	// Without specifying origins, secure cookies won't work
	corsConfig.AllowOrigins = allowedOrigins
	corsConfig.AllowCredentials = true
	corsConfig.AddAllowHeaders("authorization")
	corsConfig.AddExposeHeaders("Content-Disposition", "Content-Length")
	if err := corsConfig.Validate(); err != nil {
		return nil, fmt.Errorf("failed to configure CORS: %v", err)
	}
	r.Use(cors.New(corsConfig))

	r.Use(middleware.ErrorHandler())

	router := r.Group(basePath)

	redoc(router, basePath)

	router.GET("/health", health.Health)

	return r, nil
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
