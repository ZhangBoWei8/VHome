package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"vhome/internal/model"
	"vhome/internal/repository"
)

const (
	maxExpenseTitleLength = 128
	maxExpenseNoteLength  = 500
)

type ExpenseService struct {
	repository *repository.Repository
	location   *time.Location
}

func NewExpenseService(repo *repository.Repository) (*ExpenseService, error) {
	if repo == nil {
		return nil, errors.New("expense service repository is nil")
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, fmt.Errorf("load expense time zone: %w", err)
	}
	return &ExpenseService{repository: repo, location: location}, nil
}

type ExpenseInput struct {
	CategoryID   uint16
	ExpenseScope model.ExpenseScope
	Title        string
	AmountCents  uint64
	SpentOn      time.Time
	Note         string
	Version      uint64
}

type ExpenseRecordView struct {
	model.ExpenseRecord
	Category   model.ExpenseCategory `json:"category"`
	MemberName string                `json:"member_name"`
}

type ExpenseCategoryTotal struct {
	Category    model.ExpenseCategory `json:"category"`
	AmountCents uint64                `json:"amount_cents"`
	Percentage  float64               `json:"percentage"`
}

type MonthlyExpenseView struct {
	Month                 string                 `json:"month"`
	TotalAmountCents      uint64                 `json:"total_amount_cents"`
	PersonalAmountCents   uint64                 `json:"personal_amount_cents"`
	CollectiveAmountCents uint64                 `json:"collective_amount_cents"`
	CategoryTotals        []ExpenseCategoryTotal `json:"category_totals"`
	Records               []ExpenseRecordView    `json:"records"`
}

type HouseholdExpenseSummary struct {
	Month                    string   `json:"month"`
	TotalAmountCents         uint64   `json:"total_amount_cents"`
	PersonalAmountCents      uint64   `json:"personal_amount_cents"`
	CollectiveAmountCents    uint64   `json:"collective_amount_cents"`
	PreviousMonthAmountCents uint64   `json:"previous_month_amount_cents"`
	ChangePercent            *float64 `json:"change_percent"`
}

type ExpenseExportView string

const (
	ExpenseExportMine       ExpenseExportView = "MINE"
	ExpenseExportCollective ExpenseExportView = "COLLECTIVE"
	ExpenseExportHousehold  ExpenseExportView = "HOUSEHOLD"
)

func (s *ExpenseService) Categories(ctx context.Context, actor AuthenticatedIdentity) ([]model.ExpenseCategory, error) {
	if actor.MemberID == 0 || actor.HouseholdID == 0 {
		return nil, ErrUnauthenticated
	}
	return s.repository.ListExpenseCategories(ctx)
}

func (s *ExpenseService) CreateExpense(ctx context.Context, actor AuthenticatedIdentity, input ExpenseInput) (ExpenseRecordView, error) {
	input, category, err := s.validateInput(ctx, input, false)
	if err != nil {
		return ExpenseRecordView{}, err
	}
	record, err := s.repository.CreateExpenseRecord(ctx, repository.CreateExpenseRecordParams{
		HouseholdID:  actor.HouseholdID,
		MemberID:     actor.MemberID,
		CategoryID:   input.CategoryID,
		ExpenseScope: input.ExpenseScope,
		Title:        input.Title,
		AmountCents:  input.AmountCents,
		SpentOn:      input.SpentOn,
		Note:         input.Note,
	})
	if err != nil {
		return ExpenseRecordView{}, fmt.Errorf("create expense: %w", err)
	}
	return ExpenseRecordView{ExpenseRecord: record, Category: category, MemberName: actor.DisplayName}, nil
}

