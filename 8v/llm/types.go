package llm

import (
	"context"
	"encoding/json"
)

type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Name       string
	ToolCallID string
	ToolCalls  []ToolCall
}

type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolCall struct {
	ID       string
	Type     string
	Function ToolCallFunction
}

type ToolCallFunction struct {
	Name      string
	Arguments json.RawMessage
}

type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
}

type Client interface {
	Chat(ctx context.Context, message []Message, tools []Tool) (ChatResponse, error)
	Provider() string
	Model() string
}

// Delta is one fragment of a streamed response. Tool call fragments are not
// surfaced: they are accumulated internally and delivered whole in the final
// ChatResponse, because a half-built argument list is of no use to a caller.
type Delta struct {
	Content string
}

// StreamingClient is implemented by clients that can deliver the answer as it
// is generated. Callers should type-assert for it and fall back to Chat, so a
// provider without streaming still works.
type StreamingClient interface {
	Client

	// ChatStream calls onDelta for each fragment and returns the same fully
	// assembled response Chat would have returned. An error from onDelta
	// aborts the stream and is returned unchanged.
	ChatStream(ctx context.Context, message []Message, tools []Tool, onDelta func(Delta) error) (ChatResponse, error)
}

func System(content string) Message {
	return Message{Role: "system", Content: content}
}

func User(content string) Message {
	return Message{Role: "user", Content: content}
}

func Assistant(content string) Message {
	return Message{Role: "assistant", Content: content}
}

func AssistantWithTools(content string, calls []ToolCall) Message {
	return Message{
		Role:      "assistant",
		Content:   content,
		ToolCalls: calls,
	}
}

func ToolResult(id, name, content string) Message {
	return Message{
		Role:       "tool",
		ToolCallID: id,
		Name:       name,
		Content:    content,
	}
}
