package domain

import "time"

// WheelWatchlistEntry is a ticker the advisor scans for cash-secured put
// opportunities (and can be assigned into). Per-ticker overrides win over
// WheelConfig defaults.
type WheelWatchlistEntry struct {
	Symbol            string     `json:"symbol"`
	MaxPositionValue  *float64   `json:"max_position_value,omitempty"`
	TargetPutDelta    *float64   `json:"target_put_delta,omitempty"`
	TargetCallDelta   *float64   `json:"target_call_delta,omitempty"`
	MinPremiumYield   *float64   `json:"min_premium_yield,omitempty"`
	Notes             string     `json:"notes,omitempty"`
	Active            bool       `json:"active"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// WheelConfig is global advisor defaults. Single-row table, id='default'.
type WheelConfig struct {
	ID                 string    `json:"id"`
	DefaultPutDelta    float64   `json:"default_put_delta"`
	DefaultCallDelta   float64   `json:"default_call_delta"`
	MinDTE             int       `json:"min_dte"`
	MaxDTE             int       `json:"max_dte"`
	MinPremiumYield    float64   `json:"min_premium_yield"`
	ProfitTakePct      float64   `json:"profit_take_pct"`
	RollDTE            int       `json:"roll_dte"`
	MaxPositions       int       `json:"max_positions"`
	ClaudeReview       bool      `json:"claude_review"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Wheel advisor action kinds.
const (
	WheelActionCSPOpen  = "csp_open"
	WheelActionCCOpen   = "cc_open"
	WheelActionCSPClose = "csp_close"
	WheelActionCCClose  = "cc_close"
	WheelActionCSPRoll  = "csp_roll"
	WheelActionCCRoll   = "cc_roll"
)

// Wheel recommendation statuses.
const (
	WheelRecFresh     = "fresh"
	WheelRecTaken     = "taken"
	WheelRecDismissed = "dismissed"
	WheelRecExpired   = "expired"
)

// Data-quality labels. "live" = real market quotes; "estimated" = BSM/HV fallback.
const (
	WheelDataQualityLive      = "live"
	WheelDataQualityEstimated = "estimated"
)

// Wheel verdict — a pre-trade call on whether to act on a candidate right now.
// "take" means conditions are favorable; "wait" means marginal (data/liquidity
// concerns); "skip" means at least one hard reason to pass.
const (
	WheelVerdictTake = "take"
	WheelVerdictWait = "wait"
	WheelVerdictSkip = "skip"
)

// WheelRecommendation is a single candidate trade. A Run groups a scan's recs.
type WheelRecommendation struct {
	ID               string     `json:"id"`
	RunID            string     `json:"run_id"`
	Action           string     `json:"action"`
	Symbol           string     `json:"symbol"`
	UnderlyingPrice  float64    `json:"underlying_price"`
	OptionType       string     `json:"option_type,omitempty"` // P | C
	Strike           float64    `json:"strike,omitempty"`
	Expiration       string     `json:"expiration,omitempty"`
	DTE              int        `json:"dte,omitempty"`
	Delta            float64    `json:"delta,omitempty"`
	Bid              float64    `json:"bid,omitempty"`
	Ask              float64    `json:"ask,omitempty"`
	Mid              float64    `json:"mid,omitempty"`
	Premium          float64    `json:"premium,omitempty"`
	Collateral       float64    `json:"collateral,omitempty"`
	AnnualizedYield  float64    `json:"annualized_yield,omitempty"`
	IVRank           float64    `json:"iv_rank,omitempty"`
	Score            float64    `json:"score"`
	RulesRationale   string     `json:"rules_rationale,omitempty"`
	ReviewNote       string     `json:"review_note,omitempty"`
	ReviewScore      float64    `json:"review_score,omitempty"`

	// v2 reliability metadata
	DataQuality       string  `json:"data_quality,omitempty"`       // live | estimated
	SpreadPct         float64 `json:"spread_pct,omitempty"`         // (ask-bid)/mid
	OpenInterest      int     `json:"open_interest,omitempty"`
	Volume            int     `json:"volume,omitempty"`
	Executable        bool    `json:"executable"`                    // all hard gates pass
	ExistingPositionID string `json:"existing_position_id,omitempty"` // for close/roll recs
	OptionSymbol      string  `json:"option_symbol,omitempty"`        // tastytrade formatted — lets paper-take open without re-resolving

	// Pre-trade decision — populated by the advisor. Verdict is one of
	// take/wait/skip (see WheelVerdict* constants). Vetoes are deterministic
	// skip signals from the rules engine (e.g. "low open interest (3)"). Reasons
	// are the reviewer's "why take" / "why skip" bullets, merged from rules and
	// optional Claude review. UIs should lead with Verdict + Reasons.
	Verdict        string   `json:"verdict,omitempty"`
	Vetoes         []string `json:"vetoes,omitempty"`
	VerdictReasons []string `json:"verdict_reasons,omitempty"`

	// QuoteAsOf is tastytrade's reported last-update time for the underlying
	// quote used to price this candidate. UIs render an age badge off this;
	// a zero time means freshness is unknown.
	QuoteAsOf time.Time `json:"quote_as_of,omitempty"`

	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	TakenAt          *time.Time `json:"taken_at,omitempty"`
	DismissedAt      *time.Time `json:"dismissed_at,omitempty"`
}

// Paper trading — a simulated take on a wheel candidate. Captures the
// entry conditions, tracks the option mark on each scan, and realizes P&L
// when closed or expired. Lets the user sanity-check the advisor's
// verdicts without committing real capital.
const (
	WheelPaperStatusOpen    = "open"
	WheelPaperStatusClosed  = "closed"
	WheelPaperStatusExpired = "expired"
)

// WheelPaperPosition is a simulated wheel leg. Short option convention —
// entry_premium is the credit received per contract, current_mark is the
// cost to buy back, so unrealized P&L per contract =
// (entry_premium - current_mark) * 100 * contracts.
type WheelPaperPosition struct {
	ID             string     `json:"id"`
	SourceRecID    string     `json:"source_rec_id,omitempty"` // wheel_recommendations.id this was opened from
	Action         string     `json:"action"`                   // csp_open | cc_open
	Symbol         string     `json:"symbol"`
	OptionType     string     `json:"option_type"` // P | C
	OptionSymbol   string     `json:"option_symbol,omitempty"` // tastytrade formatted
	Strike         float64    `json:"strike"`
	Expiration     string     `json:"expiration"` // YYYY-MM-DD
	Contracts      int        `json:"contracts"`
	EntryPremium   float64    `json:"entry_premium"` // mid at open
	EntryTime      time.Time  `json:"entry_time"`
	CurrentMark    float64    `json:"current_mark"`
	LastMarkAt     time.Time  `json:"last_mark_at,omitempty"`
	RealizedPnL    float64    `json:"realized_pnl,omitempty"`   // filled on close/expire
	UnrealizedPnL  float64    `json:"unrealized_pnl,omitempty"` // computed on read
	Status         string     `json:"status"`                   // open | closed | expired
	Notes          string     `json:"notes,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`

	// Verdict carried over from the source recommendation so we can tally
	// take/wait/skip accuracy without a second lookup.
	EntryVerdict string `json:"entry_verdict,omitempty"`
}
