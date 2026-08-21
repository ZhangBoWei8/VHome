package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"vhome/internal/model"
)

const memoColumns = `
id,
household_id,
title,
description,
remind_at,
recipient_ids,
created_by,
email_sent_at,
sms_sent_at,
version,
created_at,
updated_at
`
const (
	defaultMemoListLimit = 100
	maxMemoListLimit     = 500
)

type CreateMemoParams struct {
	HouseholdID  uint64
	Title        string
	Description  string
	RemindAt     time.Time
	RecipientIDs []uint64
	CreatedBy    uint64
}

type UpdateMemoParams struct {
	ID           uint64
	HouseholdID  uint64
	Title        string
	Description  string
	RemindAt     time.Time
	RecipientIDs []uint64
	Version      uint64
}

// UpdateMemoRecipientsParams 只用于“屏蔽提醒”。
// 它不会修改标题、内容、时间，也不会重置 email_sent_at。
type UpdateMemoRecipientsParams struct {
	ID           uint64
	HouseholdID  uint64
	RecipientIDs []uint64
	Version      uint64
}

type DeleteMemoParams struct {
	ID          uint64
	HouseholdID uint64
	Version     uint64
}

type ListRecipientMemosParams struct {
	HouseholdID uint64
	RecipientID uint64
	Limit       int
	Offset      int
}

type ListRecipientMemosByRangeParams struct {
	HouseholdID      uint64
	RecipientID      uint64
	StartTime        time.Time
	EndTimeExclusive time.Time
}

type SearchRecipientMemosParams struct {
	HouseholdID uint64
	RecipientID uint64
	Keyword     string
	Limit       int
}

type ListCreatedMemosParams struct {
	HouseholdID uint64
	CreatedBy   uint64
	Limit       int
	Offset      int
}

type CountRecipientMemosInSlotParams struct {
	HouseholdID      uint64
	RecipientID      uint64
	StartTime        time.Time
	EndTimeExclusive time.Time

	// 创建时传 0；编辑时传当前 Memo ID，防止它把自己算进去。
	ExcludeMemoID uint64
}

type ListPendingEmailMemosParams struct {
	DueAfter  time.Time
	DueBefore time.Time
	Limit     int
}

type ListPendingSMSMemosParams struct {
	DueAfter  time.Time
	DueBefore time.Time
	Limit     int
}

type MarkMemoEmailSentParams struct {
	ID          uint64
	HouseholdID uint64
	Version     uint64
	SentAt      time.Time
}

type MarkMemoSMSSentParams struct {
	ID          uint64
	HouseholdID uint64
	Version     uint64
	SentAt      time.Time
}

type memoScanner interface {
	Scan(dest ...any) error
}

func scanMemo(scanner memoScanner) (model.Memo, error) {
	var memo model.Memo

	var description sql.NullString
	var recipientJSON []byte
	var emailSentAt sql.NullTime
	var smsSentAt sql.NullTime

	err := scanner.Scan(
		&memo.ID,
		&memo.HouseholdID,
		&memo.Title,
		&description,
		&memo.RemindAt,
		&recipientJSON,
		&memo.CreatedBy,
		&emailSentAt,
		&smsSentAt,
		&memo.Version,
		&memo.CreatedAt,
		&memo.UpdatedAt,
	)
	if err != nil {
		return model.Memo{}, err
	}

	if description.Valid {
		memo.Description = description.String
	}

	if len(recipientJSON) == 0 {
		return model.Memo{}, errors.New("memo recipient_ids is empty database JSON")
	}

	if err := json.Unmarshal(recipientJSON, &memo.RecipientIDs); err != nil {
		return model.Memo{}, fmt.Errorf("unmarshal json failed: %w", err)
	}

	if emailSentAt.Valid {
		value := emailSentAt.Time
		memo.EmailSentAt = &value
	}
	if smsSentAt.Valid {
		value := smsSentAt.Time
		memo.SMSSentAt = &value
	}

	return memo, nil
}

func marshalMemoRecipientIDs(recipientIDs []uint64) (string, error) {
	if recipientIDs == nil {
		recipientIDs = make([]uint64, 0)
	}

	value, err := json.Marshal(recipientIDs)
	if err != nil {
		return "", fmt.Errorf("encode memo recipient ids: %w", err)
	}
	return string(value), nil
}

func nullableMemoDescription(description string) any {
	if description == "" {
		return nil
	}
	return description
}

func normalizeMemoLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultMemoListLimit

	case limit > maxMemoListLimit:
		return maxMemoListLimit

	default:
		return limit
	}
}

func normalizeMemoOffset(offset int) int {
	if offset < 0 {
		return 0
	}

	return offset
}

