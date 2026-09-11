package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vhome/internal/model"
	"vhome/internal/service"
)

// The views in this file are what the model actually sees. They exist for
// three reasons: to expose the id and version a follow-up write tool needs, to
// drop columns the model has no use for (icons, image paths, audit stamps,
// password hashes), and to translate enums into Chinese the model can reason
// about without a lookup table.

func encode(payload any) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode tool result: %w", err)
	}

	return string(data), nil
}

func roleLabel(role model.MemberRole) string {
	switch role {
	case model.MemberRoleOwner:
		return "户主"
	case model.MemberRoleAdmin:
		return "管理员"
	case model.MemberRoleMember:
		return "普通成员"
	default:
		return string(role)
	}
}

func presenceLabel(status model.PresenceStatus) string {
	switch status {
	case model.PresenceHome:
		return "在家"
	case model.PresenceSchool:
		return "在学校"
	case model.PresenceWorking:
		return "工作中"
	case model.PresenceOut:
		return "外出"
	case model.PresenceNapping:
		return "午休中"
	case model.PresenceResting:
		return "休息中"
	case model.PresenceSick:
		return "生病中"
	case model.PresenceStudying:
		return "学习中"
	default:
		return "未知"
	}
}

func storageTypeLabel(storageType model.StorageType) string {
	switch storageType {
	case model.StorageTypeCold:
		return "冷藏冷冻"
	case model.StorageTypeAmbient:
		return "常温"
	default:
		return string(storageType)
	}
}

func mealTypeLabel(mealType model.MealType) string {
	switch mealType {
	case model.MealTypeBreakfast:
		return "早餐"
	case model.MealTypeLunch:
		return "午餐"
	case model.MealTypeDinner:
		return "晚餐"
	case model.MealTypeSnack:
		return "加餐"
	default:
		return string(mealType)
	}
}

func expenseScopeLabel(scope model.ExpenseScope) string {
	if scope == model.ExpenseScopeCollective {
		return "家庭公共支出"
	}

	return "个人支出"
}

