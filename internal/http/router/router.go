package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/config"
	"vhome/internal/http/handler"
	"vhome/internal/http/middleware"
	"vhome/internal/service"
)

func Register(
	engine *gin.Engine,
	identityService *service.IdentityService,
	pantryService *service.PantryService,
	authConfig config.AuthConfig,
) {
	bootstrapHandler :=
		handler.NewBootstrapHandler(
			identityService,
			authConfig.CookieName,
		)

	identityHandler :=
		handler.NewIdentityHandler(
			identityService,
			authConfig.CookieName,
			authConfig.CookieSecure,
		)

	requireSession :=
		middleware.RequireSession(
			identityService,
			authConfig.CookieName,
		)
	requireCSRF := middleware.RequireCSRF(identityService)
	memberHandler := handler.NewMemberHandler(identityService)
	pantryHandler := handler.NewPantryHandler(pantryService, "data/uploads")
	homeHandler := handler.NewHomeHandler(identityService, pantryService)

	api := engine.Group("/api/v1")

	registerBootstrapRoutes(
		api,
		bootstrapHandler,
	)

	registerIdentityRoutes(
		api,
		identityHandler,
		requireSession,
	)

	registerMemberAndPantryRoutes(api, memberHandler, pantryHandler, homeHandler, requireSession, requireCSRF)
}
