package agent

import (
	"context"
	"strings"
	"vhome/8v/llm"
	"vhome/8v/tools"
	"vhome/internal/service"
)

type Agent struct {
	client  llm.Client
	tools   *tools.Registry
	history []llm.Message
}

type Factory func() *Agent

func New(client llm.Client, registry *tools.Registry) *Agent {
	a := &Agent{
		client: client,
		tools:  registry,
		history: []llm.Message{
			llm.System("You are 8V, a smart family assistant focused on family health, household affairs, inventory management, reminders, and smart home services.")},
	}

	a.refreshSystemPrompt("")
	return a
}

func (a *Agent) refreshSystemPrompt(query string) {
	if len(a.history) == 0 {
		a.history = append(a.history, llm.System(""))
	}
	a.history[0] = llm.System(a.systemPrompt(query))
}

func (a *Agent) systemPrompt(query string) string {
	var b strings.Builder

	b.WriteString("You are 8v, a smart family assistant focused on family health, household affairs, inventory management, reminders, and smart home services.\n")
	b.WriteString("User tools when you need to help check storage,memo,meal.or there are some question about personal health care")
	// TODO
	b.WriteString(query)
	return b.String()
}

func (a *Agent) Run(ctx context.Context, actor service.AuthenticatedIdentity, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	a.history = append(a.history, llm.User(input))
	for i := 0; i < 4; i++ {
		resp, err := a.client.Chat(ctx, a.history, a.tools.Definitions())
		if err != nil {
			return "", err
		}

		if len(resp.ToolCalls) == 0 {
			answer := strings.TrimSpace(resp.Content)
			a.history = append(a.history, llm.Assistant(answer))
			return answer, nil
		}
		a.history = append(a.history, llm.AssistantWithTools(resp.Content, resp.ToolCalls))
		for _, call := range resp.ToolCalls {
			result, err := a.tools.Execute(ctx, tools.Invocation{Actor: actor}, call.Function.Name, string(call.Function.Arguments))
			if err != nil {
				result = err.Error()
			}
			a.history = append(a.history, llm.ToolResult(call.ID, call.Function.Name, result))
		}
	}
	return "stopped because the tool loop reached its limit", nil
}
