package notification

import "github.com/gin-gonic/gin"

func Routes(router *gin.Engine, authenticator gin.HandlerFunc, handler Handler) {
	g := router.Group("/notifications")
	g.Use(authenticator)
	g.GET("", handler.List)
	g.PUT("/read-all", handler.MarkAllRead)
	g.PUT("/:id/read", handler.MarkRead)
}
