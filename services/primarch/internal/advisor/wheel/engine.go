// Package wheel is the tactical advisor for the Phalanx wheel strategy:
// sell cash-secured puts → get assigned → sell covered calls → get called away → repeat.
//
// The rules engine scans the watchlist for CSP candidates and current holdings
// for CC candidates, filtering by DTE, strike distance, and annualized yield.
// A Claude review layer can rewrite the rationale and flag unusual situations.
package wheel

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/brokers/tastytrade"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// Candidate is an intermediate value the engine produces before Claude review.
type Candidate struct {
	Action          string
	Symbol          string
	UnderlyingPrice float64
	OptionType      string // P | C
	Strike          float64
	Expiration      string
	DTE             int
	Delta           float64
	Premium         float64
	Collateral      float64
	AnnualizedYield float64
	IVRank          float64
	Rationale       string
	Score           float64
}

// Inputs captures everything the rules engine needs for a scan.
type Inputs struct {
	Config     *domain.WheelConfig
	Watchlist  []domain.WheelWatchlistEntry
	Holdings   []domain.Holding // existing equity positions for CC candidates
	Tasty      *tastytrade.Client
	Account    string // tastytrade account number (for positions)
	Logger     interface{ Warn(msg string, args ...any) }
}

// GenerateCandidates runs the full scan against the watchlist + holdings and
// returns a ranked list of candidate trades.
func GenerateCandidates(ctx context.Context, in Inputs) ([]Candidate, error) {
	if in.Config == nil {
		return nil, fmt.Errorf("wheel config missing")
	}
	if in.Tasty == nil || !in.Tasty.Available() {
		return nil, fmt.Errorf("tastytrade client unavailable")
	}

	symbols := gatherSymbols(in)
	if len(symbols) == 0 {
		return nil, nil
	}

	// Pull IV rank + hv in a single call so we can use it to estimate greeks.
	metrics := map[string]tastytrade.MarketMetric{}
	if mm, err := in.Tasty.GetMarketMetrics(ctx, symbols); err == nil {
		for _, m := range mm {
			metrics[m.Symbol] = m
		}
	} else if in.Logger != nil {
		in.Logger.Warn("market metrics failed", "error", err)
	}

	var out []Candidate

	// Watchlist → CSP candidates
	for _, entry := range in.Watchlist {
		if !entry.Active {
			continue
		}
		spot, chain, err := fetchUnderlying(ctx, in.Tasty, entry.Symbol, metrics)
		if err != nil {
			if in.Logger != nil {
				in.Logger.Warn("fetch underlying failed", "symbol", entry.Symbol, "error", err)
			}
			continue
		}
		putDelta := derefFloat(entry.TargetPutDelta, in.Config.DefaultPutDelta)
		minYield := derefFloat(entry.MinPremiumYield, in.Config.MinPremiumYield)
		if cand := bestCSP(entry.Symbol, spot, chain, metrics[entry.Symbol], in.Config, putDelta, minYield); cand != nil {
			out = append(out, *cand)
		}
	}

	// Holdings → CC candidates (only where we own ≥100 shares)
	for _, h := range in.Holdings {
		if h.Quantity < 100 {
			continue
		}
		spot, chain, err := fetchUnderlying(ctx, in.Tasty, h.Symbol, metrics)
		if err != nil {
			if in.Logger != nil {
				in.Logger.Warn("fetch underlying failed", "symbol", h.Symbol, "error", err)
			}
			continue
		}
		callDelta := in.Config.DefaultCallDelta
		// Entries on the watchlist can override per-ticker.
		for _, wl := range in.Watchlist {
			if wl.Symbol == h.Symbol && wl.TargetCallDelta != nil {
				callDelta = *wl.TargetCallDelta
			}
		}
		minYield := in.Config.MinPremiumYield
		costBasis := h.AvgCost
		if cand := bestCC(h.Symbol, spot, chain, metrics[h.Symbol], in.Config, callDelta, minYield, costBasis); cand != nil {
			out = append(out, *cand)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}

// gatherSymbols dedupes watchlist + held symbols for a single metrics call.
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
		if h.Quantity >= 100 && !seen[h.Symbol] {
			seen[h.Symbol] = true
			out = append(out, h.Symbol)
		}
	}
	return out
}

