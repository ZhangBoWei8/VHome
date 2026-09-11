package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"

	"vhome/internal/model"
)

const agentMemoryColumns = `
id,
household_id,
scope,
member_id,
content,
source_conversation_id,
created_by,
version,
created_at,
updated_at
`

type CreateAgentMemoryParams struct {
	HouseholdID          uint64
	Scope                model.AgentMemoryScope
	MemberID             *uint64
	Content              string
	SourceConversationID *uint64
	CreatedBy            uint64
}

type DeleteAgentMemoryParams struct {
	ID          uint64
	HouseholdID uint64
	Version     uint64
}

// ListAgentMemoriesParams selects everything visible to one member: the
// household-wide rows plus that member's own private rows.
type ListAgentMemoriesParams struct {
	HouseholdID uint64
	MemberID    uint64
	Limit       int
}

type CountAgentMemoriesParams struct {
	HouseholdID uint64
	Scope       model.AgentMemoryScope

	// MemberID is ignored for the household scope.
	MemberID uint64
}

// AgentMemoryContentHash is the uniqueness key for a memory. The hash is
// indexed rather than the text itself because a utf8mb4 VARCHAR(500) exceeds
// the InnoDB index limit.
func AgentMemoryContentHash(content string) []byte {
	sum := sha256.Sum256([]byte(content))

	return sum[:]
}

type agentMemoryScanner interface {
	Scan(dest ...any) error
}

func scanAgentMemory(scanner agentMemoryScanner) (model.AgentMemory, error) {
	var memory model.AgentMemory

	var memberID sql.NullInt64
	var sourceConversationID sql.NullInt64

	err := scanner.Scan(
		&memory.ID,
		&memory.HouseholdID,
		&memory.Scope,
		&memberID,
		&memory.Content,
		&sourceConversationID,
		&memory.CreatedBy,
		&memory.Version,
		&memory.CreatedAt,
		&memory.UpdatedAt,
	)
	if err != nil {
		return model.AgentMemory{}, err
	}

	if memberID.Valid {
		value := uint64(memberID.Int64)
		memory.MemberID = &value
	}

	if sourceConversationID.Valid {
		value := uint64(sourceConversationID.Int64)
		memory.SourceConversationID = &value
	}

	return memory, nil
}

func nullableAgentMemoryID(id *uint64) any {
	if id == nil {
		return nil
	}

	return *id
}

// agentMemoryMemberKey mirrors the member_key generated column, which folds a
// NULL member_id to 0 so the unique index covers household memories too.
func agentMemoryMemberKey(memberID *uint64) uint64 {
	if memberID == nil {
		return 0
	}

	return *memberID
}

