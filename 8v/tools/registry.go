package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"vhome/8v/llm"
	"vhome/internal/model"
	"vhome/internal/service"
)

// householdTimeZone is the wall clock every date argument is interpreted in.
// The services store and compare dates in the same zone.
const householdTimeZone = "Asia/Shanghai"

type Executor func(ctx context.Context, invocation Invocation, args map[string]any) (string, error)

type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Executor    Executor

	// Dangerous marks a tool that changes household data. Such tools are
	// refused when the invocation is read-only.
	Dangerous bool

	// RequiredRole is the minimum role allowed to run the tool. The empty
	// value means any authenticated member, which matches what the web UI
	// permits for all tools registered today; it is the hook to use when a
	// future tool needs to be restricted to admins or the owner.
	RequiredRole model.MemberRole
}

// Invocation carries the per-request facts the registry needs: who is asking,
// and whether this conversation is allowed to change anything.
type Invocation struct {
	Actor       service.AuthenticatedIdentity
	AllowWrites bool
}

type Options struct {
	Pantry   *service.PantryService
	Meal     *service.MealService
	Expense  *service.ExpenseService
	Memo     *service.MemoService
	Identity *service.IdentityService
	Memory   *service.AgentMemoryService
}

type Registry struct {
	tools    map[string]Tool
	location *time.Location

	pantry   *service.PantryService
	meal     *service.MealService
	expense  *service.ExpenseService
	memo     *service.MemoService
	identity *service.IdentityService
	memory   *service.AgentMemoryService
}

func NewRegistry(opts Options) *Registry {
	location, err := time.LoadLocation(householdTimeZone)
	if err != nil {
		location = time.FixedZone("CST", 8*60*60)
	}

	r := &Registry{
		tools:    map[string]Tool{},
		location: location,

		pantry:   opts.Pantry,
		meal:     opts.Meal,
		expense:  opts.Expense,
		memo:     opts.Memo,
		identity: opts.Identity,
		memory:   opts.Memory,
	}

	// A tool whose service is missing is never advertised, so the model can
	// not call something that is guaranteed to fail.
	if r.pantry != nil {
		r.registerPantryTools()
	}
	if r.meal != nil {
		r.registerMealTools()
	}
	if r.expense != nil {
		r.registerExpenseTools()
	}
	if r.memo != nil {
		r.registerMemoTools()
	}
	if r.identity != nil {
		r.registerHouseholdTools()
	}
	if r.memory != nil {
		r.registerMemoryTools()
	}

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

// Execute runs one tool call. Every failure it returns is safe to hand back to
// the model: internal errors are logged here and replaced with a short Chinese
// explanation, so database and file system details never reach the LLM or, via
// its answer, the user.
func (r *Registry) Execute(ctx context.Context, invocation Invocation, name string, rawArgs string) (string, error) {
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("不存在名为 %s 的工具", name)
	}

	if err := authorize(tool, invocation); err != nil {
		return "", err
	}

	var args map[string]any
	if trimmed := strings.TrimSpace(rawArgs); trimmed != "" {
		if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
			return "", fmt.Errorf("工具 %s 的参数不是合法的 JSON", name)
		}
	}

	out, err := tool.Executor(ctx, invocation, args)
	if err != nil {
		return "", r.sanitize(name, invocation, err)
	}

	return out, nil
}

var roleRank = map[model.MemberRole]int{
	model.MemberRoleMember: 1,
	model.MemberRoleAdmin:  2,
	model.MemberRoleOwner:  3,
}

func authorize(tool Tool, invocation Invocation) error {
	actor := invocation.Actor

	if actor.MemberID == 0 || actor.HouseholdID == 0 {
		return errors.New("当前对话没有登录成员的身份，无法查询家庭数据")
	}

	if tool.Dangerous && !invocation.AllowWrites {
		return fmt.Errorf("工具 %s 会修改家庭数据，当前会话为只读模式", tool.Name)
	}

	if tool.RequiredRole != "" && roleRank[actor.Role] < roleRank[tool.RequiredRole] {
		return fmt.Errorf(
			"当前成员的角色是%s，没有权限执行 %s，需要%s及以上权限",
			roleLabel(actor.Role),
			tool.Name,
			roleLabel(tool.RequiredRole),
		)
	}

	return nil
}

