package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"vhome/8v/llm"
	"vhome/internal/model"
	"vhome/internal/service"
)

type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Executor    Executor
	Dangerous   bool
}

type Executor func(ctx context.Context, invocation Invocation, args map[string]any) (string, error)

type Invocation struct {
	Actor service.AuthenticatedIdentity
}
type Registry struct {
	tools       map[string]Tool
	memorySaver func(content, scope string) error
	pantry      *service.PantryService
	expense     *service.ExpenseService
}

type Options struct {
	MemorySaver func(content, scope string) error
	Pantry      *service.PantryService
	Expense     *service.ExpenseService
}

type MonthlyExpenseToolResult struct {
	Month            string                  `json:"month"`
	Currency         string                  `json:"currency"`
	TotalAmountCents uint64                  `json:"total_amount_cents"`
	Records          []ExpenseRecordToolView `json:"records"`
}

type ExpenseRecordToolView struct {
	Scope       string `json:"scope"`
	Title       string `json:"title"`
	AmountCents uint64 `json:"amount_cents"`
	SpentOn     string `json:"spent_on"`
}

func NewRegistry(root string, opts Options) *Registry {
	r := &Registry{
		tools:       map[string]Tool{},
		memorySaver: opts.MemorySaver,
		pantry:      opts.Pantry,
		expense:     opts.Expense,
	}

	r.registerBuiltins()

	return r
}

func (r *Registry) register(tool Tool) {
	r.tools[tool.Name] = tool
}

func (r *Registry) Definitions() []llm.Tool {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]llm.Tool, 0, len(names))
	for _, name := range names {
		t := r.tools[name]
		out = append(out, llm.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.Parameters,
		})
	}
	return out
}

func (r *Registry) Execute(ctx context.Context, invocation Invocation, name string, rawArgs string) (string, error) {
	var args map[string]any
	if strings.TrimSpace(rawArgs) != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return "", fmt.Errorf("invalid tool arguments: %w", err)
		}
	}
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %s", name)
	}
	out, err := tool.Executor(ctx, invocation, args)
	return out, err
}

func (r *Registry) registerBuiltins() {
	r.register(Tool{
		Name:        "search_active_inventory_by_name",
		Description: "查找某一个已存储物料的相关信息，包括物料的过期时间、热量和三大营养物质占比等",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"item_name": map[string]any{
					"type":        "string",
					"description": "要查询的实际库存物料名称，例如鸡蛋、牛奶、土豆等",
					"minLength":   1,
					"maxLength":   128,
				},
			},
			"required":             []string{"item_name"},
			"additionalProperties": false,
		},
		Executor: r.executeSearchActiveInventoryByName,
	})

	r.register(Tool{
		Name: "get_material_template_by_name",
		Description: "按照名称查询可复用物料品类的默认基础信息。" +
			"用于回答物料默认冷藏或常温保存天数、默认单位、" +
			"每100克热量以及蛋白质、脂肪和碳水信息。" +
			"该结果是物料模板的默认值，不代表家庭中存在实际库存，" +
			"也不代表某个实际批次的到期日期。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"material_name": map[string]any{
					"type":        "string",
					"description": "要查询的物料品类名称，例如鸡蛋、牛奶或土豆",
					"minLength":   1,
					"maxLength":   64,
				},
			},
			"required":             []string{"material_name"},
			"additionalProperties": false,
		},
		Executor: r.executeGetMaterialTemplateByName,
	})

	r.register(Tool{
		Name: "list_active_inventory",
		Description: "列出家庭当前实际存储、尚未丢弃并且尚未过期的物料。" +
			"默认按照最早到期时间排列。" +
			"用于回答家里当前有什么、冰箱或仓库中存有哪些物料。" +
			"当用户询问快过期、未来几天内到期的物料时传入within_days；" +
			"如果用户只说快过期但没有指定天数，使用7天。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"within_days": map[string]any{
					"type":        "integer",
					"description": "可选，仅返回剩余天数小于等于该值的物料；查询快过期且用户没有指定范围时使用7",
					"minimum":     0,
					"maximum":     365,
				},
			},
			"additionalProperties": false,
		},
		Executor: r.executeListActiveInventory,
	})

	r.register(Tool{
		Name:        "get_my_monthly_expenses",
		Description: "查询当前登录成员指定月份的个人账单，返回支出类型、名称、金额和支出日期；未指定月份时查询本月。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"month": map[string]any{
					"type":        "string",
					"description": "可选，查询月份，格式为YYYY-MM；省略时查询当前月份",
					"pattern":     `^\d{4}-(0[1-9]|1[0-2])$`,
				},
			},
			"additionalProperties": false,
		},
		Executor: r.executeGetMyMonthlyExpenses,
	})
}

