package tools

import (
	"context"
	"strings"

	"vhome/internal/service"
)

const (
	defaultInventoryLimit = 30
	maxInventoryLimit     = 50
)

func (r *Registry) registerPantryTools() {
	r.register(Tool{
		Name: "pantry_list",
		Description: "列出家庭当前实际存储、尚未丢弃且尚未过期的物料，默认按最早到期时间排列。" +
			"用于回答家里现在有什么、冰箱或储物柜里存了哪些东西。" +
			"当用户问快过期、未来几天内到期的物料时传入 within_days；" +
			"如果用户只说“快过期”但没有指定天数，使用 7。" +
			"返回结果中的 id 和 version 可以直接用于 pantry_discard。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"within_days": map[string]any{
					"type":        "integer",
					"description": "可选，只返回剩余天数小于等于该值的物料；用户说“快过期”又没给范围时用 7",
					"minimum":     0,
					"maximum":     365,
				},
				"location": map[string]any{
					"type":        "string",
					"description": "可选，只返回该存储位置的物料，例如“冷藏室”",
					"maxLength":   64,
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "可选，最多返回多少条，默认 30",
					"minimum":     1,
					"maximum":     50,
				},
			},
			"additionalProperties": false,
		},
		Executor: r.executePantryList,
	})

	r.register(Tool{
		Name: "pantry_search",
		Description: "按名称查询家里某个具体物料的实际库存情况，包括数量、单位、存放位置、" +
			"到期日期、剩余天数和每 100 克营养成分。" +
			"返回结果中的 id 和 version 可以直接用于 pantry_discard。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"item_name": map[string]any{
					"type":        "string",
					"description": "要查询的库存物料名称，例如鸡蛋、牛奶、土豆",
					"minLength":   1,
					"maxLength":   128,
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "可选，最多返回多少条，默认 20",
					"minimum":     1,
					"maximum":     50,
				},
			},
			"required":             []string{"item_name"},
			"additionalProperties": false,
		},
		Executor: r.executePantrySearch,
	})

	r.register(Tool{
		Name: "pantry_template",
		Description: "按名称查询可复用物料品类的默认基础信息：默认单位、冷藏与常温的建议保质天数、" +
			"每 100 克热量以及蛋白质、脂肪、碳水含量。" +
			"结果是品类的默认参考值，不代表家里现在有这个物料，也不是某一批库存的实际到期日；" +
			"要查实际库存请用 pantry_search。入库前可以先用它拿到默认保质天数。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"material_name": map[string]any{
					"type":        "string",
					"description": "物料品类名称，例如鸡蛋、牛奶、土豆",
					"minLength":   1,
					"maxLength":   64,
				},
			},
			"required":             []string{"material_name"},
			"additionalProperties": false,
		},
		Executor: r.executePantryTemplate,
	})

	r.register(Tool{
		Name: "pantry_add",
		Description: "把新买的物料登记进家庭库存。只有在用户明确表示要记录入库时才调用。" +
			"如果传入 template_name 并且该品类配置了保质天数，可以不传 expires_on，系统会自动推算；" +
			"否则必须传 expires_on。存储位置用 location_id 或 location_name 指定其一。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "物料名称；如果传了 template_name 可以省略，默认使用品类名称",
					"maxLength":   128,
				},
				"template_name": map[string]any{
					"type":        "string",
					"description": "可选，对应的物料品类名称。传了它就能自动带出单位、营养和保质期",
					"maxLength":   64,
				},
				"location_id": map[string]any{
					"type":        "integer",
					"description": "存储位置 id，取自系统提示里的存储位置列表",
					"minimum":     1,
				},
				"location_name": map[string]any{
					"type":        "string",
					"description": "存储位置名称，例如“冷藏室”；与 location_id 二选一",
					"maxLength":   64,
				},
				"quantity": map[string]any{
					"type":             "number",
					"description":      "数量，必须大于 0",
					"exclusiveMinimum": 0,
				},
				"unit": map[string]any{
					"type":        "string",
					"description": "可选，单位，例如 G、ML、个；省略时用品类默认单位",
					"maxLength":   16,
				},
				"stocked_on": map[string]any{
					"type":        "string",
					"description": "可选，入库日期 YYYY-MM-DD，省略时为今天",
					"pattern":     `^\d{4}-\d{2}-\d{2}$`,
				},
				"expires_on": map[string]any{
					"type":        "string",
					"description": "到期日期 YYYY-MM-DD；未提供 template_name 时必填",
					"pattern":     `^\d{4}-\d{2}-\d{2}$`,
				},
				"description": map[string]any{
					"type":        "string",
					"description": "可选备注",
					"maxLength":   200,
				},
			},
			"required":             []string{"quantity"},
			"additionalProperties": false,
		},
		Dangerous: true,
		Executor:  r.executePantryAdd,
	})

	r.register(Tool{
		Name: "pantry_discard",
		Description: "把一条库存物料标记为已丢弃（吃完、坏掉、过期都用它）。" +
			"必须先用 pantry_list 或 pantry_search 拿到该物料的 id 和 version，不要凭空猜测。" +
			"只有在用户明确要求丢弃或确认已经用完时才调用。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"item_id": map[string]any{
					"type":        "integer",
					"description": "库存物料 id，来自 pantry_list 或 pantry_search 的返回结果",
					"minimum":     1,
				},
				"version": map[string]any{
					"type":        "integer",
					"description": "该物料的 version，来自同一次查询结果，用于避免覆盖他人的修改",
					"minimum":     1,
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "可选，丢弃原因，例如“已过期”“已吃完”",
					"maxLength":   100,
				},
			},
			"required":             []string{"item_id", "version"},
			"additionalProperties": false,
		},
		Dangerous: true,
		Executor:  r.executePantryDiscard,
	})
}

