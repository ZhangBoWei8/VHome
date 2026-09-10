package tools

import (
	"context"

	"vhome/internal/service"
)

// maxMemoRangeDays matches the two month window MyMemosByRange accepts.
const maxMemoRangeDays = 60

func (r *Registry) registerMemoTools() {
	r.register(Tool{
		Name: "memo_list",
		Description: "列出发给当前成员的备忘提醒，按时间区间查询。" +
			"start_date 和 end_date 都包含在内，区间最长 60 天；" +
			"都省略时默认查询今天起未来 7 天。用于回答“我最近有什么安排”“今天有什么提醒”。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"start_date": map[string]any{
					"type":        "string",
					"description": "可选，开始日期 YYYY-MM-DD，省略时为今天",
					"pattern":     `^\d{4}-\d{2}-\d{2}$`,
				},
				"end_date": map[string]any{
					"type":        "string",
					"description": "可选，结束日期 YYYY-MM-DD（含当天），省略时为开始日期后 7 天",
					"pattern":     `^\d{4}-\d{2}-\d{2}$`,
				},
			},
			"additionalProperties": false,
		},
		Executor: r.executeMemoList,
	})

	r.register(Tool{
		Name: "memo_create",
		Description: "创建一条备忘提醒。只有在用户明确要求设置提醒时才调用。" +
			"remind_at 必须晚于当前时间，且分钟只能是 00 或 30（例如 2026-09-11 08:00 或 08:30）。" +
			"recipient_ids 用系统提示里的家庭成员 id；省略时只提醒当前成员。" +
			"同一个成员同一小时内最多 5 条提醒。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "提醒标题，例如“去开家长会”",
					"minLength":   1,
					"maxLength":   128,
				},
				"remind_at": map[string]any{
					"type":        "string",
					"description": "提醒时间 YYYY-MM-DD HH:MM，分钟必须是 00 或 30",
					"pattern":     `^\d{4}-\d{2}-\d{2}[ T]\d{2}:(00|30)$`,
				},
				"description": map[string]any{
					"type":        "string",
					"description": "可选，提醒的详细说明",
					"maxLength":   2000,
				},
				"recipient_ids": map[string]any{
					"type":        "array",
					"description": "可选，接收提醒的成员 id 列表；省略时只提醒当前成员",
					"items":       map[string]any{"type": "integer", "minimum": 1},
					"maxItems":    20,
				},
			},
			"required":             []string{"title", "remind_at"},
			"additionalProperties": false,
		},
		Dangerous: true,
		Executor:  r.executeMemoCreate,
	})
}

func (r *Registry) executeMemoList(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	start, err := r.optionalDateArg(args, "start_date")
	if err != nil {
		return "", err
	}
	if start.IsZero() {
		start = r.today()
	}

	end, err := r.optionalDateArg(args, "end_date")
	if err != nil {
		return "", err
	}
	if end.IsZero() {
		end = start.AddDate(0, 0, 7)
	}
	if end.Before(start) {
		return "", badArgument("end_date 不能早于 start_date")
	}
	if end.Sub(start).Hours()/24 > maxMemoRangeDays {
		return "", badArgument("查询区间最长 %d 天，请分段查询", maxMemoRangeDays)
	}

	memos, err := r.memo.MyMemosByRange(ctx, invocation.Actor, start, end.AddDate(0, 0, 1))
	if err != nil {
		return "", err
	}

	names := r.memberNames(ctx, invocation.Actor)

	items := make([]memoToolView, 0, len(memos))
	for _, memo := range memos {
		items = append(items, r.newMemoToolView(memo, invocation.Actor.MemberID, names))
	}

	return encode(map[string]any{
		"start_date": start.Format(dateLayout),
		"end_date":   end.Format(dateLayout),
		"count":      len(items),
		"memos":      items,
	})
}

func (r *Registry) executeMemoCreate(ctx context.Context, invocation Invocation, args map[string]any) (string, error) {
	title, err := requiredStringArg(args, "title")
	if err != nil {
		return "", err
	}

	remindAt, err := r.optionalDateTimeArg(args, "remind_at")
	if err != nil {
		return "", err
	}
	if remindAt.IsZero() {
		return "", badArgument("缺少必填参数 remind_at")
	}

	description, err := optionalStringArg(args, "description")
	if err != nil {
		return "", err
	}

	recipients, err := optionalUintListArg(args, "recipient_ids")
	if err != nil {
		return "", err
	}
	if len(recipients) == 0 {
		recipients = []uint64{invocation.Actor.MemberID}
	}

	created, err := r.memo.CreateMemo(ctx, invocation.Actor, service.CreateMemoInput{
		Title:        title,
		Description:  description,
		RemindAt:     remindAt,
		RecipientIDs: recipients,
	})
	if err != nil {
		return "", err
	}

	return encode(map[string]any{
		"created": true,
		"memo": r.newMemoToolView(
			created,
			invocation.Actor.MemberID,
			r.memberNames(ctx, invocation.Actor),
		),
	})
}

// memberNames maps member ids to display names so memo recipients read as
// people instead of numbers. A failure here only degrades the labels.
func (r *Registry) memberNames(ctx context.Context, actor service.AuthenticatedIdentity) map[uint64]string {
	names := map[uint64]string{}

	if r.identity == nil {
		return names
	}

	roster, err := r.identity.HouseholdRoster(ctx, actor)
	if err != nil {
		return names
	}

	for _, member := range roster {
		names[member.ID] = member.DisplayName
	}

	return names
}
