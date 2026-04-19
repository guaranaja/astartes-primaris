package advisor

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/cfo"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/store"
)

// Snapshot captures the user's situation at a point in time. It's durably
// stored with each thread so you can later see "what was true when the
// advisor said X."
type Snapshot struct {
	AsOf          time.Time                 `json:"as_of"`
	Phase         string                    `json:"phase"`
	Accounts      []AccountSummary          `json:"accounts"`
	RecentPayouts []PayoutSummary           `json:"recent_payouts"`
	Metrics       domain.BusinessMetrics    `json:"metrics"`
	Finance       *FinanceSummary           `json:"finance,omitempty"`
	Goals         []domain.Goal             `json:"goals,omitempty"`
	Allocations   []domain.Allocation       `json:"allocations,omitempty"`
}

type AccountSummary struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	PropFirm       string  `json:"prop_firm,omitempty"`
	Type           string  `json:"type"`
	Phase          string  `json:"phase"` // combine | fxt | live
	Balance        float64 `json:"balance"`
	ProfitSplit    float64 `json:"profit_split"`
	TotalPayouts   float64 `json:"total_payouts"`
	TotalPnL       float64 `json:"total_pnl"`
	Status         string  `json:"status"`
	FundedDate     string  `json:"funded_date,omitempty"`
}

type PayoutSummary struct {
	Date        string  `json:"date"`
	AccountID   string  `json:"account_id"`
	Gross       float64 `json:"gross"`
	Net         float64 `json:"net"`
	Destination string  `json:"destination"`
}

type FinanceSummary struct {
	NetWorth         float64 `json:"net_worth"`
	PersonalNetWorth float64 `json:"personal_net_worth"`
	FamilyNetWorth   float64 `json:"family_net_worth"`
	TradingValue     float64 `json:"trading_value"`
	MonthlyIncome    float64 `json:"monthly_income"`
	MonthlyExpenses  float64 `json:"monthly_expenses"`
	MonthlyNet       float64 `json:"monthly_net"`
	Debts            []DebtSummary `json:"debts,omitempty"`
	LiquidCash       float64       `json:"liquid_cash"`
}

type DebtSummary struct {
	Name    string  `json:"name"`
	Balance float64 `json:"balance"` // reported as positive magnitude
	Source  string  `json:"source"`
}

// BuildSnapshot assembles a situation snapshot from all data sources.
// cfo may be nil when CFO Engine isn't configured.
func BuildSnapshot(st store.DataStore, councilCFO *cfo.CouncilCFO) Snapshot {
	now := time.Now()
	snap := Snapshot{AsOf: now}

	// Trading accounts
	for _, a := range st.ListAccounts() {
		snap.Accounts = append(snap.Accounts, AccountSummary{
			ID:           a.ID,
			Name:         a.Name,
			PropFirm:     a.PropFirm,
			Type:         string(a.Type),
			Phase:        a.AccountPhase,
			Balance:      a.CurrentBalance,
			ProfitSplit:  a.ProfitSplit,
			TotalPayouts: a.TotalPayouts,
			TotalPnL:     a.TotalPnL,
			Status:       a.Status,
			FundedDate:   a.FundedDate,
		})
	}

	// Recent payouts (last 90 days)
	cutoff := now.AddDate(0, 0, -90)
	allPayouts := st.ListPayouts("")
	for _, p := range allPayouts {
		if p.RequestedAt.Before(cutoff) {
			continue
		}
		snap.RecentPayouts = append(snap.RecentPayouts, PayoutSummary{
			Date:        p.RequestedAt.Format("2006-01-02"),
			AccountID:   p.AccountID,
			Gross:       p.GrossAmount,
			Net:         p.NetAmount,
			Destination: p.Destination,
		})
	}

	// Metrics + roadmap
	snap.Metrics = st.GetBusinessMetrics()
	if rm := st.GetRoadmap(); rm != nil {
		snap.Phase = string(rm.CurrentPhase)
	}

	// Goals
	snap.Goals = st.ListGoals()

	// Allocation rules
	snap.Allocations = st.GetAllocations()

	// Finance (via CFO)
	if councilCFO != nil && councilCFO.Available() {
		if overview, err := councilCFO.GetOverview(); err == nil {
			f := &FinanceSummary{
				NetWorth:         overview.TotalNetWorth,
				PersonalNetWorth: overview.PersonalNetWorth,
				FamilyNetWorth:   overview.FamilyNetWorth,
				TradingValue:     overview.TradingValue,
				MonthlyIncome:    overview.PersonalIncome + overview.FamilyIncome,
				MonthlyExpenses:  overview.PersonalExpenses + overview.FamilyExpenses,
			}
			f.MonthlyNet = f.MonthlyIncome - f.MonthlyExpenses

			for _, a := range overview.Accounts {
				t := strings.ToLower(a.Type)
				if t == "checking" || t == "savings" || t == "cash" || t == "asset" || t == "depository" || t == "defaultasset" {
					if a.Source != cfo.SourceTrading {
						f.LiquidCash += a.Balance
					}
				}
				// Liabilities: credit cards, loans — Firefly uses "liability", Monarch uses "credit"/"loan"
				if strings.Contains(t, "liab") || strings.Contains(t, "credit") || strings.Contains(t, "loan") || strings.Contains(t, "debt") {
					bal := a.Balance
					if bal < 0 {
						bal = -bal
					}
					if bal > 0 {
						f.Debts = append(f.Debts, DebtSummary{
							Name:    a.Name,
							Balance: bal,
							Source:  a.Source,
						})
					}
				}
			}
			snap.Finance = f
		}
	}

	return snap
}

