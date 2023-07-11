package database

import (
	"github.com/gin-gonic/gin"
)

func Routes(router *gin.Engine, authenticator gin.HandlerFunc, handler Handler) {
	router.GET("/databases/external/:uuid", handler.ExternalDownload)

	tokenAuthenticationRouter := router.Group("/databases")
	tokenAuthenticationRouter.Use(authenticator)
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
	tokenAuthenticationRouter.POST("/save/:instanceId", handler.Save)
	tokenAuthenticationRouter.POST("/:id/external", handler.CreateExternalDownload)
}
