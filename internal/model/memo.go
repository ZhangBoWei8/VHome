package model

import "time"

const (
	CalendarDayHoliday         CalendarDayType = "HOLIDAY"
	CalendarDayTransferWorkday CalendarDayType = "TRANSFER_WORKDAY"
)

type Memo struct {
	ID           uint64     `json:"id"`
	HouseholdID  uint64     `json:"household_id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	RemindAt     time.Time  `json:"remind_at"`
	RecipientIDs []uint64   `json:"recipient_ids"`
	CreatedBy    uint64     `json:"created_by"`
	EmailSentAt  *time.Time `json:"email_sent_at"`
	SMSSentAt    *time.Time `json:"sms_sent_at"`
	Version      uint64     `json:"version"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type CalendarDayType string

type CalendarDayOverride struct {
	CalendarDate time.Time       `json:"calendar_date"`
	DayType      CalendarDayType `json:"day_type"`
	HolidayName  string          `json:"holiday_name"`
	SourceURL    string          `json:"source_url"`
	SyncedAt     time.Time       `json:"synced_at"`
}
