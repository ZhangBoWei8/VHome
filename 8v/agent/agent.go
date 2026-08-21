package agent

import (
	"net/http"
	"strings"
	"vhome/8v/llm"
)

type Agent struct {
	client  http.Client
	history []llm.Message
}

func New(client http.Client) *Agent {
	a := &Agent{
		client: client,
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
