package database

import (
	"github.com/dhis2-sre/im-manager/internal/handler"
	"github.com/gin-gonic/gin"
)

func ConfigureRoutes(router *gin.Engine, authMiddleware *handler.AuthenticationMiddleware, handler Handler) {
	router.GET("/databases/external/:uuid", handler.ExternalDownload)

	tokenAuthenticationRouter := router.Group("/databases")
	tokenAuthenticationRouter.Use(authMiddleware.TokenAuthentication)
	tokenAuthenticationRouter.POST("", handler.Upload)
	tokenAuthenticationRouter.POST("/:id/copy", handler.Copy)
	tokenAuthenticationRouter.GET("/:id/download", handler.Download)
	tokenAuthenticationRouter.GET("", handler.List)
	tokenAuthenticationRouter.GET("/:id", handler.FindByIdentifier)
	tokenAuthenticationRouter.PUT("/:id", handler.Update)
	tokenAuthenticationRouter.DELETE("/:id", handler.Delete)
	tokenAuthenticationRouter.POST("/:id/lock", handler.Lock)
	tokenAuthenticationRouter.DELETE("/:id/unlock", handler.Unlock)
	tokenAuthenticationRouter.POST("/save-as/:instanceId", handler.SaveAs)
	tokenAuthenticationRouter.POST("/:id/external", handler.CreateExternalDownload)
}
