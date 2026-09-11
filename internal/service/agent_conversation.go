package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"vhome/internal/model"
	"vhome/internal/repository"
)

const (
	maxAgentConversationsPerMember = 100
	maxAgentConversationTitleRunes = 40

	// maxAgentContextMessages bounds how much of a thread is replayed to the
	// model. It is a token budget, not a display limit: the client can still
	// load more history than this.
	maxAgentContextMessages = 60

	// MaxAgentConversationRounds caps one thread. A round is one exchange, so
	// the stored row count is twice this: only the user's question and the
	// assistant's answer are persisted (tool traffic is not). Past the cap the
	// oldest rounds are dropped.
	MaxAgentConversationRounds = 100

	// AgentConversationRoundWarning is where the client starts warning. It is
	// deliberately below the cap so the member can open a new thread before
	// anything is actually deleted — silently dropping their earliest messages
	// with no warning is the behaviour to avoid.
	AgentConversationRoundWarning = 90

	// maxAgentStoredMessages is MaxAgentConversationRounds in rows.
	maxAgentStoredMessages = MaxAgentConversationRounds * 2

	// maxAgentHistoryMessages bounds one history page for the client. It
	// matches the storage cap, so a full thread loads in one page.
	maxAgentHistoryMessages = maxAgentStoredMessages

	// AgentConversationRetentionDays is how long an untouched thread is kept.
	// It is exported so the client can state the rule to the user rather than
	// repeating a number that might drift.
	AgentConversationRetentionDays = 90

	agentConversationRetention = AgentConversationRetentionDays * 24 * time.Hour

	// agentConversationSweepLimit bounds one retention sweep so the daily
	// maintenance transaction stays short.
	agentConversationSweepLimit = 500
)

var ErrAgentConversationFull = errors.New("service: member has too many agent conversations")

type AgentConversationService struct {
	repository *repository.Repository
}

func NewAgentConversationService(repo *repository.Repository) (*AgentConversationService, error) {
	if repo == nil {
		return nil, errors.New("agent conversation service repository is nil")
	}

	return &AgentConversationService{repository: repo}, nil
}

// AgentTurnMessage is one message to append. It mirrors the storage shape
// rather than any LLM wire format: translating to and from the provider's
// message structure is the agent package's job.
type AgentTurnMessage struct {
	Role       model.AgentMessageRole
	Content    string
	ToolCalls  []byte
	ToolCallID string
	ToolName   string
}

// Create opens an empty thread. The title stays blank until the first message
// is appended, which is what names it.
func (s *AgentConversationService) Create(ctx context.Context, actor AuthenticatedIdentity) (model.AgentConversation, error) {
	if err := requireAgentConversationActor(actor); err != nil {
		return model.AgentConversation{}, err
	}

	existing, err := s.repository.ListAgentConversations(ctx, repository.ListAgentConversationsParams{
		HouseholdID: actor.HouseholdID,
		MemberID:    actor.MemberID,
		Limit:       maxAgentConversationsPerMember + 1,
	})
	if err != nil {
		return model.AgentConversation{}, fmt.Errorf("list agent conversations: %w", err)
	}

	if len(existing) >= maxAgentConversationsPerMember {
		return model.AgentConversation{}, ErrAgentConversationFull
	}

	conversation, err := s.repository.CreateAgentConversation(ctx, repository.CreateAgentConversationParams{
		HouseholdID: actor.HouseholdID,
		MemberID:    actor.MemberID,
	})
	if err != nil {
		return model.AgentConversation{}, fmt.Errorf("create agent conversation: %w", err)
	}

	return conversation, nil
}

func (s *AgentConversationService) ListFor(ctx context.Context, actor AuthenticatedIdentity) ([]model.AgentConversation, error) {
	if err := requireAgentConversationActor(actor); err != nil {
		return nil, err
	}

	conversations, err := s.repository.ListAgentConversations(ctx, repository.ListAgentConversationsParams{
		HouseholdID: actor.HouseholdID,
		MemberID:    actor.MemberID,
		Limit:       maxAgentConversationsPerMember,
	})
	if err != nil {
		return nil, fmt.Errorf("list agent conversations: %w", err)
	}

	return conversations, nil
}

