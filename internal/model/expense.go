package model

import "time"

type ExpenseScope string

const (
	ExpenseScopePersonal   ExpenseScope = "PERSONAL"
	ExpenseScopeCollective ExpenseScope = "COLLECTIVE"
)

func (scope ExpenseScope) Valid() bool {
	switch scope {
	case ExpenseScopePersonal, ExpenseScopeCollective:
		return true
	default:
		return false
	}
}

type ExpenseCategory struct {
	ID        uint16    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	IconKey   string    `json:"icon_key"`
	SortOrder uint16    `json:"sort_order"`
	IsBuiltin bool      `json:"is_builtin"`
	CreatedAt time.Time `json:"created_at"`
}

type ExpenseRecord struct {
	ID           uint64       `json:"id"`
	HouseholdID  uint64       `json:"household_id"`
	MemberID     uint64       `json:"member_id"`
	CategoryID   uint16       `json:"category_id"`
	ExpenseScope ExpenseScope `json:"expense_scope"`
	Title        string       `json:"title"`
	AmountCents  uint64       `json:"amount_cents"`
	SpentOn      time.Time    `json:"spent_on"`
	Note         string       `json:"note"`
	Version      uint64       `json:"version"`
	DeletedAt    *time.Time   `json:"deleted_at"`
	DeletedBy    *uint64      `json:"deleted_by"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}