func (r *Registry) executeSearchActiveInventoryByName(ctx context.Context, _ Invocation, args map[string]any) (string, error) {
	if r.pantry == nil {
		return "", fmt.Errorf("pantry service is unavailable")
	}

	name, err := requiredStringArg(args, "item_name")
	if err != nil {
		return "", err
	}

	items, err := r.pantry.SearchInventory(ctx, name, "active", "asc", 20)
	if err != nil {
		return "", fmt.Errorf(
			"search active inventory: %w",
			err,
		)
	}

	data, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf(
			"encode active inventory result: %w",
			err,
		)
	}

	return string(data), nil
}

func requiredStringArg(args map[string]any, key string) (string, error) {
	value, ok := args[key].(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}

	return value, nil
}

func optionalStringArg(args map[string]any, key string) (string, error) {
	raw, exists := args[key]
	if !exists {
		return "", nil
	}

	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf(
			"%s must be a string",
			key,
		)
	}

	return strings.TrimSpace(value), nil
}

func optionalIntArg(args map[string]any, key string) (*int, error) {
	raw, exists := args[key]
	if !exists {
		return nil, nil
	}

	value, ok := raw.(float64)
	if !ok || value != math.Trunc(value) {
		return nil, fmt.Errorf("%s must be an integer", key)
	}

	result := int(value)

	return &result, nil
}

func (r *Registry) executeGetMaterialTemplateByName(ctx context.Context, _ Invocation, args map[string]any) (string, error) {
	if r.pantry == nil {
		return "", fmt.Errorf("pantry service is unavailable")
	}

	name, err := requiredStringArg(
		args,
		"material_name",
	)
	if err != nil {
		return "", err
	}

	template, err := r.pantry.MaterialTemplateByName(ctx, name)
	if err != nil {
		return "", fmt.Errorf(
			"get material template by name: %w",
			err,
		)
	}

	data, err := json.Marshal(template)
	if err != nil {
		return "", fmt.Errorf(
			"encode material template result: %w",
			err,
		)
	}

	return string(data), nil
}

func (r *Registry) executeListActiveInventory(ctx context.Context, _ Invocation, args map[string]any) (string, error) {
	if r.pantry == nil {
		return "", fmt.Errorf("pantry service is unavailable")
	}

	withinDays, err := optionalIntArg(
		args,
		"within_days",
	)
	if err != nil {
		return "", err
	}

	var items []service.InventoryView

	if withinDays == nil {
		items, err = r.pantry.ListInventory(
			ctx,
			"active",
			"asc",
		)
	} else {
		items, err = r.pantry.ListExpiringInventory(
			ctx,
			*withinDays,
			50,
		)
	}

	if err != nil {
		return "", fmt.Errorf(
			"list active inventory: %w",
			err,
		)
	}

	data, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf(
			"encode active inventory result: %w",
			err,
		)
	}

	return string(data), nil
}

func (r *Registry) executeGetMyMonthlyExpenses(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	if r.expense == nil {
		return "", fmt.Errorf(
			"expense service is unavailable",
		)
	}

	month, err := optionalStringArg(args, "month")
	if err != nil {
		return "", err
	}

	view, err := r.expense.MyMonthlyExpenses(
		ctx,
		invocation.Actor,
		month,
	)
	if err != nil {
		return "", fmt.Errorf(
			"get my monthly expenses: %w",
			err,
		)
	}

	result := MonthlyExpenseToolResult{
		Month:            view.Month,
		Currency:         "CNY",
		TotalAmountCents: view.TotalAmountCents,
		Records: make(
			[]ExpenseRecordToolView,
			0,
			len(view.Records),
		),
	}

	for _, record := range view.Records {
		scope := "个人支出"
		if record.ExpenseScope == model.ExpenseScopeCollective {
			scope = "家庭支出"
		}

		result.Records = append(
			result.Records,
			ExpenseRecordToolView{
				Scope:       scope,
				Title:       record.Title,
				AmountCents: record.AmountCents,
				SpentOn: record.SpentOn.Format(
					"2006-01-02",
				),
			},
		)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf(
			"encode monthly expenses: %w",
			err,
		)
	}

	return string(data), nil
}
