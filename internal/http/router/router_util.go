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
