package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"vhome/internal/model"
	"vhome/internal/repository"
)

const (
	memoLocationName            = "Asia/Shanghai"
	maxMemoTitleLength          = 128
	maxMemoDescriptionLength    = 5000
	maxMemoSearchKeywordLength  = 100
	maxMemosPerRecipientPerHour = 5
)

var (
	ErrMemoTimeInvalid      = errors.New("service: memo remind time is invalid")
	ErrMemoRecipientInvalid = errors.New("service: memo recipient is invalid")
	ErrMemoSlotFull         = errors.New("service: memo time slot is full")
	ErrMemoAlreadyDismissed = errors.New("service: memo already dismissed")
)

type MemoService struct {
	repository *repository.Repository
	location   *time.Location
}

func NewMemoService(repo *repository.Repository) (*MemoService, error) {
	if repo == nil {
		return nil, errors.New("memo service repository is nil")
	}
	location, err := time.LoadLocation(memoLocationName)
	if err != nil {
		return nil, fmt.Errorf("load memo service location %q: %w", memoLocationName, err)
	}
	return &MemoService{repository: repo, location: location}, nil
}

type CreateMemoInput struct {
	Title        string
	Description  string
	RemindAt     time.Time
	RecipientIDs []uint64
}

type UpdateMemoInput struct {
	Title        string
	Description  string
	RemindAt     time.Time
	RecipientIDs []uint64
	Version      uint64
}

type MemoMemberOption struct {
	ID          uint64             `json:"id"`
	DisplayName string             `json:"display_name"`
	AvatarKey   model.MemberAvatar `json:"avatar_key"`
	IsCurrent   bool               `json:"is_current"`
}

type MemoView struct {
	model.Memo
	CanEdit    bool `json:"can_edit"`
	CanDelete  bool `json:"can_delete"`
	CanDismiss bool `json:"can_dismiss"`
}

type MemoCalendarView struct {
	Month     string                      `json:"month"`
	Memos     []MemoView                  `json:"memos"`
	Overrides []model.CalendarDayOverride `json:"calendar_overrides"`
}

type MemoCleanupResult struct {
	ExpiredDeleted          uint64 `json:"expired_deleted"`
	WithoutRecipientDeleted uint64 `json:"without_recipient_deleted"`
}

type normalizedMemoInput struct {
	title        string
	description  string
	remindAt     time.Time
	recipientIDs []uint64
}

func (s *MemoService) CreateMemo(ctx context.Context, actor AuthenticatedIdentity, input CreateMemoInput) (MemoView, error) {
	if err := requireMemoActor(actor); err != nil {
		return MemoView{}, err
	}
	normalized, err := s.validateMemoInput(ctx, actor, input.Title, input.Description, input.RemindAt, input.RecipientIDs, 0)
	if err != nil {
		return MemoView{}, err
	}
	created, err := s.repository.CreateMemo(ctx, repository.CreateMemoParams{
		HouseholdID: actor.HouseholdID, Title: normalized.title, Description: normalized.description,
		RemindAt: normalized.remindAt, RecipientIDs: normalized.recipientIDs, CreatedBy: actor.MemberID,
	})
	if err != nil {
		return MemoView{}, fmt.Errorf("create memo: %w", err)
	}
	return buildMemoView(actor, created), nil
}

func (s *MemoService) Memo(ctx context.Context, actor AuthenticatedIdentity, memoID uint64) (MemoView, error) {
	if err := requireMemoActor(actor); err != nil {
		return MemoView{}, err
	}
	memo, err := s.getMemo(ctx, actor, memoID)
	if err != nil {
		return MemoView{}, err
	}
	if !canReadMemo(actor, memo) {
		return MemoView{}, ErrForbidden
	}
	return buildMemoView(actor, memo), nil
}

func (s *MemoService) UpdateMemo(ctx context.Context, actor AuthenticatedIdentity, memoID uint64, input UpdateMemoInput) (MemoView, error) {
	if err := requireMemoActor(actor); err != nil {
		return MemoView{}, err
	}
	if memoID == 0 || input.Version == 0 {
		return MemoView{}, ErrInvalidInput
	}
	current, err := s.getMemo(ctx, actor, memoID)
	if err != nil {
		return MemoView{}, err
	}
	if !canManageMemo(actor, current) {
		return MemoView{}, ErrForbidden
	}
	normalized, err := s.validateMemoInput(ctx, actor, input.Title, input.Description, input.RemindAt, input.RecipientIDs, memoID)
	if err != nil {
		return MemoView{}, err
	}
	updated, err := s.repository.UpdateMemo(ctx, repository.UpdateMemoParams{
		ID: memoID, HouseholdID: actor.HouseholdID, Title: normalized.title,
		Description: normalized.description, RemindAt: normalized.remindAt,
		RecipientIDs: normalized.recipientIDs, Version: input.Version,
	})
	if errors.Is(err, repository.ErrConflict) {
		return MemoView{}, ErrConflict
	}
	if err != nil {
		return MemoView{}, fmt.Errorf("update memo: %w", err)
	}
	return buildMemoView(actor, updated), nil
}

