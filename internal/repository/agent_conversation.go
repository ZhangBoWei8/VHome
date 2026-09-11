package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"vhome/internal/model"
)

const agentConversationColumns = `
id,
household_id,
member_id,
title,
last_message_at,
message_count,
version,
created_at,
updated_at
`

const agentMessageColumns = `
id,
conversation_id,
household_id,
seq,
role,
content,
tool_calls,
tool_call_id,
tool_name,
created_at
`

type CreateAgentConversationParams struct {
	HouseholdID uint64
	MemberID    uint64
	Title       string
}

type ListAgentConversationsParams struct {
	HouseholdID uint64
	MemberID    uint64
	Limit       int
}

type ListAgentMessagesParams struct {
	ConversationID uint64

	// Limit takes the newest messages. The caller reverses them back into
	// chronological order.
	Limit int
}

type AppendAgentMessagesParams struct {
	ConversationID uint64
	HouseholdID    uint64
	Messages       []AgentMessageInput

	// Title is written only when the conversation still has the empty
	// placeholder title, so a thread keeps the name its first question gave it.
	Title string

	// KeepMessages caps the thread. Once it is exceeded the oldest messages are
	// dropped, so a conversation nobody ever closes cannot grow without bound.
	// Zero disables trimming.
	KeepMessages int

	At time.Time
}

type AgentMessageInput struct {
	Role       model.AgentMessageRole
	Content    string
	ToolCalls  []byte
	ToolCallID string
	ToolName   string
}

type DeleteAgentConversationParams struct {
	ID          uint64
	HouseholdID uint64
	MemberID    uint64
}

type agentConversationScanner interface {
	Scan(dest ...any) error
}

func scanAgentConversation(scanner agentConversationScanner) (model.AgentConversation, error) {
	var conversation model.AgentConversation

	err := scanner.Scan(
		&conversation.ID,
		&conversation.HouseholdID,
		&conversation.MemberID,
		&conversation.Title,
		&conversation.LastMessageAt,
		&conversation.MessageCount,
		&conversation.Version,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	)
	if err != nil {
		return model.AgentConversation{}, err
	}

	return conversation, nil
}

func scanAgentMessage(scanner agentConversationScanner) (model.AgentMessage, error) {
	var message model.AgentMessage

	var toolCalls []byte
	var toolCallID sql.NullString
	var toolName sql.NullString

	err := scanner.Scan(
		&message.ID,
		&message.ConversationID,
		&message.HouseholdID,
		&message.Seq,
		&message.Role,
		&message.Content,
		&toolCalls,
		&toolCallID,
		&toolName,
		&message.CreatedAt,
	)
	if err != nil {
		return model.AgentMessage{}, err
	}

	message.ToolCalls = toolCalls
	if toolCallID.Valid {
		message.ToolCallID = toolCallID.String
	}
	if toolName.Valid {
		message.ToolName = toolName.String
	}

	return message, nil
}

func nullableAgentString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func nullableAgentJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}

	return value
}

