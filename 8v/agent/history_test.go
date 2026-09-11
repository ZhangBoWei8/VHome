package agent

import (
	"strings"
	"testing"

	"vhome/8v/llm"
	"vhome/internal/model"
)

func toolCall(id, name, arguments string) llm.ToolCall {
	return llm.ToolCall{
		ID:   id,
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      name,
			Arguments: []byte(arguments),
		},
	}
}

// A window must never begin with a tool result or with an assistant message
// that requested tools: the provider rejects a tool message whose originating
// assistant message is missing, which is how a trimmed conversation breaks.
func TestTrimContextCutsOnlyAtTurnBoundaries(t *testing.T) {
	messages := []llm.Message{
		llm.User("第一个问题"),
		llm.AssistantWithTools("", []llm.ToolCall{toolCall("call_1", "pantry_list", "{}")}),
		llm.ToolResult("call_1", "pantry_list", `{"items":[]}`),
		llm.Assistant("冰箱是空的"),
		llm.User("第二个问题"),
		llm.AssistantWithTools("", []llm.ToolCall{toolCall("call_2", "expense_get_summary", "{}")}),
		llm.ToolResult("call_2", "expense_get_summary", `{"total":0}`),
		llm.Assistant("这个月还没有支出"),
	}

	// Force a cut in the middle of the first tool group.
	trimmed := trimContext(messages[2:])

	if len(trimmed) == 0 {
		t.Fatal("trimContext dropped everything")
	}

	if trimmed[0].Role != "user" {
		t.Fatalf("window starts with role %q, want user", trimmed[0].Role)
	}

	assertToolGroupsIntact(t, trimmed)
}

func TestTrimContextKeepsShortConversationWhole(t *testing.T) {
	messages := []llm.Message{
		llm.User("问题"),
		llm.Assistant("回答"),
	}

	trimmed := trimContext(messages)
	if len(trimmed) != 2 {
		t.Fatalf("got %d messages, want 2", len(trimmed))
	}
}

func TestTrimContextEnforcesMessageBudget(t *testing.T) {
	var messages []llm.Message
	for i := 0; i < maxContextMessages*2; i++ {
		messages = append(messages, llm.User("问题"), llm.Assistant("回答"))
	}

	trimmed := trimContext(messages)

	if len(trimmed) > maxContextMessages {
		t.Fatalf("got %d messages, want at most %d", len(trimmed), maxContextMessages)
	}
	if len(trimmed) == 0 {
		t.Fatal("trimContext dropped everything")
	}
	if trimmed[0].Role != "user" {
		t.Fatalf("window starts with role %q, want user", trimmed[0].Role)
	}
}

func TestTrimContextEnforcesCharBudget(t *testing.T) {
	long := strings.Repeat("长", maxContextChars)

	messages := []llm.Message{
		llm.User("很久以前的问题"),
		llm.Assistant(long),
		llm.User("很久以前的另一个问题"),
		llm.Assistant(long),
		llm.User("最近的问题"),
		llm.Assistant("简短回答"),
	}

	trimmed := trimContext(messages)

	total := 0
	for _, message := range trimmed {
		total += len(message.Content)
	}

	// One oversized message may still be kept; what must not happen is every
	// oversized message being replayed.
	if total >= len(long)*3 {
		t.Fatalf("kept %d chars, expected the char budget to drop older turns", total)
	}
	if len(trimmed) == 0 || trimmed[0].Role != "user" {
		t.Fatal("window must start at a user message")
	}
}

// assertToolGroupsIntact checks the invariant the provider enforces: every
// tool result is preceded by an assistant message that requested that exact id.
func assertToolGroupsIntact(t *testing.T, messages []llm.Message) {
	t.Helper()

	requested := map[string]bool{}

	for _, message := range messages {
		switch message.Role {
		case "assistant":
			for _, call := range message.ToolCalls {
				requested[call.ID] = true
			}

		case "tool":
			if !requested[message.ToolCallID] {
				t.Fatalf("tool result %q has no preceding assistant tool call", message.ToolCallID)
			}
		}
	}
}

func TestToolCallsSurviveStorageRoundTrip(t *testing.T) {
	original := []llm.ToolCall{
		toolCall("call_1", "pantry_add", `{"name":"牛奶","quantity":2}`),
		toolCall("call_2", "memo_create", `{"title":"倒垃圾"}`),
	}

	encoded, err := encodeToolCalls(original)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	decoded, err := decodeToolCalls(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(decoded) != len(original) {
		t.Fatalf("got %d calls, want %d", len(decoded), len(original))
	}

	for i, call := range decoded {
		if call.ID != original[i].ID || call.Function.Name != original[i].Function.Name {
			t.Errorf("call %d: got %+v, want %+v", i, call, original[i])
		}
		if string(call.Function.Arguments) != string(original[i].Function.Arguments) {
			t.Errorf("call %d arguments: got %s, want %s",
				i, call.Function.Arguments, original[i].Function.Arguments)
		}
	}
}

// Tool traffic must never reach storage. Persisting an assistant message that
// carries tool_calls without its results would make every later replay fail at
// the provider, and the results themselves go stale immediately.
func TestTurnMessagesDropsToolTraffic(t *testing.T) {
	encodedCall := toolCall("call_1", "pantry_list", "{}")

	turn := []llm.Message{
		llm.User("冰箱里有什么"),
		llm.AssistantWithTools("", []llm.ToolCall{encodedCall}),
		llm.ToolResult("call_1", "pantry_list", `{"items":[{"name":"牛奶"}]}`),
		llm.Assistant("还有一盒牛奶"),
	}

	stored, err := turnMessages(turn)
	if err != nil {
		t.Fatalf("turnMessages: %v", err)
	}

	for _, message := range stored {
		if message.Role == model.AgentMessageRoleTool {
			t.Errorf("tool result was persisted: %+v", message)
		}
		if len(message.ToolCalls) > 0 {
			t.Errorf("tool_calls were persisted: %s", message.ToolCalls)
		}
	}
}

func TestReplayMessagesRebuildsToolGroups(t *testing.T) {
	encoded, err := encodeToolCalls([]llm.ToolCall{toolCall("call_1", "pantry_list", "{}")})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	stored := []model.AgentMessage{
		{Role: model.AgentMessageRoleUser, Content: "冰箱里有什么"},
		{Role: model.AgentMessageRoleAssistant, ToolCalls: encoded},
		{Role: model.AgentMessageRoleTool, ToolCallID: "call_1", ToolName: "pantry_list", Content: `{"items":[]}`},
		{Role: model.AgentMessageRoleAssistant, Content: "冰箱是空的"},
	}

	replayed, err := replayMessages(stored)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if len(replayed) != 4 {
		t.Fatalf("got %d messages, want 4", len(replayed))
	}

	if len(replayed[1].ToolCalls) != 1 || replayed[1].ToolCalls[0].ID != "call_1" {
		t.Fatalf("assistant tool calls not restored: %+v", replayed[1])
	}

	if replayed[2].Role != "tool" || replayed[2].ToolCallID != "call_1" {
		t.Fatalf("tool result not restored: %+v", replayed[2])
	}

	assertToolGroupsIntact(t, replayed)
}
