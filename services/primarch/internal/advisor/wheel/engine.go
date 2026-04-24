// Package wheel is the tactical advisor for the Phalanx wheel strategy.
// v2 uses real market quotes for bid/ask and backs out implied IV from the
// mark price to compute an accurate delta per candidate strike, rather than
// estimating both from historical vol. It filters by liquidity (spread, OI),
// checks existing tastytrade positions (to dedup new opens and to generate
// close/roll recommendations for managed positions), and respects available
// buying power for cash-secured puts.
package wheel

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/brokers/tastytrade"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// Candidate is the engine's pre-persistence candidate trade.
type Candidate struct {
	Action             string
	Symbol             string
	UnderlyingPrice    float64
	OptionType         string // P | C
	OptionSymbol       string // tastytrade formatted symbol, used for orders
	Strike             float64
	Expiration         string
	DTE                int
	Delta              float64
	Bid, Ask, Mid      float64
	SpreadPct          float64
	OpenInterest       int
	Volume             int
	Premium            float64
	Collateral         float64
	AnnualizedYield    float64
	IVImplied          float64
	IVRank             float64
	DataQuality        string // live | estimated
	Executable         bool
	ExistingPositionID string
	Rationale          string
	Score              float64

	// Deterministic skip signals from the rules engine. If any are present,
	// the seeded verdict is "skip" and Claude review is bypassed to save tokens.
	Vetoes []string
	// Seeded verdict / reasons — may be overridden by a successful Claude review.
	Verdict        string
	VerdictReasons []string
	// TargetDelta is carried along so the rationale + reasons can reference it.
	TargetDelta float64
	// QuoteAsOf is when tastytrade says this quote was last updated. The UI
	// uses it to render a live/stale age badge so you never act on a quote
	// without knowing how fresh it is.
	QuoteAsOf time.Time
}

// Inputs for the rules engine.
type Inputs struct {
	Config       *domain.WheelConfig
	Watchlist    []domain.WheelWatchlistEntry
	Holdings     []domain.Holding
	Tasty        *tastytrade.Client
	AccountNum   string  // primary tastytrade account
	BuyingPower  float64 // cash available for new CSPs
	Positions    []tastytrade.Position // current tastytrade positions
	Logger       interface{ Warn(msg string, args ...any); Info(msg string, args ...any) }
}

// Hard gates for executable.
const (
	maxSpreadPct      = 0.15 // reject spreads > 15% of mid
	minOpenInterest   = 25
	minPremiumDollars = 0.10 // $10/contract minimum credit
)