func (s *MemoService) DeleteMemo(ctx context.Context, actor AuthenticatedIdentity, memoID, version uint64) error {
	if err := requireMemoActor(actor); err != nil {
		return err
	}
	if memoID == 0 || version == 0 {
		return ErrInvalidInput
	}
	current, err := s.getMemo(ctx, actor, memoID)
	if err != nil {
		return err
	}
	if !canManageMemo(actor, current) {
		return ErrForbidden
	}
	err = s.repository.DeleteMemo(ctx, repository.DeleteMemoParams{ID: memoID, HouseholdID: actor.HouseholdID, Version: version})
	if errors.Is(err, repository.ErrConflict) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("delete memo: %w", err)
	}
	return nil
}

func (s *MemoService) DismissMemo(ctx context.Context, actor AuthenticatedIdentity, memoID, version uint64) (MemoView, error) {
	if err := requireMemoActor(actor); err != nil {
		return MemoView{}, err
	}
	if memoID == 0 || version == 0 {
		return MemoView{}, ErrInvalidInput
	}
	current, err := s.getMemo(ctx, actor, memoID)
	if err != nil {
		return MemoView{}, err
	}
	if !containsMemoRecipient(current.RecipientIDs, actor.MemberID) {
		return MemoView{}, ErrMemoAlreadyDismissed
	}
	recipients := make([]uint64, 0, len(current.RecipientIDs)-1)
	for _, id := range current.RecipientIDs {
		if id != actor.MemberID {
			recipients = append(recipients, id)
		}
	}
	updated, err := s.repository.UpdateMemoRecipients(ctx, repository.UpdateMemoRecipientsParams{
		ID: memoID, HouseholdID: actor.HouseholdID, RecipientIDs: recipients, Version: version,
	})
	if errors.Is(err, repository.ErrConflict) {
		return MemoView{}, ErrConflict
	}
	if err != nil {
		return MemoView{}, fmt.Errorf("dismiss memo: %w", err)
	}
	return buildMemoView(actor, updated), nil
}

func (s *MemoService) MemberOptions(ctx context.Context, actor AuthenticatedIdentity) ([]MemoMemberOption, error) {
	if err := requireMemoActor(actor); err != nil {
		return nil, err
	}
	rows, err := s.repository.ListActiveMemberOptions(ctx, actor.HouseholdID)
	if err != nil {
		return nil, fmt.Errorf("list memo member options: %w", err)
	}
	result := make([]MemoMemberOption, 0, len(rows))
	for _, row := range rows {
		if row.ID == actor.MemberID {
			result = append(result, MemoMemberOption{ID: row.ID, DisplayName: row.DisplayName, AvatarKey: row.AvatarKey, IsCurrent: true})
			break
		}
	}
	for _, row := range rows {
		if row.ID != actor.MemberID {
			result = append(result, MemoMemberOption{ID: row.ID, DisplayName: row.DisplayName, AvatarKey: row.AvatarKey})
		}
	}
	return result, nil
}

func (s *MemoService) MyMemos(ctx context.Context, actor AuthenticatedIdentity, limit, offset int) ([]MemoView, error) {
	if err := requireMemoActor(actor); err != nil {
		return nil, err
	}
	rows, err := s.repository.ListRecipientMemos(ctx, repository.ListRecipientMemosParams{
		HouseholdID: actor.HouseholdID, RecipientID: actor.MemberID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list my memos: %w", err)
	}
	return buildMemoViews(actor, rows), nil
}

func (s *MemoService) MyMemosByRange(ctx context.Context, actor AuthenticatedIdentity, start, endExclusive time.Time) ([]MemoView, error) {
	if err := requireMemoActor(actor); err != nil {
		return nil, err
	}
	start = start.In(s.location)
	endExclusive = endExclusive.In(s.location)
	if start.IsZero() || endExclusive.IsZero() || !start.Before(endExclusive) || endExclusive.After(start.AddDate(0, 2, 0)) {
		return nil, ErrInvalidInput
	}
	rows, err := s.repository.ListRecipientMemosByRange(ctx, repository.ListRecipientMemosByRangeParams{
		HouseholdID: actor.HouseholdID, RecipientID: actor.MemberID,
		StartTime: start, EndTimeExclusive: endExclusive,
	})
	if err != nil {
		return nil, fmt.Errorf("list my memos by range: %w", err)
	}
	return buildMemoViews(actor, rows), nil
}

func (s *MemoService) MyMemoCalendar(ctx context.Context, actor AuthenticatedIdentity, month string) (MemoCalendarView, error) {
	if err := requireMemoActor(actor); err != nil {
		return MemoCalendarView{}, err
	}
	month = strings.TrimSpace(month)
	var start time.Time
	var err error
	if month == "" {
		now := time.Now().In(s.location)
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.location)
	} else {
		start, err = time.ParseInLocation("2006-01", month, s.location)
		if err != nil {
			return MemoCalendarView{}, ErrInvalidInput
		}
	}
	end := start.AddDate(0, 1, 0)
	memos, err := s.MyMemosByRange(ctx, actor, start, end)
	if err != nil {
		return MemoCalendarView{}, err
	}
	overrides, err := s.repository.ListCalendarDayOverridesByRange(ctx, start, end)
	if err != nil {
		return MemoCalendarView{}, fmt.Errorf("list calendar overrides: %w", err)
	}
	return MemoCalendarView{Month: start.Format("2006-01"), Memos: memos, Overrides: overrides}, nil
}

