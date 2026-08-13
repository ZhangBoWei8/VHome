package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"vhome/internal/model"
)

const expenseCategoryColumns = `
	id,
	code,
	name,
	icon_key,
	sort_order,
	is_builtin,
	created_at
`

const expenseRecordColumns = `
	id,
	household_id,
	member_id,
	category_id,
	expense_scope,
	title,
	amount_cents,
	spent_on,
	note,
	version,
	deleted_at,
	deleted_by,
	created_at,
	updated_at
`

type CreateExpenseRecordParams struct {
	HouseholdID  uint64
	MemberID     uint64
	CategoryID   uint16
	ExpenseScope model.ExpenseScope
	Title        string
	AmountCents  uint64
	SpentOn      time.Time
	Note         string
}

type UpdateExpenseRecordParams struct {
	ID           uint64
	HouseholdID  uint64
	CategoryID   uint16
	ExpenseScope model.ExpenseScope
	Title        string
	AmountCents  uint64
	SpentOn      time.Time
	Note         string
	Version      uint64
}

type SoftDeleteExpenseRecordParams struct {
	ID          uint64
	HouseholdID uint64
	DeletedBy   uint64
	Version     uint64
}

type ListMemberExpensesParams struct {
	MemberID         uint64
	StartDate        time.Time
	EndDateExclusive time.Time
}

type ListCollectiveExpensesParams struct {
	HouseholdID      uint64
	StartDate        time.Time
	EndDateExclusive time.Time
}

type ListHouseholdExpensesParams struct {
	HouseholdID      uint64
	StartDate        time.Time
	EndDateExclusive time.Time
}

// HouseholdExpenseTotals is deliberately an aggregate. Dashboard callers can
// learn how much the household spent without gaining access to another
// member's private expense titles or notes.
type HouseholdExpenseTotals struct {
	TotalAmountCents      uint64
	PersonalAmountCents   uint64
	CollectiveAmountCents uint64
}

type expenseCategoryScanner interface {
	Scan(dest ...any) error
}

// scanExpenseCategory centralizes the database-column mapping used by both
// category list and single-category queries. Keeping one mapping prevents a
// later column addition from producing inconsistent query results.
func scanExpenseCategory(scanner expenseCategoryScanner) (model.ExpenseCategory, error) {
	var category model.ExpenseCategory
	err := scanner.Scan(
		&category.ID,
		&category.Code,
		&category.Name,
		&category.IconKey,
		&category.SortOrder,
		&category.IsBuiltin,
		&category.CreatedAt,
	)
	if err != nil {
		return model.ExpenseCategory{}, err
	}
	return category, nil
}

type expenseRecordScanner interface {
	Scan(dest ...any) error
}

func scanExpenseRecord(scanner expenseRecordScanner) (model.ExpenseRecord, error) {
	var record model.ExpenseRecord
	var note sql.NullString
	var deletedAt sql.NullTime
	var deletedBy sql.NullInt64

	err := scanner.Scan(
		&record.ID,
		&record.HouseholdID,
		&record.MemberID,
		&record.CategoryID,
		&record.ExpenseScope,
		&record.Title,
		&record.AmountCents,
		&record.SpentOn,
		&note,
		&record.Version,
		&deletedAt,
		&deletedBy,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return model.ExpenseRecord{}, err
	}

	if note.Valid {
		record.Note = note.String
	}
	if deletedAt.Valid {
		value := deletedAt.Time
		record.DeletedAt = &value
	}
	if deletedBy.Valid {
		value := uint64(deletedBy.Int64)
		record.DeletedBy = &value
	}

	return record, nil
}

func (r *Repository) ListExpenseCategories(ctx context.Context) ([]model.ExpenseCategory, error) {
	query := `
		SELECT ` + expenseCategoryColumns + `
		FROM expense_categories
		ORDER BY sort_order ASC, id ASC
	`

	rows, err := r.q.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list expense categories: %w", err)
	}
	defer rows.Close()

	categories := make([]model.ExpenseCategory, 0)
	for rows.Next() {
		category, err := scanExpenseCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expense category list row: %w", err)
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expense categories: %w", err)
	}

	return categories, nil
}