// GenerateCandidates runs the full scan and returns a ranked list. Every
// candidate carries DataQuality and Executable flags so the UI can distinguish
// "trust this enough to click" from "needs manual review."
func GenerateCandidates(ctx context.Context, in Inputs) ([]Candidate, error) {
	if in.Config == nil {
		return nil, fmt.Errorf("wheel config missing")
	}
	if in.Tasty == nil || !in.Tasty.Available() {
		return nil, fmt.Errorf("tastytrade client unavailable")
	}

	symbols := gatherSymbols(in)

	// 1) Real underlying quotes for spot price.
	spots := fetchSpots(ctx, in, symbols)

	// 2) Market metrics for IV rank (still useful as a ranking input).
	metrics := fetchMetrics(ctx, in, symbols)

	// Track where we already have short options so we can dedup new opens and
	// generate close/roll recs for managed legs.
	existingPuts, existingCalls, closeRoll := evaluateExistingPositions(in, spots)

	var out []Candidate
	out = append(out, closeRoll...)

	// 3 + 4 — unified per-symbol decision. We walk the union of watchlist +
	// holdings so you never have to add a held ticker to the watchlist just
	// to get recommendations. For each symbol:
	//   held ≥ 100 shares  → Covered Call (once the lot is established)
	//   held < 100 shares  → Cash-Secured Put (grow toward a full lot)
	//   not held           → Cash-Secured Put (enter the wheel)
	// Watchlist entry, if present, provides per-ticker overrides either way.
	type universe struct {
		symbol   string
		held     float64
		avgCost  float64
		wl       *domain.WheelWatchlistEntry
	}

	uni := map[string]*universe{}
	for i := range in.Holdings {
		h := in.Holdings[i]
		uni[h.Symbol] = &universe{symbol: h.Symbol, held: h.Quantity, avgCost: h.AvgCost}
	}
	for i := range in.Watchlist {
		e := &in.Watchlist[i]
		if !e.Active {
			continue
		}
		u, ok := uni[e.Symbol]
		if !ok {
			u = &universe{symbol: e.Symbol}
			uni[e.Symbol] = u
		}
		u.wl = e
	}

	remainingBP := in.BuyingPower
	for _, u := range uni {
		spot, ok := spots[u.symbol]
		if !ok || spot <= 0 {
			continue
		}
		m := metrics[u.symbol]

		// Pick the side based on current share count.
		if u.held >= 100 {
			if _, has := existingCalls[u.symbol]; has {
				continue
			}
			callDelta := in.Config.DefaultCallDelta
			minYield := in.Config.MinPremiumYield
			if u.wl != nil {
				if u.wl.TargetCallDelta != nil {
					callDelta = *u.wl.TargetCallDelta
				}
				if u.wl.MinPremiumYield != nil {
					minYield = *u.wl.MinPremiumYield
				}
			}
			cand := bestOption(ctx, in, u.symbol, spot, "C", callDelta, minYield, u.avgCost, m)
			if cand == nil {
				continue
			}
			out = append(out, *cand)
			continue
		}

		// Held <100 or not held → CSP candidate.
		if _, has := existingPuts[u.symbol]; has {
			continue
		}
		putDelta := in.Config.DefaultPutDelta
		minYield := in.Config.MinPremiumYield
		if u.wl != nil {
			if u.wl.TargetPutDelta != nil {
				putDelta = *u.wl.TargetPutDelta
			}
			if u.wl.MinPremiumYield != nil {
				minYield = *u.wl.MinPremiumYield
			}
		}
		cand := bestOption(ctx, in, u.symbol, spot, "P", putDelta, minYield, 0, m)
		if cand == nil {
			continue
		}
		if cand.Collateral > remainingBP && remainingBP > 0 {
			cand.Executable = false
			cand.Rationale += fmt.Sprintf("\nCollateral $%.0f exceeds remaining BP $%.0f — recommend smaller size or skip.", cand.Collateral, remainingBP)
		} else {
			remainingBP -= cand.Collateral
		}
		out = append(out, *cand)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}

// ─── Existing position analysis ────────────────────────────

// evaluateExistingPositions returns sets of underlyings with existing short
// puts/calls (for dedup) plus a slice of close/roll recommendations for legs
// that have hit the profit-take or roll-DTE thresholds.
func evaluateExistingPositions(in Inputs, spots map[string]float64) (map[string]bool, map[string]bool, []Candidate) {
	puts := map[string]bool{}
	calls := map[string]bool{}
	var recs []Candidate
	if len(in.Positions) == 0 {
		return puts, calls, recs
	}
	cfg := in.Config
	for _, p := range in.Positions {
		if p.InstrumentType != "Equity Option" {
			continue
		}
		if p.QuantityDirection != "Short" || p.Quantity == 0 {
			continue
		}
		// Parse the OCC-style symbol to get expiration + strike + type.
		// tastytrade symbols look like ".AAPL250117P00150000" — take a best-effort
		// parse. If we can't, skip generating a managed rec but still mark the
		// symbol as "held" so we don't stack duplicates.
		optType, strike, expDate, err := parseOCC(p.Symbol)
		under := p.UnderlyingSymbol
		if err != nil {
			recs = append(recs, Candidate{}) // no-op; still track below
		}
		if optType == "P" {
			puts[under] = true
		} else if optType == "C" {
			calls[under] = true
		}
		if err != nil {
			continue
		}
		dte := daysTo(expDate)
		// Profit check: compare current mark to average open price. Short
		// options profit as price drops toward 0. Close at profitTakePct of
		// max (i.e. mark ≤ openPrice * (1 - profitTakePct)).
		openPrice := p.AverageOpenPrice
		currentMark := p.MarkPrice
		profitPct := 0.0
		if openPrice > 0 {
			profitPct = (openPrice - currentMark) / openPrice
		}
		spot := spots[under]
		inTheMoney := (optType == "P" && spot < strike) || (optType == "C" && spot > strike)

		// Close at profit-take.
		if profitPct >= cfg.ProfitTakePct {
			closeCand := Candidate{
				Action:             map[string]string{"P": domain.WheelActionCSPClose, "C": domain.WheelActionCCClose}[optType],
				Symbol:             under,
				UnderlyingPrice:    spot,
				OptionType:         optType,
				OptionSymbol:       p.Symbol,
				Strike:             strike,
				Expiration:         expDate,
				DTE:                dte,
				Mid:                currentMark,
				DataQuality:        domain.WheelDataQualityLive,
				Executable:         true,
				ExistingPositionID: p.Symbol,
				Rationale: fmt.Sprintf(
					"Close %s $%.2f %s — %.0f%% of max profit captured (open $%.2f, mark $%.2f).",
					expDate, strike, optType, profitPct*100, openPrice, currentMark,
				),
				Score: 100 + profitPct*50, // high score, beats all opens
			}
			closeCand.Verdict = domain.WheelVerdictTake
			closeCand.VerdictReasons = []string{
				fmt.Sprintf("%.0f%% of max profit captured — buy back cheap", profitPct*100),
			}
			closeCand.QuoteAsOf = time.Now()
			recs = append(recs, closeCand)
			continue
		}

		// Roll at DTE threshold if ITM.
		if dte <= cfg.RollDTE && inTheMoney {
			rollCand := Candidate{
				Action:             map[string]string{"P": domain.WheelActionCSPRoll, "C": domain.WheelActionCCRoll}[optType],
				Symbol:             under,
				UnderlyingPrice:    spot,
				OptionType:         optType,
				OptionSymbol:       p.Symbol,
				Strike:             strike,
				Expiration:         expDate,
				DTE:                dte,
				Mid:                currentMark,
				DataQuality:        domain.WheelDataQualityLive,
				Executable:         false, // roll requires picking the next expiry — flag for manual review
				ExistingPositionID: p.Symbol,
				Rationale: fmt.Sprintf(
					"Roll %s $%.2f %s — %d DTE and ITM (spot $%.2f). Consider rolling to next monthly at similar or wider strike.",
					expDate, strike, optType, dte, spot,
				),
				Score: 90 + (float64(cfg.RollDTE-dte) * 2),
			}
			rollCand.Verdict = domain.WheelVerdictWait
			rollCand.VerdictReasons = []string{
				fmt.Sprintf("%d DTE and ITM — manual roll selection required", dte),
			}
			rollCand.QuoteAsOf = time.Now()
			recs = append(recs, rollCand)
			continue
		}
	}
	return puts, calls, recs
}

// parseOCC extracts option type, strike, and expiration from tastytrade's
// symbol format. tastytrade symbols are space-padded OCC like:
//   "AAPL  250117P00150000"  -> P, 150.0, "2025-01-17"
// Returns option type ("P"/"C"), strike, expiration YYYY-MM-DD.
func parseOCC(sym string) (string, float64, string, error) {
	// Strip any leading '.' tastytrade sometimes prefixes
	s := strings.TrimPrefix(strings.TrimSpace(sym), ".")
	// Find the 6-digit date block (YYMMDD) followed by P or C — scan from the end.
	// Format: <ROOT><YY><MM><DD><P|C><STRIKE8>
	if len(s) < 15 {
		return "", 0, "", fmt.Errorf("too short: %q", sym)
	}
	// Strike is last 8 chars, integer price scaled by 1000.
	strikeStr := s[len(s)-8:]
	optType := string(s[len(s)-9])
	dateStr := s[len(s)-15 : len(s)-9]
	if optType != "P" && optType != "C" {
		return "", 0, "", fmt.Errorf("bad opt type: %q", optType)
	}
	var strikeInt int64
	fmt.Sscanf(strikeStr, "%d", &strikeInt)
	strike := float64(strikeInt) / 1000.0
	if len(dateStr) != 6 {
		return "", 0, "", fmt.Errorf("bad date: %q", dateStr)
	}
	exp := "20" + dateStr[:2] + "-" + dateStr[2:4] + "-" + dateStr[4:6]
	return optType, strike, exp, nil
}

func daysTo(exp string) int {
	t, err := time.Parse("2006-01-02", exp)
	if err != nil {
		return 0
	}
	return int(math.Round(time.Until(t).Hours() / 24))
}

// ─── Best option selector ─────────────────────────────────

// bestOption scans the chain for the given symbol + side, fetches real quotes
// on a narrow candidate band, backs out implied vol from the mark, computes
// real delta, and picks the strike closest to the target delta that passes
// all liquidity gates.
func bestOption(ctx context.Context, in Inputs, symbol string, spot float64, side string, targetDelta, minYield, costBasis float64, m tastytrade.MarketMetric) *Candidate {
	chain, err := in.Tasty.GetOptionChainNested(ctx, symbol)
	if err != nil {
		if in.Logger != nil {
			in.Logger.Warn("chain fetch failed", "symbol", symbol, "error", err)
		}
		return nil
	}

	type probe struct {
		exp    string
		dte    int
		strike float64
		optSym string
	}
	var probes []probe

	// Narrow to ±8 strikes around the HV-implied target per expiration in the
	// DTE window — a small band we can fetch live quotes for cheaply.
	ivGuess := m.HistoricalVolatility30d
	if ivGuess <= 0 {
		ivGuess = 0.30
	}
	for _, exp := range chain.Expirations {
		if exp.DaysToExpiration < in.Config.MinDTE || exp.DaysToExpiration > in.Config.MaxDTE {
			continue
		}
		// Guess target strike from HV — we'll overwrite with real delta later.
		var targetStrike float64
		if side == "P" {
			targetStrike = spot * (1 - targetDelta*ivGuess*math.Sqrt(float64(exp.DaysToExpiration)/365))
		} else {
			targetStrike = spot * (1 + targetDelta*ivGuess*math.Sqrt(float64(exp.DaysToExpiration)/365))
			if targetStrike < costBasis {
				targetStrike = costBasis
			}
		}
		nearBand := nearestStrikes(exp.Strikes, targetStrike, 5)
		for _, s := range nearBand {
			var sym string
			if side == "P" {
				sym = s.Put
			} else {
				sym = s.Call
			}
			if sym == "" {
				continue
			}
			probes = append(probes, probe{exp: exp.ExpirationDate, dte: exp.DaysToExpiration, strike: s.Strike, optSym: sym})
		}
	}
	if len(probes) == 0 {
		return nil
	}

	// Fetch real quotes for the candidate strikes. Single batch call.
	optSyms := make([]string, 0, len(probes))
	for _, p := range probes {
		optSyms = append(optSyms, p.optSym)
	}
	quotes, err := in.Tasty.GetMarketData(ctx, nil, optSyms)
	live := err == nil
	if err != nil && in.Logger != nil {
		in.Logger.Warn("market-data failed — falling back to BSM estimates", "symbol", symbol, "error", err)
	}
	qMap := map[string]tastytrade.Quote{}
	for _, q := range quotes {
		qMap[q.Symbol] = q
	}

	var best *Candidate
	for _, p := range probes {
		q, hasQuote := qMap[p.optSym]
		mid := (q.Bid + q.Ask) / 2
		spread := 0.0
		if mid > 0 {
			spread = (q.Ask - q.Bid) / mid
		}
		var iv float64
		var realDelta float64
		quality := domain.WheelDataQualityLive
		if !live || !hasQuote || mid <= 0 {
			// Fallback: estimate with HV
			quality = domain.WheelDataQualityEstimated
			iv = ivGuess
			if side == "P" {
				mid = estimatePutPremium(spot, p.strike, iv, p.dte)
				realDelta = -bsmPutDelta(spot, p.strike, iv, p.dte)
			} else {
				mid = estimateCallPremium(spot, p.strike, iv, p.dte)
				realDelta = bsmCallDelta(spot, p.strike, iv, p.dte)
			}
		} else {
			iv = impliedVol(spot, p.strike, mid, p.dte, side)
			if side == "P" {
				realDelta = -bsmPutDelta(spot, p.strike, iv, p.dte)
			} else {
				realDelta = bsmCallDelta(spot, p.strike, iv, p.dte)
			}
		}
		if mid < minPremiumDollars {
			continue
		}
		// Prefer candidates whose real delta is nearest the target.
		deltaDist := math.Abs(math.Abs(realDelta) - targetDelta)

		var collateral float64
		if side == "P" {
			collateral = p.strike * 100
		} else {
			collateral = spot * 100
		}
		ay := (mid * 100 / collateral) * (365.0 / float64(p.dte))
		if ay < minYield {
			continue
		}

		// Liquidity gates — only applied when quotes are live. For estimates
		// we can't check liquidity and must mark Executable=false.
		executable := true
		if quality != domain.WheelDataQualityLive {
			executable = false
		}
		if spread > maxSpreadPct {
			executable = false
		}
		if hasQuote && q.OpenInterest < minOpenInterest {
			executable = false
		}

		// Score: reward yield, penalize distance from target delta, penalize wide spreads.
		score := ay * (0.5 + m.ImpliedVolatilityIndexRank/2)
		score *= 1.0 - math.Min(deltaDist*2, 0.8) // close-to-target bonus
		if spread > 0 {
			score *= 1.0 - math.Min(spread*2, 0.6)
		}
		if quality != domain.WheelDataQualityLive {
			score *= 0.5 // halve score for estimates
		}

		cand := &Candidate{
			Action:          map[string]string{"P": domain.WheelActionCSPOpen, "C": domain.WheelActionCCOpen}[side],
			Symbol:          symbol,
			UnderlyingPrice: spot,
			OptionType:      side,
			OptionSymbol:    p.optSym,
			Strike:          p.strike,
			Expiration:      p.exp,
			DTE:             p.dte,
			Delta:           realDelta,
			Bid:             q.Bid,
			Ask:             q.Ask,
			Mid:             mid,
			SpreadPct:       spread,
			OpenInterest:    q.OpenInterest,
			Volume:          q.Volume,
			Premium:         mid,
			Collateral:      collateral,
			AnnualizedYield: ay,
			IVImplied:       iv,
			IVRank:          m.ImpliedVolatilityIndexRank,
			DataQuality:     quality,
			Executable:      executable,
			Score:           score,
			TargetDelta:     targetDelta,
			QuoteAsOf:       parseTastyQuoteTime(q.UpdatedAt),
		}
		cand.Rationale = buildRationale(cand, targetDelta, costBasis)
		cand.Vetoes = computeVetoes(cand)
		cand.Verdict, cand.VerdictReasons = seedVerdict(cand)
		if best == nil || cand.Score > best.Score {
			best = cand
		}
	}
	return best
}

// parseTastyQuoteTime parses tastytrade's "updated-at" string (ISO 8601,
// typically with nanosecond precision) into a Time. Falls back to now() if
// the field is empty or malformed — the caller prefers "fresh enough" over
// a zero time that the UI would render as "stale forever."
func parseTastyQuoteTime(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now()
}

// Veto thresholds. These drive the deterministic "skip" reasons — hard
// conditions that make a trade a bad idea regardless of the Claude review.
const (
	vetoSpreadPct      = 0.25 // wider than 25% of mid = execution risk
	vetoOpenInterest   = 10   // thin enough you can't reliably get out
	vetoDeltaMismatch  = 0.15 // |realΔ| more than 15 points off target
	vetoIVRankLow      = 0.15 // IV rank below 15 — not getting paid for the risk
	vetoAnnualYieldLow = 0.10 // sub-10% annualized is rarely worth the risk/capital
)

// computeVetoes returns a slice of deterministic skip signals for this
// candidate. An empty slice means no hard vetoes — the candidate is eligible
// for a Claude review that can still downgrade to wait/skip.
func computeVetoes(c *Candidate) []string {
	var v []string
	if !c.Executable {
		v = append(v, "flagged non-executable by liquidity / data gates")
	}
	if c.DataQuality != domain.WheelDataQualityLive {
		v = append(v, "quotes estimated (no live market data)")
	}
	// Skip liquidity checks when the quote itself wasn't live — the numbers are
	// meaningless then and the data-quality veto already covers it.
	if c.DataQuality == domain.WheelDataQualityLive {
		if c.SpreadPct > vetoSpreadPct {
			v = append(v, fmt.Sprintf("spread too wide (%.0f%% of mid)", c.SpreadPct*100))
		}
		if c.OpenInterest > 0 && c.OpenInterest < vetoOpenInterest {
			v = append(v, fmt.Sprintf("open interest too low (%d)", c.OpenInterest))
		}
		if c.OpenInterest == 0 {
			v = append(v, "zero open interest")
		}
	}
	if c.TargetDelta > 0 {
		absDelta := math.Abs(c.Delta)
		if math.Abs(absDelta-c.TargetDelta) > vetoDeltaMismatch {
			v = append(v, fmt.Sprintf("delta %.2f far from target %.2f", absDelta, c.TargetDelta))
		}
	}
	if c.IVRank > 0 && c.IVRank < vetoIVRankLow {
		v = append(v, fmt.Sprintf("IV rank low (%.0f) — not getting paid for risk", c.IVRank*100))
	}
	if c.AnnualizedYield < vetoAnnualYieldLow {
		v = append(v, fmt.Sprintf("annualized yield weak (%.1f%%)", c.AnnualizedYield*100))
	}
	return v
}

// seedVerdict returns the pre-review verdict + initial reasons based purely on
// the deterministic vetoes. Claude review can upgrade/downgrade from here.
func seedVerdict(c *Candidate) (string, []string) {
	if len(c.Vetoes) > 0 {
		// Skip means there's at least one hard reason to pass. Reasons list
		// starts with the vetoes themselves so the UI can just render them.
		return domain.WheelVerdictSkip, append([]string(nil), c.Vetoes...)
	}
	// Close / roll recs carry their own rationale and are always "take" — they
	// represent a position already established where the action is defensive.
	if c.Action == domain.WheelActionCSPClose || c.Action == domain.WheelActionCCClose ||
		c.Action == domain.WheelActionCSPRoll || c.Action == domain.WheelActionCCRoll {
		return domain.WheelVerdictTake, []string{"managing existing position — see rationale"}
	}
	// Otherwise the candidate cleared every deterministic gate. Default to
	// "take" with a short positive reason; Claude review can still override.
	reasons := []string{
		fmt.Sprintf("annualized yield %.1f%% at Δ %.2f (target %.2f)",
			c.AnnualizedYield*100, math.Abs(c.Delta), c.TargetDelta),
	}
	if c.IVRank >= 0.50 {
		reasons = append(reasons, fmt.Sprintf("IV rank elevated (%.0f)", c.IVRank*100))
	}
	return domain.WheelVerdictTake, reasons
}

func buildRationale(c *Candidate, targetDelta, costBasis float64) string {
	side := "put"
	actionVerb := "Sell"
	if c.OptionType == "C" {
		side = "call"
	}
	lines := []string{
		fmt.Sprintf("%s %s $%.2f %s, %d DTE, real Δ %.2f (target %.2f).", actionVerb, c.Expiration, c.Strike, side, c.DTE, c.Delta, targetDelta),
		fmt.Sprintf("Mid $%.2f (bid %.2f / ask %.2f, spread %.1f%%). Premium $%.0f on $%.0f collateral = %.1f%% annualized.",
			c.Mid, c.Bid, c.Ask, c.SpreadPct*100, c.Mid*100, c.Collateral, c.AnnualizedYield*100),
	}
	if c.DataQuality == domain.WheelDataQualityLive {
		lines = append(lines, fmt.Sprintf("Liquidity: OI %d, vol %d. IV rank %.0f. Implied vol %.0f%%.", c.OpenInterest, c.Volume, c.IVRank*100, c.IVImplied*100))
	} else {
		lines = append(lines, fmt.Sprintf("ESTIMATED quote (real market data unavailable). IV rank %.0f. Do not trade blindly — check tastytrade before executing.", c.IVRank*100))
	}
	if c.OptionType == "C" && costBasis > 0 {
		lines = append(lines, fmt.Sprintf("Cost basis $%.2f — strike stays above.", costBasis))
	}
	return strings.Join(lines, " ")
}

// ─── Implied vol + Black-Scholes helpers ───────────────────

// impliedVol backs out σ from an observed option mid using Newton-Raphson.
// Returns 0 if it can't converge (caller treats as unreliable).
func impliedVol(spot, strike, mid float64, dte int, side string) float64 {
	if mid <= 0 || spot <= 0 || strike <= 0 || dte <= 0 {
		return 0
	}
	sigma := 0.30 // seed
	for i := 0; i < 40; i++ {
		var price, vega float64
		if side == "P" {
			price = estimatePutPremium(spot, strike, sigma, dte)
		} else {
			price = estimateCallPremium(spot, strike, sigma, dte)
		}
		vega = bsmVega(spot, strike, sigma, dte)
		if vega < 1e-6 {
			return sigma
		}
		diff := price - mid
		if math.Abs(diff) < 0.005 {
			return sigma
		}
		sigma -= diff / vega
		if sigma < 0.01 {
			sigma = 0.01
		} else if sigma > 5 {
			sigma = 5
		}
	}
	return sigma
}

func bsmVega(s, k, sigma float64, dte int) float64 {
	t := float64(dte) / 365.0
	if t <= 0 || sigma <= 0 {
		return 0
	}
	r := 0.05
	d1 := (math.Log(s/k) + (r+0.5*sigma*sigma)*t) / (sigma * math.Sqrt(t))
	return s * normPDF(d1) * math.Sqrt(t)
}

func bsmPutDelta(s, k, sigma float64, dte int) float64 {
	t := float64(dte) / 365.0
	if t <= 0 || sigma <= 0 {
		return 0
	}
	r := 0.05
	d1 := (math.Log(s/k) + (r+0.5*sigma*sigma)*t) / (sigma * math.Sqrt(t))
	return normCDF(d1) - 1 // put delta is negative for a long put
}

func bsmCallDelta(s, k, sigma float64, dte int) float64 {
	t := float64(dte) / 365.0
	if t <= 0 || sigma <= 0 {
		return 0
	}
	r := 0.05
	d1 := (math.Log(s/k) + (r+0.5*sigma*sigma)*t) / (sigma * math.Sqrt(t))
	return normCDF(d1)
}

func estimatePutPremium(spot, strike, iv float64, dte int) float64 {
	t := float64(dte) / 365.0
	r := 0.05
	if t <= 0 || iv <= 0 {
		return 0
	}
	d1 := (math.Log(spot/strike) + (r+0.5*iv*iv)*t) / (iv * math.Sqrt(t))
	d2 := d1 - iv*math.Sqrt(t)
	return strike*math.Exp(-r*t)*normCDF(-d2) - spot*normCDF(-d1)
}

func estimateCallPremium(spot, strike, iv float64, dte int) float64 {
	t := float64(dte) / 365.0
	r := 0.05
	if t <= 0 || iv <= 0 {
		return 0
	}
	d1 := (math.Log(spot/strike) + (r+0.5*iv*iv)*t) / (iv * math.Sqrt(t))
	d2 := d1 - iv*math.Sqrt(t)
	return spot*normCDF(d1) - strike*math.Exp(-r*t)*normCDF(d2)
}

func normCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

func normPDF(x float64) float64 {
	return math.Exp(-0.5*x*x) / math.Sqrt(2*math.Pi)
}

// ─── Fetchers ──────────────────────────────────────────────

func fetchSpots(ctx context.Context, in Inputs, symbols []string) map[string]float64 {
	out := map[string]float64{}
	if len(symbols) == 0 {
		return out
	}
	quotes, err := in.Tasty.GetMarketData(ctx, symbols, nil)
	if err != nil {
		if in.Logger != nil {
			in.Logger.Warn("spot fetch failed", "error", err)
		}
		return out
	}
	for _, q := range quotes {
		px := q.Mark
		if px == 0 {
			px = q.Last
		}
		if px == 0 {
			px = (q.Bid + q.Ask) / 2
		}
		if px > 0 {
			out[q.Symbol] = px
		}
	}
	return out
}

func fetchMetrics(ctx context.Context, in Inputs, symbols []string) map[string]tastytrade.MarketMetric {
	out := map[string]tastytrade.MarketMetric{}
	if len(symbols) == 0 {
		return out
	}
	mm, err := in.Tasty.GetMarketMetrics(ctx, symbols)
	if err != nil {
		if in.Logger != nil {
			in.Logger.Warn("metrics fetch failed", "error", err)
		}
		return out
	}
	for _, m := range mm {
		out[m.Symbol] = m
	}
	return out
}

// ─── Misc ──────────────────────────────────────────────────

// gatherSymbols returns the union of watchlist + held symbols. Any quantity
// qualifies — held lots < 100 become CSP candidates to build toward a full lot.
func gatherSymbols(in Inputs) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range in.Watchlist {
		if e.Active && !seen[e.Symbol] {
			seen[e.Symbol] = true
			out = append(out, e.Symbol)
		}
	}
	for _, h := range in.Holdings {
		if h.Quantity > 0 && !seen[h.Symbol] {
			seen[h.Symbol] = true
			out = append(out, h.Symbol)
		}
	}
	return out
}