func weekdayLabel(weekday time.Weekday) string {
	return [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[weekday]
}

// ---------------------------------------------------------------- pantry

type inventoryToolView struct {
	ID            uint64  `json:"id"`
	Version       uint64  `json:"version"`
	Name          string  `json:"name"`
	Quantity      float64 `json:"quantity"`
	Unit          string  `json:"unit"`
	Location      string  `json:"location"`
	StockedOn     string  `json:"stocked_on"`
	ExpiresOn     string  `json:"expires_on"`
	RemainingDays int     `json:"remaining_days"`

	CaloriesPer100G     *float64 `json:"calories_per_100g,omitempty"`
	ProteinPer100G      *float64 `json:"protein_per_100g,omitempty"`
	FatPer100G          *float64 `json:"fat_per_100g,omitempty"`
	CarbohydratePer100G *float64 `json:"carbohydrate_per_100g,omitempty"`

	Description string `json:"description,omitempty"`
}

func newInventoryToolView(item service.InventoryView) inventoryToolView {
	return inventoryToolView{
		ID:            item.ID,
		Version:       item.Version,
		Name:          item.Name,
		Quantity:      item.Quantity,
		Unit:          item.Unit,
		Location:      item.LocationName,
		StockedOn:     item.StockedOn.Format(dateLayout),
		ExpiresOn:     item.ExpiresOn.Format(dateLayout),
		RemainingDays: item.RemainingDays,

		CaloriesPer100G:     item.CaloriesPer100G,
		ProteinPer100G:      item.ProteinPer100G,
		FatPer100G:          item.FatPer100G,
		CarbohydratePer100G: item.CarbohydratePer100G,

		Description: item.Description,
	}
}

func newInventoryToolViews(items []service.InventoryView) []inventoryToolView {
	out := make([]inventoryToolView, 0, len(items))
	for _, item := range items {
		out = append(out, newInventoryToolView(item))
	}

	return out
}

type materialTemplateToolView struct {
	Name                 string `json:"name"`
	DefaultUnit          string `json:"default_unit"`
	ColdShelfLifeDays    *int   `json:"cold_shelf_life_days"`
	AmbientShelfLifeDays *int   `json:"ambient_shelf_life_days"`

	CaloriesPer100G     *float64 `json:"calories_per_100g"`
	ProteinPer100G      *float64 `json:"protein_per_100g"`
	FatPer100G          *float64 `json:"fat_per_100g"`
	CarbohydratePer100G *float64 `json:"carbohydrate_per_100g"`

	Note string `json:"note"`
}

func newMaterialTemplateToolView(template model.MaterialTemplate) materialTemplateToolView {
	return materialTemplateToolView{
		Name:                 template.Name,
		DefaultUnit:          template.DefaultUnit,
		ColdShelfLifeDays:    template.ColdShelfLifeDays,
		AmbientShelfLifeDays: template.AmbientShelfLifeDays,

		CaloriesPer100G:     template.CaloriesPer100G,
		ProteinPer100G:      template.ProteinPer100G,
		FatPer100G:          template.FatPer100G,
		CarbohydratePer100G: template.CarbohydratePer100G,

		Note: "这是物料品类的默认参考值，不代表家里当前有这个物料，也不是某一批实际库存的到期日",
	}
}

// ---------------------------------------------------------------- meal

type foodToolView struct {
	ID                  uint64   `json:"id"`
	Name                string   `json:"name"`
	CaloriesPer100G     float64  `json:"calories_per_100g"`
	ProteinPer100G      *float64 `json:"protein_per_100g,omitempty"`
	FatPer100G          *float64 `json:"fat_per_100g,omitempty"`
	CarbohydratePer100G *float64 `json:"carbohydrate_per_100g,omitempty"`
	NutritionComplete   bool     `json:"nutrition_complete"`
}

func newFoodToolViews(foods []service.FoodView) []foodToolView {
	out := make([]foodToolView, 0, len(foods))
	for _, food := range foods {
		out = append(out, foodToolView{
			ID:                  food.ID,
			Name:                food.Name,
			CaloriesPer100G:     food.CaloriesPer100G,
			ProteinPer100G:      food.ProteinPer100G,
			FatPer100G:          food.FatPer100G,
			CarbohydratePer100G: food.CarbohydratePer100G,
			NutritionComplete:   food.NutritionComplete,
		})
	}

	return out
}

type mealRecordToolView struct {
	MealType    string  `json:"meal_type"`
	FoodName    string  `json:"food_name"`
	WeightGrams float64 `json:"weight_grams"`
	Calories    float64 `json:"calories"`
}

type mealDayToolView struct {
	Date       string `json:"date"`
	MemberName string `json:"member_name"`

	TotalCalories     float64 `json:"total_calories"`
	TotalProtein      float64 `json:"total_protein_grams"`
	TotalFat          float64 `json:"total_fat_grams"`
	TotalCarbohydrate float64 `json:"total_carbohydrate_grams"`

	RecordCount         uint64 `json:"record_count"`
	NutritionIncomplete bool   `json:"nutrition_incomplete"`

	Records []mealRecordToolView `json:"records"`
}

func newMealDayToolView(day service.MealDayView) mealDayToolView {
	records := make([]mealRecordToolView, 0, len(day.Records))
	for _, record := range day.Records {
		records = append(records, mealRecordToolView{
			MealType:    mealTypeLabel(record.MealType),
			FoodName:    record.FoodNameSnapshot,
			WeightGrams: record.WeightGrams,
			Calories:    round1(record.Calories),
		})
	}

	return mealDayToolView{
		Date:       day.Date,
		MemberName: day.Member.DisplayName,

		TotalCalories:     round1(day.Summary.Calories),
		TotalProtein:      round1(day.Summary.Protein),
		TotalFat:          round1(day.Summary.Fat),
		TotalCarbohydrate: round1(day.Summary.Carbohydrate),

		RecordCount:         day.Summary.RecordCount,
		NutritionIncomplete: day.Summary.NutritionIncomplete,

		Records: records,
	}
}

type mealCalendarDayToolView struct {
	Date        string  `json:"date"`
	Calories    float64 `json:"calories"`
	RecordCount uint64  `json:"record_count"`
}

type mealMonthToolView struct {
	Month              string  `json:"month"`
	RecordedDays       int     `json:"recorded_days"`
	TotalCalories      float64 `json:"total_calories"`
	AverageCaloriesDay float64 `json:"average_calories_per_recorded_day"`

	Days []mealCalendarDayToolView `json:"days"`
}

func newMealMonthToolView(calendar service.MealCalendarView) mealMonthToolView {
	view := mealMonthToolView{
		Month: calendar.Month,
		Days:  make([]mealCalendarDayToolView, 0, len(calendar.Days)),
	}

	for _, day := range calendar.Days {
		if day.RecordCount == 0 {
			continue
		}

		view.RecordedDays++
		view.TotalCalories += day.Calories
		view.Days = append(view.Days, mealCalendarDayToolView{
			Date:        day.Date,
			Calories:    round1(day.Calories),
			RecordCount: day.RecordCount,
		})
	}

	view.TotalCalories = round1(view.TotalCalories)
	if view.RecordedDays > 0 {
		view.AverageCaloriesDay = round1(view.TotalCalories / float64(view.RecordedDays))
	}

	return view
}

// ---------------------------------------------------------------- expense

type expenseRecordToolView struct {
	ID      uint64 `json:"id"`
	Version uint64 `json:"version"`

	Scope      string  `json:"scope"`
	Category   string  `json:"category"`
	Title      string  `json:"title"`
	AmountYuan float64 `json:"amount_yuan"`
	SpentOn    string  `json:"spent_on"`

	MemberName string `json:"member_name,omitempty"`
	Note       string `json:"note,omitempty"`
}

func newExpenseRecordToolView(record service.ExpenseRecordView) expenseRecordToolView {
	return expenseRecordToolView{
		ID:      record.ID,
		Version: record.Version,

		Scope:      expenseScopeLabel(record.ExpenseScope),
		Category:   record.Category.Name,
		Title:      record.Title,
		AmountYuan: yuan(record.AmountCents),
		SpentOn:    record.SpentOn.Format(dateLayout),

		MemberName: record.MemberName,
		Note:       record.Note,
	}
}

func newExpenseRecordToolViews(records []service.ExpenseRecordView) []expenseRecordToolView {
	out := make([]expenseRecordToolView, 0, len(records))
	for _, record := range records {
		out = append(out, newExpenseRecordToolView(record))
	}

	return out
}

type expenseCategoryTotalToolView struct {
	Category   string  `json:"category"`
	AmountYuan float64 `json:"amount_yuan"`
	Percentage float64 `json:"percentage"`
}

type monthlyExpenseToolView struct {
	Month    string `json:"month"`
	Currency string `json:"currency"`

	TotalYuan      float64 `json:"total_yuan"`
	PersonalYuan   float64 `json:"personal_yuan"`
	CollectiveYuan float64 `json:"collective_yuan"`

	CategoryTotals []expenseCategoryTotalToolView `json:"category_totals"`
	Records        []expenseRecordToolView        `json:"records"`
}

func newMonthlyExpenseToolView(view service.MonthlyExpenseView) monthlyExpenseToolView {
	totals := make([]expenseCategoryTotalToolView, 0, len(view.CategoryTotals))
	for _, total := range view.CategoryTotals {
		totals = append(totals, expenseCategoryTotalToolView{
			Category:   total.Category.Name,
			AmountYuan: yuan(total.AmountCents),
			Percentage: round1(total.Percentage),
		})
	}

	return monthlyExpenseToolView{
		Month:    view.Month,
		Currency: "CNY",

		TotalYuan:      yuan(view.TotalAmountCents),
		PersonalYuan:   yuan(view.PersonalAmountCents),
		CollectiveYuan: yuan(view.CollectiveAmountCents),

		CategoryTotals: totals,
		Records:        newExpenseRecordToolViews(view.Records),
	}
}

type expenseCategoryToolView struct {
	ID   uint16 `json:"id"`
	Name string `json:"name"`
}

func newExpenseCategoryToolViews(categories []model.ExpenseCategory) []expenseCategoryToolView {
	out := make([]expenseCategoryToolView, 0, len(categories))
	for _, category := range categories {
		out = append(out, expenseCategoryToolView{
			ID:   category.ID,
			Name: category.Name,
		})
	}

	return out
}

type householdExpenseSummaryToolView struct {
	Month    string `json:"month"`
	Currency string `json:"currency"`

	TotalYuan         float64  `json:"total_yuan"`
	PersonalYuan      float64  `json:"personal_yuan"`
	CollectiveYuan    float64  `json:"collective_yuan"`
	PreviousMonthYuan float64  `json:"previous_month_total_yuan"`
	ChangePercent     *float64 `json:"change_percent_vs_previous_month"`
}

func newHouseholdExpenseSummaryToolView(summary service.HouseholdExpenseSummary) householdExpenseSummaryToolView {
	view := householdExpenseSummaryToolView{
		Month:    summary.Month,
		Currency: "CNY",

		TotalYuan:         yuan(summary.TotalAmountCents),
		PersonalYuan:      yuan(summary.PersonalAmountCents),
		CollectiveYuan:    yuan(summary.CollectiveAmountCents),
		PreviousMonthYuan: yuan(summary.PreviousMonthAmountCents),
	}

	if summary.ChangePercent != nil {
		rounded := round1(*summary.ChangePercent)
		view.ChangePercent = &rounded
	}

	return view
}

// ---------------------------------------------------------------- memo

type memoToolView struct {
	ID      uint64 `json:"id"`
	Version uint64 `json:"version"`

	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	RemindAt    string `json:"remind_at"`

	Recipients  []string `json:"recipients"`
	CreatedByMe bool     `json:"created_by_me"`
}

func (r *Registry) newMemoToolView(memo service.MemoView, actorID uint64, names map[uint64]string) memoToolView {
	recipients := make([]string, 0, len(memo.RecipientIDs))
	for _, id := range memo.RecipientIDs {
		if name, ok := names[id]; ok {
			recipients = append(recipients, name)
			continue
		}
		recipients = append(recipients, fmt.Sprintf("成员#%d", id))
	}

	return memoToolView{
		ID:      memo.ID,
		Version: memo.Version,

		Title:       memo.Title,
		Description: memo.Description,
		RemindAt:    memo.RemindAt.In(r.location).Format(dateTimeLayout),

		Recipients:  recipients,
		CreatedByMe: memo.CreatedBy == actorID,
	}
}

// ---------------------------------------------------------------- household

type memberToolView struct {
	ID            uint64 `json:"id"`
	DisplayName   string `json:"display_name"`
	Role          string `json:"role"`
	Presence      string `json:"presence_status"`
	IsCurrentUser bool   `json:"is_current_user"`
}

func newMemberToolViews(members []service.RosterMember, actorID uint64) []memberToolView {
	out := make([]memberToolView, 0, len(members))
	for _, member := range members {
		out = append(out, memberToolView{
			ID:            member.ID,
			DisplayName:   member.DisplayName,
			Role:          roleLabel(member.Role),
			Presence:      presenceLabel(member.Presence),
			IsCurrentUser: member.ID == actorID,
		})
	}

	return out
}

// ---------------------------------------------------------------- memory

type memoryToolView struct {
	ID      uint64 `json:"id"`
	Scope   string `json:"scope"`
	Content string `json:"content"`
	Version uint64 `json:"version"`
}

func newMemoryToolViews(memories []model.AgentMemory) []memoryToolView {
	out := make([]memoryToolView, 0, len(memories))
	for _, memory := range memories {
		out = append(out, memoryToolView{
			ID:      memory.ID,
			Scope:   string(memory.Scope),
			Content: memory.Content,
			Version: memory.Version,
		})
	}

	return out
}

// ---------------------------------------------------------------- snapshot

// HouseholdSnapshot is the slow-changing context injected into the system
// prompt. Keeping the roster and the storage locations here saves the model a
// tool round trip on almost every request that writes something.
type HouseholdSnapshot struct {
	Now       time.Time
	Actor     service.AuthenticatedIdentity
	Members   []service.RosterMember
	Locations []model.StorageLocation
	Memories  []model.AgentMemory
}

// PromptSection renders the snapshot as the facts block of the system prompt.
func (s HouseholdSnapshot) PromptSection() string {
	var b strings.Builder

	b.WriteString("## 当前家庭状况\n")

	fmt.Fprintf(
		&b,
		"当前时间：%s（%s），时区 Asia/Shanghai\n",
		s.Now.Format("2006-01-02 15:04"),
		weekdayLabel(s.Now.Weekday()),
	)
	fmt.Fprintf(&b, "今天：%s，本月：%s\n",
		s.Now.Format(dateLayout), s.Now.Format(monthLayout))

	fmt.Fprintf(
		&b,
		"当前对话的成员：%s（id=%d，%s）\n",
		s.Actor.DisplayName,
		s.Actor.MemberID,
		roleLabel(s.Actor.Role),
	)

	if len(s.Members) > 0 {
		b.WriteString("家庭成员（创建提醒时用这里的 id）：\n")
		for _, member := range s.Members {
			fmt.Fprintf(
				&b,
				"- %s：id=%d，%s，当前状态%s\n",
				member.DisplayName,
				member.ID,
				roleLabel(member.Role),
				presenceLabel(member.Presence),
			)
		}
	}

	if len(s.Locations) > 0 {
		b.WriteString("存储位置（入库时用这里的 id 或名称）：\n")
		for _, location := range s.Locations {
			fmt.Fprintf(
				&b,
				"- %s：id=%d，%s\n",
				location.Name,
				location.ID,
				storageTypeLabel(location.StorageType),
			)
		}
	}

	b.WriteString(s.memorySection())

	return b.String()
}

// memorySection renders the 记忆库. These are durable facts, not live data:
// they are stated as background the model should honour, with an explicit
// warning not to mistake them for current inventory or schedule data.
func (s HouseholdSnapshot) memorySection() string {
	if len(s.Memories) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString("\n## 你记住的事情\n")
	b.WriteString("这些是长期有效的家庭偏好和约束，回答和建议时要遵守。" +
		"它们不是实时数据，库存、账单、提醒的具体数字仍然必须调用工具查询。\n")

	for _, memory := range s.Memories {
		scope := "全家"
		if memory.Scope == model.AgentMemoryScopeMember {
			scope = "仅" + s.Actor.DisplayName
		}

		fmt.Fprintf(&b, "- [%s] %s\n", scope, memory.Content)
	}

	return b.String()
}
