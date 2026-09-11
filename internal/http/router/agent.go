package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/http/handler"
)

func registerAgentRoutes(api *gin.RouterGroup, agentHandler *handler.AgentHandler, requireSession gin.HandlerFunc, requireCSRF gin.HandlerFunc) {
	protected := api.Group("/agent")
	protected.Use(requireSession)

	protected.GET("/conversations", agentHandler.Conversations)
	protected.GET("/conversations/:id/messages", agentHandler.Messages)

	write := protected.Group("")
	write.Use(requireCSRF)

	// Chat is a POST because it creates a turn, and it streams its answer as
	// Server-Sent Events.
	write.POST("/chat", agentHandler.Chat)
	write.DELETE("/conversations/:id", agentHandler.DeleteConversation)
}