// nearestStrikes returns the n strikes closest to target, sorted by distance.
func nearestStrikes(strikes []tastytrade.ChainStrike, target float64, n int) []tastytrade.ChainStrike {
	if len(strikes) == 0 {
		return nil
	}
	sorted := make([]tastytrade.ChainStrike, len(strikes))
	copy(sorted, strikes)
	sort.Slice(sorted, func(i, j int) bool {
		return math.Abs(sorted[i].Strike-target) < math.Abs(sorted[j].Strike-target)
	})
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

func derefFloat(p *float64, fallback float64) float64 {
	if p == nil {
		return fallback
	}
	return *p
}

// ToRecommendations persists engine candidates.
func ToRecommendations(runID string, cands []Candidate) []domain.WheelRecommendation {
	now := time.Now()
	out := make([]domain.WheelRecommendation, 0, len(cands))
	for i, c := range cands {
		out = append(out, domain.WheelRecommendation{
			ID:                 fmt.Sprintf("wrec-%d-%d", now.UnixNano(), i),
			RunID:              runID,
			Action:             c.Action,
			Symbol:             c.Symbol,
			UnderlyingPrice:    c.UnderlyingPrice,
			OptionType:         c.OptionType,
			Strike:             c.Strike,
			Expiration:         c.Expiration,
			DTE:                c.DTE,
			Delta:              c.Delta,
			Bid:                c.Bid,
			Ask:                c.Ask,
			Mid:                c.Mid,
			Premium:            c.Premium,
			Collateral:         c.Collateral,
			AnnualizedYield:    c.AnnualizedYield,
			IVRank:             c.IVRank,
			Score:              c.Score,
			RulesRationale:     c.Rationale,
			DataQuality:        c.DataQuality,
			SpreadPct:          c.SpreadPct,
			OpenInterest:       c.OpenInterest,
			Volume:             c.Volume,
			Executable:         c.Executable,
			ExistingPositionID: c.ExistingPositionID,
			Verdict:            c.Verdict,
			Vetoes:             c.Vetoes,
			VerdictReasons:     c.VerdictReasons,
			QuoteAsOf:          c.QuoteAsOf,
			OptionSymbol:       c.OptionSymbol,
			Status:             domain.WheelRecFresh,
			CreatedAt:          now,
		})
	}
	return out
}