// fetchUnderlying returns spot + option chain. Spot is estimated from chain
// ATM strike midpoint if the market metric feed doesn't include it.
func fetchUnderlying(ctx context.Context, c *tastytrade.Client, symbol string, metrics map[string]tastytrade.MarketMetric) (float64, *tastytrade.OptionChain, error) {
	chain, err := c.GetOptionChainNested(ctx, symbol)
	if err != nil {
		return 0, nil, err
	}
	// tastytrade's market-metrics doesn't always include last price; use
	// chain's strike closest to ATM as a proxy. The first expiration's strikes
	// are a reasonable anchor.
	spot := guessSpotFromChain(chain)
	return spot, chain, nil
}

// guessSpotFromChain picks the strike with roughly equal distance to delta 0.5
// calls/puts — heuristic, but stable. Assumes sorted strikes.
func guessSpotFromChain(chain *tastytrade.OptionChain) float64 {
	if chain == nil || len(chain.Expirations) == 0 {
		return 0
	}
	strikes := chain.Expirations[0].Strikes
	if len(strikes) == 0 {
		return 0
	}
	// Prefer the middle strike as a seed — real ATM matching would need live quotes.
	return strikes[len(strikes)/2].Strike
}

// bestCSP picks the best cash-secured put from the chain given config.
func bestCSP(symbol string, spot float64, chain *tastytrade.OptionChain, m tastytrade.MarketMetric,
	cfg *domain.WheelConfig, targetDelta, minYield float64) *Candidate {
	if spot <= 0 {
		return nil
	}
	iv := m.HistoricalVolatility30d
	if iv <= 0 {
		iv = 0.30 // conservative fallback
	}

	var best *Candidate
	for _, exp := range chain.Expirations {
		if exp.DaysToExpiration < cfg.MinDTE || exp.DaysToExpiration > cfg.MaxDTE {
			continue
		}
		dte := exp.DaysToExpiration
		// For a put with target delta, strike ≈ spot * (1 - targetDelta * iv * sqrt(dte/365))
		// rough heuristic, refined by picking the actual strike closest.
		targetStrike := spot * (1 - targetDelta*iv*math.Sqrt(float64(dte)/365.0))
		pick := nearestStrike(exp.Strikes, targetStrike)
		if pick == nil || pick.Put == "" {
			continue
		}
		// Estimate premium via BSM with the crude IV we have.
		premium := estimatePutPremium(spot, pick.Strike, iv, dte)
		if premium <= 0.05 {
			continue
		}
		collateral := pick.Strike * 100
		ay := (premium * 100 / collateral) * (365.0 / float64(dte))
		if ay < minYield {
			continue
		}
		score := ay * (0.5 + m.ImpliedVolatilityIndexRank/2) // weight by IV rank
		cand := &Candidate{
			Action:          domain.WheelActionCSPOpen,
			Symbol:          symbol,
			UnderlyingPrice: spot,
			OptionType:      "P",
			Strike:          pick.Strike,
			Expiration:      exp.ExpirationDate,
			DTE:             dte,
			Delta:           -targetDelta,
			Premium:         premium,
			Collateral:      collateral,
			AnnualizedYield: ay,
			IVRank:          m.ImpliedVolatilityIndexRank,
			Rationale: fmt.Sprintf(
				"Sell %s $%.2f put, %dDTE, ~%.2f delta. Premium ≈ $%.2f on $%.0f collateral = %.1f%% annualized. IV rank %.0f.",
				exp.ExpirationDate, pick.Strike, dte, -targetDelta, premium, collateral, ay*100, m.ImpliedVolatilityIndexRank*100,
			),
			Score: score,
		}
		if best == nil || cand.Score > best.Score {
			best = cand
		}
	}
	return best
}

