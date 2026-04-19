package banking

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/cfo"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/store"
)

// Service glues a Provider, the store, and Firefly together.
// It owns the "pull from provider → push to Firefly" lifecycle.
type Service struct {
	provider Provider
	crypter  *TokenCrypter
	store    store.DataStore
	firefly  *cfo.FireflyClient
	logger   *slog.Logger
}

// NewService wires the banking service. Returns nil if either the provider
// or the crypter isn't configured — callers handle the nil gracefully.
func NewService(provider Provider, crypter *TokenCrypter, st store.DataStore, firefly *cfo.FireflyClient, logger *slog.Logger) *Service {
	if provider == nil || !provider.Available() || crypter == nil {
		return nil
	}
	return &Service{
		provider: provider,
		crypter:  crypter,
		store:    st,
		firefly:  firefly,
		logger:   logger,
	}
}

// Available reports whether banking is usable end-to-end.
func (s *Service) Available() bool {
	return s != nil && s.provider != nil && s.provider.Available() && s.crypter != nil
}

// ProviderName exposes the underlying provider id for status endpoints.
func (s *Service) ProviderName() string {
	if s == nil || s.provider == nil {
		return ""
	}
	return s.provider.Name()
}

// CreateLinkToken is a thin passthrough the API handler calls.
func (s *Service) CreateLinkToken(ctx context.Context, userID string) (string, time.Time, error) {
	if !s.Available() {
		return "", time.Time{}, fmt.Errorf("banking not configured")
	}
	return s.provider.CreateLinkToken(ctx, userID)
}

// ExchangeAndStore handles the onSuccess -> persist flow. After successful
// exchange we encrypt the access_token and write the BankConnection row.
// Returns the persisted connection (with access token stripped).
func (s *Service) ExchangeAndStore(ctx context.Context, publicToken string) (*domain.BankConnection, error) {
	if !s.Available() {
		return nil, fmt.Errorf("banking not configured")
	}
	accessToken, itemID, accts, err := s.provider.ExchangePublicToken(ctx, publicToken)
	if err != nil {
		return nil, fmt.Errorf("exchange public_token: %w", err)
	}
	ct, err := s.crypter.Encrypt(accessToken)
	if err != nil {
		return nil, fmt.Errorf("encrypt access token: %w", err)
	}
	conn := &domain.BankConnection{
		ID:              fmt.Sprintf("bc-%d", time.Now().UnixNano()),
		Provider:        s.provider.Name(),
		ProviderItemID:  itemID,
		InstitutionID:   accts.InstitutionID,
		InstitutionName: accts.InstitutionName,
		Status:          "active",
		AccessTokenCT:   ct,
		Accounts:        accts.Accounts,
	}
	if err := s.store.CreateBankConnection(conn); err != nil {
		// Best-effort revoke to avoid orphan token on provider side.
		_ = s.provider.Remove(ctx, accessToken)
		return nil, fmt.Errorf("persist connection: %w", err)
	}
	s.logger.Info("bank connection created", "id", conn.ID, "institution", conn.InstitutionName, "accounts", len(accts.Accounts))

	// Kick off a first sync in background so the UI feels snappy.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if _, err := s.SyncConnection(ctx, conn.ID); err != nil {
			s.logger.Warn("initial banking sync failed", "conn", conn.ID, "error", err)
		}
	}()

	// Strip ciphertext before returning to caller; handler marshals to client.
	safe := *conn
	safe.AccessTokenCT = ""
	safe.SyncCursor = ""
	return &safe, nil
}

