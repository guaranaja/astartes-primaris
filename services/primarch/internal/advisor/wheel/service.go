package wheel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/advisor"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/brokers/tastytrade"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/cfo"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/store"
)

// Service is the wheel advisor: scans chains on a cadence, persists
// recommendations, optionally runs a Claude review, and syncs tastytrade
// account balance to Firefly.
type Service struct {
	store    store.DataStore
	tasty    *tastytrade.Client
	claude   *advisor.Client
	firefly  *cfo.FireflyClient
	logger   *slog.Logger

	mu           sync.Mutex
	stopFn       context.CancelFunc
	running      bool
	triggerCh    chan struct{}
	accountCache []string
	// verdictCache memoizes Claude reviews keyed by
	// symbol|action|strike|expiration. Scans repeat every hour during market
	// hours — without caching we'd spend tokens re-reviewing the same contracts.
	verdictCache map[string]cachedVerdict
	// lastPrimeRun tracks the date (YYYY-MM-DD in ET) of the most recent prime
	// scan so we don't re-fire if the 1-minute detection window overlaps a
	// restart or multiple checks.
	lastPrimeRun string
}

type cachedVerdict struct {
	verdict string
	reasons []string
	expires time.Time
}

// verdictCacheTTL — how long a Claude review stays warm for an unchanged
// candidate. Short enough to re-evaluate across trading sessions, long enough
// to absorb repeated intra-day scans.
const verdictCacheTTL = 6 * time.Hour

// NewService wires the wheel advisor. claude and firefly may be nil.
func NewService(st store.DataStore, t *tastytrade.Client, cl *advisor.Client, ff *cfo.FireflyClient, logger *slog.Logger) *Service {
	if t == nil || !t.Available() {
		return nil
	}
	return &Service{
		store:        st,
		tasty:        t,
		claude:       cl,
		firefly:      ff,
		logger:       logger,
		triggerCh:    make(chan struct{}, 1),
		verdictCache: make(map[string]cachedVerdict),
	}
}

// Available reports whether the service is usable.
func (s *Service) Available() bool {
	return s != nil && s.tasty != nil && s.tasty.Available()
}

// Start begins the scheduled scan loop. Runs once immediately, then ticks on
// an hourly cadence during U.S. equity market hours.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	subCtx, cancel := context.WithCancel(ctx)
	s.stopFn = cancel
	s.running = true
	s.mu.Unlock()
	go s.runLoop(subCtx)
}

// Stop is safe to call multiple times.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.stopFn()
	s.running = false
}

// TriggerNow forces an out-of-band scan.
func (s *Service) TriggerNow() {
	select {
	case s.triggerCh <- struct{}{}:
	default:
	}
}

func (s *Service) runLoop(ctx context.Context) {
	s.RunOnce(ctx)
	t := time.NewTicker(1 * time.Hour)
	defer t.Stop()
	expire := time.NewTicker(6 * time.Hour)
	defer expire.Stop()
	// Poll every minute for the weekly prime-scan window. A ticker is plenty
	// cheap (one check, no work outside the window), and it self-heals across
	// restarts since it resumes from whatever the wall clock says.
	prime := time.NewTicker(1 * time.Minute)
	defer prime.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if inMarketHours(time.Now()) {
				s.RunOnce(ctx)
			}
		case <-expire.C:
			// Age out fresh recs older than 2 days.
			if n, err := s.store.ExpireOldWheelRecommendations(time.Now().Add(-48 * time.Hour)); err == nil && n > 0 {
				s.logger.Info("wheel recs expired", "count", n)
			}
		case <-prime.C:
			s.maybeRunPrime(ctx, time.Now())
		case <-s.triggerCh:
			s.RunOnce(ctx)
		}
	}
}

// maybeRunPrime fires the weekly Friday-11:00-ET canonical scan exactly once
// per Friday. It writes a distinct log prefix so downstream alerting (e.g. a
// future Discord notifier) can match on "wheel PRIME scan".
func (s *Service) maybeRunPrime(ctx context.Context, now time.Time) {
	if !isPrimeScanTime(now) {
		return
	}
	today := now.In(primeScanTZ()).Format("2006-01-02")
	s.mu.Lock()
	if s.lastPrimeRun == today {
		s.mu.Unlock()
		return
	}
	s.lastPrimeRun = today
	s.mu.Unlock()
	s.logger.Info("wheel PRIME scan starting", "local_time", now.In(primeScanTZ()).Format("Mon 15:04 MST"))
	s.RunOnce(ctx)
	s.logger.Info("wheel PRIME scan complete", "local_time", now.In(primeScanTZ()).Format("Mon 15:04 MST"))
}

