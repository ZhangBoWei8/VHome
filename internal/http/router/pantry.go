package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/http/handler"
)

func registerMemberAndPantryRoutes(api *gin.RouterGroup, members *handler.MemberHandler, pantry *handler.PantryHandler, home *handler.HomeHandler, requireSession, requireCSRF gin.HandlerFunc) {
	protected := api.Group("")
	protected.Use(requireSession)

	protected.GET("/members", members.List)
	memberWrite := protected.Group("/members")
	memberWrite.Use(requireCSRF)
	memberWrite.POST("/:id/approve", members.Approve)
	memberWrite.POST("/:id/reject", members.Reject)
	memberWrite.PATCH("/:id/role", members.Role)
	memberWrite.DELETE("/:id", members.Disable)

	protected.GET("/storage-locations", pantry.Locations)
	protected.GET("/material-templates", pantry.Templates)
	protected.GET("/inventory-items", pantry.Inventory)
	protected.GET("/notifications", home.Notifications)
	protected.GET("/dashboard", home.Dashboard)
	protected.GET("/household/settings", home.Settings)
	protected.GET("/members/me/profile", home.Profile)

	write := protected.Group("")
	write.Use(requireCSRF)
	write.POST("/storage-locations", pantry.CreateLocation)
	write.POST("/material-templates", pantry.CreateTemplate)
	write.POST("/inventory-items", pantry.CreateInventory)
	write.PATCH("/inventory-items/:id", pantry.UpdateInventory)
	write.POST("/inventory-items/:id/discard", pantry.Discard)
	write.POST("/notifications/materials/:id/:milestone/read", pantry.ReadReminder)
	write.PATCH("/household/settings", home.UpdateSettings)
	write.PATCH("/members/me/profile", home.UpdateProfile)
}
