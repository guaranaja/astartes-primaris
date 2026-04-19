package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// registerActivityRoutes wires the unified activity feed endpoints.
func (s *Server) registerActivityRoutes() {
	s.mux.HandleFunc("GET /api/v1/council/activity", s.handleListActivity)
	s.mux.HandleFunc("GET /api/v1/council/activity/sync-state", s.handleSyncState)
	s.mux.HandleFunc("POST /api/v1/council/activity/sync", s.handleTriggerSync)
}

// handleListActivity returns a merged, time-sorted activity stream across
// Firefly transactions, Monarch transactions, and Primarch trading payouts.
//
// Query params:
//   - since=YYYY-MM-DD   (default: 30 days ago)
//   - until=YYYY-MM-DD   (default: now)
//   - source=firefly|monarch|trading  (repeatable; default: all)
//   - category=<name>    (filter Firefly/Monarch by category)
//   - tag=<name>         (filter Firefly by tag)
//   - budget=<name>      (filter Firefly by budget)
//   - limit=<int>        (default 200, max 1000)
func (s *Server) handleListActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	since := parseDateDefault(q.Get("since"), time.Now().AddDate(0, 0, -30))
	until := parseDateDefault(q.Get("until"), time.Now())
	limit := parseIntDefault(q.Get("limit"), 200)
	if limit > 1000 {
		limit = 1000
	}

	requestedSources := map[string]bool{}
	for _, src := range q["source"] {
		for _, part := range strings.Split(src, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				requestedSources[part] = true
			}
		}
	}
	allSources := len(requestedSources) == 0
	want := func(s string) bool { return allSources || requestedSources[s] }

	filter := domain.ActivityFilter{
		Since:      &since,
		Until:      &until,
		Category:   q.Get("category"),
		Tag:        q.Get("tag"),
		BudgetName: q.Get("budget"),
		Limit:      limit,
	}

	var items []domain.ActivityItem

	if want("firefly") {
		for _, t := range s.store.QueryFFTransactions(filter) {
			items = append(items, fireflyToActivity(t))
		}
	}
	if want("monarch") {
		for _, t := range s.store.QueryMNTransactions(filter) {
			items = append(items, monarchToActivity(t))
		}
	}
	if want("trading") && filter.Category == "" && filter.Tag == "" && filter.BudgetName == "" {
		// Trading payouts: surface as positive amounts.
		for _, p := range s.store.ListPayouts("") {
			if p.RequestedAt.Before(since) || p.RequestedAt.After(until.Add(24*time.Hour)) {
				continue
			}
			items = append(items, domain.ActivityItem{
				ID:          "payout-" + p.ID,
				Source:      "trading",
				Kind:        "payout",
				Date:        p.RequestedAt.Format("2006-01-02"),
				Timestamp:   p.RequestedAt,
				Amount:      p.NetAmount,
				Currency:    "USD",
				Description: "Trading payout → " + p.Destination,
				Account:     p.AccountID,
				RefID:       p.ID,
			})
		}
		// Prop fees: outflows tagged prop-fee.
		for _, f := range s.store.ListPropFees("") {
			dt, err := time.Parse("2006-01-02", f.PaidDate)
			if err != nil {
				continue
			}
			if dt.Before(since) || dt.After(until.Add(24*time.Hour)) {
				continue
			}
			items = append(items, domain.ActivityItem{
				ID:          "fee-" + f.ID,
				Source:      "trading",
				Kind:        "prop_fee",
				Date:        f.PaidDate,
				Timestamp:   dt,
				Amount:      -f.Amount,
				Currency:    "USD",
				Description: "Prop fee (" + f.FeeType + "): " + f.PropFirm,
				Tags:        []string{"prop-fee"},
				RefID:       f.ID,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp.After(items[j].Timestamp)
	})
	if len(items) > limit {
		items = items[:limit]
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"since": since.Format("2006-01-02"),
		"until": until.Format("2006-01-02"),
		"count": len(items),
		"items": items,
	})
}

func (s *Server) handleSyncState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListFinanceSyncState())
}

func (s *Server) handleTriggerSync(w http.ResponseWriter, r *http.Request) {
	if s.financeWorker == nil {
		writeError(w, http.StatusServiceUnavailable, "finance worker not configured")
		return
	}
	s.financeWorker.TriggerNow()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sync triggered"})
}

// ─── Mappers ────────────────────────────────────────────────

func fireflyToActivity(t domain.FFTransaction) domain.ActivityItem {
	// Firefly amounts are positive; sign depends on txn type.
	amt := t.Amount
	switch t.Type {
	case "withdrawal":
		amt = -amt
	case "deposit":
		// keep positive
	case "transfer":
		// neutral — show as-is (positive). UI can render differently by kind.
	}
	desc := t.Description
	if desc == "" && t.Category != "" {
		desc = t.Category
	}
	account := t.SourceAccount
	if t.Type == "deposit" && t.DestAccount != "" {
		account = t.DestAccount
	}
	ts, _ := time.Parse("2006-01-02", t.Date)
	return domain.ActivityItem{
		ID:          "ff-" + t.ID,
		Source:      "firefly",
		Kind:        t.Type,
		Date:        t.Date,
		Timestamp:   ts,
		Amount:      amt,
		Currency:    t.Currency,
		Description: desc,
		Category:    t.Category,
		Account:     account,
		Tags:        t.Tags,
		RefID:       t.ID,
	}
}

func monarchToActivity(t domain.MNTransaction) domain.ActivityItem {
	// Monarch amounts are already signed.
	desc := t.Merchant
	if desc == "" {
		desc = t.Category
	}
	ts, _ := time.Parse("2006-01-02", t.Date)
	kind := "withdrawal"
	if t.Amount > 0 {
		kind = "deposit"
	}
	return domain.ActivityItem{
		ID:          "mn-" + t.ID,
		Source:      "monarch",
		Kind:        kind,
		Date:        t.Date,
		Timestamp:   ts,
		Amount:      t.Amount,
		Currency:    "USD",
		Description: desc,
		Category:    t.Category,
		Account:     t.Account,
		RefID:       t.ID,
	}
}

func parseDateDefault(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fallback
	}
	return t
}

func parseIntDefault(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}