// argumentError marks a failure caused by the model's own arguments. Those
// messages are written for the model to read and correct, so they pass through
// sanitize unchanged.
type argumentError struct{ message string }

func (e argumentError) Error() string { return e.message }

func badArgument(format string, args ...any) error {
	return argumentError{message: fmt.Sprintf(format, args...)}
}

func (r *Registry) sanitize(name string, invocation Invocation, err error) error {
	var argErr argumentError
	if errors.As(err, &argErr) {
		return argErr
	}

	switch {
	case errors.Is(err, service.ErrUnauthenticated):
		return errors.New("登录状态已失效，请重新登录后再试")

	case errors.Is(err, service.ErrForbidden):
		return errors.New("当前成员没有执行该操作的权限")

	case errors.Is(err, service.ErrNotFound):
		return errors.New("没有找到对应的记录，请先查询确认后再操作")

	case errors.Is(err, service.ErrConflict):
		return errors.New("这条记录刚刚被其他人修改过，请重新查询最新数据后再操作")

	case errors.Is(err, service.ErrInvalidInput):
		return errors.New("参数不符合要求，请检查后重试")

	case errors.Is(err, service.ErrMemoTimeInvalid):
		return errors.New("提醒时间必须晚于当前时间，且分钟必须是 00 或 30")

	case errors.Is(err, service.ErrMemoRecipientInvalid):
		return errors.New("提醒对象无效：成员不存在、未启用或不属于当前家庭")

	case errors.Is(err, service.ErrMemoSlotFull):
		return errors.New("该成员在这个小时内的提醒已达上限（最多 5 条），请换一个时间")

	case errors.Is(err, service.ErrMaterialNameExists):
		return errors.New("同名的物料品类已经存在")

	case errors.Is(err, service.ErrAgentMemoryFull):
		return errors.New("记忆库已经满了，请先让用户确认删掉一些不再需要的记忆")

	case errors.Is(err, service.ErrAgentMemoryScopeInvalid):
		return errors.New("参数 scope 必须是 HOUSEHOLD 或 MEMBER")
	}

	// Anything else is a bug or an infrastructure failure. Keep the detail in
	// the server log and give the model a generic message.
	slog.Error(
		"agent tool execution failed",
		"tool", name,
		"member_id", invocation.Actor.MemberID,
		"error", err,
	)

	return errors.New("这个操作暂时失败了，请稍后重试或改用其他方式")
}

// Snapshot collects the small, slow-changing facts that belong in the system
// prompt rather than behind a tool call: who lives here and where things are
// stored. Failures are not fatal — the agent simply runs with less context.
func (r *Registry) Snapshot(ctx context.Context, actor service.AuthenticatedIdentity) HouseholdSnapshot {
	snapshot := HouseholdSnapshot{
		Now:   time.Now().In(r.location),
		Actor: actor,
	}

	if r.identity != nil {
		members, err := r.identity.HouseholdRoster(ctx, actor)
		if err != nil {
			slog.Warn("agent snapshot: load roster", "error", err)
		} else {
			snapshot.Members = members
		}
	}

	if r.pantry != nil {
		locations, err := r.pantry.Locations(ctx)
		if err != nil {
			slog.Warn("agent snapshot: load storage locations", "error", err)
		} else {
			for _, location := range locations {
				if location.Enabled {
					snapshot.Locations = append(snapshot.Locations, location)
				}
			}
		}
	}

	if r.memory != nil {
		memories, err := r.memory.ListFor(ctx, actor)
		if err != nil {
			slog.Warn("agent snapshot: load memories", "error", err)
		} else {
			snapshot.Memories = memories
		}
	}

	return snapshot
}
