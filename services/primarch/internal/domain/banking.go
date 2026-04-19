package domain

import (
	"encoding/json"
	"time"
)

// BankConnection is a linked financial institution via an aggregator (Plaid/Teller/etc).
// The access token is stored encrypted in AccessTokenCT; never exposed over the API.
type BankConnection struct {
	ID                string          `json:"id"`
	Provider          string          `json:"provider"`           // plaid | teller | simplefin
	ProviderItemID    string          `json:"provider_item_id,omitempty"`
	InstitutionID     string          `json:"institution_id,omitempty"`
	InstitutionName   string          `json:"institution_name"`
	Status            string          `json:"status"`             // active | error | revoked
	LastError         string          `json:"last_error,omitempty"`
	SyncCursor        string          `json:"-"`                  // never expose
	AccessTokenCT     string          `json:"-"`                  // never expose
	Accounts          []BankAccount   `json:"accounts"`
	FireflyAccountMap json.RawMessage `json:"firefly_account_map,omitempty"`
	LastSyncedAt      *time.Time      `json:"last_synced_at,omitempty"`
	LastOKAt          *time.Time      `json:"last_ok_at,omitempty"`
	LastTxnCount      int             `json:"last_txn_count"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// BankAccount is a single account under a BankConnection (checking, savings, credit).
type BankAccount struct {
	ID              string  `json:"id"`              // provider account ID
	Name            string  `json:"name"`
	Mask            string  `json:"mask,omitempty"`  // last-4 digits
	Type            string  `json:"type"`            // depository | credit | loan | investment
	Subtype         string  `json:"subtype,omitempty"`
	CurrentBalance  float64 `json:"current_balance"`
	AvailableBalance float64 `json:"available_balance,omitempty"`
	Currency        string  `json:"currency,omitempty"`
}
