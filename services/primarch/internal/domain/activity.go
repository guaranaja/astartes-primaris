package domain

import (
	"encoding/json"
	"time"
)

// FFTransaction is a cached Firefly III transaction (for dashboard queries).
type FFTransaction struct {
	ID            string          `json:"id"`
	JournalID     string          `json:"journal_id,omitempty"`
	Type          string          `json:"type"` // deposit | withdrawal | transfer
	Date          string          `json:"date"` // YYYY-MM-DD
	Amount        float64         `json:"amount"`
	Currency      string          `json:"currency"`
	Description   string          `json:"description,omitempty"`
	Category      string          `json:"category,omitempty"`
	BudgetName    string          `json:"budget_name,omitempty"`
	BillID        string          `json:"bill_id,omitempty"`
	SourceAccount string          `json:"source_account,omitempty"`
	DestAccount   string          `json:"dest_account,omitempty"`
	Tags          []string        `json:"tags,omitempty"`
	Notes         string          `json:"notes,omitempty"`
	ExternalURL   string          `json:"external_url,omitempty"`
	Raw           json.RawMessage `json:"-"` // internal audit
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// MNTransaction is a cached Monarch Money transaction.
type MNTransaction struct {
	ID          string          `json:"id"`
	Date        string          `json:"date"`
	Amount      float64         `json:"amount"` // signed — negative = expense
	Merchant    string          `json:"merchant,omitempty"`
	Category    string          `json:"category,omitempty"`
	Account     string          `json:"account,omitempty"`
	Notes       string          `json:"notes,omitempty"`
	IsRecurring bool            `json:"is_recurring,omitempty"`
	Raw         json.RawMessage `json:"-"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// FinanceSyncState is the per-source health record for the ingest worker.
type FinanceSyncState struct {
	Source        string     `json:"source"`
	LastSyncedAt  *time.Time `json:"last_synced_at,omitempty"`
	LastOKAt      *time.Time `json:"last_ok_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	LastCount     int        `json:"last_count"`
	WindowDays    int        `json:"window_days"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ActivityFilter narrows a merged activity query.
type ActivityFilter struct {
	Sources  []string // subset of: firefly, monarch, trading
	Since    *time.Time
	Until    *time.Time
	Category string
	Tag      string
	BudgetName string
	Limit    int
}

// ActivityItem is a source-normalized row for the unified activity feed.
type ActivityItem struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"` // firefly | monarch | trading
	Kind        string    `json:"kind"`   // deposit | withdrawal | transfer | payout | prop_fee
	Date        string    `json:"date"`
	Timestamp   time.Time `json:"timestamp"`
	Amount      float64   `json:"amount"` // signed: positive = income, negative = expense/transfer-out
	Currency    string    `json:"currency"`
	Description string    `json:"description"`
	Category    string    `json:"category,omitempty"`
	Account     string    `json:"account,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	RefID       string    `json:"ref_id,omitempty"` // native ID in source system
}
