// Package ingest runs background sync from external finance sources (Firefly
// III + Monarch Money) into Postgres so dashboard queries are fast and work
// offline. Idempotent upserts keyed on the source system's native ID.
package ingest

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/cfo"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/store"
)

const (
	// DefaultInterval is how often the worker wakes up to check for new data.
	DefaultInterval = 15 * time.Minute

	// DefaultWindowDays is the trailing window we re-sync on every run.
	// 180 days keeps trend queries healthy without being wasteful.
	DefaultWindowDays = 180
)

// FinanceWorker periodically ingests Firefly III and Monarch Money data
// into the local Postgres cache. All writes are idempotent upserts.
type FinanceWorker struct {
	store   store.DataStore
	firefly *cfo.FireflyClient
	monarch *cfo.MonarchClient
	logger  *slog.Logger

	interval   time.Duration
	windowDays int

	mu       sync.Mutex
	stopFn   context.CancelFunc
	running  bool
	manualCh chan struct{}

	// bankingSync is an optional hook that runs before each Firefly/Monarch
	// sync. The banking service uses this to push new bank transactions into
	// Firefly so they're already written by the time we ingest from Firefly.
	bankingSync func(context.Context)
}

// NewFinanceWorker constructs a worker. firefly/monarch may be nil.
func NewFinanceWorker(st store.DataStore, ff *cfo.FireflyClient, mn *cfo.MonarchClient, logger *slog.Logger) *FinanceWorker {
	return &FinanceWorker{
		store:      st,
		firefly:    ff,
		monarch:    mn,
		logger:     logger,
		interval:   DefaultInterval,
		windowDays: DefaultWindowDays,
		manualCh:   make(chan struct{}, 1),
	}
}

// Start begins the periodic sync loop in a background goroutine.
// Runs one sync pass immediately, then ticks on interval. Cancel via Stop.
func (w *FinanceWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	subCtx, cancel := context.WithCancel(ctx)
	w.stopFn = cancel
	w.running = true
	w.mu.Unlock()

	go w.run(subCtx)
}

// Stop terminates the loop. Safe to call multiple times.
func (w *FinanceWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	w.stopFn()
	w.running = false
}

// TriggerNow kicks off an out-of-band sync. No-op if already running.
func (w *FinanceWorker) TriggerNow() {
	select {
	case w.manualCh <- struct{}{}:
	default:
	}
}

// SetBankingSync registers an optional hook that runs before the
// Firefly/Monarch ingest on each tick. Used by the banking service to
// push new bank transactions into Firefly first.
func (w *FinanceWorker) SetBankingSync(fn func(context.Context)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.bankingSync = fn
}

func (w *FinanceWorker) run(ctx context.Context) {
	w.syncAll(ctx)
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.syncAll(ctx)
		case <-w.manualCh:
			w.syncAll(ctx)
		}
	}
}

func (w *FinanceWorker) syncAll(ctx context.Context) {
	// Banking first — any new bank transactions get written to Firefly before
	// we read Firefly into the cache, so the activity feed is fully fresh.
	w.mu.Lock()
	bankingSync := w.bankingSync
	w.mu.Unlock()
	if bankingSync != nil {
		bankingSync(ctx)
	}
	if w.firefly != nil {
		w.syncFirefly(ctx)
	}
	if w.monarch != nil {
		w.syncMonarch(ctx)
	}
}

func (w *FinanceWorker) syncFirefly(ctx context.Context) {
	start := time.Now().AddDate(0, 0, -w.windowDays)
	end := time.Now()
	syncedAt := time.Now()

	state := &domain.FinanceSyncState{
		Source:       "firefly",
		LastSyncedAt: &syncedAt,
		WindowDays:   w.windowDays,
	}

	txns, err := w.firefly.ListTransactions(start, end)
	if err != nil {
		state.LastError = err.Error()
		w.logger.Warn("firefly sync failed", "error", err)
		_ = w.store.UpsertFinanceSyncState(state)
		return
	}

	count := 0
	for _, ft := range txns {
		raw, _ := json.Marshal(ft)
		cached := &domain.FFTransaction{
			ID:          ft.ID,
			Type:        ft.Type,
			Date:        normalizeDate(ft.Date),
			Amount:      ft.Amount,
			Currency:    defaultStr(ft.Currency, "USD"),
			Description: ft.Description,
			Category:    ft.Category,
			SourceAccount: ft.Source,
			DestAccount:   ft.Destination,
			Tags:        ft.Tags,
			Raw:         raw,
		}
		if err := w.store.UpsertFFTransaction(cached); err != nil {
			w.logger.Warn("upsert firefly txn", "id", ft.ID, "error", err)
			continue
		}
		count++
	}

	state.LastCount = count
	state.LastOKAt = &syncedAt
	_ = w.store.UpsertFinanceSyncState(state)
	w.logger.Info("firefly sync complete", "count", count, "window_days", w.windowDays)
}

func (w *FinanceWorker) syncMonarch(ctx context.Context) {
	start := time.Now().AddDate(0, 0, -w.windowDays)
	end := time.Now()
	syncedAt := time.Now()

	state := &domain.FinanceSyncState{
		Source:       "monarch",
		LastSyncedAt: &syncedAt,
		WindowDays:   w.windowDays,
	}

	txns, err := w.monarch.ListTransactions(start, end)
	if err != nil {
		state.LastError = err.Error()
		w.logger.Warn("monarch sync failed", "error", err)
		_ = w.store.UpsertFinanceSyncState(state)
		return
	}

	count := 0
	for _, mt := range txns {
		raw, _ := json.Marshal(mt)
		cached := &domain.MNTransaction{
			ID:          mt.ID,
			Date:        normalizeDate(mt.Date),
			Amount:      mt.Amount,
			Merchant:    mt.Merchant,
			Category:    mt.Category,
			Account:     mt.Account,
			Notes:       mt.Notes,
			IsRecurring: mt.IsRecurring,
			Raw:         raw,
		}
		if err := w.store.UpsertMNTransaction(cached); err != nil {
			w.logger.Warn("upsert monarch txn", "id", mt.ID, "error", err)
			continue
		}
		count++
	}

	state.LastCount = count
	state.LastOKAt = &syncedAt
	_ = w.store.UpsertFinanceSyncState(state)
	w.logger.Info("monarch sync complete", "count", count, "window_days", w.windowDays)
}

// normalizeDate accepts Firefly's ISO timestamp and returns YYYY-MM-DD.
func normalizeDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