// RunOnce does a full scan: gather inputs, generate candidates, optional
// Claude review, persist, and sync the tastytrade balance to Firefly.
func (s *Service) RunOnce(ctx context.Context) {
	if !s.Available() {
		return
	}
	cfg, err := s.store.GetWheelConfig()
	if err != nil {
		s.logger.Warn("wheel config missing", "error", err)
		return
	}

	// Sync tastytrade equity positions into Arsenal holdings before the scan
	// so CC candidates reflect the real portfolio. Manual rows are never touched.
	if err := s.syncPositionsToHoldings(ctx); err != nil {
		s.logger.Warn("tastytrade position sync failed", "error", err)
	}

	watchlist := s.store.ListWheelWatchlist()
	holdings := s.store.ListHoldings()

	runID := fmt.Sprintf("wr-%d", time.Now().UnixNano())
	scanCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	// Pull current positions and buying power so the engine can dedup opens,
	// generate close/roll recs for existing legs, and respect cash limits.
	s.mu.Lock()
	accts := s.accountCache
	s.mu.Unlock()
	if len(accts) == 0 {
		if found, err := s.tasty.ListAccountNumbers(scanCtx); err == nil {
			s.mu.Lock()
			s.accountCache = found
			accts = found
			s.mu.Unlock()
		}
	}
	var positions []tastytrade.Position
	var buyingPower float64
	primaryAcct := ""
	if len(accts) > 0 {
		primaryAcct = accts[0]
		if p, err := s.tasty.ListPositions(scanCtx, primaryAcct); err == nil {
			positions = p
		} else {
			s.logger.Warn("positions fetch failed", "error", err)
		}
		if bal, err := s.tasty.GetAccountBalance(scanCtx, primaryAcct); err == nil {
			// Use derivative buying power; for CSPs it's the right cap.
			buyingPower = bal.BuyingPower
		} else {
			s.logger.Warn("balance fetch failed", "error", err)
		}
	}

	cands, err := GenerateCandidates(scanCtx, Inputs{
		Config:      cfg,
		Watchlist:   watchlist,
		Holdings:    holdings,
		Tasty:       s.tasty,
		AccountNum:  primaryAcct,
		BuyingPower: buyingPower,
		Positions:   positions,
		Logger:      s.logger,
	})
	if err != nil {
		s.logger.Warn("wheel generate failed", "error", err)
		return
	}

	// Optional Claude review on the top-N candidates.
	if cfg.ClaudeReview && s.claude != nil && s.claude.Available() {
		s.reviewWithClaude(scanCtx, cands, cfg)
	}

	recs := ToRecommendations(runID, cands)
	if err := s.store.InsertWheelRecommendations(recs); err != nil {
		s.logger.Warn("wheel recs persist failed", "error", err)
	} else {
		s.logger.Info("wheel scan complete", "run", runID, "candidates", len(recs))
	}

	// Sync tastytrade balance to Firefly as a "Tastytrade Brokerage" virtual
	// asset account — keeps Firefly as the ledger of record for net worth.
	s.syncBalanceToFirefly(scanCtx)
}