func (r *Repository) GetMemoByID(ctx context.Context, memoID uint64, houseID uint64) (model.Memo, error) {
	query := `SELECT` + memoColumns + `FROM memos WHERE id = ? AND household_id = ? LIMIT 1`
	memo, err := scanMemo(r.q.QueryRowContext(ctx, query, memoID, houseID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Memo{}, ErrNotFound
	}
	if err != nil {
		return model.Memo{}, fmt.Errorf("get memo by id: %w", err)
	}
	return memo, nil
}

func (r *Repository) CreateMemo(ctx context.Context, cmp CreateMemoParams) (model.Memo, error) {
	recipientJSON, err := marshalMemoRecipientIDs(cmp.RecipientIDs)
	if err != nil {
		return model.Memo{}, err
	}
	const query = `INSERT INTO memos (household_id,title,description,remind_at,recipient_ids,created_by) VALUES (?,?,?,?,?,?)`
	result, err := r.q.ExecContext(ctx, query, cmp.HouseholdID, cmp.Title, nullableMemoDescription(cmp.Description), cmp.RemindAt, recipientJSON, cmp.CreatedBy)
	if err != nil {
		return model.Memo{}, fmt.Errorf("create memo: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return model.Memo{}, fmt.Errorf("get created memo id: %w", err)
	}

	return r.GetMemoByID(ctx, uint64(id), cmp.HouseholdID)
}

func (r *Repository) UpdateMemo(ctx context.Context, cmp UpdateMemoParams) (model.Memo, error) {
	recipientJSON, err := marshalMemoRecipientIDs(cmp.RecipientIDs)
	if err != nil {
		return model.Memo{}, err
	}
	const query = `UPDATE memos SET title = ?,description = ?,remind_at = ?,recipient_ids = ?,email_sent_at = NULL,sms_sent_at = NULL,version = version + 1 WHERE id = ? AND household_id = ? AND version = ?`
	result, err := r.q.ExecContext(ctx, query, cmp.Title, nullableMemoDescription(cmp.Description), cmp.RemindAt, recipientJSON, cmp.ID, cmp.HouseholdID, cmp.Version)
	if err != nil {
		return model.Memo{}, fmt.Errorf("update memo: %w", err)
	}
	if err := requireMemoRow(result); err != nil {
		return model.Memo{}, err
	}
	return r.GetMemoByID(ctx, cmp.ID, cmp.HouseholdID)
}

func (r *Repository) UpdateMemoRecipients(ctx context.Context, cmp UpdateMemoRecipientsParams) (model.Memo, error) {
	recipientJSON, err := marshalMemoRecipientIDs(cmp.RecipientIDs)
	if err != nil {
		return model.Memo{}, err
	}
	const query = `UPDATE memos SET recipient_ids = ?,version = version + 1 WHERE id = ? AND household_id = ? AND version = ?`
	result, err := r.q.ExecContext(ctx, query, recipientJSON, cmp.ID, cmp.HouseholdID, cmp.Version)
	if err != nil {
		return model.Memo{}, fmt.Errorf("update memo recipients: %w", err)
	}
	if err := requireMemoRow(result); err != nil {
		return model.Memo{}, err
	}
	return r.GetMemoByID(ctx, cmp.ID, cmp.HouseholdID)
}

func (r *Repository) DeleteMemo(ctx context.Context, cmp DeleteMemoParams) error {
	const query = `DELETE FROM memos WHERE id = ? AND household_id = ? AND version = ?`
	result, err := r.q.ExecContext(ctx, query, cmp.ID, cmp.HouseholdID, cmp.Version)
	if err != nil {
		return fmt.Errorf("delete memo: %w", err)
	}

	return requireMemoRow(result)
}

func (r *Repository) ListRecipientMemos(ctx context.Context, cmp ListRecipientMemosParams) ([]model.Memo, error) {
	query := `SELECT` + memoColumns + `FROM memos WHERE household_id = ? AND JSON_CONTAINS(recipient_ids,JSON_ARRAY(?))
	ORDER BY remind_at ASC, id ASC LIMIT ? OFFSET ?`
	return r.listMemos(ctx, query, cmp.HouseholdID, cmp.RecipientID, normalizeMemoLimit(cmp.Limit), normalizeMemoOffset(cmp.Offset))
}

// ListRecipientMemosByRange 同时支持日历月视图、某一天和某个小时。
func (r *Repository) ListRecipientMemosByRange(ctx context.Context, params ListRecipientMemosByRangeParams) ([]model.Memo, error) {
	query := `
		SELECT ` + memoColumns + `
		FROM memos
		WHERE household_id = ?
		  AND JSON_CONTAINS(
			  recipient_ids,
			  JSON_ARRAY(?)
		  )
		  AND remind_at >= ?
		  AND remind_at < ?
		ORDER BY remind_at ASC, id ASC
	`

	return r.listMemos(ctx, query, params.HouseholdID, params.RecipientID, params.StartTime, params.EndTimeExclusive)
}

// SearchRecipientMemos 只搜索当前成员收到的备忘录。
// 当前家庭数据量很小，使用 LIKE 已经足够。
func (r *Repository) SearchRecipientMemos(ctx context.Context, params SearchRecipientMemosParams) ([]model.Memo, error) {
	query := `
		SELECT ` + memoColumns + `
		FROM memos
		WHERE household_id = ?
		  AND JSON_CONTAINS(
			  recipient_ids,
			  JSON_ARRAY(?)
		  )
		  AND (
			  title LIKE ?
			  OR COALESCE(description, '') LIKE ?
		  )
		ORDER BY remind_at ASC, id ASC
		LIMIT ?
	`

	pattern := "%" + params.Keyword + "%"

	return r.listMemos(ctx, query, params.HouseholdID, params.RecipientID, pattern, pattern, normalizeMemoLimit(params.Limit))
}

// ListCreatedMemos 查询当前成员创建的全部事项。
// 即使创建者不在 recipient_ids 中，也可以从“我创建的”列表中管理。
func (r *Repository) ListCreatedMemos(ctx context.Context, params ListCreatedMemosParams) ([]model.Memo, error) {
	query := `
		SELECT ` + memoColumns + `
		FROM memos
		WHERE household_id = ?
		  AND created_by = ?
		ORDER BY remind_at ASC, id ASC
		LIMIT ? OFFSET ?
	`

	return r.listMemos(ctx, query, params.HouseholdID, params.CreatedBy, normalizeMemoLimit(params.Limit), normalizeMemoOffset(params.Offset))
}

// CountRecipientMemosInSlot 用于检查一个成员在某个小时内是否已经有五条。
// Service 对每个 RecipientID 分别调用。
func (r *Repository) CountRecipientMemosInSlot(ctx context.Context, params CountRecipientMemosInSlotParams) (uint64, error) {
	const query = `
		SELECT COUNT(*)
		FROM memos
		WHERE household_id = ?
		  AND JSON_CONTAINS(
			  recipient_ids,
			  JSON_ARRAY(?)
		  )
		  AND remind_at >= ?
		  AND remind_at < ?
		  AND (? = 0 OR id <> ?)
	`

	var count uint64

	err := r.q.QueryRowContext(ctx, query, params.HouseholdID, params.RecipientID, params.StartTime, params.EndTimeExclusive, params.ExcludeMemoID, params.ExcludeMemoID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count recipient memos in slot: %w", err)
	}

	return count, nil
}

// DeleteMemosBefore 每月第一天清理上个月及更早的事项。
// 返回实际删除数量，供日志记录。
func (r *Repository) DeleteMemosBefore(ctx context.Context, cutoff time.Time) (uint64, error) {
	const query = `
		DELETE FROM memos
		WHERE remind_at < ?
	`

	result, err := r.q.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete memos before cutoff: %w", err)
	}

	return memoRowsAffected(result)
}

// DeleteMemosWithoutRecipients 清理所有人都已经屏蔽的空备忘录。
func (r *Repository) DeleteMemosWithoutRecipients(ctx context.Context) (uint64, error) {
	const query = `
		DELETE FROM memos
		WHERE JSON_LENGTH(recipient_ids) = 0
	`

	result, err := r.q.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf(
			"delete memos without recipients: %w",
			err,
		)
	}

	return memoRowsAffected(result)
}