func (s *ExpenseService) UpdateExpense(ctx context.Context, actor AuthenticatedIdentity, expenseID uint64, input ExpenseInput) (ExpenseRecordView, error) {
	current, err := s.getManageableExpense(ctx, actor, expenseID)
	if err != nil {
		return ExpenseRecordView{}, err
	}
	if current.MemberID != actor.MemberID && input.ExpenseScope != model.ExpenseScopeCollective {
		return ExpenseRecordView{}, ErrForbidden
	}
	input, category, err := s.validateInput(ctx, input, true)
	if err != nil {
		return ExpenseRecordView{}, err
	}
	record, err := s.repository.UpdateExpenseRecord(ctx, repository.UpdateExpenseRecordParams{
		ID:           expenseID,
		HouseholdID:  actor.HouseholdID,
		CategoryID:   input.CategoryID,
		ExpenseScope: input.ExpenseScope,
		Title:        input.Title,
		AmountCents:  input.AmountCents,
		SpentOn:      input.SpentOn,
		Note:         input.Note,
		Version:      input.Version,
	})
	if errors.Is(err, repository.ErrConflict) {
		return ExpenseRecordView{}, ErrConflict
	}
	if err != nil {
		return ExpenseRecordView{}, fmt.Errorf("update expense: %w", err)
	}
	memberName, err := s.memberName(ctx, record.MemberID)
	if err != nil {
		return ExpenseRecordView{}, err
	}
	return ExpenseRecordView{ExpenseRecord: record, Category: category, MemberName: memberName}, nil
}

func (s *ExpenseService) DeleteExpense(ctx context.Context, actor AuthenticatedIdentity, expenseID, version uint64) error {
	if version == 0 {
		return ErrInvalidInput
	}
	if _, err := s.getManageableExpense(ctx, actor, expenseID); err != nil {
		return err
	}
	err := s.repository.SoftDeleteExpenseRecord(ctx, repository.SoftDeleteExpenseRecordParams{
		ID: expenseID, HouseholdID: actor.HouseholdID, DeletedBy: actor.MemberID, Version: version,
	})
	if errors.Is(err, repository.ErrConflict) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("delete expense: %w", err)
	}
	return nil
}

func (s *ExpenseService) MyMonthlyExpenses(ctx context.Context, actor AuthenticatedIdentity, month string) (MonthlyExpenseView, error) {
	start, end, normalizedMonth, err := s.monthRange(month)
	if err != nil {
		return MonthlyExpenseView{}, err
	}
	records, err := s.repository.ListMemberExpenses(ctx, repository.ListMemberExpensesParams{
		MemberID: actor.MemberID, StartDate: start, EndDateExclusive: end,
	})
	if err != nil {
		return MonthlyExpenseView{}, err
	}
	return s.buildMonthlyView(ctx, normalizedMonth, records)
}

func (s *ExpenseService) CollectiveMonthlyExpenses(ctx context.Context, actor AuthenticatedIdentity, month string) (MonthlyExpenseView, error) {
	start, end, normalizedMonth, err := s.monthRange(month)
	if err != nil {
		return MonthlyExpenseView{}, err
	}
	records, err := s.repository.ListCollectiveExpenses(ctx, repository.ListCollectiveExpensesParams{
		HouseholdID: actor.HouseholdID, StartDate: start, EndDateExclusive: end,
	})
	if err != nil {
		return MonthlyExpenseView{}, err
	}
	return s.buildMonthlyView(ctx, normalizedMonth, records)
}

func (s *ExpenseService) CurrentHouseholdSummary(ctx context.Context, actor AuthenticatedIdentity) (HouseholdExpenseSummary, error) {
	now := time.Now().In(s.location)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.location)
	end := start.AddDate(0, 1, 0)
	previousStart := start.AddDate(0, -1, 0)

	current, err := s.repository.SumHouseholdExpensesByPeriod(ctx, repository.ListHouseholdExpensesParams{
		HouseholdID: actor.HouseholdID, StartDate: start, EndDateExclusive: end,
	})
	if err != nil {
		return HouseholdExpenseSummary{}, err
	}
	previous, err := s.repository.SumHouseholdExpensesByPeriod(ctx, repository.ListHouseholdExpensesParams{
		HouseholdID: actor.HouseholdID, StartDate: previousStart, EndDateExclusive: start,
	})
	if err != nil {
		return HouseholdExpenseSummary{}, err
	}

	var change *float64
	if previous.TotalAmountCents > 0 {
		value := math.Round((float64(current.TotalAmountCents)-float64(previous.TotalAmountCents))/float64(previous.TotalAmountCents)*1000) / 10
		change = &value
	}
	return HouseholdExpenseSummary{
		Month:                    start.Format("2006-01"),
		TotalAmountCents:         current.TotalAmountCents,
		PersonalAmountCents:      current.PersonalAmountCents,
		CollectiveAmountCents:    current.CollectiveAmountCents,
		PreviousMonthAmountCents: previous.TotalAmountCents,
		ChangePercent:            change,
	}, nil
}