// reviewWithClaude sends un-vetoed candidates to Claude for a structured
// verdict + reasons. Vetoed candidates are skipped — their deterministic
// verdict already says "skip" and we don't want to burn tokens confirming it.
// Results are merged back onto the candidate (Verdict, VerdictReasons,
// ReviewNote) and cached by (symbol, action, strike, expiration) for the TTL.
func (s *Service) reviewWithClaude(ctx context.Context, cands []Candidate, cfg *domain.WheelConfig) {
	if len(cands) == 0 {
		return
	}
	// Split candidates into reviewable (no vetoes, no managed-position actions)
	// and skipped — we'll pull cached verdicts for the skipped group too so
	// close/roll recs still carry a reviewer-annotated reason if one exists.
	reviewable := make([]int, 0, len(cands))
	for i := range cands {
		if len(cands[i].Vetoes) > 0 {
			continue // deterministic skip — keep engine verdict
		}
		if cands[i].Action == domain.WheelActionCSPClose || cands[i].Action == domain.WheelActionCCClose ||
			cands[i].Action == domain.WheelActionCSPRoll || cands[i].Action == domain.WheelActionCCRoll {
			continue // management actions don't need Claude's opinion
		}
		reviewable = append(reviewable, i)
	}

	// Serve from cache where possible. We only call Claude for cache misses.
	now := time.Now()
	var uncached []int
	for _, i := range reviewable {
		key := verdictKey(&cands[i])
		s.mu.Lock()
		cv, ok := s.verdictCache[key]
		s.mu.Unlock()
		if ok && now.Before(cv.expires) {
			cands[i].Verdict = cv.verdict
			cands[i].VerdictReasons = append([]string(nil), cv.reasons...)
			continue
		}
		uncached = append(uncached, i)
	}
	if len(uncached) == 0 {
		return
	}
	// Cap per-scan review batch so token usage stays bounded even with a wide
	// watchlist. Remaining candidates keep their seeded (rules-based) verdict.
	const maxReview = 15
	if len(uncached) > maxReview {
		uncached = uncached[:maxReview]
	}

	// Build a prompt asking for strict JSON so parsing is unambiguous.
	var b strings.Builder
	b.WriteString("You are an options-wheel tactical reviewer. For EACH numbered candidate below, decide whether to TAKE, WAIT, or SKIP the trade right now and list 2–3 concrete reasons. Reasons should be short bullet-style phrases (e.g. \"IV rank elevated at 78\", \"earnings in 3 days — wait\", \"delta cleanly matches 0.20 target\"). Consider: earnings proximity, IV rank vs. history, delta fit, annualized yield, spread/liquidity, macro backdrop.\n\nRespond with ONLY a JSON array — no prose, no markdown fences. Each element: {\"index\": <int>, \"verdict\": \"take|wait|skip\", \"reasons\": [\"...\", \"...\"]}.\n\nCandidates:\n")
	for _, i := range uncached {
		c := cands[i]
		fmt.Fprintf(&b, "%d. %s %s strike $%.2f, %d DTE (exp %s), Δ %.2f (target %.2f), yield %.1f%% annualized, IV rank %.0f, spread %.0f%%, OI %d, data %s.\n",
			i+1, c.Symbol, actionLabel(c.Action), c.Strike, c.DTE, c.Expiration,
			c.Delta, c.TargetDelta, c.AnnualizedYield*100, c.IVRank*100, c.SpreadPct*100, c.OpenInterest, c.DataQuality)
	}
	reply, err := s.claude.Complete(ctx,
		"You are a concise, opinionated options trader. Output STRICT JSON only — no prose.",
		[]advisor.Message{{Role: "user", Content: b.String()}})
	if err != nil {
		s.logger.Warn("claude review failed", "error", err)
		return
	}
	s.applyVerdicts(reply.Content, cands, uncached)
}

// applyVerdicts is the Service-bound method-expression wrapper around
// parseAndApplyVerdicts that holds the cache mutex for each cache write.
func (s *Service) applyVerdicts(reply string, cands []Candidate, uncached []int) {
	parseAndApplyVerdicts(reply, cands, uncached, s.logger, &s.mu, s.verdictCache)
}

// verdictKey is the cache key for a candidate's Claude verdict.
func verdictKey(c *Candidate) string {
	return fmt.Sprintf("%s|%s|%.2f|%s", c.Symbol, c.Action, c.Strike, c.Expiration)
}

