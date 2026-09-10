package tools

import "context"

func (r *Registry) registerMealTools() {
	r.register(Tool{
		Name: "meal_search_food",
		Description: "在家庭食物库中按名称搜索食物，返回每 100 克的热量、蛋白质、脂肪和碳水含量。" +
			"用于回答某种食物的营养成分，或在讨论饮食时查证热量数据。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "食物名称关键词，例如米饭、鸡胸肉、牛奶",
					"minLength":   1,
					"maxLength":   64,
				},
			},
			"required":             []string{"keyword"},
			"additionalProperties": false,
		},
		Executor: r.executeMealSearchFood,
	})

	r.register(Tool{
		Name: "meal_get_day",
		Description: "查询当前成员某一天的饮食记录和当天摄入的总热量与三大营养素。" +
			"未指定日期时查询今天。用于回答“我今天吃了什么”“今天摄入多少热量”。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"date": map[string]any{
					"type":        "string",
					"description": "可选，查询日期 YYYY-MM-DD，省略时为今天",
					"pattern":     `^\d{4}-\d{2}-\d{2}$`,
				},
			},
			"additionalProperties": false,
		},
		Executor: r.executeMealGetDay,
	})

	r.register(Tool{
		Name: "meal_get_month",
		Description: "查询当前成员某个月的每日热量摄入，返回有记录的天数、总热量和日均热量。" +
			"未指定月份时查询本月。用于回答饮食趋势、这个月吃得怎么样这类问题。",
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
		Executor: r.executeMealGetMonth,
	})
}

func (r *Registry) executeMealSearchFood(ctx context.Context, _ Invocation, args map[string]any) (string, error) {
	keyword, err := requiredStringArg(args, "keyword")
	if err != nil {
		return "", err
	}

	foods, err := r.meal.ListFoods(ctx, keyword)
	if err != nil {
		return "", err
	}

	return encode(map[string]any{
		"count": len(foods),
		"foods": newFoodToolViews(foods),
	})
}

func (r *Registry) executeMealGetDay(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	date, err := r.optionalDateArg(args, "date")
	if err != nil {
		return "", err
	}
	if date.IsZero() {
		date = r.today()
	}

	day, err := r.meal.MyMealDay(ctx, invocation.Actor, date)
	if err != nil {
		return "", err
	}

	return encode(newMealDayToolView(day))
}

func (r *Registry) executeMealGetMonth(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	month, err := optionalMonthArg(args, "month")
	if err != nil {
		return "", err
	}
	if month == "" {
		month = r.today().Format(monthLayout)
	}

	calendar, err := r.meal.MyMealCalendar(ctx, invocation.Actor, month)
	if err != nil {
		return "", err
	}

	return encode(newMealMonthToolView(calendar))
}
