package tools

import (
	"context"
	"strings"

	"vhome/internal/model"
	"vhome/internal/service"
)

// maxExpenseRangeDays keeps a single range query from pulling a whole decade of
// records into the model's context.
const maxExpenseRangeDays = 400

func (r *Registry) registerExpenseTools() {
	r.register(Tool{
		Name: "expense_list_categories",
		Description: "列出家庭可用的支出分类及其 id。" +
			"记账前先调用它拿到 category_id，再调用 expense_record。",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Executor: r.executeExpenseListCategories,
	})

	r.register(Tool{
		Name: "expense_get_my_month",
		Description: "查询当前成员某个月的个人账单，返回总额、按分类的汇总和每一笔明细。" +
			"未指定月份时查询本月。金额单位是元。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"month": map[string]any{
					"type":        "string",
					"description": "可选，查询月份 YYYY-MM，省略时为本月",
					"pattern":     `^\d{4}-(0[1-9]|1[0-2])$`,
				},
			},
			"additionalProperties": false,
		},
		Executor: r.executeExpenseGetMyMonth,
	})

	r.register(Tool{
		Name: "expense_get_summary",
		Description: "查询家庭本月的支出总览：总额、个人支出与公共支出的拆分、" +
			"上个月总额以及环比变化百分比。用于回答“这个月家里花得多不多”这类问题。",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Executor: r.executeExpenseGetSummary,
	})

	r.register(Tool{
		Name: "expense_query_range",
		Description: "按日期区间查询支出明细，用于跨月统计，例如“上个季度买菜花了多少”。" +
			"start_date 和 end_date 都包含在内，区间最长 400 天。" +
			"scope 决定查谁的账：mine 为本人，collective 为家庭公共支出，household 为全家所有支出。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"start_date": map[string]any{
					"type":        "string",
					"description": "开始日期 YYYY-MM-DD，含当天",
					"pattern":     `^\d{4}-\d{2}-\d{2}$`,
				},
				"end_date": map[string]any{
					"type":        "string",
					"description": "结束日期 YYYY-MM-DD，含当天",
					"pattern":     `^\d{4}-\d{2}-\d{2}$`,
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "查询范围，默认 mine",
					"enum":        []string{"mine", "collective", "household"},
				},
				"category": map[string]any{
					"type":        "string",
					"description": "可选，只保留该分类的记录，例如“食品”",
					"maxLength":   32,
				},
			},
			"required":             []string{"start_date", "end_date"},
			"additionalProperties": false,
		},
		Executor: r.executeExpenseQueryRange,
	})

	r.register(Tool{
		Name: "expense_record",
		Description: "为当前成员记一笔支出。只有在用户明确要求记账时才调用。" +
			"category_id 必须来自 expense_list_categories；金额用元（例如 87.5），不要用分。" +
			"scope 为 personal 表示个人支出，collective 表示家庭公共支出，默认 personal。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "支出名称，例如“买菜”“加油”",
					"minLength":   1,
					"maxLength":   64,
				},
				"amount_yuan": map[string]any{
					"type":             "number",
					"description":      "金额，单位元，例如 87.5",
					"exclusiveMinimum": 0,
				},
				"category_id": map[string]any{
					"type":        "integer",
					"description": "支出分类 id，来自 expense_list_categories",
					"minimum":     1,
				},
				"spent_on": map[string]any{
					"type":        "string",
					"description": "可选，支出日期 YYYY-MM-DD，省略时为今天",
					"pattern":     `^\d{4}-\d{2}-\d{2}$`,
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "支出归属，默认 personal",
					"enum":        []string{"personal", "collective"},
				},
				"note": map[string]any{
					"type":        "string",
					"description": "可选备注",
					"maxLength":   200,
				},
			},
			"required":             []string{"title", "amount_yuan", "category_id"},
			"additionalProperties": false,
		},
		Dangerous: true,
		Executor:  r.executeExpenseRecord,
	})
}

func (r *Registry) executeExpenseListCategories(ctx context.Context, invocation Invocation, _ map[string]any) (string, error) {
	categories, err := r.expense.Categories(ctx, invocation.Actor)
	if err != nil {
		return "", err
	}

	return encode(map[string]any{
		"categories": newExpenseCategoryToolViews(categories),
	})
}

