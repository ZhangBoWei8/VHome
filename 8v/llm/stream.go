package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
)

// streamChunk is one `data:` payload of an OpenAI-compatible stream.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				// Index identifies which tool call a fragment belongs to.
				// It is the only reliable key: id arrives once, in the first
				// fragment, and is absent from every later one.
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
}

// toolCallAccumulator rebuilds whole tool calls from streamed fragments. A
// provider sends the id and name once and then dribbles the JSON arguments
// across many chunks, so the arguments are concatenated as raw text and only
// parsed once the stream ends.
type toolCallAccumulator struct {
	calls map[int]*ToolCall
	args  map[int]*strings.Builder
}

func newToolCallAccumulator() *toolCallAccumulator {
	return &toolCallAccumulator{
		calls: map[int]*ToolCall{},
		args:  map[int]*strings.Builder{},
	}
}

func (a *toolCallAccumulator) add(index int, id, callType, name, argumentFragment string) {
	call, exists := a.calls[index]
	if !exists {
		call = &ToolCall{Type: "function"}
		a.calls[index] = call
		a.args[index] = &strings.Builder{}
	}

	// Later fragments carry empty strings for fields already delivered, so
	// only non-empty values overwrite.
	if id != "" {
		call.ID = id
	}
	if callType != "" {
		call.Type = callType
	}
	if name != "" {
		call.Function.Name = name
	}
	if argumentFragment != "" {
		a.args[index].WriteString(argumentFragment)
	}
}

// result returns the calls ordered by their stream index, which is the order
// the model asked for them in.
func (a *toolCallAccumulator) result() []ToolCall {
	if len(a.calls) == 0 {
		return nil
	}

	indexes := make([]int, 0, len(a.calls))
	for index := range a.calls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	calls := make([]ToolCall, 0, len(indexes))
	for _, index := range indexes {
		call := *a.calls[index]

		arguments := a.args[index].String()
		if strings.TrimSpace(arguments) == "" {
			// An argument-less tool must still present valid JSON.
			arguments = "{}"
		}
		call.Function.Arguments = json.RawMessage(arguments)

		calls = append(calls, call)
	}

	return calls
}

func (c *OpenAICompatiableClient) ChatStream(ctx context.Context, message []Message, tools []Tool, onDelta func(Delta) error) (ChatResponse, error) {
	body := map[string]any{
		"model":       c.cfg.Model,
		"messages":    openAIMessage(message),
		"temperature": 0.2,
		"stream":      true,
	}

	if len(tools) > 0 {
		body["tools"] = openAITools(tools)
		body["tool_choice"] = "auto"
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, err
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return ChatResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

		return ChatResponse{}, fmt.Errorf("llm stream request failed: %s :%s", resp.Status, string(data))
	}

	return readChatStream(resp.Body, onDelta)
}

// readChatStream consumes an SSE body into one response. It is separated from
// the HTTP call so it can be tested against recorded fragments.
func readChatStream(body io.Reader, onDelta func(Delta) error) (ChatResponse, error) {
	var content strings.Builder
	accumulator := newToolCallAccumulator()

	scanner := bufio.NewScanner(body)

	// A single SSE line can carry a large argument fragment; the 64KB default
	// would abort the stream with "token too long".
	scanner.Buffer(make([]byte, 0, 64<<10), 8<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// A provider may interleave keep-alive or comment payloads that
			// are not chunks. Skipping them is safer than failing the turn.
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			content.WriteString(delta.Content)

			if onDelta != nil {
				if err := onDelta(Delta{Content: delta.Content}); err != nil {
					return ChatResponse{}, err
				}
			}
		}

		for _, call := range delta.ToolCalls {
			accumulator.add(
				call.Index,
				call.ID,
				call.Type,
				call.Function.Name,
				call.Function.Arguments,
			)
		}
	}

	if err := scanner.Err(); err != nil {
		return ChatResponse{}, fmt.Errorf("read llm stream: %w", err)
	}

	return ChatResponse{
		Content:   content.String(),
		ToolCalls: accumulator.result(),
	}, nil
}
