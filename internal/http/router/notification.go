package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/http/handler"
)

func registerNotificationSettingsRoutes(api *gin.RouterGroup, notification *handler.NotificationHandler, requireSession, requireCSRF gin.HandlerFunc) {
	protected := api.Group("")
	protected.Use(requireSession)
	protected.GET("/household/settings/notifications", notification.Settings)

	write := protected.Group("")
	write.Use(requireCSRF)
	write.PATCH("/household/settings/notifications", notification.UpdateSettings)
	write.POST("/household/settings/notifications/email/test", notification.TestEmail)
}
