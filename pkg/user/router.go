package user

import (
	"github.com/dhis2-sre/im-manager/internal/middleware"
	"github.com/gin-gonic/gin"
)

func Routes(r *gin.Engine, authenticationMiddleware middleware.AuthenticationMiddleware, handler Handler) {
	r.POST("/users", handler.SignUp)
	r.POST("/refresh", handler.RefreshToken)

	basicAuthenticationRouter := r.Group("")
	basicAuthenticationRouter.Use(authenticationMiddleware.BasicAuthentication)
	basicAuthenticationRouter.POST("/tokens", handler.SignIn)

	tokenAuthenticationRouter := r.Group("")
	tokenAuthenticationRouter.Use(authenticationMiddleware.TokenAuthentication)
	tokenAuthenticationRouter.GET("/me", handler.Me)
	tokenAuthenticationRouter.DELETE("/users", handler.SignOut)
	tokenAuthenticationRouter.GET("/users/:id", handler.FindById)
}
