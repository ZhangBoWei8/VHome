package tools

import "context"

func (r *Registry) registerHouseholdTools() {
	r.register(Tool{
		Name: "household_list_members",
		Description: "列出家庭的全部在用成员，包含成员 id、称呼、角色和当前所在状态" +
			"（在家、外出、工作中、在学校等）。" +
			"用于回答“谁在家”“家里有几口人”，或在创建提醒前确认成员 id。" +
			"成员的当前状态是会变化的，回答与此有关的问题时应实时调用，不要依赖历史信息。",
		Parameters: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		Executor: r.executeHouseholdListMembers,
	})
}

func (r *Registry) executeHouseholdListMembers(ctx context.Context, invocation Invocation, _ map[string]any) (string, error) {
	roster, err := r.identity.HouseholdRoster(ctx, invocation.Actor)
	if err != nil {
		return "", err
	}

	return encode(map[string]any{
		"count":   len(roster),
		"members": newMemberToolViews(roster, invocation.Actor.MemberID),
	})
}