func (s *ExpenseService) ExportExpenses(ctx context.Context, actor AuthenticatedIdentity, start, endExclusive time.Time, view ExpenseExportView) ([]ExpenseRecordView, error) {
	start = s.dateOnly(start)
	endExclusive = s.dateOnly(endExclusive)
	// The end date may be later than today when a user exports the current
	// calendar month; future rows cannot exist, so this remains a read-only
	// range convenience. A future start is still invalid.
	if !start.Before(endExclusive) || start.After(s.today()) || endExclusive.After(start.AddDate(10, 0, 0)) {
		return nil, ErrInvalidInput
	}

	var records []model.ExpenseRecord
	var err error
	switch view {
	case ExpenseExportMine:
		records, err = s.repository.ListMemberExpenses(ctx, repository.ListMemberExpensesParams{
			MemberID: actor.MemberID, StartDate: start, EndDateExclusive: endExclusive,
		})
	case ExpenseExportCollective:
		records, err = s.repository.ListCollectiveExpenses(ctx, repository.ListCollectiveExpensesParams{
			HouseholdID: actor.HouseholdID, StartDate: start, EndDateExclusive: endExclusive,
		})
	case ExpenseExportHousehold:
		if actor.Role != model.MemberRoleOwner {
			return nil, ErrForbidden
		}
		records, err = s.repository.ListHouseholdExpenses(ctx, repository.ListHouseholdExpensesParams{
			HouseholdID: actor.HouseholdID, StartDate: start, EndDateExclusive: endExclusive,
		})
	default:
		return nil, ErrInvalidInput
	}
	if err != nil {
		return nil, err
	}
	return s.buildRecordViews(ctx, records)
}

func (s *ExpenseService) validateInput(ctx context.Context, input ExpenseInput, requireVersion bool) (ExpenseInput, model.ExpenseCategory, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Note = strings.TrimSpace(input.Note)
	input.SpentOn = s.dateOnly(input.SpentOn)
	if input.CategoryID == 0 || !input.ExpenseScope.Valid() || input.AmountCents == 0 ||
		input.Title == "" || len([]rune(input.Title)) > maxExpenseTitleLength ||
		len([]rune(input.Note)) > maxExpenseNoteLength || input.SpentOn.IsZero() ||
		input.SpentOn.After(s.today()) || (requireVersion && input.Version == 0) {
		return ExpenseInput{}, model.ExpenseCategory{}, ErrInvalidInput
	}
	category, err := s.repository.GetExpenseCategoryByID(ctx, input.CategoryID)
	if errors.Is(err, repository.ErrNotFound) {
		return ExpenseInput{}, model.ExpenseCategory{}, ErrInvalidInput
	}
	if err != nil {
		return ExpenseInput{}, model.ExpenseCategory{}, err
	}
	return input, category, nil
}

