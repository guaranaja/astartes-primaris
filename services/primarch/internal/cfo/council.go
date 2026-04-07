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
	Source   string  `json:"source"`
	Repeat   string  `json:"repeat_freq"`
	Currency string  `json:"currency"`
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
	firefly *FireflyClient
	monarch *MonarchClient
	logger  *slog.Logger
}

// NewCouncilCFO creates the unified finance orchestrator.
// Either client can be nil if not configured.
func NewCouncilCFO(firefly *FireflyClient, monarch *MonarchClient, logger *slog.Logger) *CouncilCFO {
	return &CouncilCFO{
		firefly: firefly,
		monarch: monarch,
		logger:  logger,
	}
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
			Currency:    "USD",
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