// Get loads one conversation the actor owns. A thread belonging to another
// member is reported as missing rather than forbidden, so conversation ids
// cannot be probed for existence.
func (s *AgentConversationService) Get(ctx context.Context, actor AuthenticatedIdentity, conversationID uint64) (model.AgentConversation, error) {
	if err := requireAgentConversationActor(actor); err != nil {
		return model.AgentConversation{}, err
	}

	conversation, err := s.repository.GetAgentConversationByID(ctx, conversationID, actor.HouseholdID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.AgentConversation{}, ErrNotFound
		}

		return model.AgentConversation{}, fmt.Errorf("get agent conversation: %w", err)
	}

	if conversation.MemberID != actor.MemberID {
		return model.AgentConversation{}, ErrNotFound
	}

	return conversation, nil
}

// ContextMessages returns the tail of the thread for replay into the model.
func (s *AgentConversationService) ContextMessages(ctx context.Context, actor AuthenticatedIdentity, conversationID uint64) ([]model.AgentMessage, error) {
	return s.messages(ctx, actor, conversationID, maxAgentContextMessages)
}

// HistoryMessages returns the thread for display in the client.
func (s *AgentConversationService) HistoryMessages(ctx context.Context, actor AuthenticatedIdentity, conversationID uint64) ([]model.AgentMessage, error) {
	return s.messages(ctx, actor, conversationID, maxAgentHistoryMessages)
}

func (s *AgentConversationService) messages(ctx context.Context, actor AuthenticatedIdentity, conversationID uint64, limit int) ([]model.AgentMessage, error) {
	// Ownership is checked before any message is read.
	if _, err := s.Get(ctx, actor, conversationID); err != nil {
		return nil, err
	}

	messages, err := s.repository.ListAgentMessages(ctx, repository.ListAgentMessagesParams{
		ConversationID: conversationID,
		Limit:          limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list agent messages: %w", err)
	}

	return messages, nil
}

// AppendTurn stores everything one turn produced and advances the
// conversation. The whole turn is written in a single transaction: a partial
// turn would leave an assistant tool_calls message without its results, which
// the provider rejects on the next replay.
func (s *AgentConversationService) AppendTurn(
	ctx context.Context,
	actor AuthenticatedIdentity,
	conversationID uint64,
	messages []AgentTurnMessage,
	title string,
) error {
	if err := requireAgentConversationActor(actor); err != nil {
		return err
	}

	if len(messages) == 0 {
		return nil
	}

	if _, err := s.Get(ctx, actor, conversationID); err != nil {
		return err
	}

	inputs := make([]repository.AgentMessageInput, 0, len(messages))
	for _, message := range messages {
		inputs = append(inputs, repository.AgentMessageInput{
			Role:       message.Role,
			Content:    message.Content,
			ToolCalls:  message.ToolCalls,
			ToolCallID: message.ToolCallID,
			ToolName:   message.ToolName,
		})
	}

	err := s.repository.WithinTransaction(ctx, func(tx *repository.Repository) error {
		return tx.AppendAgentMessages(ctx, repository.AppendAgentMessagesParams{
			ConversationID: conversationID,
			HouseholdID:    actor.HouseholdID,
			Messages:       inputs,
			Title:          repository.AgentConversationTitle(title, maxAgentConversationTitleRunes),
			KeepMessages:   maxAgentStoredMessages,
			At:             time.Now(),
		})
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}

		return fmt.Errorf("append agent messages: %w", err)
	}

	return nil
}

func (s *AgentConversationService) Delete(ctx context.Context, actor AuthenticatedIdentity, conversationID uint64) error {
	if err := requireAgentConversationActor(actor); err != nil {
		return err
	}

	err := s.repository.DeleteAgentConversation(ctx, repository.DeleteAgentConversationParams{
		ID:          conversationID,
		HouseholdID: actor.HouseholdID,
		MemberID:    actor.MemberID,
	})
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}

		return fmt.Errorf("delete agent conversation: %w", err)
	}

	return nil
}

// CleanupConversations drops threads nobody has touched for the retention
// window. It is called by the daily maintenance worker.
func (s *AgentConversationService) CleanupConversations(ctx context.Context, now time.Time) (int64, error) {
	deleted, err := s.repository.DeleteIdleAgentConversations(
		ctx,
		now.Add(-agentConversationRetention),
		agentConversationSweepLimit,
	)
	if err != nil {
		return 0, fmt.Errorf("cleanup agent conversations: %w", err)
	}

	return deleted, nil
}

func requireAgentConversationActor(actor AuthenticatedIdentity) error {
	if actor.MemberID == 0 || actor.HouseholdID == 0 {
		return ErrUnauthenticated
	}

	return nil
}
