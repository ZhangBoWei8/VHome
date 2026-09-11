package agent

import (
	"github.com/google/wire"

	"vhome/8v/agconfig"
	"vhome/8v/llm"
	"vhome/8v/tools"
)

// ProviderSet wires the whole 8v assistant: provider config -> LLM client ->
// tool registry -> agent.
//
// The agent is a singleton rather than a per-request factory: it holds no
// conversation state any more, so one instance serves every request.
var ProviderSet = wire.NewSet(
	agconfig.Load,
	llm.NewClient,

	wire.Struct(new(tools.Options), "*"),
	tools.NewRegistry,

	New,
)
