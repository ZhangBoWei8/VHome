package model

import "time"

// AgentMemoryScope decides who a remembered fact belongs to, and therefore
// whose system prompt it is injected into.
type AgentMemoryScope string

const (
	// AgentMemoryScopeHousehold is a fact about the household as a whole and
	// is visible to every member.
	AgentMemoryScopeHousehold AgentMemoryScope = "HOUSEHOLD"

	// AgentMemoryScopeMember is a fact about one member. It never reaches
	// another member's conversation.
	AgentMemoryScopeMember AgentMemoryScope = "MEMBER"
)

func (s AgentMemoryScope) Valid() bool {
	switch s {
	case AgentMemoryScopeHousehold, AgentMemoryScopeMember:
		return true
	default:
		return false
	}
}

// AgentMessageRole is the role of one stored conversation message. The system
// role is absent on purpose: it is rebuilt from live data every turn and never
// persisted.
type AgentMessageRole string

const (
	AgentMessageRoleUser      AgentMessageRole = "user"
	AgentMessageRoleAssistant AgentMessageRole = "assistant"
	AgentMessageRoleTool      AgentMessageRole = "tool"
)

// AgentConversation is one chat thread belonging to a single member.
type AgentConversation struct {
	ID            uint64    `json:"id"`
	HouseholdID   uint64    `json:"household_id"`
	MemberID      uint64    `json:"member_id"`
	Title         string    `json:"title"`
	LastMessageAt time.Time `json:"last_message_at"`
	MessageCount  uint32    `json:"message_count"`
	Version       uint64    `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AgentMessage is one stored turn. ToolCalls is set only on assistant messages
// that invoked tools; ToolCallID and ToolName only on tool results. An
// assistant message carrying ToolCalls must always be replayed together with
// every matching tool result, or the provider rejects the request.
type AgentMessage struct {
	ID             uint64           `json:"id"`
	ConversationID uint64           `json:"conversation_id"`
	HouseholdID    uint64           `json:"household_id"`
	Seq            uint32           `json:"seq"`
	Role           AgentMessageRole `json:"role"`
	Content        string           `json:"content"`
	ToolCalls      []byte           `json:"-"`
	ToolCallID     string           `json:"tool_call_id,omitempty"`
	ToolName       string           `json:"tool_name,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

// AgentMemory is one durable fact 8V has been asked to remember. MemberID is
// set exactly when Scope is AgentMemoryScopeMember; the database enforces the
// same invariant with a CHECK constraint.
type AgentMemory struct {
	ID                   uint64           `json:"id"`
	HouseholdID          uint64           `json:"household_id"`
	Scope                AgentMemoryScope `json:"scope"`
	MemberID             *uint64          `json:"member_id"`
	Content              string           `json:"content"`
	SourceConversationID *uint64          `json:"source_conversation_id"`
	CreatedBy            uint64           `json:"created_by"`
	Version              uint64           `json:"version"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
}