func (r *Registry) executePantryList(ctx context.Context, _ Invocation, args map[string]any) (string, error) {
	withinDays, err := optionalIntArg(args, "within_days")
	if err != nil {
		return "", err
	}

	location, err := optionalStringArg(args, "location")
	if err != nil {
		return "", err
	}

	limit, err := r.inventoryLimit(args, defaultInventoryLimit)
	if err != nil {
		return "", err
	}

	var items []service.InventoryView
	if withinDays == nil {
		items, err = r.pantry.ListInventory(ctx, "active", "asc")
	} else {
		items, err = r.pantry.ListExpiringInventory(ctx, *withinDays, limit)
	}
	if err != nil {
		return "", err
	}

	if location != "" {
		filtered := make([]service.InventoryView, 0, len(items))
		for _, item := range items {
			if strings.Contains(item.LocationName, location) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	if len(items) > limit {
		items = items[:limit]
	}

	return encode(map[string]any{
		"count": len(items),
		"items": newInventoryToolViews(items),
	})
}

func (r *Registry) executePantrySearch(ctx context.Context, _ Invocation, args map[string]any) (string, error) {
	name, err := requiredStringArg(args, "item_name")
	if err != nil {
		return "", err
	}

	limit, err := r.inventoryLimit(args, 20)
	if err != nil {
		return "", err
	}

	items, err := r.pantry.SearchInventory(ctx, name, "active", "asc", limit)
	if err != nil {
		return "", err
	}

	return encode(map[string]any{
		"count": len(items),
		"items": newInventoryToolViews(items),
	})
}

func (r *Registry) executePantryTemplate(ctx context.Context, _ Invocation, args map[string]any) (string, error) {
	name, err := requiredStringArg(args, "material_name")
	if err != nil {
		return "", err
	}

	template, err := r.pantry.MaterialTemplateByName(ctx, name)
	if err != nil {
		return "", err
	}

	return encode(newMaterialTemplateToolView(template))
}

func (r *Registry) executePantryAdd(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	quantity, err := requiredFloatArg(args, "quantity")
	if err != nil {
		return "", err
	}
	if quantity <= 0 {
		return "", badArgument("参数 quantity 必须大于 0")
	}

	locationID, err := r.resolveLocation(ctx, args)
	if err != nil {
		return "", err
	}

	input := service.InventoryInput{
		LocationID: locationID,
		Quantity:   quantity,
	}

	if input.Name, err = optionalStringArg(args, "name"); err != nil {
		return "", err
	}
	if input.Unit, err = optionalStringArg(args, "unit"); err != nil {
		return "", err
	}
	if input.Description, err = optionalStringArg(args, "description"); err != nil {
		return "", err
	}

	templateName, err := optionalStringArg(args, "template_name")
	if err != nil {
		return "", err
	}
	if templateName != "" {
		template, templateErr := r.pantry.MaterialTemplateByName(ctx, templateName)
		if templateErr != nil {
			return "", badArgument(
				"没有找到名为 %q 的物料品类，请改用 name 加 expires_on 直接登记", templateName,
			)
		}

		templateID := template.ID
		input.TemplateID = &templateID
	}

	if input.StockedOn, err = r.optionalDateArg(args, "stocked_on"); err != nil {
		return "", err
	}
	if input.ExpiresOn, err = r.optionalDateArg(args, "expires_on"); err != nil {
		return "", err
	}

	if input.Name == "" && templateName == "" {
		return "", badArgument("必须提供 name 或 template_name")
	}
	if input.ExpiresOn.IsZero() && templateName == "" {
		return "", badArgument("没有提供 template_name 时必须提供 expires_on")
	}

	created, err := r.pantry.CreateInventory(ctx, invocation.Actor, input)
	if err != nil {
		return "", err
	}

	return encode(map[string]any{
		"created": true,
		"item":    newInventoryToolView(created),
	})
}

func (r *Registry) executePantryDiscard(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	itemID, err := requiredUintArg(args, "item_id")
	if err != nil {
		return "", err
	}

	version, err := requiredUintArg(args, "version")
	if err != nil {
		return "", err
	}

	reason, err := optionalStringArg(args, "reason")
	if err != nil {
		return "", err
	}

	if err := r.pantry.Discard(ctx, invocation.Actor, itemID, version, reason); err != nil {
		return "", err
	}

	return encode(map[string]any{
		"discarded": true,
		"item_id":   itemID,
		"reason":    reason,
	})
}

// resolveLocation accepts either the numeric id from the system prompt or a
// human name, so the model can use whichever it has at hand.
func (r *Registry) resolveLocation(ctx context.Context, args map[string]any) (uint64, error) {
	locationID, err := optionalUintArg(args, "location_id")
	if err != nil {
		return 0, err
	}
	if locationID != nil && *locationID > 0 {
		return *locationID, nil
	}

	name, err := optionalStringArg(args, "location_name")
	if err != nil {
		return 0, err
	}
	if name == "" {
		return 0, badArgument("必须提供 location_id 或 location_name")
	}

	locations, err := r.pantry.Locations(ctx)
	if err != nil {
		return 0, err
	}

	available := make([]string, 0, len(locations))
	for _, location := range locations {
		if !location.Enabled {
			continue
		}
		if location.Name == name {
			return location.ID, nil
		}
		available = append(available, location.Name)
	}

	for _, location := range locations {
		if location.Enabled && strings.Contains(location.Name, name) {
			return location.ID, nil
		}
	}

	return 0, badArgument(
		"没有名为 %q 的存储位置，可选的位置有：%s",
		name, strings.Join(available, "、"),
	)
}

func (r *Registry) inventoryLimit(args map[string]any, fallback int) (int, error) {
	limit, err := optionalIntArg(args, "limit")
	if err != nil {
		return 0, err
	}
	if limit == nil || *limit <= 0 {
		return fallback, nil
	}
	if *limit > maxInventoryLimit {
		return maxInventoryLimit, nil
	}

	return *limit, nil
}
