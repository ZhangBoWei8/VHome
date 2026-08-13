package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/http/handler"
)

func registerExpenseRoutes(
	api *gin.RouterGroup,
	expense *handler.ExpenseHandler,
	requireSession gin.HandlerFunc,
	requireCSRF gin.HandlerFunc,
) {
	protected := api.Group("")
	protected.Use(requireSession)
	protected.GET("/expense-categories", expense.Categories)
	protected.GET("/expenses/me", expense.MyMonthly)
	protected.GET("/expenses/collective", expense.CollectiveMonthly)
	protected.GET("/expenses/export", expense.Export)

	write := protected.Group("")
	write.Use(requireCSRF)
	write.POST("/expenses", expense.Create)
	write.PATCH("/expenses/:id", expense.Update)
	write.DELETE("/expenses/:id", expense.Delete)
}
