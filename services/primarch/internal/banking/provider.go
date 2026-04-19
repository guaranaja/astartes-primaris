// Package banking handles live bank connectivity — Plaid today, Teller/SimpleFIN
// tomorrow. A Provider normalizes the vendor-specific auth + pull semantics into
// a small interface the rest of Primaris can use.
//
// Access tokens are never in memory longer than needed and are stored encrypted
// at rest via the tokencrypt package. Transactions pulled from a provider are
// pushed into Firefly III via the existing CreateTransaction flow, keeping
// Firefly as the single source of truth.
package banking

import (
	"context"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// NormalizedTxn is the provider-agnostic shape used for internal processing.
type NormalizedTxn struct {
	ProviderID       string    // vendor transaction ID (idempotency key)
	AccountID        string    // vendor account ID
	AccountName      string    // for pushing to Firefly by name
	Date             string    // YYYY-MM-DD
	Amount           float64   // signed; positive = credit/income, negative = debit/expense
	Currency         string
	Description      string
	MerchantName     string
	Category         string
	Pending          bool
	AuthorizedAt     time.Time // null if not available
	Removed          bool      // Plaid /transactions/sync can mark previously-fetched txns as removed
}

// Accounts is what comes back from a fresh link or a re-sync.
type Accounts struct {
	InstitutionID   string
	InstitutionName string
	Accounts        []domain.BankAccount
}

// SyncResult is returned from SyncTransactions.
type SyncResult struct {
	Added    []NormalizedTxn
	Modified []NormalizedTxn
	Removed  []NormalizedTxn
	NextCursor string
	// Whether there might be more pages (caller should loop until false).
	HasMore bool
}

// Provider abstracts bank-aggregation vendors so we can add Teller/SimpleFIN later.
type Provider interface {
	// Name returns the provider identifier ("plaid", "teller", etc).
	Name() string

	// Available reports whether the provider is configured (API keys present).
	Available() bool

	// CreateLinkToken returns an ephemeral token the frontend uses to open the
	// provider's Link/Connect flow.
	CreateLinkToken(ctx context.Context, userID string) (linkToken string, expiresAt time.Time, err error)

	// ExchangePublicToken trades a short-lived public_token (from successful Link)
	// for the long-lived access_token, plus the item_id and a first snapshot of
	// the accounts under it.
	ExchangePublicToken(ctx context.Context, publicToken string) (accessToken, itemID string, accts Accounts, err error)

	// SyncTransactions pulls delta-since-cursor from the provider. An empty cursor
	// means "give me everything available." Returns added/modified/removed txns
	// and the new cursor to persist.
	SyncTransactions(ctx context.Context, accessToken, cursor string) (SyncResult, error)

	// FetchAccounts refreshes the current account list + balances under an item.
	FetchAccounts(ctx context.Context, accessToken string) (Accounts, error)

	// Remove revokes the provider-side item (stops future access).
	Remove(ctx context.Context, accessToken string) error
}
