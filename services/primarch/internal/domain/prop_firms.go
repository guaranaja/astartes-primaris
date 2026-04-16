// ECOSYSTEM: prop-firm-identity
// Engines (astartes-futures) should set AccountSnapshot.PropFirm so the Council
// can apply the right house rules (profit split, consistency %, payout cadence).
package domain

// PropFirm identifies a funded-account vendor and its house rules.
// Rules here are the defaults Primaris uses when recording payouts, fees,
// and combine-pass events. Per-account overrides on TradingAccount still win.
type PropFirm struct {
	ID                string  `json:"id"`                 // "topstep", "apex", "tpt", "tradeday"
	Name              string  `json:"name"`               // "TopstepX"
	ProfitSplit       float64 `json:"profit_split"`       // trader's share, e.g. 0.90
	FirstPayoutSplit  float64 `json:"first_payout_split"` // some firms cap initial payouts
	MinWithdrawal     float64 `json:"min_withdrawal"`
	ConsistencyPct    float64 `json:"consistency_pct"`    // best-day cap, e.g. 0.50 = best day must be ≤50% of total
	MinTradingDays    int     `json:"min_trading_days"`   // days required before first payout
	PayoutCycleDays   int     `json:"payout_cycle_days"`  // min days between payouts
	DefaultEvalFee    float64 `json:"default_eval_fee"`   // combine purchase price (discounted to typical)
	DefaultActivation float64 `json:"default_activation"` // PA/funded activation fee if any
	DefaultResetFee   float64 `json:"default_reset_fee"`  // reset after breach
	Website           string  `json:"website"`
	Notes             string  `json:"notes,omitempty"`
}

// PropFirmRegistry is the canonical list of firms Primaris knows about.
// Edit in source — rules are stable enough to not need a table.
var PropFirmRegistry = []PropFirm{
	{
		ID:                "topstep",
		Name:              "Topstep (TopstepX)",
		ProfitSplit:       0.90,
		FirstPayoutSplit:  1.00, // first $10k at 100%
		MinWithdrawal:     0,
		ConsistencyPct:    0.50,
		MinTradingDays:    5,
		PayoutCycleDays:   0,
		DefaultEvalFee:    49,
		DefaultActivation: 149,
		DefaultResetFee:   79,
		Website:           "https://www.topstep.com",
		Notes:             "TopstepX platform; best-day ≤50% of total P&L rule; first $10k funded payout at 100%.",
	},
	{
		ID:                "apex",
		Name:              "Apex Trader Funding",
		ProfitSplit:       0.90,
		FirstPayoutSplit:  1.00,
		MinWithdrawal:     500,
		ConsistencyPct:    0.30,
		MinTradingDays:    7,
		PayoutCycleDays:   8,
		DefaultEvalFee:    147,
		DefaultActivation: 85,
		DefaultResetFee:   80,
		Website:           "https://apextraderfunding.com",
		Notes:             "30% consistency rule on PA; 8-day minimum between payouts; multiple-account stacking allowed.",
	},
	{
		ID:                "tpt",
		Name:              "Take Profit Trader",
		ProfitSplit:       0.80,
		FirstPayoutSplit:  0.80,
		MinWithdrawal:     250,
		ConsistencyPct:    0,
		MinTradingDays:    5,
		PayoutCycleDays:   14,
		DefaultEvalFee:    150,
		DefaultActivation: 130,
		DefaultResetFee:   99,
		Website:           "https://takeprofittrader.com",
		Notes:             "80/20 split until first $12.5k, then 90/10.",
	},
	{
		ID:                "tradeday",
		Name:              "TradeDay",
		ProfitSplit:       0.90,
		FirstPayoutSplit:  1.00,
		MinWithdrawal:     0,
		ConsistencyPct:    0,
		MinTradingDays:    3,
		PayoutCycleDays:   0,
		DefaultEvalFee:    99,
		DefaultActivation: 0,
		DefaultResetFee:   99,
		Website:           "https://tradeday.com",
		Notes:             "No activation fee; 3-day minimum; weekly payouts.",
	},
	{
		ID:                "mff",
		Name:              "My Funded Futures",
		ProfitSplit:       0.90,
		FirstPayoutSplit:  1.00,
		MinWithdrawal:     1000,
		ConsistencyPct:    0.40,
		MinTradingDays:    5,
		PayoutCycleDays:   0,
		DefaultEvalFee:    165,
		DefaultActivation: 135,
		DefaultResetFee:   99,
		Website:           "https://myfundedfutures.com",
		Notes:             "Keeps first $10k at 100%.",
	},
}

// GetPropFirm returns a firm by ID, or nil if unknown.
func GetPropFirm(id string) *PropFirm {
	for i := range PropFirmRegistry {
		if PropFirmRegistry[i].ID == id {
			return &PropFirmRegistry[i]
		}
	}
	return nil
}
