package banking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// Plaid environments.
const (
	PlaidEnvSandbox     = "sandbox"
	PlaidEnvDevelopment = "development"
	PlaidEnvProduction  = "production"
)

// plaidEnvURL maps env name → Plaid API base URL.
var plaidEnvURL = map[string]string{
	PlaidEnvSandbox:     "https://sandbox.plaid.com",
	PlaidEnvDevelopment: "https://development.plaid.com",
	PlaidEnvProduction:  "https://production.plaid.com",
}

// PlaidProvider implements Provider against Plaid's REST API using stdlib HTTP.
// No SDK dep — keeps dependency graph small and makes the call shapes explicit.
type PlaidProvider struct {
	clientID string
	secret   string
	baseURL  string
	env      string
	http     *http.Client
	logger   *slog.Logger
}

// NewPlaidProvider constructs a provider from env vars:
//   PLAID_CLIENT_ID   — Plaid dashboard client_id
//   PLAID_SECRET      — Plaid dashboard secret (scoped to the env below)
//   PLAID_ENV         — sandbox | development | production (default: sandbox)
// Returns nil (not an error) if any required var is missing — callers are
// expected to tolerate the provider being unconfigured.
func NewPlaidProvider(logger *slog.Logger) *PlaidProvider {
	clientID := os.Getenv("PLAID_CLIENT_ID")
	secret := os.Getenv("PLAID_SECRET")
	// Treat the "unset" sentinel (used for placeholder Secret Manager versions
	// pre-approval) the same as empty — banking simply stays disabled.
	if clientID == "" || secret == "" || clientID == "unset" || secret == "unset" {
		if logger != nil {
			logger.Warn("Plaid not configured (PLAID_CLIENT_ID/PLAID_SECRET missing or unset) — banking disabled")
		}
		return nil
	}
	env := os.Getenv("PLAID_ENV")
	if env == "" {
		env = PlaidEnvSandbox
	}
	base, ok := plaidEnvURL[env]
	if !ok {
		if logger != nil {
			logger.Warn("unknown PLAID_ENV, defaulting to sandbox", "env", env)
		}
		base = plaidEnvURL[PlaidEnvSandbox]
		env = PlaidEnvSandbox
	}
	return &PlaidProvider{
		clientID: clientID,
		secret:   secret,
		baseURL:  base,
		env:      env,
		http:     &http.Client{Timeout: 30 * time.Second},
		logger:   logger,
	}
}

func (p *PlaidProvider) Name() string    { return "plaid" }
func (p *PlaidProvider) Available() bool { return p != nil && p.clientID != "" && p.secret != "" }
func (p *PlaidProvider) Env() string     { return p.env }

// post is the generic JSON POST with auth baked in.
func (p *PlaidProvider) post(ctx context.Context, path string, body map[string]interface{}, out interface{}) error {
	if body == nil {
		body = map[string]interface{}{}
	}
	body["client_id"] = p.clientID
	body["secret"] = p.secret
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 400 {
		return fmt.Errorf("plaid %s returned %d: %s", path, res.StatusCode, truncate(string(respBody), 400))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
	}
	return nil
}

// ─── Link flow ──────────────────────────────────────────────

type linkTokenCreateResp struct {
	LinkToken  string `json:"link_token"`
	Expiration string `json:"expiration"` // ISO8601
	RequestID  string `json:"request_id"`
}

// CreateLinkToken asks Plaid for an ephemeral token the frontend's Plaid Link
// SDK uses to open the connection UI.
func (p *PlaidProvider) CreateLinkToken(ctx context.Context, userID string) (string, time.Time, error) {
	body := map[string]interface{}{
		"client_name":   "Astartes Primaris",
		"country_codes": []string{"US"},
		"language":      "en",
		"user": map[string]interface{}{
			"client_user_id": userID,
		},
		"products": []string{"transactions"},
	}
	var resp linkTokenCreateResp
	if err := p.post(ctx, "/link/token/create", body, &resp); err != nil {
		return "", time.Time{}, err
	}
	exp, _ := time.Parse(time.RFC3339, resp.Expiration)
	return resp.LinkToken, exp, nil
}

type itemPublicTokenExchangeResp struct {
	AccessToken string `json:"access_token"`
	ItemID      string `json:"item_id"`
	RequestID   string `json:"request_id"`
}

type plaidAccountsGetResp struct {
	Accounts []plaidAccount `json:"accounts"`
	Item     struct {
		ItemID        string `json:"item_id"`
		InstitutionID string `json:"institution_id"`
	} `json:"item"`
}

type plaidAccount struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	Mask      string `json:"mask"`
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	Balances  struct {
		Available       float64 `json:"available"`
		Current         float64 `json:"current"`
		ISOCurrencyCode string  `json:"iso_currency_code"`
	} `json:"balances"`
}

type plaidInstitutionGetByIDResp struct {
	Institution struct {
		InstitutionID string `json:"institution_id"`
		Name          string `json:"name"`
	} `json:"institution"`
}