func (r *Repository) GetAgentConversationByID(ctx context.Context, conversationID uint64, householdID uint64) (model.AgentConversation, error) {
	query := `SELECT` + agentConversationColumns + `FROM agent_conversations WHERE id = ? AND household_id = ? LIMIT 1`

	conversation, err := scanAgentConversation(r.q.QueryRowContext(ctx, query, conversationID, householdID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.AgentConversation{}, ErrNotFound
	}
	if err != nil {
		return model.AgentConversation{}, fmt.Errorf("get agent conversation by id: %w", err)
	}

	return conversation, nil
}

func (r *Repository) CreateAgentConversation(ctx context.Context, cmp CreateAgentConversationParams) (model.AgentConversation, error) {
	const query = `INSERT INTO agent_conversations (household_id, member_id, title) VALUES (?,?,?)`

	result, err := r.q.ExecContext(ctx, query, cmp.HouseholdID, cmp.MemberID, cmp.Title)
	if err != nil {
		return model.AgentConversation{}, fmt.Errorf("create agent conversation: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return model.AgentConversation{}, fmt.Errorf("get created agent conversation id: %w", err)
	}

	return r.GetAgentConversationByID(ctx, uint64(id), cmp.HouseholdID)
}

func (r *Repository) ListAgentConversations(ctx context.Context, cmp ListAgentConversationsParams) ([]model.AgentConversation, error) {
	query := `SELECT` + agentConversationColumns + `
FROM agent_conversations
WHERE household_id = ? AND member_id = ?
ORDER BY last_message_at DESC, id DESC
LIMIT ?`

	rows, err := r.q.QueryContext(ctx, query, cmp.HouseholdID, cmp.MemberID, cmp.Limit)
	if err != nil {
		return nil, fmt.Errorf("list agent conversations: %w", err)
	}
	defer rows.Close()

	conversations := make([]model.AgentConversation, 0)
	for rows.Next() {
		conversation, scanErr := scanAgentConversation(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan agent conversation: %w", scanErr)
		}

		conversations = append(conversations, conversation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent conversations: %w", err)
	}

	return conversations, nil
}

// ListAgentMessages returns the newest Limit messages in chronological order.
// The window is taken from the end because that is the part of the thread the
// model needs.
func (r *Repository) ListAgentMessages(ctx context.Context, cmp ListAgentMessagesParams) ([]model.AgentMessage, error) {
	query := `SELECT * FROM (
    SELECT` + agentMessageColumns + `
    FROM agent_messages
    WHERE conversation_id = ?
    ORDER BY seq DESC
    LIMIT ?
) AS recent ORDER BY seq`

	rows, err := r.q.QueryContext(ctx, query, cmp.ConversationID, cmp.Limit)
	if err != nil {
		return nil, fmt.Errorf("list agent messages: %w", err)
	}
	defer rows.Close()

	messages := make([]model.AgentMessage, 0)
	for rows.Next() {
		message, scanErr := scanAgentMessage(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan agent message: %w", scanErr)
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent messages: %w", err)
	}

	return messages, nil
}

// AppendAgentMessages writes one turn's messages and advances the conversation
// counters. Call it inside WithinTransaction: the seq numbers are derived from
// the current maximum, so a concurrent append would otherwise collide on
// uk_agent_messages_seq.
func (r *Repository) AppendAgentMessages(ctx context.Context, cmp AppendAgentMessagesParams) error {
	if len(cmp.Messages) == 0 {
		return nil
	}

	var nextSeq uint32
	err := r.q.QueryRowContext(
		ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM agent_messages WHERE conversation_id = ?`,
		cmp.ConversationID,
	).Scan(&nextSeq)
	if err != nil {
		return fmt.Errorf("next agent message seq: %w", err)
	}

	const insertQuery = `
INSERT INTO agent_messages
    (conversation_id, household_id, seq, role, content, tool_calls, tool_call_id, tool_name)
VALUES (?,?,?,?,?,?,?,?)`

	for offset, message := range cmp.Messages {
		_, execErr := r.q.ExecContext(
			ctx,
			insertQuery,
			cmp.ConversationID,
			cmp.HouseholdID,
			nextSeq+uint32(offset),
			message.Role,
			message.Content,
			nullableAgentJSON(message.ToolCalls),
			nullableAgentString(message.ToolCallID),
			nullableAgentString(message.ToolName),
		)
		if execErr != nil {
			return fmt.Errorf("insert agent message: %w", execErr)
		}
	}

	if cmp.KeepMessages > 0 {
		if err := r.trimAgentMessages(ctx, cmp.ConversationID, cmp.KeepMessages); err != nil {
			return err
		}
	}

	// COALESCE(NULLIF(title,''), ?) keeps the title the first question gave
	// the thread and fills it in only while it is still the placeholder.
	//
	// message_count is recomputed rather than incremented: trimming may have
	// just deleted rows, and a running total would drift away from what is
	// actually stored.
	const updateQuery = `
UPDATE agent_conversations
SET title = COALESCE(NULLIF(title, ''), ?),
    last_message_at = ?,
    message_count = (SELECT COUNT(*) FROM agent_messages WHERE conversation_id = ?),
    version = version + 1
WHERE id = ? AND household_id = ?`

	result, err := r.q.ExecContext(
		ctx,
		updateQuery,
		nullableAgentString(cmp.Title),
		cmp.At,
		cmp.ConversationID,
		cmp.ConversationID,
		cmp.HouseholdID,
	)
	if err != nil {
		return fmt.Errorf("update agent conversation counters: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get agent conversation affected row count: %w", err)
	}
	if affected != 1 {
		return ErrNotFound
	}

	return nil
}

// trimAgentMessages drops everything older than the newest keep messages.
//
// MySQL cannot DELETE with an ORDER BY over a subquery on the same table, so
// the cut-off seq is looked up first and the delete is a plain range. seq is
// monotonic per conversation and carries a unique index, which makes this both
// correct and cheap.
func (r *Repository) trimAgentMessages(ctx context.Context, conversationID uint64, keep int) error {
	const thresholdQuery = `
SELECT seq FROM agent_messages
WHERE conversation_id = ?
ORDER BY seq DESC
LIMIT 1 OFFSET ?`

	var oldestKeptSeq uint32
	err := r.q.QueryRowContext(ctx, thresholdQuery, conversationID, keep-1).Scan(&oldestKeptSeq)
	if errors.Is(err, sql.ErrNoRows) {
		// Fewer messages than the cap: nothing to trim.
		return nil
	}
	if err != nil {
		return fmt.Errorf("find agent message trim threshold: %w", err)
	}

	const deleteQuery = `DELETE FROM agent_messages WHERE conversation_id = ? AND seq < ?`

	if _, err := r.q.ExecContext(ctx, deleteQuery, conversationID, oldestKeptSeq); err != nil {
		return fmt.Errorf("trim agent messages: %w", err)
	}

	return nil
}

// DeleteAgentConversation removes a thread. The messages go with it through
// the ON DELETE CASCADE foreign key. Scoping the statement by member_id means
// one member can never delete another's thread, whatever id they send.
func (r *Repository) DeleteAgentConversation(ctx context.Context, cmp DeleteAgentConversationParams) error {
	const query = `DELETE FROM agent_conversations WHERE id = ? AND household_id = ? AND member_id = ?`

	result, err := r.q.ExecContext(ctx, query, cmp.ID, cmp.HouseholdID, cmp.MemberID)
	if err != nil {
		return fmt.Errorf("delete agent conversation: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get agent conversation affected row count: %w", err)
	}
	if affected != 1 {
		return ErrNotFound
	}

	return nil
}

// DeleteIdleAgentConversations is the retention sweep run by the daily
// maintenance worker.
func (r *Repository) DeleteIdleAgentConversations(ctx context.Context, idleBefore time.Time, limit int) (int64, error) {
	const query = `DELETE FROM agent_conversations WHERE last_message_at < ? LIMIT ?`

	result, err := r.q.ExecContext(ctx, query, idleBefore, limit)
	if err != nil {
		return 0, fmt.Errorf("delete idle agent conversations: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get deleted agent conversation count: %w", err)
	}

	return affected, nil
}

// AgentConversationTitle derives a list title from the first user message.
func AgentConversationTitle(input string, maxRunes int) string {
	normalized := strings.Join(strings.Fields(input), " ")

	runes := []rune(normalized)
	if len(runes) <= maxRunes {
		return normalized
	}

	return string(runes[:maxRunes-1]) + "…"
}
