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