func (s *MemoService) MyMemosForDay(ctx context.Context, actor AuthenticatedIdentity, date string) ([]MemoView, error) {
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(date), s.location)
	if err != nil {
		return nil, ErrInvalidInput
	}
	return s.MyMemosByRange(ctx, actor, day, day.AddDate(0, 0, 1))
}

func (s *MemoService) SearchMyMemos(ctx context.Context, actor AuthenticatedIdentity, keyword string, limit int) ([]MemoView, error) {
	if err := requireMemoActor(actor); err != nil {
		return nil, err
	}
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || utf8.RuneCountInString(keyword) > maxMemoSearchKeywordLength {
		return nil, ErrInvalidInput
	}
	rows, err := s.repository.SearchRecipientMemos(ctx, repository.SearchRecipientMemosParams{
		HouseholdID: actor.HouseholdID, RecipientID: actor.MemberID, Keyword: keyword, Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("search my memos: %w", err)
	}
	return buildMemoViews(actor, rows), nil
}

func (s *MemoService) CreatedMemos(ctx context.Context, actor AuthenticatedIdentity, limit, offset int) ([]MemoView, error) {
	if err := requireMemoActor(actor); err != nil {
		return nil, err
	}
	rows, err := s.repository.ListCreatedMemos(ctx, repository.ListCreatedMemosParams{
		HouseholdID: actor.HouseholdID, CreatedBy: actor.MemberID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list created memos: %w", err)
	}
	return buildMemoViews(actor, rows), nil
}

func (s *MemoService) DueMemos(ctx context.Context, actor AuthenticatedIdentity, now time.Time) ([]MemoView, error) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(s.location)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.location)
	return s.MyMemosByRange(ctx, actor, start, now.Add(time.Microsecond))
}

func (s *MemoService) CleanupMemos(ctx context.Context, now time.Time) (MemoCleanupResult, error) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(s.location)
	cutoff := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.location)
	expired, err := s.repository.DeleteMemosBefore(ctx, cutoff)
	if err != nil {
		return MemoCleanupResult{}, fmt.Errorf("delete expired memos: %w", err)
	}
	empty, err := s.repository.DeleteMemosWithoutRecipients(ctx)
	if err != nil {
		return MemoCleanupResult{ExpiredDeleted: expired}, fmt.Errorf("delete memos without recipients: %w", err)
	}
	return MemoCleanupResult{ExpiredDeleted: expired, WithoutRecipientDeleted: empty}, nil
}

func (s *MemoService) PendingEmailMemos(ctx context.Context, dueAfter, dueBefore time.Time, limit int) ([]model.Memo, error) {
	if dueBefore.IsZero() {
		return nil, ErrInvalidInput
	}
	rows, err := s.repository.ListPendingEmailMemos(ctx, repository.ListPendingEmailMemosParams{
		DueAfter: dueAfter.In(s.location), DueBefore: dueBefore.In(s.location), Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list pending memo email jobs: %w", err)
	}
	return rows, nil
}

func (s *MemoService) MarkEmailSent(ctx context.Context, memo model.Memo, sentAt time.Time) error {
	if memo.ID == 0 || memo.HouseholdID == 0 || memo.Version == 0 || sentAt.IsZero() {
		return ErrInvalidInput
	}
	err := s.repository.MarkMemoEmailSent(ctx, repository.MarkMemoEmailSentParams{
		ID: memo.ID, HouseholdID: memo.HouseholdID, Version: memo.Version, SentAt: sentAt.In(s.location),
	})
	if errors.Is(err, repository.ErrConflict) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("mark memo email sent: %w", err)
	}
	return nil
}

