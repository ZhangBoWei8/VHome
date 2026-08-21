package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/http/handler"
)

func registerMemoRoutes(api *gin.RouterGroup, memo *handler.MemoHandler, requireSession, requireCSRF gin.HandlerFunc) {
	protected := api.Group("")
	protected.Use(requireSession)
	protected.GET("/memos/member-options", memo.MemberOptions)
	protected.GET("/memos/me", memo.Mine)
	protected.GET("/memos/me/calendar", memo.Calendar)
	protected.GET("/memos/me/day", memo.Day)
	protected.GET("/memos/me/search", memo.Search)
	protected.GET("/memos/created-by-me", memo.CreatedByMe)
	protected.GET("/memos/:id", memo.Get)

	write := protected.Group("")
	write.Use(requireCSRF)
	write.POST("/memos", memo.Create)
	write.PATCH("/memos/:id", memo.Update)
	write.DELETE("/memos/:id", memo.Delete)
	write.POST("/memos/:id/dismiss", memo.Dismiss)
	write.POST("/calendar/sync", memo.SyncCalendar)
}