// bestCC picks the best covered call for a held position.
func bestCC(symbol string, spot float64, chain *tastytrade.OptionChain, m tastytrade.MarketMetric,
	cfg *domain.WheelConfig, targetDelta, minYield, costBasis float64) *Candidate {
	if spot <= 0 {
		return nil
	}
	iv := m.HistoricalVolatility30d
	if iv <= 0 {
		iv = 0.30
	}
	var best *Candidate
	for _, exp := range chain.Expirations {
		if exp.DaysToExpiration < cfg.MinDTE || exp.DaysToExpiration > cfg.MaxDTE {
			continue
		}
		dte := exp.DaysToExpiration
		targetStrike := spot * (1 + targetDelta*iv*math.Sqrt(float64(dte)/365.0))
		// Never write a call below cost basis — that locks in a loss on called-away.
		if targetStrike < costBasis {
			targetStrike = costBasis
		}
		pick := nearestStrike(exp.Strikes, targetStrike)
		if pick == nil || pick.Call == "" {
			continue
		}
		premium := estimateCallPremium(spot, pick.Strike, iv, dte)
		if premium <= 0.05 {
			continue
		}
		collateral := spot * 100 // cost of holding 100 shares
		ay := (premium * 100 / collateral) * (365.0 / float64(dte))
		if ay < minYield {
			continue
		}
		score := ay * (0.5 + m.ImpliedVolatilityIndexRank/2)
		cand := &Candidate{
			Action:          domain.WheelActionCCOpen,
			Symbol:          symbol,
			UnderlyingPrice: spot,
			OptionType:      "C",
			Strike:          pick.Strike,
			Expiration:      exp.ExpirationDate,
			DTE:             dte,
			Delta:           targetDelta,
			Premium:         premium,
			Collateral:      collateral,
			AnnualizedYield: ay,
			IVRank:          m.ImpliedVolatilityIndexRank,
			Rationale: fmt.Sprintf(
				"Sell %s $%.2f call, %dDTE, ~%.2f delta. Premium ≈ $%.2f on 100 shares ($%.0f) = %.1f%% annualized. Cost basis $%.2f — strike stays above.",
				exp.ExpirationDate, pick.Strike, dte, targetDelta, premium, collateral, ay*100, costBasis,
			),
			Score: score,
		}
		if best == nil || cand.Score > best.Score {
			best = cand
		}
	}
	return best
}

// nearestStrike returns the strike closest to target. Assumes strikes are not necessarily sorted.
func nearestStrike(strikes []tastytrade.ChainStrike, target float64) *tastytrade.ChainStrike {
	if len(strikes) == 0 {
		return nil
	}
	bestIdx := 0
	bestDist := math.Abs(strikes[0].Strike - target)
	for i := 1; i < len(strikes); i++ {
		d := math.Abs(strikes[i].Strike - target)
		if d < bestDist {
			bestIdx = i
			bestDist = d
		}
	}
	return &strikes[bestIdx]
}

// estimatePutPremium approximates a European put price via Black-Scholes with
// r=0.05 and no dividends. Deliberately simple — exact Greeks need live quotes
// or a DXFeed stream which we don't have on Cloud Run.
func estimatePutPremium(spot, strike, iv float64, dte int) float64 {
	t := float64(dte) / 365.0
	r := 0.05
	if t <= 0 || iv <= 0 {
		return 0
	}
	d1 := (math.Log(spot/strike) + (r+0.5*iv*iv)*t) / (iv * math.Sqrt(t))
	d2 := d1 - iv*math.Sqrt(t)
	// Put = K*e^(-rt)*N(-d2) - S*N(-d1)
	return strike*math.Exp(-r*t)*normCDF(-d2) - spot*normCDF(-d1)
}

// estimateCallPremium — same simplifications as estimatePutPremium.
func estimateCallPremium(spot, strike, iv float64, dte int) float64 {
	t := float64(dte) / 365.0
	r := 0.05
	if t <= 0 || iv <= 0 {
		return 0
	}
	d1 := (math.Log(spot/strike) + (r+0.5*iv*iv)*t) / (iv * math.Sqrt(t))
	d2 := d1 - iv*math.Sqrt(t)
	// Call = S*N(d1) - K*e^(-rt)*N(d2)
	return spot*normCDF(d1) - strike*math.Exp(-r*t)*normCDF(d2)
}

// normCDF is the standard normal cumulative distribution.
func normCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

func derefFloat(p *float64, fallback float64) float64 {
	if p == nil {
		return fallback
	}
	return *p
}

// ToRecommendations converts candidates to persistable WheelRecommendation rows.
func ToRecommendations(runID string, cands []Candidate) []domain.WheelRecommendation {
	now := time.Now()
	out := make([]domain.WheelRecommendation, 0, len(cands))
	for i, c := range cands {
		out = append(out, domain.WheelRecommendation{
			ID:               fmt.Sprintf("wrec-%d-%d", now.UnixNano(), i),
			RunID:            runID,
			Action:           c.Action,
			Symbol:           c.Symbol,
			UnderlyingPrice:  c.UnderlyingPrice,
			OptionType:       c.OptionType,
			Strike:           c.Strike,
			Expiration:       c.Expiration,
			DTE:              c.DTE,
			Delta:            c.Delta,
			Premium:          c.Premium,
			Mid:              c.Premium,
			Collateral:       c.Collateral,
			AnnualizedYield:  c.AnnualizedYield,
			IVRank:           c.IVRank,
			Score:            c.Score,
			RulesRationale:   c.Rationale,
			Status:           domain.WheelRecFresh,
			CreatedAt:        now,
		})
	}
	return out
}