func (s *MemoService) validateMemoInput(ctx context.Context, actor AuthenticatedIdentity, title, description string, remindAt time.Time, recipientIDs []uint64, excludeMemoID uint64) (normalizedMemoInput, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" || utf8.RuneCountInString(title) > maxMemoTitleLength || utf8.RuneCountInString(description) > maxMemoDescriptionLength {
		return normalizedMemoInput{}, ErrInvalidInput
	}
	if remindAt.IsZero() {
		return normalizedMemoInput{}, ErrMemoTimeInvalid
	}
	remindAt = remindAt.In(s.location)
	if !remindAt.After(time.Now().In(s.location)) || remindAt.Second() != 0 || remindAt.Nanosecond() != 0 || (remindAt.Minute() != 0 && remindAt.Minute() != 30) {
		return normalizedMemoInput{}, ErrMemoTimeInvalid
	}
	recipients := normalizeMemoRecipientIDs(recipientIDs)
	if len(recipients) == 0 {
		return normalizedMemoInput{}, ErrMemoRecipientInvalid
	}
	activeMembers, err := s.repository.ListActiveMemberOptions(ctx, actor.HouseholdID)
	if err != nil {
		return normalizedMemoInput{}, fmt.Errorf("list active memo recipients: %w", err)
	}
	activeIDs := make(map[uint64]struct{}, len(activeMembers))
	for _, member := range activeMembers {
		activeIDs[member.ID] = struct{}{}
	}
	for _, recipientID := range recipients {
		if _, ok := activeIDs[recipientID]; !ok {
			return normalizedMemoInput{}, ErrMemoRecipientInvalid
		}
	}
	slotStart := time.Date(remindAt.Year(), remindAt.Month(), remindAt.Day(), remindAt.Hour(), 0, 0, 0, s.location)
	slotEnd := slotStart.Add(time.Hour)
	for _, recipientID := range recipients {
		count, err := s.repository.CountRecipientMemosInSlot(ctx, repository.CountRecipientMemosInSlotParams{
			HouseholdID: actor.HouseholdID, RecipientID: recipientID, StartTime: slotStart,
			EndTimeExclusive: slotEnd, ExcludeMemoID: excludeMemoID,
		})
		if err != nil {
			return normalizedMemoInput{}, fmt.Errorf("count memo time slot: %w", err)
		}
		if count >= maxMemosPerRecipientPerHour {
			return normalizedMemoInput{}, ErrMemoSlotFull
		}
	}
	return normalizedMemoInput{title: title, description: description, remindAt: remindAt, recipientIDs: recipients}, nil
}

func (s *MemoService) getMemo(ctx context.Context, actor AuthenticatedIdentity, memoID uint64) (model.Memo, error) {
	if memoID == 0 {
		return model.Memo{}, ErrInvalidInput
	}
	memo, err := s.repository.GetMemoByID(ctx, memoID, actor.HouseholdID)
	if errors.Is(err, repository.ErrNotFound) {
		return model.Memo{}, ErrNotFound
	}
	if err != nil {
		return model.Memo{}, fmt.Errorf("get memo: %w", err)
	}
	return memo, nil
}

func requireMemoActor(actor AuthenticatedIdentity) error {
	if actor.MemberID == 0 || actor.HouseholdID == 0 {
		return ErrUnauthenticated
	}
	return nil
}

func canManageMemo(actor AuthenticatedIdentity, memo model.Memo) bool {
	return actor.Role == model.MemberRoleOwner || actor.Role == model.MemberRoleAdmin || memo.CreatedBy == actor.MemberID
}

func canReadMemo(actor AuthenticatedIdentity, memo model.Memo) bool {
	return canManageMemo(actor, memo) || containsMemoRecipient(memo.RecipientIDs, actor.MemberID)
}

func containsMemoRecipient(recipientIDs []uint64, memberID uint64) bool {
	return slices.Contains(recipientIDs, memberID)
}

func normalizeMemoRecipientIDs(recipientIDs []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(recipientIDs))
	result := make([]uint64, 0, len(recipientIDs))
	for _, id := range recipientIDs {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	slices.Sort(result)
	return result
}

func buildMemoView(actor AuthenticatedIdentity, memo model.Memo) MemoView {
	manageable := canManageMemo(actor, memo)
	return MemoView{Memo: memo, CanEdit: manageable, CanDelete: manageable, CanDismiss: containsMemoRecipient(memo.RecipientIDs, actor.MemberID)}
}

func buildMemoViews(actor AuthenticatedIdentity, memos []model.Memo) []MemoView {
	result := make([]MemoView, 0, len(memos))
	for _, memo := range memos {
		result = append(result, buildMemoView(actor, memo))
	}
	return result
}
