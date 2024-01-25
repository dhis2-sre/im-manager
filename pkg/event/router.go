package event

import (
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine, authenticator gin.HandlerFunc, handler Handler) {
	router := r.Group("")
	router.Use(authenticator)
	router.GET("/subscribe", handler.Subscribe)
}
