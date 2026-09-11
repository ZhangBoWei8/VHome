package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"vhome/8v/agent"
	"vhome/internal/http/response"
	"vhome/internal/model"
	"vhome/internal/service"
)

const maxAgentInputLength = 4000

type AgentHandler struct {
	agent         *agent.Agent
	conversations *service.AgentConversationService
}

func NewAgentHandler(assistant *agent.Agent, conversations *service.AgentConversationService) *AgentHandler {
	return &AgentHandler{
		agent:         assistant,
		conversations: conversations,
	}
}

type agentChatRequest struct {
	// ConversationID is optional: omitting it starts a new thread.
	ConversationID uint64 `json:"conversation_id"`
	Input          string `json:"input" binding:"required,max=4000"`
}

type agentConversationData struct {
	ID            uint64 `json:"id"`
	Title         string `json:"title"`
	LastMessageAt string `json:"last_message_at"`
	MessageCount  uint32 `json:"message_count"`

	// Rounds is MessageCount as exchanges, which is the unit the limits and
	// the warnings are expressed in.
	Rounds uint32 `json:"rounds"`
}

// agentLimitsData travels with the conversation list so the client never has
// to hardcode thresholds that live in the service layer.
type agentLimitsData struct {
	MaxRounds     int `json:"max_rounds"`
	WarnAtRounds  int `json:"warn_at_rounds"`
	RetentionDays int `json:"retention_days"`
}

type agentConversationsData struct {
	Conversations []agentConversationData `json:"conversations"`
	Limits        agentLimitsData         `json:"limits"`
}

