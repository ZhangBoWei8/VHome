package agent

import (
	"github.com/google/wire"

	"vhome/8v/agconfig"
	"vhome/8v/llm"
	"vhome/8v/tools"
)

// ProviderSet wires the whole 8v assistant: provider config -> LLM client ->
// tool registry -> per-request agent factory.
var ProviderSet = wire.NewSet(
	agconfig.Load,
	llm.NewClient,

	wire.Struct(new(tools.Options), "*"),
	tools.NewRegistry,

	ProvideFactory,
)
