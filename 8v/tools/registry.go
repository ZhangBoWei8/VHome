package tools

import (
	"context"
)

type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Executor    Executor
	Dangerous   bool
}

type Executor func(ctx context.Context, args map[string]any) (string, error)

type Registry struct {
	tools       map[string]Tool
	memorySaver func(content, scope string) error
}

type Options struct {
	MemorySaver func(content, scope string) error
}

func NewRegistry(root string, opts Options) *Registry {
	r := &Registry{
		tools:       map[string]Tool{},
		memorySaver: opts.MemorySaver,
	}

	return r
}

func (r *Registry) register(tool Tool) {
	r.tools[tool.Name] = tool
}
func (r *Registry) registerBuiltins() {
	r.register(Tool{
		Name:        "search_expense",
		Description: "查找某一个已存储物料的相关信息，包括物料的过期时间、热量和三大营养物质占比等",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expenseName": map[string]any{
					"type":        "string",
					"description": "name of the expense that you want to search",
				},
			},
		},
		Executor: func(ctx context.Context, args map[string]any) (string, error) {
			return "", nil
		},
	})

	r.register(Tool{
		Name: "list_expenses",
	})
}
