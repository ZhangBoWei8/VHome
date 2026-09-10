package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vhome/8v/agent"
	"vhome/internal/http/response"
)

const maxAgentInputLength = 4000

type AgentHandler struct {
	factory agent.Factory
}

func NewAgentHandler(factory agent.Factory) *AgentHandler {
	return &AgentHandler{
		factory: factory,
	}
}

type agentChatRequest struct {
	Input string `json:"input" binding:"required,max=4000"`
}

type agentChatData struct {
	Answer string `json:"answer"`
}

func (h *AgentHandler) Chat(c *gin.Context) {
	var request agentChatRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeBadRequest(c, err)
		return
	}

	request.Input = strings.TrimSpace(request.Input)
	if request.Input == "" {
		writeBadRequest(c, fmt.Errorf("input is required"))
		return
	}

	currentAgent := h.factory()

	answer, err := currentAgent.Run(
		c.Request.Context(),
		currentActor(c),
		request.Input,
	)
	if err != nil {
		_ = c.Error(err)

		response.WriteError(
			c,
			http.StatusBadGateway,
			"AGENT_UNAVAILABLE",
			"家庭 Agent 暂时无法回答，请稍后重试",
		)
		return
	}

	response.WriteData(
		c,
		http.StatusOK,
		agentChatData{
			Answer: answer,
		},
	)
}
