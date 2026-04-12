package cfo

import (
	"fmt"
	"log/slog"
	"time"
)

// Source tags for unified finance data.
const (
	SourcePersonal = "personal" // CFO Engine (Firefly III)
	SourceFamily   = "family"   // Monarch Money
	SourceTrading  = "trading"  // Primaris Council
)

// UnifiedAccount is a normalized account from any source.
type UnifiedAccount struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Source      string  `json:"source"`      // personal, family, trading
	Type        string  `json:"type"`        // checking, savings, credit, investment, trading
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
	Institution string  `json:"institution"`
	UpdatedAt   string  `json:"updated_at"`
}

// UnifiedTransaction is a normalized transaction from any source.
type UnifiedTransaction struct {
	ID          string  `json:"id"`
	Date        string  `json:"date"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Source      string  `json:"source"`
	Account     string  `json:"account"`
	IsRecurring bool    `json:"is_recurring,omitempty"`
}

// UnifiedBudget is a normalized budget category.
type UnifiedBudget struct {
	Category string  `json:"category"`
	Budgeted float64 `json:"budgeted"`
	Spent    float64 `json:"spent"`
	Source   string  `json:"source"`
}

// UnifiedBill is a normalized recurring bill.
type UnifiedBill struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	Source   string  `json:"source"`   // personal, family, trading
	Kind     string  `json:"kind"`     // "life" (real-world bills) or "system" (trading infra costs)
	Category string  `json:"category"` // monarch category or manual category
	Repeat   string  `json:"repeat_freq"`
	Currency string  `json:"currency"`
}

// UnifiedGoal is a normalized savings goal.
type UnifiedGoal struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	TargetAmount  float64 `json:"target_amount"`
	CurrentAmount float64 `json:"current_amount"`
	Percentage    float64 `json:"percentage"`
	Source        string  `json:"source"`
	Notes         string  `json:"notes,omitempty"`
	TargetDate    string  `json:"target_date,omitempty"`
}

// FinancialOverview is the unified snapshot Council presents.
type FinancialOverview struct {
	// Net worth
	PersonalNetWorth float64 `json:"personal_net_worth"`
	FamilyNetWorth   float64 `json:"family_net_worth"`
	TradingValue     float64 `json:"trading_value"`
	TotalNetWorth    float64 `json:"total_net_worth"`

	// Cash flow (current month)
	PersonalIncome   float64 `json:"personal_income"`
	PersonalExpenses float64 `json:"personal_expenses"`
	FamilyIncome     float64 `json:"family_income"`
	FamilyExpenses   float64 `json:"family_expenses"`
	TradingPnL       float64 `json:"trading_pnl"`

	// Accounts by source
	Accounts []UnifiedAccount `json:"accounts"`

	// Upcoming bills
	Bills []UnifiedBill `json:"bills"`

	// Recent transactions (last 30 days, merged)
	RecentTransactions []UnifiedTransaction `json:"recent_transactions,omitempty"`

	AsOf time.Time `json:"as_of"`
}

// CouncilCFO orchestrates data from all financial sources.
type CouncilCFO struct {
	firefly         *FireflyClient
	monarch         *MonarchClient
	logger          *slog.Logger
	defaultCurrency string // cached from Firefly, falls back to "USD"
}

// NewCouncilCFO creates the unified finance orchestrator.
// Either client can be nil if not configured.
func NewCouncilCFO(firefly *FireflyClient, monarch *MonarchClient, logger *slog.Logger) *CouncilCFO {
	c := &CouncilCFO{
		firefly:         firefly,
		monarch:         monarch,
		logger:          logger,
		defaultCurrency: "USD",
	}
	// Pull default currency from Firefly — it owns all currency decisions
	if firefly != nil {
		accounts, err := firefly.ListAccounts()
		if err == nil {
			for _, a := range accounts {
				if a.AccountRole == "defaultAsset" && a.Currency != "" {
					c.defaultCurrency = a.Currency
					break
				}
			}
		}
	}
	return c
}

// DefaultCurrency returns the base currency from Firefly III.
// All non-Firefly sources (Monarch, trading) use this for consistency.
func (c *CouncilCFO) DefaultCurrency() string {
	return c.defaultCurrency
}

// GetOverview builds the unified financial overview.
func (c *CouncilCFO) GetOverview() (*FinancialOverview, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	overview := &FinancialOverview{AsOf: now}

	// Pull personal finance data from CFO Engine
	if c.firefly != nil {
		if err := c.loadFireflyData(overview, monthStart, now); err != nil {
			c.logger.Warn("failed to load CFO Engine data", "error", err)
		}
	}

	// Pull family finance data from Monarch
	if c.monarch != nil {
		if err := c.loadMonarchData(overview, monthStart, now); err != nil {
			c.logger.Warn("failed to load Monarch data", "error", err)
		}
	}

	// Total net worth across all sources
	overview.TotalNetWorth = overview.PersonalNetWorth + overview.FamilyNetWorth + overview.TradingValue

	return overview, nil
}

func (c *CouncilCFO) loadFireflyData(o *FinancialOverview, start, end time.Time) error {
	// Accounts
	accounts, err := c.firefly.ListAccounts()
	if err != nil {
		return fmt.Errorf("firefly accounts: %w", err)
	}
	for _, a := range accounts {
		if !a.Active {
			continue
		}
		ua := UnifiedAccount{
			ID:       "ff-" + a.ID,
			Name:     a.Name,
			Source:   SourcePersonal,
			Type:     a.AccountRole,
			Balance:  a.Balance,
			Currency: a.Currency,
		}
		o.Accounts = append(o.Accounts, ua)
		if a.IncludeNetWorth {
			o.PersonalNetWorth += a.Balance
		}
	}

	// Summary for income/expenses
	summary, err := c.firefly.GetSummary(start, end)
	if err != nil {
		c.logger.Warn("firefly summary failed", "error", err)
	} else {
		for k, v := range summary {
			switch k {
			case "spent-in-budgets", "left-to-spend-in-budgets":
				// skip derived values
			default:
				if k == "earned-in-budgets" || k == "earned" {
					o.PersonalIncome += v.Value
				}
				if k == "spent" {
					o.PersonalExpenses += -v.Value // Firefly reports spending as negative
				}
			}
		}
	}

	// Bills
	bills, err := c.firefly.ListBills()
	if err != nil {
		c.logger.Warn("firefly bills failed", "error", err)
	} else {
		for _, b := range bills {
			o.Bills = append(o.Bills, UnifiedBill{
				ID:       "ff-" + b.ID,
				Name:     b.Name,
				Amount:   (b.AmountMin + b.AmountMax) / 2,
				Source:   SourcePersonal,
				Kind:     "system",
				Repeat:   b.Repeat,
				Currency: b.Currency,
			})
		}
	}

	// Recent transactions
	txns, err := c.firefly.ListTransactions(start, end)
	if err != nil {
		c.logger.Warn("firefly transactions failed", "error", err)
	} else {
		for _, t := range txns {
			o.RecentTransactions = append(o.RecentTransactions, UnifiedTransaction{
				ID:          "ff-" + t.ID,
				Date:        t.Date,
				Amount:      t.Amount,
				Description: t.Description,
				Category:    t.Category,
				Source:      SourcePersonal,
				Account:     t.Source,
			})
		}
	}

	return nil
}

func (c *CouncilCFO) loadMonarchData(o *FinancialOverview, start, end time.Time) error {
	// Accounts
	accounts, err := c.monarch.ListAccounts()
	if err != nil {
		return fmt.Errorf("monarch accounts: %w", err)
	}
	for _, a := range accounts {
		if a.IsHidden {
			continue
		}
		ua := UnifiedAccount{
			ID:          "mn-" + a.ID,
			Name:        a.DisplayName,
			Source:      SourceFamily,
			Type:        a.Type,
			Balance:     a.Balance,
			Currency:    c.defaultCurrency,
			Institution: a.Institution,
			UpdatedAt:   a.UpdatedAt,
		}
		o.Accounts = append(o.Accounts, ua)
		o.FamilyNetWorth += a.Balance
	}

	// Cash flow
	cashFlow, err := c.monarch.GetCashFlow(start, end)
	if err != nil {
		c.logger.Warn("monarch cash flow failed", "error", err)
	} else {
		o.FamilyIncome = cashFlow.Income
		o.FamilyExpenses = cashFlow.Expenses
	}

	// Transactions
	txns, err := c.monarch.ListTransactions(start, end)
	if err != nil {
		c.logger.Warn("monarch transactions failed", "error", err)
	} else {
		for _, t := range txns {
			o.RecentTransactions = append(o.RecentTransactions, UnifiedTransaction{
				ID:          "mn-" + t.ID,
				Date:        t.Date,
				Amount:      t.Amount,
				Description: t.Merchant,
				Category:    t.Category,
				Source:      SourceFamily,
				Account:     t.Account,
				IsRecurring: t.IsRecurring,
			})
		}
	}

	return nil
}

// ─── Council Integration Methods ──────────────────────────
// These methods power the Council endpoints, replacing the in-memory store
// for household finance data while keeping trading data in Primarch.

// GetBills returns bills from Firefly III as unified bills.
func (c *CouncilCFO) GetBills() ([]UnifiedBill, error) {
	if c.firefly == nil {
		return nil, fmt.Errorf("cfo engine not configured")
	}
	bills, err := c.firefly.ListBills()
	if err != nil {
		return nil, err
	}
	out := make([]UnifiedBill, len(bills))
	for i, b := range bills {
		out[i] = UnifiedBill{
			ID:       "ff-" + b.ID,
			Name:     b.Name,
			Amount:   (b.AmountMin + b.AmountMax) / 2,
			Source:   SourcePersonal,
			Kind:     "system", // CFO Engine tracks trading infra costs
			Repeat:   b.Repeat,
			Currency: b.Currency,
		}
	}
	return out, nil
}

// GetBudgets returns budgets from Firefly III and Monarch Money, merged.
func (c *CouncilCFO) GetBudgets() ([]UnifiedBudget, error) {
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, -1)

	var out []UnifiedBudget

	// Firefly III budgets (personal)
	if c.firefly != nil {
		budgets, err := c.firefly.ListBudgets()
		if err != nil {
			c.logger.Warn("firefly budgets failed", "error", err)
		} else {
			for _, b := range budgets {
				if !b.Active {
					continue
				}
				ub := UnifiedBudget{
					Category: b.Name,
					Source:   SourcePersonal,
				}
				limits, err := c.firefly.GetBudgetLimits(b.ID, monthStart, monthEnd)
				if err != nil {
					c.logger.Warn("budget limits failed", "budget", b.Name, "error", err)
				} else if len(limits) > 0 {
					ub.Budgeted = limits[0].Amount
					ub.Spent = limits[0].Spent
				}
				out = append(out, ub)
			}
		}
	}

	// Monarch Money budgets (family)
	if c.monarch != nil {
		mBudgets, err := c.monarch.GetBudgets(monthStart, monthEnd)
		if err != nil {
			c.logger.Warn("monarch budgets failed", "error", err)
		} else {
			for _, mb := range mBudgets {
				out = append(out, UnifiedBudget{
					Category: mb.Category,
					Budgeted: mb.Budgeted,
					Spent:    mb.Spent,
					Source:   SourceFamily,
				})
			}
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no budget sources configured")
	}
	return out, nil
}

// GetGoals returns piggy banks from Firefly III as unified goals.
func (c *CouncilCFO) GetGoals() ([]UnifiedGoal, error) {
	if c.firefly == nil {
		return nil, fmt.Errorf("cfo engine not configured")
	}
	piggyBanks, err := c.firefly.ListPiggyBanks()
	if err != nil {
		return nil, err
	}
	var out []UnifiedGoal
	for _, pb := range piggyBanks {
		if !pb.Active {
			continue
		}
		out = append(out, UnifiedGoal{
			ID:            "ff-" + pb.ID,
			Name:          pb.Name,
			TargetAmount:  pb.TargetAmount,
			CurrentAmount: pb.CurrentAmount,
			Percentage:    pb.Percentage,
			Source:        SourcePersonal,
			Notes:         pb.Notes,
			TargetDate:    pb.TargetDate,
		})
	}
	return out, nil
}

// MonthlyFinanceMetrics holds income/expense/net for the current month.
type MonthlyFinanceMetrics struct {
	Income   float64 `json:"income"`
	Expenses float64 `json:"expenses"`
	Net      float64 `json:"net"`
}

// GetMonthlyMetrics returns current month income/expenses from Firefly III.
func (c *CouncilCFO) GetMonthlyMetrics() (*MonthlyFinanceMetrics, error) {
	if c.firefly == nil {
		return nil, fmt.Errorf("cfo engine not configured")
	}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	summary, err := c.firefly.GetSummary(monthStart, now)
	if err != nil {
		return nil, err
	}
	m := &MonthlyFinanceMetrics{}
	for k, v := range summary {
		switch {
		case k == "earned" || k == "earned-in-budgets":
			m.Income += v.Value
		case k == "spent":
			m.Expenses += -v.Value // Firefly reports spending as negative
		}
	}
	m.Net = m.Income - m.Expenses
	return m, nil
}

// BillingSummary holds unified billing data for the Council screen.
type BillingSummary struct {
	TotalExpenses   float64       `json:"total_expenses"`
	LifeExpenses    float64       `json:"life_expenses"`    // Real-world bills (rent, utils, etc.)
	SystemExpenses  float64       `json:"system_expenses"`  // Trading infra (prop fees, data feeds)
	TotalPaid       float64       `json:"total_paid"`
	TotalPending    float64       `json:"total_pending"`
	TradingCoverage float64       `json:"trading_coverage"` // Trading income / life expenses
	Bills           []UnifiedBill `json:"bills"`
}

// monthlyAmount normalizes a bill to its monthly cost.
func monthlyAmount(b UnifiedBill) float64 {
	switch b.Repeat {
	case "weekly":
		return b.Amount * 4.33
	case "half-year":
		return b.Amount / 6
	case "yearly":
		return b.Amount / 12
	case "quarterly":
		return b.Amount / 3
	default:
		return b.Amount
	}
}

// GetBillingSummary builds a billing overview from all sources.
func (c *CouncilCFO) GetBillingSummary(tradingIncome float64) (*BillingSummary, error) {
	var allBills []UnifiedBill

	// Firefly III bills (system costs — prop firm fees, data feeds, etc.)
	if c.firefly != nil {
		bills, err := c.GetBills()
		if err != nil {
			c.logger.Warn("firefly bills failed", "error", err)
		} else {
			for i := range bills {
				bills[i].Kind = "system"
			}
			allBills = append(allBills, bills...)
		}
	}

	// Monarch recurring transactions (real life bills)
	if c.monarch != nil {
		recurring, err := c.monarch.GetRecurringTransactions()
		if err != nil {
			c.logger.Warn("monarch recurring failed", "error", err)
		} else {
			for _, t := range recurring {
				amt := t.Amount
				if amt < 0 {
					amt = -amt // expenses come as negative
				}
				allBills = append(allBills, UnifiedBill{
					ID:       "mn-" + t.ID,
					Name:     t.Merchant,
					Amount:   amt,
					Source:   SourceFamily,
					Kind:     "life",
					Category: t.Category,
					Repeat:   "monthly",
					Currency: c.defaultCurrency,
				})
			}
		}
	}

	s := &BillingSummary{Bills: allBills}
	for _, b := range allBills {
		monthly := monthlyAmount(b)
		s.TotalExpenses += monthly
		if b.Kind == "life" {
			s.LifeExpenses += monthly
		} else {
			s.SystemExpenses += monthly
		}
	}
	s.TotalPending = s.TotalExpenses - s.TotalPaid

	// Trading coverage = trading income vs life expenses (the real question)
	if s.LifeExpenses > 0 && tradingIncome > 0 {
		s.TradingCoverage = tradingIncome / s.LifeExpenses
		if s.TradingCoverage > 1 {
			s.TradingCoverage = 1
		}
	}
	return s, nil
}

// RecordPayoutTransaction creates a deposit transaction in Firefly III
// when a trading payout is recorded. Tags it for tracking.
func (c *CouncilCFO) RecordPayoutTransaction(accountName string, amount float64, destination string) error {
	if c.firefly == nil {
		return fmt.Errorf("cfo engine not configured")
	}
	txn := FFTransactionStore{
		Type:            "deposit",
		Description:     fmt.Sprintf("Trading payout: %s", accountName),
		Date:            time.Now().Format("2006-01-02"),
		Amount:          fmt.Sprintf("%.2f", amount),
		SourceName:      accountName,
		DestinationName: destination,
		CategoryName:    "Trading Income",
		Tags:            []string{"trading-payout", "astartes"},
	}
	return c.firefly.CreateTransaction(txn)
}

// Available returns true if at least the Firefly III client is configured.
func (c *CouncilCFO) Available() bool {
	return c.firefly != nil
}

// PingAll checks connectivity to all configured sources.
func (c *CouncilCFO) PingAll() map[string]string {
	status := make(map[string]string)
	if c.firefly != nil {
		if err := c.firefly.Ping(); err != nil {
			status["cfo_engine"] = "error: " + err.Error()
		} else {
			status["cfo_engine"] = "connected"
		}
	} else {
		status["cfo_engine"] = "not configured"
	}
	if c.monarch != nil {
		if err := c.monarch.Ping(); err != nil {
			status["monarch"] = "error: " + err.Error()
		} else {
			status["monarch"] = "connected"
		}
	} else {
		status["monarch"] = "not configured"
	}
	return status
}