// ExchangePublicToken trades the public_token (from Link onSuccess) for the
// long-lived access_token, then fetches accounts + institution metadata.
func (p *PlaidProvider) ExchangePublicToken(ctx context.Context, publicToken string) (string, string, Accounts, error) {
	var ex itemPublicTokenExchangeResp
	if err := p.post(ctx, "/item/public_token/exchange", map[string]interface{}{
		"public_token": publicToken,
	}, &ex); err != nil {
		return "", "", Accounts{}, err
	}
	accts, err := p.fetchAccountsAndInstitution(ctx, ex.AccessToken)
	if err != nil {
		// Token is valid; institution lookup failed. Return what we have.
		if p.logger != nil {
			p.logger.Warn("plaid institution lookup failed after exchange", "error", err)
		}
	}
	return ex.AccessToken, ex.ItemID, accts, nil
}

func (p *PlaidProvider) FetchAccounts(ctx context.Context, accessToken string) (Accounts, error) {
	return p.fetchAccountsAndInstitution(ctx, accessToken)
}

func (p *PlaidProvider) fetchAccountsAndInstitution(ctx context.Context, accessToken string) (Accounts, error) {
	var resp plaidAccountsGetResp
	if err := p.post(ctx, "/accounts/get", map[string]interface{}{
		"access_token": accessToken,
	}, &resp); err != nil {
		return Accounts{}, err
	}
	out := Accounts{
		InstitutionID: resp.Item.InstitutionID,
	}
	for _, a := range resp.Accounts {
		out.Accounts = append(out.Accounts, domain.BankAccount{
			ID:               a.AccountID,
			Name:             a.Name,
			Mask:             a.Mask,
			Type:             a.Type,
			Subtype:          a.Subtype,
			CurrentBalance:   a.Balances.Current,
			AvailableBalance: a.Balances.Available,
			Currency:         a.Balances.ISOCurrencyCode,
		})
	}
	if resp.Item.InstitutionID != "" {
		var inst plaidInstitutionGetByIDResp
		if err := p.post(ctx, "/institutions/get_by_id", map[string]interface{}{
			"institution_id": resp.Item.InstitutionID,
			"country_codes":  []string{"US"},
		}, &inst); err == nil {
			out.InstitutionName = inst.Institution.Name
		}
	}
	if out.InstitutionName == "" {
		out.InstitutionName = "Bank"
	}
	return out, nil
}

// ─── Transactions (delta sync) ──────────────────────────────

type plaidTxnSyncResp struct {
	Added    []plaidTxn `json:"added"`
	Modified []plaidTxn `json:"modified"`
	Removed  []struct {
		TransactionID string `json:"transaction_id"`
		AccountID     string `json:"account_id"`
	} `json:"removed"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

type plaidTxn struct {
	TransactionID    string    `json:"transaction_id"`
	AccountID        string    `json:"account_id"`
	Amount           float64   `json:"amount"` // Plaid convention: positive = debit
	ISOCurrencyCode  string    `json:"iso_currency_code"`
	Date             string    `json:"date"`
	AuthorizedDate   string    `json:"authorized_date"`
	Name             string    `json:"name"`
	MerchantName     string    `json:"merchant_name"`
	PersonalFinanceCategory struct {
		Primary string `json:"primary"`
		Detailed string `json:"detailed"`
	} `json:"personal_finance_category"`
	Pending bool `json:"pending"`
}

// SyncTransactions calls /transactions/sync — Plaid's delta endpoint.
// The cursor is opaque; empty means "first-time sync, start from the beginning."
func (p *PlaidProvider) SyncTransactions(ctx context.Context, accessToken, cursor string) (SyncResult, error) {
	body := map[string]interface{}{
		"access_token": accessToken,
	}
	if cursor != "" {
		body["cursor"] = cursor
	}
	var resp plaidTxnSyncResp
	if err := p.post(ctx, "/transactions/sync", body, &resp); err != nil {
		return SyncResult{}, err
	}
	out := SyncResult{
		NextCursor: resp.NextCursor,
		HasMore:    resp.HasMore,
	}
	for _, t := range resp.Added {
		out.Added = append(out.Added, p.normalizeTxn(t))
	}
	for _, t := range resp.Modified {
		out.Modified = append(out.Modified, p.normalizeTxn(t))
	}
	for _, r := range resp.Removed {
		out.Removed = append(out.Removed, NormalizedTxn{
			ProviderID: r.TransactionID,
			AccountID:  r.AccountID,
			Removed:    true,
		})
	}
	return out, nil
}

// normalizeTxn converts Plaid's convention (positive = debit) to ours
// (positive = credit/income, negative = debit/expense).
func (p *PlaidProvider) normalizeTxn(t plaidTxn) NormalizedTxn {
	amt := -t.Amount
	cat := t.PersonalFinanceCategory.Primary
	if cat == "" {
		cat = t.PersonalFinanceCategory.Detailed
	}
	authorized, _ := time.Parse("2006-01-02", t.AuthorizedDate)
	return NormalizedTxn{
		ProviderID:   t.TransactionID,
		AccountID:    t.AccountID,
		Date:         t.Date,
		Amount:       amt,
		Currency:     t.ISOCurrencyCode,
		Description:  t.Name,
		MerchantName: t.MerchantName,
		Category:     cat,
		Pending:      t.Pending,
		AuthorizedAt: authorized,
	}
}

// ─── Item removal ───────────────────────────────────────────

func (p *PlaidProvider) Remove(ctx context.Context, accessToken string) error {
	return p.post(ctx, "/item/remove", map[string]interface{}{
		"access_token": accessToken,
	}, nil)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