// SyncConnection pulls /transactions/sync, pushes new/modified transactions to
// Firefly via CreateTransaction, and persists the new cursor.
func (s *Service) SyncConnection(ctx context.Context, connID string) (int, error) {
	if !s.Available() {
		return 0, fmt.Errorf("banking not configured")
	}
	conn, err := s.store.GetBankConnection(connID)
	if err != nil {
		return 0, err
	}
	if conn.Status == "revoked" {
		return 0, fmt.Errorf("connection is revoked")
	}
	accessToken, err := s.crypter.Decrypt(conn.AccessTokenCT)
	if err != nil {
		return 0, fmt.Errorf("decrypt token: %w", err)
	}

	// Refresh account balances first (cheap) so the UI reflects current state.
	if refreshed, err := s.provider.FetchAccounts(ctx, accessToken); err == nil {
		if len(refreshed.Accounts) > 0 {
			conn.Accounts = refreshed.Accounts
		}
		if refreshed.InstitutionName != "" && conn.InstitutionName == "Bank" {
			conn.InstitutionName = refreshed.InstitutionName
		}
	}

	// Build a quick provider-account-id → name map for pushing to Firefly.
	acctName := make(map[string]string, len(conn.Accounts))
	for _, a := range conn.Accounts {
		acctName[a.ID] = a.Name
	}

	total := 0
	cursor := conn.SyncCursor
	for {
		res, err := s.provider.SyncTransactions(ctx, accessToken, cursor)
		if err != nil {
			conn.Status = "error"
			conn.LastError = err.Error()
			now := time.Now()
			conn.LastSyncedAt = &now
			_ = s.store.UpdateBankConnection(conn)
			return total, err
		}
		for _, t := range res.Added {
			t.AccountName = acctName[t.AccountID]
			if err := s.pushToFirefly(t, conn); err != nil {
				s.logger.Warn("firefly push failed", "txn", t.ProviderID, "error", err)
				continue
			}
			total++
		}
		// For the skeleton we ignore modified/removed — Firefly upserts by our
		// own tagging convention; fidelity on corrections can come later.
		cursor = res.NextCursor
		if !res.HasMore {
			break
		}
	}

	now := time.Now()
	conn.SyncCursor = cursor
	conn.Status = "active"
	conn.LastError = ""
	conn.LastSyncedAt = &now
	conn.LastOKAt = &now
	conn.LastTxnCount = total
	if err := s.store.UpdateBankConnection(conn); err != nil {
		return total, err
	}
	s.logger.Info("bank sync complete", "conn", connID, "institution", conn.InstitutionName, "added", total)
	return total, nil
}

// SyncAll runs SyncConnection across every active connection. Called from the
// background finance worker on its cadence.
func (s *Service) SyncAll(ctx context.Context) {
	if !s.Available() {
		return
	}
	for _, c := range s.store.ListBankConnections() {
		if c.Status == "revoked" {
			continue
		}
		if _, err := s.SyncConnection(ctx, c.ID); err != nil {
			s.logger.Warn("banking sync_all: connection failed", "conn", c.ID, "error", err)
		}
	}
}

// RemoveConnection revokes the provider item and deletes the row.
func (s *Service) RemoveConnection(ctx context.Context, connID string) error {
	if !s.Available() {
		return fmt.Errorf("banking not configured")
	}
	conn, err := s.store.GetBankConnection(connID)
	if err != nil {
		return err
	}
	if conn.AccessTokenCT != "" {
		if accessToken, err := s.crypter.Decrypt(conn.AccessTokenCT); err == nil {
			if err := s.provider.Remove(ctx, accessToken); err != nil {
				s.logger.Warn("provider remove failed", "conn", connID, "error", err)
			}
		}
	}
	return s.store.DeleteBankConnection(connID)
}

// pushToFirefly creates a Firefly III transaction from a normalized bank txn.
// Keeps Firefly as the ledger of record; our activity cache pulls it back on
// the next finance ingest tick.
func (s *Service) pushToFirefly(t NormalizedTxn, conn *domain.BankConnection) error {
	if s.firefly == nil {
		return fmt.Errorf("firefly not configured")
	}
	txnType := "deposit"
	amt := t.Amount
	source := t.MerchantName
	if source == "" {
		source = t.Description
	}
	dest := t.AccountName
	if dest == "" {
		dest = conn.InstitutionName
	}
	if t.Amount < 0 {
		txnType = "withdrawal"
		amt = -t.Amount
		// For withdrawals: source = your account, destination = merchant.
		source, dest = dest, source
		if dest == "" {
			dest = "Expenses"
		}
	}

	desc := t.Description
	if desc == "" {
		desc = t.MerchantName
	}

	tags := []string{"plaid", "banking:" + conn.Provider, "inst:" + conn.InstitutionName}
	if t.Pending {
		tags = append(tags, "pending")
	}

	txn := cfo.FFTransactionStore{
		Type:            txnType,
		Description:     desc,
		Date:            t.Date,
		Amount:          fmt.Sprintf("%.2f", amt),
		SourceName:      source,
		DestinationName: dest,
		CategoryName:    t.Category,
		Tags:            tags,
	}
	return s.firefly.CreateTransaction(txn)
}
