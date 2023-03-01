package server

import (
	"context"
	"io"
	"net/http/pprof"
	rpprof "runtime/pprof"
	"strings"

	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/dhis2-sre/im-manager/pkg/health"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/dhis2-sre/im-manager/pkg/integration"
	"github.com/dhis2-sre/im-manager/pkg/stack"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	redocMiddleware "github.com/go-openapi/runtime/middleware"
)

func profile(c *gin.Context) {
	if c.FullPath() == "" { // not found
		c.Next()
		return
	}

	// pprofile labels will be supported when https://github.com/parca-dev/parca/pull/2667 is merged
	labels := rpprof.Labels("http_method", c.Request.Method, "http_endpoint", c.FullPath())
	rpprof.Do(c.Request.Context(), labels, func(ctx context.Context) {
		c.Request = c.Request.Clone(ctx)
		c.Next()
	})
}

// workGin is just to create some work for the CPU that is then visible in the profile
func workGin(c *gin.Context) {
	var sum int
	for i := 0; i < 1_000_000_000; i++ {
		sum++
	}
	io.Copy(c.Writer, strings.NewReader("lots of work to calculate\n"))
}

func GetEngine(basePath string, stackHandler stack.Handler, instanceHandler instance.Handler, integrationHandler integration.Handler, authMiddleware *handler.AuthenticationMiddleware) *gin.Engine {
	r := gin.Default()

	r.Use(profile)
	r.GET("/testpprof", workGin)
	// TODO use safe profilers safely as defined in https://github.com/DataDog/go-profiler-notes/blob/main/guide/README.md
	pfRouter := r.Group("/debug/pprof")
	pfRouter.GET("/", gin.WrapF(pprof.Index))
	pfRouter.GET("/cmdline", gin.WrapF(pprof.Cmdline))
	pfRouter.GET("/profile", gin.WrapF(pprof.Profile))
	// TODO add POST /symbol ?
	pfRouter.GET("/symbol", gin.WrapF(pprof.Symbol))
	pfRouter.GET("/trace", gin.WrapF(pprof.Trace))
	// TODO are allocs and heap complementary or just a different view on the same thing
	pfRouter.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
	pfRouter.GET("/heap", gin.WrapH(pprof.Handler("heap")))
	pfRouter.GET("/block", gin.WrapH(pprof.Handler("block")))
	pfRouter.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
	// https://github.com/DataDog/go-profiler-notes/blob/main/guide/README.md
	// safe rate: 1000 goroutines
	pfRouter.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))

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
