package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"vhome/internal/model"
	"vhome/internal/repository"
)

const (
	maxAgentMemoryContentLength = 500
	minAgentMemoryContentLength = 2

	// Every memory is injected into the system prompt on every turn, so the
	// caps are a token budget as much as a storage limit.
	maxHouseholdAgentMemories = 50
	maxMemberAgentMemories    = 50

	// The prompt only ever needs what a single member can see: the household
	// rows plus their own.
	maxVisibleAgentMemories = maxHouseholdAgentMemories + maxMemberAgentMemories
)

var (
	ErrAgentMemoryFull = errors.New("service: agent memory bank is full")

	// ErrAgentMemoryScopeInvalid is returned when the model asks for a scope
	// that is neither HOUSEHOLD nor MEMBER.
	ErrAgentMemoryScopeInvalid = errors.New("service: agent memory scope is invalid")
)

type AgentMemoryService struct {
	repository *repository.Repository
}

func NewAgentMemoryService(repo *repository.Repository) (*AgentMemoryService, error) {
	if repo == nil {
		return nil, errors.New("agent memory service repository is nil")
	}

	return &AgentMemoryService{repository: repo}, nil
}

type RememberInput struct {
	Content string
	Scope   model.AgentMemoryScope

	// SourceConversationID records where the fact came from. It is optional:
	// Stage A has no conversations yet.
	SourceConversationID *uint64
}

// RememberResult tells the caller whether a new fact was stored or an existing
// one was refreshed, so the agent can say "记住了" versus "这条我已经记着了".
type RememberResult struct {
	Memory  model.AgentMemory
	Created bool
}

// Remember stores one durable fact. A member-scoped memory always belongs to
// the actor: the model cannot write into another member's memory bank, which
// keeps one member from planting facts in another's prompt.
func (s *AgentMemoryService) Remember(ctx context.Context, actor AuthenticatedIdentity, input RememberInput) (RememberResult, error) {
	if err := requireAgentMemoryActor(actor); err != nil {
		return RememberResult{}, err
	}

	content, err := normalizeAgentMemoryContent(input.Content)
	if err != nil {
		return RememberResult{}, err
	}

	if !input.Scope.Valid() {
		return RememberResult{}, ErrAgentMemoryScopeInvalid
	}

	var memberID *uint64
	if input.Scope == model.AgentMemoryScopeMember {
		owner := actor.MemberID
		memberID = &owner
	}

	// The cap is checked before the write. An upsert that only refreshes an
	// existing row is still allowed at the cap, because it adds nothing.
	count, err := s.repository.CountAgentMemories(ctx, repository.CountAgentMemoriesParams{
		HouseholdID: actor.HouseholdID,
		Scope:       input.Scope,
		MemberID:    actor.MemberID,
	})
	if err != nil {
		return RememberResult{}, fmt.Errorf("count agent memories: %w", err)
	}

	if count >= agentMemoryCap(input.Scope) {
		if _, exists, lookupErr := s.findByContent(ctx, actor, memberID, content); lookupErr != nil {
			return RememberResult{}, lookupErr
		} else if !exists {
			return RememberResult{}, ErrAgentMemoryFull
		}
	}

	memory, created, err := s.repository.CreateAgentMemory(ctx, repository.CreateAgentMemoryParams{
		HouseholdID:          actor.HouseholdID,
		Scope:                input.Scope,
		MemberID:             memberID,
		Content:              content,
		SourceConversationID: input.SourceConversationID,
		CreatedBy:            actor.MemberID,
	})
	if err != nil {
		return RememberResult{}, fmt.Errorf("create agent memory: %w", err)
	}

	return RememberResult{Memory: memory, Created: created}, nil
}

// Forget deletes one memory. A member-scoped row can only be removed by its
// owner; a household row may be removed by anyone, matching the fact that
// anyone can create one.
func (s *AgentMemoryService) Forget(ctx context.Context, actor AuthenticatedIdentity, memoryID uint64, version uint64) error {
	if err := requireAgentMemoryActor(actor); err != nil {
		return err
	}

	memory, err := s.repository.GetAgentMemoryByID(ctx, memoryID, actor.HouseholdID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}

		return fmt.Errorf("get agent memory: %w", err)
	}

	// Report a foreign member's private memory as missing rather than
	// forbidden: its existence is itself private.
	if memory.MemberID != nil && *memory.MemberID != actor.MemberID {
		return ErrNotFound
	}

	if err := s.repository.DeleteAgentMemory(ctx, repository.DeleteAgentMemoryParams{
		ID:          memoryID,
		HouseholdID: actor.HouseholdID,
		Version:     version,
	}); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return ErrConflict
		}

		return fmt.Errorf("delete agent memory: %w", err)
	}

	return nil
}

// ListFor returns everything visible to the actor: the household memories plus
// their own private ones.
func (s *AgentMemoryService) ListFor(ctx context.Context, actor AuthenticatedIdentity) ([]model.AgentMemory, error) {
	if err := requireAgentMemoryActor(actor); err != nil {
		return nil, err
	}

	memories, err := s.repository.ListAgentMemoriesForMember(ctx, repository.ListAgentMemoriesParams{
		HouseholdID: actor.HouseholdID,
		MemberID:    actor.MemberID,
		Limit:       maxVisibleAgentMemories,
	})
	if err != nil {
		return nil, fmt.Errorf("list agent memories: %w", err)
	}

	return memories, nil
}

func (s *AgentMemoryService) findByContent(
	ctx context.Context,
	actor AuthenticatedIdentity,
	memberID *uint64,
	content string,
) (model.AgentMemory, bool, error) {
	memories, err := s.repository.ListAgentMemoriesForMember(ctx, repository.ListAgentMemoriesParams{
		HouseholdID: actor.HouseholdID,
		MemberID:    actor.MemberID,
		Limit:       maxVisibleAgentMemories,
	})
	if err != nil {
		return model.AgentMemory{}, false, fmt.Errorf("list agent memories: %w", err)
	}

	wantOwner := uint64(0)
	if memberID != nil {
		wantOwner = *memberID
	}

	for _, memory := range memories {
		owner := uint64(0)
		if memory.MemberID != nil {
			owner = *memory.MemberID
		}

		if owner == wantOwner && memory.Content == content {
			return memory, true, nil
		}
	}

	return model.AgentMemory{}, false, nil
}

func agentMemoryCap(scope model.AgentMemoryScope) int {
	if scope == model.AgentMemoryScopeHousehold {
		return maxHouseholdAgentMemories
	}

	return maxMemberAgentMemories
}

func normalizeAgentMemoryContent(content string) (string, error) {
	// Newlines would break the one-fact-per-line layout of the prompt section.
	normalized := strings.Join(strings.Fields(content), " ")

	length := utf8.RuneCountInString(normalized)
	if length < minAgentMemoryContentLength || length > maxAgentMemoryContentLength {
		return "", ErrInvalidInput
	}

	return normalized, nil
}

func requireAgentMemoryActor(actor AuthenticatedIdentity) error {
	if actor.MemberID == 0 || actor.HouseholdID == 0 {
		return ErrUnauthenticated
	}

	return nil
}
