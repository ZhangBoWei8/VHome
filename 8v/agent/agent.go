package agent

import (
	"context"
	"strings"

	"vhome/8v/llm"
	"vhome/8v/tools"
	"vhome/internal/service"
)

// maxToolRounds bounds one answer: the model may call tools this many times
// before it has to reply with words.
const maxToolRounds = 5

type Agent struct {
	client  llm.Client
	tools   *tools.Registry
	history []llm.Message

	// allowWrites gates every tool marked Dangerous. Set it to false to run
	// the assistant in a read-only mode.
	allowWrites bool
}

type Factory func() *Agent

// ProvideFactory turns the singleton LLM client and tool registry into the
// per-request Agent constructor the HTTP handler depends on.
func ProvideFactory(client llm.Client, registry *tools.Registry) Factory {
	return func() *Agent {
		return New(client, registry)
	}
}

func New(client llm.Client, registry *tools.Registry) *Agent {
	return &Agent{
		client:      client,
		tools:       registry,
		allowWrites: true,
		history:     []llm.Message{llm.System(baseSystemPrompt)},
	}
}

const baseSystemPrompt = `你是 8V，一个家庭助手，负责家庭健康、家务、库存管理、提醒和家庭事务。

## 工作方式
- 需要家庭的真实数据时必须调用工具，绝不凭记忆或猜测回答库存、账单、饮食和提醒的具体数字。
- 工具返回的 JSON 是事实来源。基于事实做判断和建议，但不要编造事实里没有的内容。
- 查不到数据时如实说明，并说清你查了什么，不要用推测填补。
- 回答用简体中文，简洁自然，像家人之间说话。不要罗列 JSON，也不要提工具名。

## 写操作
- pantry_add、pantry_discard、expense_record、memo_create 会真正修改家庭数据。
- 只有当用户明确表达了这个意图时才调用，不要主动替用户记账、入库或设提醒。
- 信息不全时先问清楚再写，不要用默认值蒙混过去。
- 写入成功后，用一句话复述你实际写了什么（金额、日期、数量、提醒时间），方便用户核对。

## 修改和删除
- 修改和删除库存或账单需要先查到这条记录的 id 和 version，把查询结果里的这两个值原样传给写工具，不要自己编。
- 如果工具提示记录已被他人修改，重新查询后再试一次。`

func (a *Agent) systemPrompt(snapshot tools.HouseholdSnapshot) string {
	var b strings.Builder

	b.WriteString(baseSystemPrompt)
	b.WriteString("\n\n")
	b.WriteString(snapshot.PromptSection())

	return b.String()
}

func (a *Agent) Run(ctx context.Context, actor service.AuthenticatedIdentity, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}

	// The household facts are refreshed on every run: the clock has moved and
	// someone may have come home since the previous message.
	snapshot := a.tools.Snapshot(ctx, actor)
	a.history[0] = llm.System(a.systemPrompt(snapshot))

	invocation := tools.Invocation{
		Actor:       actor,
		AllowWrites: a.allowWrites,
	}

	a.history = append(a.history, llm.User(input))

	for round := 0; round < maxToolRounds; round++ {
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
			result, err := a.tools.Execute(
				ctx,
				invocation,
				call.Function.Name,
				string(call.Function.Arguments),
			)
			if err != nil {
				// Registry.Execute only returns messages that are safe for the
				// model to read and, if it chooses, to relay to the user.
				result = "调用失败：" + err.Error()
			}

			a.history = append(a.history, llm.ToolResult(call.ID, call.Function.Name, result))
		}
	}

	return "我查询的步骤太多了，没能整理出结果。可以把问题拆得更具体一些再问我吗？", nil
}