// AsJSON serializes the snapshot for durable storage.
func (s Snapshot) AsJSON() json.RawMessage {
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	return b
}

// AsMarkdown renders the snapshot as human-readable markdown for the system prompt.
func (s Snapshot) AsMarkdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Current Situation (as of %s)\n\n", s.AsOf.Format("2006-01-02"))

	if s.Phase != "" {
		fmt.Fprintf(&b, "**Roadmap phase:** %s\n\n", s.Phase)
	}

	// Trading accounts
	fmt.Fprintf(&b, "## Trading Accounts (%d)\n\n", len(s.Accounts))
	if len(s.Accounts) == 0 {
		b.WriteString("_No accounts registered._\n\n")
	} else {
		for _, a := range s.Accounts {
			funded := ""
			if a.FundedDate != "" {
				funded = fmt.Sprintf(" · funded %s", a.FundedDate)
			}
			fmt.Fprintf(&b, "- **%s** (%s, %s) — $%.0f balance · P&L $%.0f · payouts $%.0f · split %.0f%% · status %s%s\n",
				a.Name, a.PropFirm, a.Phase, a.Balance, a.TotalPnL, a.TotalPayouts, a.ProfitSplit*100, a.Status, funded)
		}
		b.WriteString("\n")
	}

	// Metrics
	m := s.Metrics
	fmt.Fprintf(&b, "## Trading Performance\n\n")
	fmt.Fprintf(&b, "- Lifetime P&L: $%.0f (funded $%.0f / sim $%.0f)\n", m.LifetimePnL, m.FundedPnL, m.SimPnL)
	fmt.Fprintf(&b, "- Lifetime payouts: $%.0f\n", m.LifetimePayouts)
	fmt.Fprintf(&b, "- Monthly net: $%.0f · expenses $%.0f\n", m.MonthlyNetIncome, m.MonthlyExpenses)
	fmt.Fprintf(&b, "- Funded capital: $%.0f · personal account value: $%.0f\n", m.FundedCapital, m.PersonalAccountValue)
	fmt.Fprintf(&b, "- Profitable days: %d/%d\n\n", m.ProfitableDays, m.TotalTradingDays)

	// Recent payouts
	fmt.Fprintf(&b, "## Recent Payouts (last 90d, %d)\n\n", len(s.RecentPayouts))
	if len(s.RecentPayouts) == 0 {
		b.WriteString("_None._\n\n")
	} else {
		for _, p := range s.RecentPayouts {
			fmt.Fprintf(&b, "- %s — %s: gross $%.0f, net $%.0f → %s\n",
				p.Date, p.AccountID, p.Gross, p.Net, p.Destination)
		}
		b.WriteString("\n")
	}

	// Finance
	if s.Finance != nil {
		f := s.Finance
		fmt.Fprintf(&b, "## Financial State\n\n")
		fmt.Fprintf(&b, "- Net worth: $%.0f (personal $%.0f · family $%.0f · trading $%.0f)\n",
			f.NetWorth, f.PersonalNetWorth, f.FamilyNetWorth, f.TradingValue)
		fmt.Fprintf(&b, "- Monthly income $%.0f · expenses $%.0f · net $%+.0f\n", f.MonthlyIncome, f.MonthlyExpenses, f.MonthlyNet)
		fmt.Fprintf(&b, "- Liquid cash (non-trading): $%.0f\n\n", f.LiquidCash)

		if len(f.Debts) > 0 {
			fmt.Fprintf(&b, "### Debts (%d)\n\n", len(f.Debts))
			for _, d := range f.Debts {
				fmt.Fprintf(&b, "- %s: $%.0f (%s)\n", d.Name, d.Balance, d.Source)
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString("## Financial State\n\n_CFO integration not available — advisor cannot see personal finances this session._\n\n")
	}

	// Allocation rules
	if len(s.Allocations) > 0 {
		b.WriteString("## Income Allocation Rules\n\n")
		for _, a := range s.Allocations {
			fmt.Fprintf(&b, "- %s: %.0f%%\n", a.Category, a.Percentage)
		}
		b.WriteString("\n")
	}

	// Goals
	if len(s.Goals) > 0 {
		fmt.Fprintf(&b, "## Goals (%d)\n\n", len(s.Goals))
		for _, g := range s.Goals {
			pct := 0.0
			if g.TargetAmount > 0 {
				pct = (g.CurrentAmount / g.TargetAmount) * 100
			}
			fmt.Fprintf(&b, "- %s: $%.0f / $%.0f (%.0f%%)\n", g.Name, g.CurrentAmount, g.TargetAmount, pct)
		}
		b.WriteString("\n")
	}

	return b.String()
}