// parseAndApplyVerdicts extracts a JSON verdict array from a Claude reply and
// merges each entry into the matching candidate. Tolerates minor wrapping
// (surrounding prose, ```json fences) by extracting the first bracketed array.
func parseAndApplyVerdicts(reply string, cands []Candidate, uncached []int, logger interface {
	Warn(msg string, args ...any)
}, mu *sync.Mutex, cache map[string]cachedVerdict) {
	jsonBlob := extractJSONArray(reply)
	if jsonBlob == "" {
		if logger != nil {
			logger.Warn("claude review: no JSON array in reply", "preview", truncateReply(reply, 200))
		}
		return
	}
	type vitem struct {
		Index   int      `json:"index"`
		Verdict string   `json:"verdict"`
		Reasons []string `json:"reasons"`
	}
	var items []vitem
	if err := json.Unmarshal([]byte(jsonBlob), &items); err != nil {
		if logger != nil {
			logger.Warn("claude review: JSON decode failed", "error", err, "blob", truncateReply(jsonBlob, 200))
		}
		return
	}
	expires := time.Now().Add(verdictCacheTTL)
	for _, it := range items {
		// Map the 1-based index in the reply back to the original candidate
		// slot. We only sent candidates at `uncached` indices to Claude.
		slot := it.Index - 1
		if slot < 0 || slot >= len(cands) {
			continue
		}
		valid := false
		for _, u := range uncached {
			if u == slot {
				valid = true
				break
			}
		}
		if !valid {
			continue
		}
		v := strings.ToLower(strings.TrimSpace(it.Verdict))
		switch v {
		case domain.WheelVerdictTake, domain.WheelVerdictWait, domain.WheelVerdictSkip:
			// ok
		default:
			continue
		}
		cands[slot].Verdict = v
		cands[slot].VerdictReasons = it.Reasons
		// Append a short reviewer note to the rationale for audit trails.
		if len(it.Reasons) > 0 {
			cands[slot].Rationale = cands[slot].Rationale + "\n\nReviewer: " +
				strings.ToUpper(v) + " — " + strings.Join(it.Reasons, "; ")
		}
		mu.Lock()
		cache[verdictKey(&cands[slot])] = cachedVerdict{
			verdict: v,
			reasons: append([]string(nil), it.Reasons...),
			expires: expires,
		}
		mu.Unlock()
	}
}