// ListPendingEmailMemos 查询已经到期且尚未处理邮件的备忘录。
func (r *Repository) ListPendingEmailMemos(ctx context.Context, params ListPendingEmailMemosParams) ([]model.Memo, error) {
	query := `
		SELECT ` + memoColumns + `
		FROM memos
		WHERE remind_at <= ?
		  AND remind_at >= ?
		  AND email_sent_at IS NULL
		  AND JSON_LENGTH(recipient_ids) > 0
		ORDER BY remind_at ASC, id ASC
		LIMIT ?
	`

	return r.listMemos(ctx, query, params.DueBefore, params.DueAfter, normalizeMemoLimit(params.Limit))
}

func (r *Repository) ListPendingSMSMemos(ctx context.Context, params ListPendingSMSMemosParams) ([]model.Memo, error) {
	query := `
		SELECT ` + memoColumns + `
		FROM memos
		WHERE remind_at <= ?
		  AND remind_at >= ?
		  AND sms_sent_at IS NULL
		  AND JSON_LENGTH(recipient_ids) > 0
		ORDER BY remind_at ASC, id ASC
		LIMIT ?
	`
	return r.listMemos(ctx, query, params.DueBefore, params.DueAfter, normalizeMemoLimit(params.Limit))
}

// MarkMemoEmailSent 标记邮件任务已经完成。
// 不增加 version，避免后台状态更新导致前端无故出现版本冲突。
//
// 但会校验读取邮件时的 Version：
// 如果发送期间用户修改了备忘录，则不标记新版备忘录为已发送。
func (r *Repository) MarkMemoEmailSent(ctx context.Context, params MarkMemoEmailSentParams) error {
	const query = `
		UPDATE memos
		SET email_sent_at = ?
		WHERE id = ?
		  AND household_id = ?
		  AND version = ?
		  AND email_sent_at IS NULL
	`

	result, err := r.q.ExecContext(ctx, query, params.SentAt, params.ID, params.HouseholdID, params.Version)
	if err != nil {
		return fmt.Errorf(
			"mark memo email sent: %w",
			err,
		)
	}

	return requireMemoRow(result)
}

