package llm

import (
	"strings"
	"testing"
)

// A streamed tool call arrives in fragments: the id and name come once, then
// the JSON arguments dribble in across many chunks. Reassembling them by index
// is what makes tool use work at all over a stream.
func TestReadChatStreamAssemblesFragmentedToolCalls(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"pantry_add","arguments":""}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"name\""}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"牛奶\"}"}}]}}]}`,
		"data: [DONE]",
		"",
	}, "\n\n")

	response, err := readChatStream(strings.NewReader(body), nil)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}

	if len(response.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(response.ToolCalls))
	}

	call := response.ToolCalls[0]
	if call.ID != "call_1" {
		t.Errorf("id: got %q, want call_1", call.ID)
	}
	if call.Function.Name != "pantry_add" {
		t.Errorf("name: got %q, want pantry_add", call.Function.Name)
	}
	if got, want := string(call.Function.Arguments), `{"name":"牛奶"}`; got != want {
		t.Errorf("arguments: got %s, want %s", got, want)
	}
}

func TestReadChatStreamOrdersParallelToolCallsByIndex(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"call_b","function":{"name":"memo_list","arguments":"{}"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_a","function":{"name":"pantry_list","arguments":"{}"}}]}}]}`,
		"data: [DONE]",
		"",
	}, "\n\n")

	response, err := readChatStream(strings.NewReader(body), nil)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}

	if len(response.ToolCalls) != 2 {
		t.Fatalf("got %d tool calls, want 2", len(response.ToolCalls))
	}

	if response.ToolCalls[0].ID != "call_a" || response.ToolCalls[1].ID != "call_b" {
		t.Fatalf("calls out of index order: %q then %q",
			response.ToolCalls[0].ID, response.ToolCalls[1].ID)
	}
}

func TestReadChatStreamDeliversContentDeltas(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"冰箱里"}}]}`,
		`data: {"choices":[{"delta":{"content":"还有牛奶"}}]}`,
		"data: [DONE]",
		"",
	}, "\n\n")

	var fragments []string
	response, err := readChatStream(strings.NewReader(body), func(delta Delta) error {
		fragments = append(fragments, delta.Content)

		return nil
	})
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}

	if got, want := strings.Join(fragments, "|"), "冰箱里|还有牛奶"; got != want {
		t.Errorf("deltas: got %s, want %s", got, want)
	}

	if response.Content != "冰箱里还有牛奶" {
		t.Errorf("assembled content: got %q", response.Content)
	}
}

// A tool the model calls with no arguments must still produce parseable JSON,
// or the registry rejects it before the tool ever runs.
func TestReadChatStreamDefaultsEmptyArgumentsToObject(t *testing.T) {
	body := "data: " +
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"household_list_members","arguments":""}}]}}]}` +
		"\n\ndata: [DONE]\n\n"

	response, err := readChatStream(strings.NewReader(body), nil)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}

	if got := string(response.ToolCalls[0].Function.Arguments); got != "{}" {
		t.Errorf("arguments: got %q, want {}", got)
	}
}

func TestReadChatStreamSkipsUnparseablePayloads(t *testing.T) {
	body := strings.Join([]string{
		": keep-alive comment",
		`data: {"choices":[{"delta":{"content":"好的"}}]}`,
		"data: not-json",
		"data: [DONE]",
		"",
	}, "\n\n")

	response, err := readChatStream(strings.NewReader(body), nil)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}

	if response.Content != "好的" {
		t.Errorf("content: got %q, want 好的", response.Content)
	}
}