func (r *Repository) GetAgentMemoryByID(ctx context.Context, memoryID uint64, householdID uint64) (model.AgentMemory, error) {
	query := `SELECT` + agentMemoryColumns + `FROM agent_memories WHERE id = ? AND household_id = ? LIMIT 1`

	memory, err := scanAgentMemory(r.q.QueryRowContext(ctx, query, memoryID, householdID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.AgentMemory{}, ErrNotFound
	}
	if err != nil {
		return model.AgentMemory{}, fmt.Errorf("get agent memory by id: %w", err)
	}

	return memory, nil
}

// CreateAgentMemory is idempotent on the content hash: asking 8V to remember
// the same thing twice refreshes the existing row instead of filling the
// system prompt with duplicates. The returned bool reports whether a new row
// was inserted, so the caller can tell the model which of the two happened.
func (r *Repository) CreateAgentMemory(ctx context.Context, cmp CreateAgentMemoryParams) (model.AgentMemory, bool, error) {
	const query = `
INSERT INTO agent_memories
    (household_id, scope, member_id, content, content_hash, source_conversation_id, created_by)
VALUES (?,?,?,?,?,?,?)
ON DUPLICATE KEY UPDATE
    content = VALUES(content),
    source_conversation_id = VALUES(source_conversation_id),
    version = version + 1`

	result, err := r.q.ExecContext(
		ctx,
		query,
		cmp.HouseholdID,
		cmp.Scope,
		nullableAgentMemoryID(cmp.MemberID),
		cmp.Content,
		AgentMemoryContentHash(cmp.Content),
		nullableAgentMemoryID(cmp.SourceConversationID),
		cmp.CreatedBy,
	)
	if err != nil {
		return model.AgentMemory{}, false, fmt.Errorf("create agent memory: %w", err)
	}

	// MySQL reports 1 affected row for an insert and 2 for an ON DUPLICATE KEY
	// update that changed something. A row that matched but changed nothing
	// reports 0, which is still not an insert.
	affected, err := result.RowsAffected()
	if err != nil {
		return model.AgentMemory{}, false, fmt.Errorf("get agent memory affected row count: %w", err)
	}
	inserted := affected == 1

	id, err := result.LastInsertId()
	if err != nil {
		return model.AgentMemory{}, false, fmt.Errorf("get created agent memory id: %w", err)
	}

	// LastInsertId is only meaningful for the insert path. On the duplicate
	// path look the row up by its hash instead.
	if !inserted {
		memory, lookupErr := r.getAgentMemoryByContent(ctx, cmp)
		if lookupErr != nil {
			return model.AgentMemory{}, false, lookupErr
		}

		return memory, false, nil
	}

	memory, err := r.GetAgentMemoryByID(ctx, uint64(id), cmp.HouseholdID)
	if err != nil {
		return model.AgentMemory{}, false, err
	}

	return memory, true, nil
}

func (r *Repository) getAgentMemoryByContent(ctx context.Context, cmp CreateAgentMemoryParams) (model.AgentMemory, error) {
	// member_key is the generated column the unique index is built on, so this
	// matches exactly the row the upsert collided with.
	query := `SELECT` + agentMemoryColumns + `
FROM agent_memories
WHERE household_id = ? AND member_key = ? AND content_hash = ?
LIMIT 1`

	memory, err := scanAgentMemory(r.q.QueryRowContext(
		ctx,
		query,
		cmp.HouseholdID,
		agentMemoryMemberKey(cmp.MemberID),
		AgentMemoryContentHash(cmp.Content),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return model.AgentMemory{}, ErrNotFound
	}
	if err != nil {
		return model.AgentMemory{}, fmt.Errorf("get agent memory by content: %w", err)
	}

	return memory, nil
}

func (r *Repository) DeleteAgentMemory(ctx context.Context, cmp DeleteAgentMemoryParams) error {
	const query = `DELETE FROM agent_memories WHERE id = ? AND household_id = ? AND version = ?`

	result, err := r.q.ExecContext(ctx, query, cmp.ID, cmp.HouseholdID, cmp.Version)
	if err != nil {
		return fmt.Errorf("delete agent memory: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get agent memory affected row count: %w", err)
	}

	if affected != 1 {
		return ErrConflict
	}

	return nil
}

// ListAgentMemoriesForMember returns household memories first, then the
// member's own, each oldest-first so the prompt ordering stays stable between
// turns and the model is not nudged by a shuffling list.
func (r *Repository) ListAgentMemoriesForMember(ctx context.Context, cmp ListAgentMemoriesParams) ([]model.AgentMemory, error) {
	query := `SELECT` + agentMemoryColumns + `
FROM agent_memories
WHERE household_id = ?
  AND (member_id IS NULL OR member_id = ?)
ORDER BY (member_id IS NOT NULL), id
LIMIT ?`

	rows, err := r.q.QueryContext(ctx, query, cmp.HouseholdID, cmp.MemberID, cmp.Limit)
	if err != nil {
		return nil, fmt.Errorf("list agent memories: %w", err)
	}
	defer rows.Close()

	memories := make([]model.AgentMemory, 0)
	for rows.Next() {
		memory, scanErr := scanAgentMemory(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan agent memory: %w", scanErr)
		}

		memories = append(memories, memory)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent memories: %w", err)
	}

	return memories, nil
}

// CountAgentMemories backs the per-scope cap that keeps the system prompt from
// growing without bound. The two scopes are counted with separate queries:
// the household scope keys on member_id IS NULL, which no placeholder can
// express.
func (r *Repository) CountAgentMemories(ctx context.Context, cmp CountAgentMemoriesParams) (int, error) {
	const householdQuery = `
SELECT COUNT(*) FROM agent_memories
WHERE household_id = ? AND scope = 'HOUSEHOLD' AND member_id IS NULL`

	const memberQuery = `
SELECT COUNT(*) FROM agent_memories
WHERE household_id = ? AND scope = 'MEMBER' AND member_id = ?`

	var (
		count int
		err   error
	)

	if cmp.Scope == model.AgentMemoryScopeHousehold {
		err = r.q.QueryRowContext(ctx, householdQuery, cmp.HouseholdID).Scan(&count)
	} else {
		err = r.q.QueryRowContext(ctx, memberQuery, cmp.HouseholdID, cmp.MemberID).Scan(&count)
	}

	if err != nil {
		return 0, fmt.Errorf("count agent memories: %w", err)
	}

	return count, nil
}
