package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/http/handler"
)

func registerBootstrapRoutes(api *gin.RouterGroup, bootstrapHandler *handler.BootstrapHandler) {
	api.GET(
		"/bootstrap",
		bootstrapHandler.Get,
	)
}

// registerHealthRoutes exposes the liveness probe. It is intentionally outside
// every guard: the container health check and the deploy script call it before
// anyone has logged in.
func registerHealthRoutes(api *gin.RouterGroup, healthHandler *handler.HealthHandler) {
	api.GET(
		"/healthz",
		healthHandler.Get,
	)
}