type agentMessageData struct {
	ID        uint64 `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	ToolName  string `json:"tool_name,omitempty"`
	CreatedAt string `json:"created_at"`
}

// Chat answers one message as a Server-Sent Events stream.
//
// Once the first event is flushed the {data}/{error} envelope is no longer
// available, so failures are reported two different ways: as a normal error
// response while the headers are still pending, and as an `error` event after
// that. sseWriter tracks which of the two applies.
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

	stream := newSSEWriter(c)

	result, err := h.agent.Run(
		c.Request.Context(),
		currentActor(c),
		request.ConversationID,
		request.Input,
		stream,
	)
	if err != nil {
		// A client that hung up is not a failure worth reporting.
		if errors.Is(err, errStreamClosed) || c.Request.Context().Err() != nil {
			return
		}

		if !stream.started {
			// Ownership and validation failures still have the envelope
			// available, so they keep their usual status codes.
			if isAgentRequestError(err) {
				writeServiceError(c, err)
				return
			}

			_ = c.Error(err)
			response.WriteError(
				c,
				http.StatusBadGateway,
				"AGENT_UNAVAILABLE",
				"家庭 Agent 暂时无法回答，请稍后重试",
			)
			return
		}

		_ = c.Error(err)
		stream.send("error", map[string]any{
			"code":    "AGENT_UNAVAILABLE",
			"message": "家庭 Agent 暂时无法回答，请稍后重试",
		})
		return
	}

	stream.send("done", map[string]any{
		"conversation_id": result.ConversationID,
		"answer":          result.Answer,
	})
}

func (h *AgentHandler) Conversations(c *gin.Context) {
	conversations, err := h.conversations.ListFor(c.Request.Context(), currentActor(c))
	if err != nil {
		writeServiceError(c, err)
		return
	}

	items := make([]agentConversationData, 0, len(conversations))
	for _, conversation := range conversations {
		items = append(items, newAgentConversationData(conversation))
	}

	response.WriteData(c, http.StatusOK, agentConversationsData{
		Conversations: items,
		Limits: agentLimitsData{
			MaxRounds:     service.MaxAgentConversationRounds,
			WarnAtRounds:  service.AgentConversationRoundWarning,
			RetentionDays: service.AgentConversationRetentionDays,
		},
	})
}

func (h *AgentHandler) Messages(c *gin.Context) {
	conversationID, err := agentConversationIDParam(c)
	if err != nil {
		writeBadRequest(c, err)
		return
	}

	messages, err := h.conversations.HistoryMessages(c.Request.Context(), currentActor(c), conversationID)
	if err != nil {
		writeServiceError(c, err)
		return
	}

	// Tool traffic is the agent's working notes, not conversation: the client
	// replays only what the user actually saw.
	data := make([]agentMessageData, 0, len(messages))
	for _, message := range messages {
		if message.Role == model.AgentMessageRoleTool {
			continue
		}
		if message.Role == model.AgentMessageRoleAssistant && strings.TrimSpace(message.Content) == "" {
			continue
		}

		data = append(data, agentMessageData{
			ID:        message.ID,
			Role:      string(message.Role),
			Content:   message.Content,
			CreatedAt: message.CreatedAt.Format(agentTimeLayout),
		})
	}

	response.WriteData(c, http.StatusOK, data)
}

func (h *AgentHandler) DeleteConversation(c *gin.Context) {
	conversationID, err := agentConversationIDParam(c)
	if err != nil {
		writeBadRequest(c, err)
		return
	}

	if err := h.conversations.Delete(c.Request.Context(), currentActor(c), conversationID); err != nil {
		writeServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

const agentTimeLayout = "2006-01-02 15:04:05"

func newAgentConversationData(conversation model.AgentConversation) agentConversationData {
	return agentConversationData{
		ID:            conversation.ID,
		Title:         conversation.Title,
		LastMessageAt: conversation.LastMessageAt.Format(agentTimeLayout),
		MessageCount:  conversation.MessageCount,
		// Round up: a turn still in flight has its question stored without an
		// answer yet, and that half exchange should already count.
		Rounds: (conversation.MessageCount + 1) / 2,
	}
}

func agentConversationIDParam(c *gin.Context) (uint64, error) {
	raw := c.Param("id")

	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid conversation id %q", raw)
	}

	return id, nil
}

// isAgentRequestError reports whether a failure is the caller's fault and so
// deserves its mapped status code rather than a blanket 502.
func isAgentRequestError(err error) bool {
	return errors.Is(err, service.ErrNotFound) ||
		errors.Is(err, service.ErrUnauthenticated) ||
		errors.Is(err, service.ErrForbidden) ||
		errors.Is(err, service.ErrInvalidInput) ||
		errors.Is(err, service.ErrAgentConversationFull)
}

// errStreamClosed marks a write to a client that has gone away. It travels
// back up through the agent's sink calls to abort the turn.
var errStreamClosed = errors.New("handler: agent stream closed")

// sseWriter adapts the response writer to agent.Sink.
type sseWriter struct {
	c       *gin.Context
	started bool
	broken  bool
}

func newSSEWriter(c *gin.Context) *sseWriter {
	return &sseWriter{c: c}
}

func (w *sseWriter) begin() {
	if w.started {
		return
	}

	header := w.c.Writer.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache")
	header.Set("Connection", "keep-alive")

	// nginx buffers proxied responses by default, which would hold the whole
	// stream back until the turn finished. deploy/nginx.conf disables
	// buffering for /api/; this header covers any other proxy in front.
	header.Set("X-Accel-Buffering", "no")

	w.c.Writer.WriteHeader(http.StatusOK)
	w.started = true
}

func (w *sseWriter) send(event string, payload any) {
	if w.broken {
		return
	}

	w.begin()

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	if _, err := fmt.Fprintf(w.c.Writer, "event: %s\ndata: %s\n\n", event, data); err != nil {
		w.broken = true
		return
	}

	w.c.Writer.Flush()
}

func (w *sseWriter) OnConversation(conversationID uint64) error {
	w.send("meta", map[string]any{"conversation_id": conversationID})

	return w.err()
}

func (w *sseWriter) OnToolCall(name string) error {
	w.send("tool", map[string]any{"name": name})

	return w.err()
}

func (w *sseWriter) OnToken(text string) error {
	w.send("token", map[string]any{"text": text})

	return w.err()
}

func (w *sseWriter) err() error {
	if w.broken {
		return errStreamClosed
	}

	return nil
}