// extractJSONArray grabs the first "[...]" block from text, so we can tolerate
// ```json fences, explanatory prose, or trailing commentary.
func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func truncateReply(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// syncPositionsToHoldings pulls tastytrade equity positions and upserts them
// into Arsenal holdings keyed on (source=tastytrade, symbol). Option positions
// are ignored. Stale synced rows (positions no longer held) are deleted.
// Manual holdings are never touched.
func (s *Service) syncPositionsToHoldings(ctx context.Context) error {
	s.mu.Lock()
	accts := s.accountCache
	s.mu.Unlock()
	if len(accts) == 0 {
		found, err := s.tasty.ListAccountNumbers(ctx)
		if err != nil {
			return fmt.Errorf("list accounts: %w", err)
		}
		s.mu.Lock()
		s.accountCache = found
		accts = found
		s.mu.Unlock()
	}

	// Build one aggregate view across accounts: sum quantity per symbol,
	// weighted-average cost. If the trader has the same stock in two
	// tastytrade accounts, we roll them up (the advisor cares about the
	// lot size, not which account holds it).
	type agg struct {
		qty   float64
		cost  float64 // total cost across accounts
	}
	positions := map[string]*agg{}
	var sourceID string
	for _, accountNumber := range accts {
		pos, err := s.tasty.ListPositions(ctx, accountNumber)
		if err != nil {
			s.logger.Warn("tastytrade positions failed", "account", accountNumber, "error", err)
			continue
		}
		for _, p := range pos {
			// Only equity positions (skip options, futures).
			if p.InstrumentType != "Equity" {
				continue
			}
			// Skip zero / short positions — Arsenal is for long stock.
			if p.QuantityDirection != "Long" || p.Quantity <= 0 {
				continue
			}
			sym := p.UnderlyingSymbol
			if sym == "" {
				sym = p.Symbol
			}
			a, ok := positions[sym]
			if !ok {
				a = &agg{}
				positions[sym] = a
			}
			a.qty += p.Quantity
			a.cost += p.Quantity * p.AverageOpenPrice
		}
		sourceID = accountNumber // tracked in source_id for audit; last wins if multi-account
	}

	syncedSymbols := make([]string, 0, len(positions))
	for sym, a := range positions {
		avgCost := 0.0
		if a.qty > 0 {
			avgCost = a.cost / a.qty
		}
		h := &domain.Holding{
			Symbol:   sym,
			Quantity: a.qty,
			AvgCost:  avgCost,
			Source:   "tastytrade",
			SourceID: sourceID,
			Notes:    "Auto-synced from tastytrade",
		}
		if err := s.store.UpsertHoldingBySource(h); err != nil {
			s.logger.Warn("upsert holding failed", "symbol", sym, "error", err)
			continue
		}
		syncedSymbols = append(syncedSymbols, sym)
	}

	// Prune synced rows that no longer have a position.
	if removed, err := s.store.DeleteHoldingsBySourceExcept("tastytrade", syncedSymbols); err == nil && removed > 0 {
		s.logger.Info("tastytrade holdings pruned", "removed", removed)
	}

	s.logger.Info("tastytrade positions synced to holdings", "symbols", len(syncedSymbols))
	return nil
}

// syncBalanceToFirefly snapshots the tastytrade account net-liq into a virtual
// Firefly asset account so Firefly's net worth reflects the brokerage.
func (s *Service) syncBalanceToFirefly(ctx context.Context) {
	if s.firefly == nil {
		return
	}
	s.mu.Lock()
	accts := s.accountCache
	s.mu.Unlock()
	if len(accts) == 0 {
		found, err := s.tasty.ListAccountNumbers(ctx)
		if err != nil {
			s.logger.Warn("tastytrade list accounts failed", "error", err)
			return
		}
		s.mu.Lock()
		s.accountCache = found
		accts = found
		s.mu.Unlock()
	}
	for _, accountNumber := range accts {
		bal, err := s.tasty.GetAccountBalance(ctx, accountNumber)
		if err != nil {
			s.logger.Warn("tastytrade balance failed", "account", accountNumber, "error", err)
			continue
		}
		dest := "Tastytrade " + accountNumber
		// Post a zero-sum "balance snapshot" deposit tagged so it's easy to
		// filter/clean up. The first write creates the asset account in Firefly.
		// Subsequent writes add small reconciling deltas — we compute those by
		// comparing to the last known value stored in the transaction note.
		// For simplicity v1: post a $0 reconcile transaction and rely on a
		// future reconciliation step to write real deltas. For now, we just
		// ensure the asset account exists by posting a deposit equal to net-liq
		// ONCE when missing (detected by tag search); here we log and skip
		// money movement to avoid double-counting.
		s.logger.Info("tastytrade balance snapshot",
			"account", accountNumber,
			"net_liq", bal.NetLiquidatingValue,
			"cash", bal.CashBalance,
			"firefly_target", dest,
		)
		// TODO(v2): implement proper reconciliation — needs to track last
		// posted balance per account and post the delta as a transfer. For v1
		// we expose the balance via the API + frontend without writing to
		// Firefly, so there's no risk of double-counting. Flip when we have a
		// test path.
	}
}

// ─── Helpers ────────────────────────────────────────────────

func actionLabel(a string) string {
	switch a {
	case domain.WheelActionCSPOpen:
		return "sell CSP"
	case domain.WheelActionCCOpen:
		return "sell CC"
	case domain.WheelActionCSPClose:
		return "close CSP"
	case domain.WheelActionCCClose:
		return "close CC"
	case domain.WheelActionCSPRoll:
		return "roll CSP"
	case domain.WheelActionCCRoll:
		return "roll CC"
	}
	return a
}

// inMarketHours returns true during a "reliable-quote" window — roughly
// 10:00-16:00 ET (14:00-20:00 UTC during EDT). Deliberately skips the first
// 30 minutes after open because NBBO is wide and erratic while the opening
// auction settles, which pollutes spreads / veto signals in the rules engine.
// Loose Mon-Fri; no holiday calendar, no DST correction.
func inMarketHours(t time.Time) bool {
	t = t.UTC()
	wd := t.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	mins := t.Hour()*60 + t.Minute()
	// 14:00 UTC (~10:00 ET) to 20:00 UTC (~16:00 ET)
	return mins >= 14*60 && mins <= 20*60
}

// primeScanTZ is the wall-clock zone used for the weekly prime scan. Falls
// back to UTC if the tzdata isn't available on the container.
func primeScanTZ() *time.Location {
	if loc, err := time.LoadLocation("America/New_York"); err == nil {
		return loc
	}
	return time.UTC
}

// isPrimeScanTime reports whether t falls inside the weekly prime-scan
// firing window: Friday 11:00–11:04 ET. A 5-minute slack lets the 1-minute
// scheduler ticker hit the window even if startup happened mid-minute.
// Callers MUST dedup via lastPrimeRun so the window doesn't fire repeatedly.
func isPrimeScanTime(t time.Time) bool {
	local := t.In(primeScanTZ())
	if local.Weekday() != time.Friday {
		return false
	}
	return local.Hour() == 11 && local.Minute() < 5
}
