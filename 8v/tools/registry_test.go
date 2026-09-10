package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"vhome/internal/model"
	"vhome/internal/service"
)

// newTestRegistry builds a registry whose services are present but have no
// database behind them. Every case below fails during argument validation,
// which happens before any service call, so no database is needed.
func newTestRegistry() *Registry {
	return NewRegistry(Options{
		Pantry:   &service.PantryService{},
		Meal:     &service.MealService{},
		Expense:  &service.ExpenseService{},
		Memo:     &service.MemoService{},
		Identity: &service.IdentityService{},
	})
}

func testActor() service.AuthenticatedIdentity {
	return service.AuthenticatedIdentity{
		MemberID:    1,
		HouseholdID: 1,
		DisplayName: "测试成员",
		Role:        model.MemberRoleMember,
	}
}

func TestRegistryAdvertisesEveryTool(t *testing.T) {
	want := []string{
		"expense_get_my_month",
		"expense_get_summary",
		"expense_list_categories",
		"expense_query_range",
		"expense_record",
		"household_list_members",
		"meal_get_day",
		"meal_get_month",
		"meal_search_food",
		"memo_create",
		"memo_list",
		"pantry_add",
		"pantry_discard",
		"pantry_list",
		"pantry_search",
		"pantry_template",
	}

	definitions := newTestRegistry().Definitions()
	if len(definitions) != len(want) {
		t.Fatalf("got %d tools, want %d", len(definitions), len(want))
	}

	// Definitions is sorted by name, so a positional comparison is stable.
	for i, name := range want {
		if definitions[i].Name != name {
			t.Errorf("tool %d: got %q, want %q", i, definitions[i].Name, name)
		}
		if definitions[i].Description == "" {
			t.Errorf("tool %q has no description", name)
		}
		if definitions[i].Parameters == nil {
			t.Errorf("tool %q has no parameter schema", name)
		}
	}
}

// A registry built without a service must not advertise that service's tools:
// the model should never see a tool that is certain to fail.
func TestRegistrySkipsToolsWithoutService(t *testing.T) {
	if got := len(NewRegistry(Options{}).Definitions()); got != 0 {
		t.Fatalf("registry without services advertised %d tools", got)
	}

	only := NewRegistry(Options{Pantry: &service.PantryService{}})
	for _, definition := range only.Definitions() {
		if got := definition.Name[:6]; got != "pantry" {
			t.Errorf("unexpected tool %q for a pantry-only registry", definition.Name)
		}
	}
}

func TestExecuteRejectsAnonymousActor(t *testing.T) {
	_, err := newTestRegistry().Execute(
		context.Background(), Invocation{}, "pantry_list", "",
	)
	if err == nil {
		t.Fatal("expected an anonymous invocation to be refused")
	}
}

func TestExecuteRejectsWritesInReadOnlyMode(t *testing.T) {
	registry := newTestRegistry()
	invocation := Invocation{Actor: testActor()} // AllowWrites is false

	for _, name := range []string{"pantry_add", "pantry_discard", "expense_record", "memo_create"} {
		if _, err := registry.Execute(context.Background(), invocation, name, "{}"); err == nil {
			t.Errorf("%s: expected a refusal while writes are disabled", name)
		}
	}

	// Reads stay available in read-only mode; this one fails on its arguments,
	// which proves authorization let it through.
	_, err := registry.Execute(context.Background(), invocation, "pantry_search", "{}")
	var argErr argumentError
	if !errors.As(err, &argErr) {
		t.Errorf("read tool was blocked in read-only mode: %v", err)
	}
}

func TestAuthorizeEnforcesRequiredRole(t *testing.T) {
	invocation := Invocation{Actor: testActor(), AllowWrites: true}

	if err := authorize(Tool{Name: "x", RequiredRole: model.MemberRoleOwner}, invocation); err == nil {
		t.Error("a plain member must not pass an owner-only tool")
	}
	if err := authorize(Tool{Name: "x", RequiredRole: model.MemberRoleMember}, invocation); err != nil {
		t.Errorf("a plain member must pass a member-level tool: %v", err)
	}
	if err := authorize(Tool{Name: "x"}, invocation); err != nil {
		t.Errorf("an unrestricted tool must accept any member: %v", err)
	}
}

// Argument mistakes are the model's to fix, so they must reach it verbatim
// rather than being replaced by the generic failure message.
func TestExecuteReturnsArgumentErrorsVerbatim(t *testing.T) {
	registry := newTestRegistry()
	invocation := Invocation{Actor: testActor(), AllowWrites: true}

	cases := []struct{ name, args, want string }{
		{"pantry_search", `{}`, "缺少必填参数 item_name"},
		{"pantry_add", `{"quantity":1}`, "必须提供 location_id 或 location_name"},
		{"pantry_discard", `{"item_id":1}`, "缺少必填参数 version"},
		{"meal_get_day", `{"date":"09/10/2026"}`, "参数 date 必须是 YYYY-MM-DD 格式的日期"},
		{"expense_record", `{"title":"x","amount_yuan":-5,"category_id":1}`, "参数 amount_yuan 必须大于 0"},
		{"expense_query_range", `{"start_date":"2026-09-10","end_date":"2026-09-01"}`, "end_date 不能早于 start_date"},
		{"memo_create", `{"title":"x","remind_at":"tomorrow"}`, "参数 remind_at 必须是 YYYY-MM-DD HH:MM 格式的时间"},
	}

	for _, c := range cases {
		_, err := registry.Execute(context.Background(), invocation, c.name, c.args)
		if err == nil {
			t.Errorf("%s: expected an error", c.name)
			continue
		}

		var argErr argumentError
		if !errors.As(err, &argErr) {
			t.Errorf("%s: error was replaced by a generic message: %v", c.name, err)
			continue
		}
		if err.Error() != c.want {
			t.Errorf("%s: got %q, want %q", c.name, err.Error(), c.want)
		}
	}
}

func TestExecuteRejectsUnknownToolAndBrokenArguments(t *testing.T) {
	registry := newTestRegistry()
	invocation := Invocation{Actor: testActor(), AllowWrites: true}

	if _, err := registry.Execute(context.Background(), invocation, "no_such_tool", ""); err == nil {
		t.Error("expected an unknown tool to be refused")
	}
	if _, err := registry.Execute(context.Background(), invocation, "pantry_list", "{oops"); err == nil {
		t.Error("expected malformed JSON arguments to be refused")
	}
}

// The prompt section must survive a registry that could not load the roster or
// the storage locations: a degraded snapshot is better than a failed answer.
func TestSnapshotPromptSectionWithoutData(t *testing.T) {
	snapshot := NewRegistry(Options{}).Snapshot(context.Background(), testActor())

	section := snapshot.PromptSection()
	for _, want := range []string{"当前时间", "今天", "测试成员", "普通成员"} {
		if !strings.Contains(section, want) {
			t.Errorf("prompt section is missing %q:\n%s", want, section)
		}
	}
}