func (s *ExpenseService) getManageableExpense(ctx context.Context, actor AuthenticatedIdentity, expenseID uint64) (model.ExpenseRecord, error) {
	if expenseID == 0 {
		return model.ExpenseRecord{}, ErrInvalidInput
	}
	record, err := s.repository.GetActiveExpenseRecordByID(ctx, expenseID, actor.HouseholdID)
	if errors.Is(err, repository.ErrNotFound) {
		return model.ExpenseRecord{}, ErrNotFound
	}
	if err != nil {
		return model.ExpenseRecord{}, err
	}
	if record.MemberID == actor.MemberID {
		return record, nil
	}
	if record.ExpenseScope == model.ExpenseScopeCollective &&
		(actor.Role == model.MemberRoleOwner || actor.Role == model.MemberRoleAdmin) {
		return record, nil
	}
	return model.ExpenseRecord{}, ErrForbidden
}

func (s *ExpenseService) buildMonthlyView(ctx context.Context, month string, records []model.ExpenseRecord) (MonthlyExpenseView, error) {
	views, err := s.buildRecordViews(ctx, records)
	if err != nil {
		return MonthlyExpenseView{}, err
	}
	categoryAmounts := make(map[uint16]uint64)
	out := MonthlyExpenseView{Month: month, Records: views, CategoryTotals: make([]ExpenseCategoryTotal, 0)}
	for _, record := range views {
		out.TotalAmountCents += record.AmountCents
		categoryAmounts[record.CategoryID] += record.AmountCents
		if record.ExpenseScope == model.ExpenseScopePersonal {
			out.PersonalAmountCents += record.AmountCents
		} else {
			out.CollectiveAmountCents += record.AmountCents
		}
	}
	categoryByID := make(map[uint16]model.ExpenseCategory)
	for _, record := range views {
		categoryByID[record.CategoryID] = record.Category
	}
	for id, amount := range categoryAmounts {
		percentage := float64(0)
		if out.TotalAmountCents > 0 {
			percentage = math.Round(float64(amount)/float64(out.TotalAmountCents)*1000) / 10
		}
		out.CategoryTotals = append(out.CategoryTotals, ExpenseCategoryTotal{
			Category: categoryByID[id], AmountCents: amount, Percentage: percentage,
		})
	}
	sort.Slice(out.CategoryTotals, func(i, j int) bool {
		return out.CategoryTotals[i].Category.SortOrder < out.CategoryTotals[j].Category.SortOrder
	})
	return out, nil
}

func (s *ExpenseService) buildRecordViews(ctx context.Context, records []model.ExpenseRecord) ([]ExpenseRecordView, error) {
	categories, err := s.repository.ListExpenseCategories(ctx)
	if err != nil {
		return nil, err
	}
	categoryByID := make(map[uint16]model.ExpenseCategory, len(categories))
	for _, category := range categories {
		categoryByID[category.ID] = category
	}
	memberNames := make(map[uint64]string)
	views := make([]ExpenseRecordView, 0, len(records))
	for _, record := range records {
		name, ok := memberNames[record.MemberID]
		if !ok {
			name, err = s.memberName(ctx, record.MemberID)
			if err != nil {
				return nil, err
			}
			memberNames[record.MemberID] = name
		}
		views = append(views, ExpenseRecordView{
			ExpenseRecord: record, Category: categoryByID[record.CategoryID], MemberName: name,
		})
	}
	return views, nil
}

func (s *ExpenseService) memberName(ctx context.Context, memberID uint64) (string, error) {
	member, err := s.repository.GetMemberByID(ctx, memberID)
	if err != nil {
		return "", fmt.Errorf("get expense member: %w", err)
	}
	return member.DisplayName, nil
}

func (s *ExpenseService) monthRange(value string) (time.Time, time.Time, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = time.Now().In(s.location).Format("2006-01")
	}
	start, err := time.ParseInLocation("2006-01", value, s.location)
	if err != nil || start.After(time.Now().In(s.location)) {
		return time.Time{}, time.Time{}, "", ErrInvalidInput
	}
	return start, start.AddDate(0, 1, 0), start.Format("2006-01"), nil
}

func (s *ExpenseService) dateOnly(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	value = value.In(s.location)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, s.location)
}

func (s *ExpenseService) today() time.Time {
	return s.dateOnly(time.Now())
}
