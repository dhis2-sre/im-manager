package server

import (
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/health"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/logger"
	"github.com/gin-gonic/gin"
	redocMiddleware "github.com/go-openapi/runtime/middleware"
	"github.com/rs/zerolog"
)

func GetEngine(basePath string) *gin.Engine {
	r := gin.Default()

	setupCors(r)

	r.Use(middleware.ErrorHandler())

	router := r.Group(basePath)

	setupRedoc(router, basePath)

	router.GET("/health", health.Health)

	setupLogging(r)

	return r
}

func setupCors(r *gin.Engine) {
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowCredentials = true
	corsConfig.AddAllowHeaders("authorization")
	corsConfig.AddExposeHeaders("Content-Disposition", "Content-Length")
	r.Use(cors.New(corsConfig))
}

func setupRedoc(router *gin.RouterGroup, basePath string) {
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

func setupLogging(r *gin.Engine) {
	jsonLogger := logger.WithLogger(func(c *gin.Context, log zerolog.Logger) zerolog.Logger {
		authorization := c.Request.Header.Get("Authorization")
		return log.Output(gin.DefaultWriter).
			With().
			Str("authorization_header", authorization).
			Logger()
	})
	r.Use(logger.SetLogger(jsonLogger))
}