func (r *Repository) GetExpenseCategoryByID(ctx context.Context, categoryID uint16) (model.ExpenseCategory, error) {
	query := `
		SELECT ` + expenseCategoryColumns + `
		FROM expense_categories
		WHERE id = ?
		LIMIT 1
	`

	category, err := scanExpenseCategory(r.q.QueryRowContext(ctx, query, categoryID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.ExpenseCategory{}, ErrNotFound
	}
	if err != nil {
		return model.ExpenseCategory{}, fmt.Errorf("get expense category by id: %w", err)
	}
	return category, nil
}

func (r *Repository) GetActiveExpenseRecordByID(ctx context.Context, expenseID uint64, householdID uint64) (model.ExpenseRecord, error) {
	query := `
		SELECT ` + expenseRecordColumns + `
		FROM expense_records
		WHERE id = ?
		  AND household_id = ?
		  AND deleted_at IS NULL
		LIMIT 1
	`

	record, err := scanExpenseRecord(
		r.q.QueryRowContext(ctx, query, expenseID, householdID),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ExpenseRecord{}, ErrNotFound
	}
	if err != nil {
		return model.ExpenseRecord{}, fmt.Errorf("get active expense record by id: %w", err)
	}
	return record, nil
}

func (r *Repository) CreateExpenseRecord(ctx context.Context, params CreateExpenseRecordParams) (model.ExpenseRecord, error) {
	const query = `
		INSERT INTO expense_records (
			household_id,
			member_id,
			category_id,
			expense_scope,
			title,
			amount_cents,
			spent_on,
			note
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.HouseholdID,
		params.MemberID,
		params.CategoryID,
		string(params.ExpenseScope),
		params.Title,
		params.AmountCents,
		params.SpentOn,
		nullableExpenseNote(params.Note),
	)
	if err != nil {
		return model.ExpenseRecord{}, fmt.Errorf("create expense record: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.ExpenseRecord{}, fmt.Errorf("get created expense record id: %w", err)
	}
	return r.GetActiveExpenseRecordByID(ctx, uint64(id), params.HouseholdID)
}

func (r *Repository) UpdateExpenseRecord(ctx context.Context, params UpdateExpenseRecordParams) (model.ExpenseRecord, error) {
	const query = `
		UPDATE expense_records
		SET category_id = ?,
			expense_scope = ?,
			title = ?,
			amount_cents = ?,
			spent_on = ?,
			note = ?,
			version = version + 1
		WHERE id = ?
		  AND household_id = ?
		  AND deleted_at IS NULL
		  AND version = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.CategoryID,
		string(params.ExpenseScope),
		params.Title,
		params.AmountCents,
		params.SpentOn,
		nullableExpenseNote(params.Note),
		params.ID,
		params.HouseholdID,
		params.Version,
	)
	if err != nil {
		return model.ExpenseRecord{}, fmt.Errorf("update expense record: %w", err)
	}
	if err := requireExpenseRow(result); err != nil {
		return model.ExpenseRecord{}, err
	}
	return r.GetActiveExpenseRecordByID(ctx, params.ID, params.HouseholdID)
}

func (r *Repository) SoftDeleteExpenseRecord(ctx context.Context, params SoftDeleteExpenseRecordParams) error {
	const query = `
		UPDATE expense_records
		SET deleted_at = UTC_TIMESTAMP(6),
			deleted_by = ?,
			version = version + 1
		WHERE id = ?
		  AND household_id = ?
		  AND deleted_at IS NULL
		  AND version = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.DeletedBy,
		params.ID,
		params.HouseholdID,
		params.Version,
	)
	if err != nil {
		return fmt.Errorf("soft delete expense record: %w", err)
	}
	return requireExpenseRow(result)
}

func (r *Repository) ListMemberExpenses(ctx context.Context, params ListMemberExpensesParams) ([]model.ExpenseRecord, error) {
	query := `
		SELECT ` + expenseRecordColumns + `
		FROM expense_records
		WHERE member_id = ?
		  AND deleted_at IS NULL
		  AND spent_on >= ?
		  AND spent_on < ?
		ORDER BY spent_on DESC, id DESC
	`

	return r.listExpenseRecords(
		ctx,
		query,
		params.MemberID,
		params.StartDate,
		params.EndDateExclusive,
	)
}

func (r *Repository) ListCollectiveExpenses(ctx context.Context, params ListCollectiveExpensesParams) ([]model.ExpenseRecord, error) {
	query := `
		SELECT ` + expenseRecordColumns + `
		FROM expense_records
		WHERE household_id = ?
		  AND expense_scope = 'COLLECTIVE'
		  AND deleted_at IS NULL
		  AND spent_on >= ?
		  AND spent_on < ?
		ORDER BY spent_on DESC, id DESC
	`

	return r.listExpenseRecords(
		ctx,
		query,
		params.HouseholdID,
		params.StartDate,
		params.EndDateExclusive,
	)
}

func (r *Repository) ListHouseholdExpenses(ctx context.Context, params ListHouseholdExpensesParams) ([]model.ExpenseRecord, error) {
	query := `
		SELECT ` + expenseRecordColumns + `
		FROM expense_records
		WHERE household_id = ?
		  AND deleted_at IS NULL
		  AND spent_on >= ?
		  AND spent_on < ?
		ORDER BY spent_on DESC, id DESC
	`

	return r.listExpenseRecords(
		ctx,
		query,
		params.HouseholdID,
		params.StartDate,
		params.EndDateExclusive,
	)
}

func (r *Repository) SumHouseholdExpensesByPeriod(ctx context.Context, params ListHouseholdExpensesParams) (HouseholdExpenseTotals, error) {
	const query = `
		SELECT
			COALESCE(SUM(amount_cents), 0),
			COALESCE(SUM(CASE WHEN expense_scope = 'PERSONAL' THEN amount_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN expense_scope = 'COLLECTIVE' THEN amount_cents ELSE 0 END), 0)
		FROM expense_records
		WHERE household_id = ?
		  AND deleted_at IS NULL
		  AND spent_on >= ?
		  AND spent_on < ?
	`

	var totals HouseholdExpenseTotals
	err := r.q.QueryRowContext(
		ctx,
		query,
		params.HouseholdID,
		params.StartDate,
		params.EndDateExclusive,
	).Scan(
		&totals.TotalAmountCents,
		&totals.PersonalAmountCents,
		&totals.CollectiveAmountCents,
	)
	if err != nil {
		return HouseholdExpenseTotals{}, fmt.Errorf("sum household expenses by period: %w", err)
	}
	return totals, nil
}

func (r *Repository) listExpenseRecords(ctx context.Context, query string, args ...any) ([]model.ExpenseRecord, error) {
	rows, err := r.q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list expense records: %w", err)
	}
	defer rows.Close()

	records := make([]model.ExpenseRecord, 0)
	for rows.Next() {
		record, err := scanExpenseRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan expense record list row: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expense records: %w", err)
	}

	return records, nil
}

func nullableExpenseNote(note string) any {
	if note == "" {
		return nil
	}
	return note
}

func requireExpenseRow(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get expense affected row count: %w", err)
	}
	if affected != 1 {
		return ErrConflict
	}
	return nil
}
