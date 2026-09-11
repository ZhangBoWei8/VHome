package agent

import (
	"encoding/json"
	"fmt"

	"vhome/8v/llm"
	"vhome/internal/model"
	"vhome/internal/service"
)

const (
	// maxContextMessages and maxContextChars bound what is replayed to the
	// model. Whichever is hit first decides the cut.
	maxContextMessages = 40
	maxContextChars    = 12000
)

// storedToolCall is the database representation of one tool call. It mirrors
// the OpenAI wire shape rather than the Go field names so a row stays readable
// when inspected directly in MySQL.
type storedToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func encodeToolCalls(calls []llm.ToolCall) ([]byte, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	stored := make([]storedToolCall, 0, len(calls))
	for _, call := range calls {
		var item storedToolCall
		item.ID = call.ID
		item.Type = call.Type
		if item.Type == "" {
			item.Type = "function"
		}
		item.Function.Name = call.Function.Name
		item.Function.Arguments = string(call.Function.Arguments)

		stored = append(stored, item)
	}

	encoded, err := json.Marshal(stored)
	if err != nil {
		return nil, fmt.Errorf("encode tool calls: %w", err)
	}

	return encoded, nil
}

func decodeToolCalls(raw []byte) ([]llm.ToolCall, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var stored []storedToolCall
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, fmt.Errorf("decode tool calls: %w", err)
	}

	calls := make([]llm.ToolCall, 0, len(stored))
	for _, item := range stored {
		calls = append(calls, llm.ToolCall{
			ID:   item.ID,
			Type: item.Type,
			Function: llm.ToolCallFunction{
				Name:      item.Function.Name,
				Arguments: json.RawMessage(item.Function.Arguments),
			},
		})
	}

	return calls, nil
}

// replayMessages turns stored rows back into provider messages. The system
// message is not among them: it is rebuilt from live data on every turn.
func replayMessages(stored []model.AgentMessage) ([]llm.Message, error) {
	messages := make([]llm.Message, 0, len(stored))

	for _, row := range stored {
		switch row.Role {
		case model.AgentMessageRoleUser:
			messages = append(messages, llm.User(row.Content))

		case model.AgentMessageRoleAssistant:
			calls, err := decodeToolCalls(row.ToolCalls)
			if err != nil {
				return nil, err
			}

			if len(calls) == 0 {
				messages = append(messages, llm.Assistant(row.Content))
			} else {
				messages = append(messages, llm.AssistantWithTools(row.Content, calls))
			}

		case model.AgentMessageRoleTool:
			messages = append(messages, llm.ToolResult(row.ToolCallID, row.ToolName, row.Content))
		}
	}

	return messages, nil
}

// trimContext returns the tail of a conversation that fits the budget.
//
// The cut can only land on a user message. A `tool` message is meaningless
// without the assistant message that requested it, and an assistant message
// carrying tool_calls is invalid without its results, so cutting mid-turn
// makes the provider reject the whole request. Starting each window at a user
// message keeps every tool group whole.
func trimContext(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return nil
	}

	start := 0
	if len(messages) > maxContextMessages {
		start = len(messages) - maxContextMessages
	}

	// Walk backwards from the end while the character budget allows, so the
	// newest complete turns are the ones kept.
	chars := 0
	budgetStart := len(messages)
	for i := len(messages) - 1; i >= start; i-- {
		chars += len(messages[i].Content)
		if chars > maxContextChars && budgetStart < len(messages) {
			break
		}
		budgetStart = i
	}

	if budgetStart > start {
		start = budgetStart
	}

	// Advance to the next turn boundary. Anything before it is a fragment.
	for start < len(messages) && messages[start].Role != "user" {
		start++
	}

	if start >= len(messages) {
		return nil
	}

	return messages[start:]
}

// turnMessages converts the messages one turn produced into the service's
// storage shape, keeping only what the user actually said and was told.
//
// Tool traffic is dropped here rather than only at the call site, so that the
// invariant holds for any future caller. Two reasons it must:
//
//   - Tool results describe state that changes. A stored copy is stale as soon
//     as it is written, and the user never sees it. A follow-up needing a
//     detail re-runs the tool and gets fresh data.
//   - An assistant message carrying tool_calls is rejected by the provider if
//     its matching tool results are absent, so the pair can only be kept or
//     dropped together — and keeping it buys nothing.
func turnMessages(messages []llm.Message) ([]service.AgentTurnMessage, error) {
	out := make([]service.AgentTurnMessage, 0, len(messages))

	for _, message := range messages {
		switch message.Role {
		case "user":
			out = append(out, service.AgentTurnMessage{
				Role:    model.AgentMessageRoleUser,
				Content: message.Content,
			})

		case "assistant":
			if len(message.ToolCalls) > 0 {
				continue
			}

			out = append(out, service.AgentTurnMessage{
				Role:    model.AgentMessageRoleAssistant,
				Content: message.Content,
			})
		}
	}

	return out, nil
}