func (r *Repository) MarkMemoSMSSent(ctx context.Context, params MarkMemoSMSSentParams) error {
	const query = `
		UPDATE memos
		SET sms_sent_at = ?
		WHERE id = ?
		  AND household_id = ?
		  AND version = ?
		  AND sms_sent_at IS NULL
	`
	result, err := r.q.ExecContext(ctx, query, params.SentAt, params.ID, params.HouseholdID, params.Version)
	if err != nil {
		return fmt.Errorf("mark memo SMS sent: %w", err)
	}
	return requireMemoRow(result)
}

func (r *Repository) ListCalendarDayOverridesByRange(ctx context.Context, start, endExclusive time.Time) ([]model.CalendarDayOverride, error) {
	const query = `
		SELECT calendar_date, day_type, holiday_name, source_url, synced_at
		FROM calendar_day_overrides
		WHERE calendar_date >= ? AND calendar_date < ?
		ORDER BY calendar_date ASC
	`
	rows, err := r.q.QueryContext(ctx, query, start, endExclusive)
	if err != nil {
		return nil, fmt.Errorf("list calendar day overrides: %w", err)
	}
	defer rows.Close()
	result := make([]model.CalendarDayOverride, 0)
	for rows.Next() {
		var item model.CalendarDayOverride
		if err := rows.Scan(&item.CalendarDate, &item.DayType, &item.HolidayName, &item.SourceURL, &item.SyncedAt); err != nil {
			return nil, fmt.Errorf("scan calendar day override: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate calendar day overrides: %w", err)
	}
	return result, nil
}

func (r *Repository) CountCalendarDayOverridesByYear(ctx context.Context, year int) (uint64, error) {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(1, 0, 0)
	var count uint64
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_day_overrides WHERE calendar_date >= ? AND calendar_date < ?`, start, end).Scan(&count); err != nil {
		return 0, fmt.Errorf("count calendar overrides by year: %w", err)
	}
	return count, nil
}

func (r *Repository) UpsertCalendarDayOverride(ctx context.Context, item model.CalendarDayOverride) error {
	const query = `
		INSERT INTO calendar_day_overrides (calendar_date, day_type, holiday_name, source_url, synced_at)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE day_type = VALUES(day_type), holiday_name = VALUES(holiday_name),
			source_url = VALUES(source_url), synced_at = VALUES(synced_at)
	`
	if _, err := r.q.ExecContext(ctx, query, item.CalendarDate, item.DayType, item.HolidayName, item.SourceURL, item.SyncedAt); err != nil {
		return fmt.Errorf("upsert calendar day override: %w", err)
	}
	return nil
}

// 限制更新/删除必须要影响一条记录
func requireMemoRow(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"get memo affected row count: %w",
			err,
		)
	}

	if affected != 1 {
		return ErrConflict
	}

	return nil
}

func (r *Repository) listMemos(ctx context.Context, query string, args ...any) ([]model.Memo, error) {
	rows, err := r.q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query memo list: %w", err)
	}
	defer rows.Close()

	memos := make([]model.Memo, 0)

	for rows.Next() {
		memo, err := scanMemo(rows)
		if err != nil {
			return nil, fmt.Errorf("scan memo list row: %w", err)
		}

		memos = append(memos, memo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memo list: %w", err)
	}

	return memos, nil
}

func memoRowsAffected(result sql.Result) (uint64, error) {
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf(
			"get memo affected row count: %w",
			err,
		)
	}

	if affected < 0 {
		return 0, errors.New(
			"memo affected row count is negative",
		)
	}

	return uint64(affected), nil
}
