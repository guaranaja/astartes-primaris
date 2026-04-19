package domain

import (
	"encoding/json"
	"time"
)

// AdvisorThread is a durable Claude conversation scoped to a topic.
type AdvisorThread struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Topic           string          `json:"topic"`              // "chat" | "milestone" | "playbook"
	PlaybookKey     string          `json:"playbook_key,omitempty"`
	Status          string          `json:"status"`             // "active" | "archived"
	ContextSnapshot json.RawMessage `json:"context_snapshot,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	LastMessageAt   *time.Time      `json:"last_message_at,omitempty"`
	Messages        []AdvisorMessage `json:"messages,omitempty"` // populated on detail fetch
}

// AdvisorMessage is a single turn in an AdvisorThread.
type AdvisorMessage struct {
	ID        string          `json:"id"`
	ThreadID  string          `json:"thread_id"`
	Role      string          `json:"role"` // "user" | "assistant" | "system"
	Content   string          `json:"content"`
	ToolCalls json.RawMessage `json:"tool_calls,omitempty"`
	Model     string          `json:"model,omitempty"`
	TokensIn  int             `json:"tokens_in,omitempty"`
	TokensOut int             `json:"tokens_out,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// Playbook keys — canonical set the advisor knows how to lead.
const (
	PlaybookLLCStructure = "llc_structure"
	PlaybookAccountArch  = "account_arch"
	PlaybookDebtOrder    = "debt_order"
	PlaybookHardware     = "hardware"
)
