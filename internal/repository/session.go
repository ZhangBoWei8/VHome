package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	"vhome/internal/model"
)

const sessionColumns = `
	id,
	household_id,
	member_id,
	token_hash,
	csrf_token_hash,
	expires_at,
	last_seen_at,
	created_ip,
	user_agent,
	revoked_at,
	revoke_reason,
	created_at
`

type CreateSessionParams struct {
	HouseholdID   uint64
	MemberID      uint64
	TokenHash     []byte
	CSRFTokenHash []byte
	ExpiresAt     time.Time
	CreatedIP     net.IP
	UserAgent     string
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(scanner sessionScanner) (model.Session, error) {
	var session model.Session

	var createdIP []byte
	var userAgent sql.NullString
	var revokedAt sql.NullTime
	var revokeReason sql.NullString

	err := scanner.Scan(
		&session.ID,
		&session.HouseholdID,
		&session.MemberID,
		&session.TokenHash,
		&session.CSRFTokenHash,
		&session.ExpiresAt,
		&session.LastSeenAt,
		&createdIP,
		&userAgent,
		&revokedAt,
		&revokeReason,
		&session.CreatedAt,
	)
	if err != nil {
		return model.Session{}, err
	}

	if len(createdIP) > 0 {
		session.CreatedIP = net.IP(
			append([]byte(nil), createdIP...),
		)
	}

	if userAgent.Valid {
		session.UserAgent = userAgent.String
	}

	if revokedAt.Valid {
		session.RevokedAt = &revokedAt.Time
	}

	if revokeReason.Valid {
		session.RevokeReason =
			model.SessionRevokeReason(revokeReason.String)
	}

	return session, nil
}

// 创建会话并保存Token
func (r *Repository) CreateSession(ctx context.Context, params CreateSessionParams) (model.Session, error) {
	if len(params.TokenHash) != sha256.Size {
		return model.Session{}, fmt.Errorf(
			"%w: token hash must be %d bytes",
			ErrInvalidArgument,
			sha256.Size,
		)
	}

	if len(params.CSRFTokenHash) != sha256.Size {
		return model.Session{}, fmt.Errorf(
			"%w: csrf token hash must be %d bytes",
			ErrInvalidArgument,
			sha256.Size,
		)
	}

	if params.ExpiresAt.IsZero() {
		return model.Session{}, fmt.Errorf(
			"%w: expires_at is required",
			ErrInvalidArgument,
		)
	}

	const query = `
		INSERT INTO sessions (
			household_id,
			member_id,
			token_hash,
			csrf_token_hash,
			expires_at,
			created_ip,
			user_agent
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		params.HouseholdID,
		params.MemberID,
		params.TokenHash,
		params.CSRFTokenHash,
		params.ExpiresAt.UTC(),
		nullableIP(params.CreatedIP),
		nullableString(params.UserAgent),
	)
	if err != nil {
		if isDuplicateKey(err) {
			return model.Session{}, fmt.Errorf(
				"%w: session token already exists",
				ErrConflict,
			)
		}

		return model.Session{}, fmt.Errorf(
			"create session: %w",
			err,
		)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.Session{}, fmt.Errorf(
			"get created session id: %w",
			err,
		)
	}

	return r.GetSessionByID(ctx, uint64(id))
}

func (r *Repository) GetSessionByID(ctx context.Context, id uint64) (model.Session, error) {
	query := `
		SELECT ` + sessionColumns + `
		FROM sessions
		WHERE id = ?
		LIMIT 1
	`

	session, err := scanSession(
		r.q.QueryRowContext(ctx, query, id),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Session{}, ErrNotFound
		}

		return model.Session{}, fmt.Errorf(
			"get session by id: %w",
			err,
		)
	}

	return session, nil
}

func (r *Repository) GetActiveSessionByTokenHash(ctx context.Context, tokenHash []byte) (model.Session, error) {
	if len(tokenHash) != sha256.Size {
		return model.Session{}, fmt.Errorf(
			"%w: token hash must be %d bytes",
			ErrInvalidArgument,
			sha256.Size,
		)
	}

	query := `
		SELECT ` + sessionColumns + `
		FROM sessions
		WHERE token_hash = ?
		  AND revoked_at IS NULL
		  AND expires_at > UTC_TIMESTAMP(6)
		LIMIT 1
	`

	session, err := scanSession(
		r.q.QueryRowContext(ctx, query, tokenHash),
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Session{}, ErrNotFound
		}

		return model.Session{}, fmt.Errorf(
			"get active session by token hash: %w",
			err,
		)
	}

	return session, nil
}

func (r *Repository) TouchSession(ctx context.Context, id uint64) error {
	const query = `
		UPDATE sessions
		SET last_seen_at = UTC_TIMESTAMP(6)
		WHERE id = ?
		  AND revoked_at IS NULL
		  AND expires_at > UTC_TIMESTAMP(6)
		  AND last_seen_at <
		      UTC_TIMESTAMP(6) - INTERVAL 5 MINUTE
	`

	_, err := r.q.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("touch session: %w", err)
	}

	return nil
}

func (r *Repository) RevokeSession(ctx context.Context, id uint64, reason model.SessionRevokeReason) error {
	if reason == "" {
		return fmt.Errorf(
			"%w: revoke reason is required",
			ErrInvalidArgument,
		)
	}

	const query = `
		UPDATE sessions
		SET revoked_at = UTC_TIMESTAMP(6),
		    revoke_reason = ?
		WHERE id = ?
		  AND revoked_at IS NULL
	`

	_, err := r.q.ExecContext(ctx, query, reason, id)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}

func (r *Repository) RevokeMemberSessions(ctx context.Context, memberID uint64, reason model.SessionRevokeReason) (uint64, error) {
	if reason == "" {
		return 0, fmt.Errorf(
			"%w: revoke reason is required",
			ErrInvalidArgument,
		)
	}

	const query = `
		UPDATE sessions
		SET revoked_at = UTC_TIMESTAMP(6),
		    revoke_reason = ?
		WHERE member_id = ?
		  AND revoked_at IS NULL
	`

	result, err := r.q.ExecContext(
		ctx,
		query,
		reason,
		memberID,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"revoke member sessions: %w",
			err,
		)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf(
			"get revoked session count: %w",
			err,
		)
	}

	return uint64(affected), nil
}

func nullableIP(ip net.IP) any {
	if ip == nil {
		return nil
	}

	normalized := ip.To16()
	if normalized == nil {
		return nil
	}

	return []byte(normalized)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}