func (r *Registry) executeExpenseGetMyMonth(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	month, err := optionalMonthArg(args, "month")
	if err != nil {
		return "", err
	}

	view, err := r.expense.MyMonthlyExpenses(ctx, invocation.Actor, month)
	if err != nil {
		return "", err
	}

	return encode(newMonthlyExpenseToolView(view))
}

func (r *Registry) executeExpenseGetSummary(ctx context.Context, invocation Invocation, _ map[string]any) (string, error) {
	summary, err := r.expense.CurrentHouseholdSummary(ctx, invocation.Actor)
	if err != nil {
		return "", err
	}

	return encode(newHouseholdExpenseSummaryToolView(summary))
}

func (r *Registry) executeExpenseQueryRange(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	start, err := r.optionalDateArg(args, "start_date")
	if err != nil {
		return "", err
	}
	end, err := r.optionalDateArg(args, "end_date")
	if err != nil {
		return "", err
	}
	if start.IsZero() || end.IsZero() {
		return "", badArgument("start_date 和 end_date 都是必填的")
	}
	if end.Before(start) {
		return "", badArgument("end_date 不能早于 start_date")
	}
	if end.Sub(start).Hours()/24 > maxExpenseRangeDays {
		return "", badArgument("查询区间最长 %d 天，请分段查询", maxExpenseRangeDays)
	}

	scope, err := optionalStringArg(args, "scope")
	if err != nil {
		return "", err
	}

	view := service.ExpenseExportMine
	switch strings.ToLower(scope) {
	case "", "mine":
		view = service.ExpenseExportMine
	case "collective":
		view = service.ExpenseExportCollective
	case "household":
		view = service.ExpenseExportHousehold
	default:
		return "", badArgument("scope 只能是 mine、collective 或 household")
	}

	category, err := optionalStringArg(args, "category")
	if err != nil {
		return "", err
	}

	// ExportExpenses takes an exclusive end, while the tool contract is an
	// inclusive one, which is what users mean by "到 9 月 30 日".
	records, err := r.expense.ExportExpenses(
		ctx, invocation.Actor, start, end.AddDate(0, 0, 1), view,
	)
	if err != nil {
		return "", err
	}

	var totalCents uint64
	items := make([]expenseRecordToolView, 0, len(records))
	for _, record := range records {
		if category != "" && !strings.Contains(record.Category.Name, category) {
			continue
		}
		totalCents += record.AmountCents
		items = append(items, newExpenseRecordToolView(record))
	}

	return encode(map[string]any{
		"start_date": start.Format(dateLayout),
		"end_date":   end.Format(dateLayout),
		"currency":   "CNY",
		"count":      len(items),
		"total_yuan": yuan(totalCents),
		"records":    items,
	})
}

func (r *Registry) executeExpenseRecord(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	title, err := requiredStringArg(args, "title")
	if err != nil {
		return "", err
	}

	amount, err := requiredFloatArg(args, "amount_yuan")
	if err != nil {
		return "", err
	}
	if amount <= 0 {
		return "", badArgument("参数 amount_yuan 必须大于 0")
	}

	categoryID, err := requiredUintArg(args, "category_id")
	if err != nil {
		return "", err
	}
	if categoryID > 65535 {
		return "", badArgument("category_id 不是有效的分类 id")
	}

	spentOn, err := r.optionalDateArg(args, "spent_on")
	if err != nil {
		return "", err
	}
	if spentOn.IsZero() {
		spentOn = r.today()
	}

	scope, err := optionalStringArg(args, "scope")
	if err != nil {
		return "", err
	}

	expenseScope := model.ExpenseScopePersonal
	switch strings.ToLower(scope) {
	case "", "personal":
		expenseScope = model.ExpenseScopePersonal
	case "collective":
		expenseScope = model.ExpenseScopeCollective
	default:
		return "", badArgument("scope 只能是 personal 或 collective")
	}

	note, err := optionalStringArg(args, "note")
	if err != nil {
		return "", err
	}

	created, err := r.expense.CreateExpense(ctx, invocation.Actor, service.ExpenseInput{
		CategoryID:   uint16(categoryID),
		ExpenseScope: expenseScope,
		Title:        title,
		AmountCents:  centsFromYuan(amount),
		SpentOn:      spentOn,
		Note:         note,
	})
	if err != nil {
		return "", err
	}

	return encode(map[string]any{
		"created": true,
		"record":  newExpenseRecordToolView(created),
	})
}
