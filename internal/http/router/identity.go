package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/http/handler"
)

func registerIdentityRoutes(api *gin.RouterGroup, identityHandler *handler.IdentityHandler, requireSession gin.HandlerFunc) {
	api.POST(
		"/setup",
		identityHandler.Setup,
	)

	api.POST(
		"/members/registrations",
		identityHandler.RegisterMember,
	)

	authRoutes := api.Group("/auth")

	authRoutes.POST("/sessions", identityHandler.Login)

	protectedAuthRoutes := authRoutes.Group("")

	protectedAuthRoutes.Use(requireSession)

	protectedAuthRoutes.GET("/session", identityHandler.CurrentSession)

	protectedAuthRoutes.DELETE("/session", identityHandler.Logout)
}
