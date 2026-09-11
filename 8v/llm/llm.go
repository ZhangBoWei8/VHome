package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"vhome/8v/agconfig"
)

type OpenAICompatiableClient struct {
	provider string
	cfg      agconfig.ProviderConfig
	http     *http.Client
}

type Outs struct {
	Choices []Out `json:"choices"`
}

type Out struct {
	Message ResponseMessage `json:"message"`
}

type ResponseMessage struct {
	Content   string         `json:"content"`
	ToolCalls []RespToolCall `json:"tool_calls"`
}

type RespToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func NewClient(cfg agconfig.AGConfig) (Client, error) {
	provider := strings.ToLower(cfg.DefaultProvider)
	p := cfg.Provider(provider)

	if p.APIKey == "" {
		return nil, fmt.Errorf("missing API key for provider: %s", provider)
	}
	if p.BaseURL == "" || p.Model == "" {
		return nil, fmt.Errorf("provider %s missing base_url or model", provider)
	}

	return &OpenAICompatiableClient{
		provider: provider,
		cfg:      p,
		// No whole-request Timeout: it would sever a long stream mid-answer,
		// since a streamed body legitimately stays open while the model
		// generates. The deadline that matters is time-to-first-byte, which
		// ResponseHeaderTimeout covers; overall cancellation comes from the
		// request context.
		http: &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				ResponseHeaderTimeout: 60 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}, nil
}

func (c *OpenAICompatiableClient) Chat(ctx context.Context, message []Message, tools []Tool) (ChatResponse, error) {
	body := map[string]any{
		"model":       c.cfg.Model,
		"messages":    openAIMessage(message),
		"temperature": 0.2,
		"stream":      false,
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
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return ChatResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ChatResponse{}, fmt.Errorf("llm request failed: %s :%s", resp.Status, string(data))
	}

	var out Outs
	if err := json.Unmarshal(data, &out); err != nil {
		return ChatResponse{}, err
	}
	if len(out.Choices) == 0 {
		return ChatResponse{}, nil
	}

	msg := out.Choices[0].Message
	calls := make([]ToolCall, 0, len(msg.ToolCalls))
	for _, call := range msg.ToolCalls {
		calls = append(calls, ToolCall{
			ID:   call.ID,
			Type: call.Type,
			Function: ToolCallFunction{
				Name:      call.Function.Name,
				Arguments: json.RawMessage(call.Function.Arguments),
			},
		})
	}

	return ChatResponse{
		Content:   msg.Content,
		ToolCalls: calls,
	}, nil
}

func (c *OpenAICompatiableClient) Provider() string {
	return c.provider
}

func (c *OpenAICompatiableClient) Model() string {
	return c.cfg.Model
}

// 适配openai风格的信息，
func openAIMessage(message []Message) []map[string]any {
	result := make([]map[string]any, 0, len(message))

	for _, msg := range message {
		item := map[string]any{
			"role": msg.Role,
		}
		if msg.Name != "" {
			item["name"] = msg.Name
		}

		if msg.ToolCallID != "" {
			item["tool_call_id"] = msg.ToolCallID
		}

		if len(msg.ToolCalls) > 0 {
			calls := make([]map[string]any, 0, len(msg.ToolCalls))
			for _, call := range msg.ToolCalls {
				calls = append(calls, map[string]any{
					"id":   call.ID,
					"type": "function",
					"function": map[string]any{
						"name":      call.Function.Name,
						"arguments": string(call.Function.Arguments),
					},
				})
			}
			item["tool_calls"] = calls
		}

		item["content"] = msg.Content
		result = append(result, item)
	}
	return result
}

func openAITools(tools []Tool) []map[string]any {
	result := make([]map[string]any, 0, len(tools))

	for _, tool := range tools {
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.Parameters,
			},
		})
	}

	return result
}
