package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"vhome/internal/model"
)

const memberColumns = `
	id,
	household_id,
	username,
	username_normalized,
	display_name,
	password_hash,
	role,
	status,
	reviewed_by,
	reviewed_at,
	last_login_at,
	presence_status,
	avatar_key,
	version,
	created_at,
	updated_at
`

type CreateMemberParams struct {
	HouseholdID        uint64
	Username           string
	UsernameNormalized string
	DisplayName        string
	PasswordHash       string
	Role               model.MemberRole
	Status             model.MemberStatus
	ReviewedBy         *uint64
	ReviewedAt         *time.Time
}

type ReviewMemberParams struct {
	MemberID    uint64
	HouseholdID uint64
	ReviewerID  uint64
	Version     uint64
}

type UpdateMemberRoleParams struct {
	MemberID    uint64
	HouseholdID uint64
	Role        model.MemberRole
	Version     uint64
}

type UpdateMemberProfileParams struct {
	MemberID       uint64
	HouseholdID    uint64
	DisplayName    string
	AvatarKey      model.MemberAvatar
	PresenceStatus model.PresenceStatus
	Version        uint64
}

type DisableMemberParams struct {
	MemberID    uint64
	HouseholdID uint64
	Version     uint64
}

type memberScanner interface {
	Scan(dest ...any) error
}

type MemberOption struct {
	ID          uint64
	DisplayName string
	AvatarKey   string
}

func scanMember(scanner memberScanner) (model.Member, error) {
	var member model.Member

	var reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	var lastLoginAt sql.NullTime
	var presenceStatus sql.NullString

	err := scanner.Scan(
		&member.ID,
		&member.HouseholdID,
		&member.Username,
		&member.UsernameNormalized,
		&member.DisplayName,
		&member.PasswordHash,
		&member.Role,
		&member.Status,
		&reviewedBy,
		&reviewedAt,
		&lastLoginAt,
		&presenceStatus,
		&member.AvatarKey,
		&member.Version,
		&member.CreatedAt,
		&member.UpdatedAt,
	)
	if err != nil {
		return model.Member{}, err
	}

	if reviewedBy.Valid {
		value := uint64(reviewedBy.Int64)
		member.ReviewedBy = &value
	}

	if reviewedAt.Valid {
		value := reviewedAt.Time
		member.ReviewedAt = &value
	}

	if lastLoginAt.Valid {
		value := lastLoginAt.Time
		member.LastLoginAt = &value
	}
	if presenceStatus.Valid {
		member.PresenceStatus = model.PresenceStatus(presenceStatus.String)
	}

	return member, nil
}

func nullableUint64(value *uint64) any {
	if value == nil {
		return nil
	}

	return *value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}

	return *value
}

func (r *Repository) CreateMember(ctx context.Context, params CreateMemberParams) (model.Member, error) {
	const query = `
		INSERT INTO members (
			household_id,
			username,
			username_normalized,
			display_name,
			password_hash,
			role,
			status,
			reviewed_by,
			reviewed_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.HouseholdID,
		params.Username,
		params.UsernameNormalized,
		params.DisplayName,
		params.PasswordHash,
		string(params.Role),
		string(params.Status),
		nullableUint64(params.ReviewedBy),
		nullableTime(params.ReviewedAt),
	)
	if isDuplicateKey(err) {
		return model.Member{}, ErrConflict
	}
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"create member: %w",
			err,
		)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"get created member id: %w",
			err,
		)
	}

	return r.GetMemberByID(ctx, uint64(id))
}

func (r *Repository) GetMemberByID(ctx context.Context, id uint64) (model.Member, error) {
	query := `
		SELECT ` + memberColumns + `
		FROM members
		WHERE id = ?
		LIMIT 1
	`

	member, err := scanMember(
		r.q.QueryRowContext(ctx, query, id),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Member{}, ErrNotFound
	}
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"get member by id: %w",
			err,
		)
	}

	return member, nil
}

func (r *Repository) GetMemberByUsername(ctx context.Context, householdID uint64, usernameNormalized string) (model.Member, error) {
	query := `
		SELECT ` + memberColumns + `
		FROM members
		WHERE household_id = ?
		  AND username_normalized = ?
		LIMIT 1
	`

	member, err := scanMember(
		r.q.QueryRowContext(
			ctx,
			query,
			householdID,
			usernameNormalized,
		),
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Member{}, ErrNotFound
	}
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"get member by username: %w",
			err,
		)
	}

	return member, nil
}

func (r *Repository) ListMembersByStatus(ctx context.Context, householdID uint64, status model.MemberStatus) ([]model.Member, error) {
	query := `
		SELECT ` + memberColumns + `
		FROM members
		WHERE household_id = ?
		  AND status = ?
		ORDER BY created_at ASC, id ASC
	`

	rows, err := r.q.QueryContext(
		ctx,
		query,
		householdID,
		string(status),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list members by status: %w",
			err,
		)
	}
	defer rows.Close()

	members := make([]model.Member, 0)

	for rows.Next() {
		member, err := scanMember(rows)
		if err != nil {
			return nil, fmt.Errorf(
				"scan member list row: %w",
				err,
			)
		}

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate member list: %w",
			err,
		)
	}

	return members, nil
}

func (r *Repository) ListMembers(ctx context.Context, householdID uint64) ([]model.Member, error) {
	rows, err := r.q.QueryContext(ctx, `SELECT `+memberColumns+` FROM members WHERE household_id=? ORDER BY FIELD(role,'OWNER','ADMIN','MEMBER'),created_at`, householdID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()
	members := make([]model.Member, 0)
	for rows.Next() {
		member, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *Repository) CountMembersByStatus(ctx context.Context, householdID uint64, status model.MemberStatus) (uint64, error) {
	const query = `
		SELECT COUNT(*)
		FROM members
		WHERE household_id = ?
		  AND status = ?
	`

	var count uint64

	if err := r.q.QueryRowContext(
		ctx,
		query,
		householdID,
		string(status),
	).Scan(&count); err != nil {
		return 0, fmt.Errorf(
			"count members by status: %w",
			err,
		)
	}

	return count, nil
}

func (r *Repository) ApproveMember(ctx context.Context, params ReviewMemberParams) (model.Member, error) {
	return r.reviewMember(
		ctx,
		params,
		model.MemberStatusActive,
	)
}

func (r *Repository) RejectMember(ctx context.Context, params ReviewMemberParams) (model.Member, error) {
	return r.reviewMember(
		ctx,
		params,
		model.MemberStatusRejected,
	)
}

func (r *Repository) reviewMember(ctx context.Context, params ReviewMemberParams, targetStatus model.MemberStatus) (model.Member, error) {
	const query = `
		UPDATE members
		SET
			status = ?,
			reviewed_by = ?,
			reviewed_at = UTC_TIMESTAMP(6),
			version = version + 1
		WHERE id = ?
		  AND household_id = ?
		  AND status = 'PENDING'
		  AND version = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		string(targetStatus),
		params.ReviewerID,
		params.MemberID,
		params.HouseholdID,
		params.Version,
	)
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"review member: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"get reviewed member rows: %w",
			err,
		)
	}

	if affected == 0 {
		return model.Member{}, ErrConflict
	}

	return r.GetMemberByID(ctx, params.MemberID)
}

