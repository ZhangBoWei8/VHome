package tools

import (
	"context"

	"vhome/internal/model"
	"vhome/internal/service"
)

func (r *Registry) registerMemoryTools() {
	r.register(Tool{
		Name: "memory_remember",
		Description: "把一条长期有效的家庭事实写进记忆库，之后每次对话都会自动带上，不需要再查。" +
			"只记录长期不变的偏好、习惯和限制，例如“爸爸不吃辣”“周日去超市采购”“家里有猫，不能放百合”。" +
			"绝对不要用它记录会变化的数据：库存数量、账单金额、提醒时间、谁在家，这些都有专门的查询工具。" +
			"scope=HOUSEHOLD 表示全家共享的事实，scope=MEMBER 表示只和当前成员有关的私人偏好。" +
			"只有用户明确要求记住，或者说出了一条明显长期有效的偏好时才调用。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"content": map[string]any{
					"type":        "string",
					"description": "要记住的事实，一句话，不超过 500 字。写成陈述句，不要写成对话。",
				},
				"scope": map[string]any{
					"type":        "string",
					"enum":        []string{"HOUSEHOLD", "MEMBER"},
					"description": "HOUSEHOLD=全家共享；MEMBER=只属于当前对话的成员。",
				},
			},
			"required":             []string{"content", "scope"},
			"additionalProperties": false,
		},
		Dangerous: true,
		Executor:  r.executeMemoryRemember,
	})

	r.register(Tool{
		Name: "memory_forget",
		Description: "从记忆库里删除一条记忆。" +
			"必须先用 memory_list 查到这条记忆的 id 和 version，把查到的值原样传进来，不要自己编。" +
			"只有用户明确要求忘记或更正某件事时才调用。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "integer",
					"description": "memory_list 返回的记忆 id。",
				},
				"version": map[string]any{
					"type":        "integer",
					"description": "memory_list 返回的 version，用于并发校验。",
				},
			},
			"required":             []string{"id", "version"},
			"additionalProperties": false,
		},
		Dangerous: true,
		Executor:  r.executeMemoryForget,
	})

	r.register(Tool{
		Name: "memory_list",
		Description: "列出当前成员可见的全部记忆，包含每条的 id 和 version。" +
			"记忆内容已经写在你的系统提示里，所以只在需要拿到 id 来删除或更正某条记忆时才调用。",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Executor: r.executeMemoryList,
	})
}

func (r *Registry) executeMemoryRemember(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	content, err := requiredStringArg(args, "content")
	if err != nil {
		return "", err
	}

	rawScope, err := requiredStringArg(args, "scope")
	if err != nil {
		return "", err
	}

	scope := model.AgentMemoryScope(rawScope)
	if !scope.Valid() {
		return "", badArgument("参数 scope 必须是 HOUSEHOLD 或 MEMBER")
	}

	result, err := r.memory.Remember(ctx, invocation.Actor, service.RememberInput{
		Content: content,
		Scope:   scope,
	})
	if err != nil {
		return "", err
	}

	return encode(map[string]any{
		"id":      result.Memory.ID,
		"version": result.Memory.Version,
		"scope":   result.Memory.Scope,
		"content": result.Memory.Content,
		// The model needs to know which of the two happened so it does not
		// announce "记住了" for something it already knew.
		"created": result.Created,
	})
}

func (r *Registry) executeMemoryForget(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	id, err := requiredUintArg(args, "id")
	if err != nil {
		return "", err
	}

	version, err := requiredUintArg(args, "version")
	if err != nil {
		return "", err
	}

	if err := r.memory.Forget(ctx, invocation.Actor, id, version); err != nil {
		return "", err
	}

	return encode(map[string]any{
		"id":      id,
		"deleted": true,
	})
}

func (r *Registry) executeMemoryList(ctx context.Context, invocation Invocation, _ map[string]any) (string, error) {
	memories, err := r.memory.ListFor(ctx, invocation.Actor)
	if err != nil {
		return "", err
	}

	return encode(map[string]any{
		"count":    len(memories),
		"memories": newMemoryToolViews(memories),
	})
}
