package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/http/handler"
)

// registerMealRoutes keeps read endpoints behind the session middleware and
// all mutations behind both session and CSRF protection. Meal write routes are
// deliberately scoped to /meals/me: the client never supplies an owner ID.
func registerMealRoutes(api *gin.RouterGroup, meal *handler.MealHandler, requireSession gin.HandlerFunc, requireCSRF gin.HandlerFunc) {
	protected := api.Group("")
	protected.Use(requireSession)

	protected.GET("/foods", meal.Foods)
	protected.GET("/foods/:id", meal.Food)
	protected.GET("/meals/member-options", meal.MemberOptions)
	protected.GET("/meals/me", meal.MyMealDay)
	protected.GET("/meals/me/calendar", meal.MyMealCalendar)
	protected.GET("/meals/members/:memberID/today", meal.MemberToday)

	write := protected.Group("")
	write.Use(requireCSRF)
	write.POST("/foods", meal.CreateFood)
	write.PATCH("/foods/:id", meal.UpdateFood)
	write.DELETE("/foods/:id", meal.DeleteFood)
	write.POST("/foods/:id/restore", meal.RestoreFood)
	write.POST("/meals/me/records", meal.CreateMyMealRecord)
	write.PATCH("/meals/me/records/:id", meal.UpdateMyMealRecord)
	write.DELETE("/meals/me/records/:id", meal.DeleteMyMealRecord)
}