func (r *Repository) UpdateMemberRole(ctx context.Context, params UpdateMemberRoleParams) (model.Member, error) {
	if params.Role != model.MemberRoleAdmin &&
		params.Role != model.MemberRoleMember {
		return model.Member{}, ErrConflict
	}

	const query = `
		UPDATE members
		SET
			role = ?,
			version = version + 1
		WHERE id = ?
		  AND household_id = ?
		  AND role <> 'OWNER'
		  AND status = 'ACTIVE'
		  AND version = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		string(params.Role),
		params.MemberID,
		params.HouseholdID,
		params.Version,
	)
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"update member role: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"get member role update rows: %w",
			err,
		)
	}

	if affected == 0 {
		return model.Member{}, ErrConflict
	}

	return r.GetMemberByID(ctx, params.MemberID)
}

func (r *Repository) UpdateMemberProfile(ctx context.Context, params UpdateMemberProfileParams) (model.Member, error) {
	var presenceValue any
	if params.PresenceStatus != "" {
		presenceValue = string(params.PresenceStatus)
	}

	const query = `
		UPDATE members
		SET
			display_name = ?,
			avatar_key = ?,
			presence_status = ?,
			version = version + 1
		WHERE id = ?
		  AND household_id = ?
		  AND version = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.DisplayName,
		string(params.AvatarKey),
		presenceValue,
		params.MemberID,
		params.HouseholdID,
		params.Version,
	)
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"update member profile: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"get member profile update rows: %w",
			err,
		)
	}

	if affected == 0 {
		return model.Member{}, ErrConflict
	}

	return r.GetMemberByID(ctx, params.MemberID)
}

func (r *Repository) DisableMember(ctx context.Context, params DisableMemberParams) (model.Member, error) {
	const query = `
		UPDATE members
		SET
			status = 'DISABLED',
			version = version + 1
		WHERE id = ?
		  AND household_id = ?
		  AND role <> 'OWNER'
		  AND version = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.MemberID,
		params.HouseholdID,
		params.Version,
	)
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"disable member: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return model.Member{}, fmt.Errorf(
			"get disabled member rows: %w",
			err,
		)
	}

	if affected == 0 {
		return model.Member{}, ErrConflict
	}

	return r.GetMemberByID(ctx, params.MemberID)
}

func (r *Repository) UpdateMemberLastLogin(ctx context.Context, memberID uint64) error {
	const query = `
		UPDATE members
		SET last_login_at = UTC_TIMESTAMP(6)
		WHERE id = ?
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		memberID,
	)
	if err != nil {
		return fmt.Errorf(
			"update member last login: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(
			"get last login update rows: %w",
			err,
		)
	}

	if affected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *Repository) ListActiveMemberOptions(ctx context.Context, householdID uint64) ([]MemberOption, error) {
	query := `SELECT id,display_name,avatar_key
	FROM members
	WHERE household_id = ? AND status = 'ACTIVE'
	`
	rows, err := r.q.QueryContext(ctx, query, householdID)
	if err != nil {
		return nil, fmt.Errorf("list active member error: %w.", err)
	}
	defer rows.Close()
	members := make([]MemberOption, 0)
	for rows.Next() {
		var member MemberOption

		if err := rows.Scan(&member.ID, &member.DisplayName, &member.AvatarKey); err != nil {
			return nil, fmt.Errorf("unmarshal member information error: %w.", err)
		}
		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate active member options: %w",
			err,
		)
	}

	return members, nil
}
