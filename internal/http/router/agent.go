package router

import (
	"github.com/gin-gonic/gin"

	"vhome/internal/http/handler"
)

func registerAgentRoutes(api *gin.RouterGroup, agentHandler *handler.AgentHandler, requireSession gin.HandlerFunc, requireCSRF gin.HandlerFunc) {
	protected := api.Group("/agent")
	protected.Use(requireSession)
	protected.Use(requireCSRF)

	protected.POST("/chat", agentHandler.Chat)
}
