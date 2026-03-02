// Package domain defines the core types of the Astartes Primaris platform.
package domain

import "time"

// Fortress represents a datacenter-level grouping (asset class).
type Fortress struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	AssetClass string            `json:"asset_class"` // futures, options, equities
	Companies  []Company         `json:"companies,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// CompanyType categorizes a company within a fortress.
type CompanyType string

const (
	CompanyVeteran CompanyType = "veteran" // Primary live accounts
	CompanyBattle  CompanyType = "battle"  // Secondary / scaling
	CompanyReserve CompanyType = "reserve" // Paper / staging
	CompanyScout   CompanyType = "scout"   // Experimental
)

// Company represents a cluster-level grouping (account group / strategy family).
type Company struct {
	ID         string            `json:"id"`
	FortressID string            `json:"fortress_id"`
	Name       string            `json:"name"`
	Type       CompanyType       `json:"type"`
	Marines    []Marine          `json:"marines,omitempty"`
	RiskLimits RiskLimits        `json:"risk_limits"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// MarineStatus represents the current lifecycle phase of a marine.
type MarineStatus string

const (
	StatusDormant   MarineStatus = "dormant"
	StatusWaking    MarineStatus = "waking"
	StatusOrienting MarineStatus = "orienting"
	StatusDeciding  MarineStatus = "deciding"
	StatusActing    MarineStatus = "acting"
	StatusReporting MarineStatus = "reporting"
	StatusSleeping  MarineStatus = "sleeping"
	StatusFailed    MarineStatus = "failed"
	StatusDisabled  MarineStatus = "disabled"
)

// Marine represents a single strategy instance (the VM equivalent).
type Marine struct {
	ID              string            `json:"id"`
	CompanyID       string            `json:"company_id"`
	Name            string            `json:"name"`
	StrategyName    string            `json:"strategy_name"`
	StrategyVersion string            `json:"strategy_version"`
	BrokerAccountID string            `json:"broker_account_id,omitempty"`
	Status          MarineStatus      `json:"status"`
	Schedule        ScheduleConfig    `json:"schedule"`
	Parameters      map[string]string `json:"parameters,omitempty"`
	Resources       ResourceLimits    `json:"resources"`
	RunnerType      RunnerType        `json:"runner_type"`
	RunnerConfig    RunnerConfig      `json:"runner_config"`
	LastWake        *time.Time        `json:"last_wake,omitempty"`
	LastSleep       *time.Time        `json:"last_sleep,omitempty"`
	CyclesToday     int               `json:"cycles_today"`
	DailyPnL        float64           `json:"daily_pnl"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// ScheduleType determines how a marine is triggered.
type ScheduleType string

const (
	ScheduleInterval ScheduleType = "interval" // Every N duration
	ScheduleCron     ScheduleType = "cron"     // Cron expression
	ScheduleEvent    ScheduleType = "event"    // On Vox event
	ScheduleManual   ScheduleType = "manual"   // On-demand only
)

// ScheduleConfig controls when a marine wakes.
type ScheduleConfig struct {
	Type         ScheduleType  `json:"type"`
	Interval     string        `json:"interval,omitempty"`      // "30s", "1m", "5m"
	Cron         string        `json:"cron,omitempty"`          // "*/5 * * * *"
	EventChannel string        `json:"event_channel,omitempty"` // Vox channel
	TradingHours *TradingHours `json:"trading_hours,omitempty"`
	Enabled      bool          `json:"enabled"`
}

// TradingHours restricts when a marine can be active.
type TradingHours struct {
	Timezone    string   `json:"timezone"`     // "America/Chicago"
	MarketOpen  string   `json:"market_open"`  // "08:30"
	MarketClose string   `json:"market_close"` // "15:00"
	ActiveDays  []string `json:"active_days"`  // ["MON","TUE",...]
}

// RiskLimits define trading boundaries at any level.
type RiskLimits struct {
	MaxPositionSize float64 `json:"max_position_size"`
	MaxDailyLoss    float64 `json:"max_daily_loss"`
	MaxDrawdownPct  float64 `json:"max_drawdown_pct"`
	MaxCapitalRisk  float64 `json:"max_capital_risk"`
	KillSwitch      bool    `json:"kill_switch"`
}

// ResourceLimits control container resource allocation.
type ResourceLimits struct {
	MemoryMB       int `json:"memory_mb"`
	CPUMillicores  int `json:"cpu_millicores"`
	TimeoutSeconds int `json:"timeout_seconds"`
}

// RunnerType determines how a marine's strategy is executed.
type RunnerType string

const (
	RunnerDocker  RunnerType = "docker"  // Run as Docker container
	RunnerProcess RunnerType = "process" // Run as local process (for astartes-futures)
	RunnerRemote  RunnerType = "remote"  // Connect to running process via API
)

// RunnerConfig holds execution configuration for a marine.
type RunnerConfig struct {
	// Docker runner
	Image      string            `json:"image,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Volumes    []string          `json:"volumes,omitempty"`
	Network    string            `json:"network,omitempty"`

	// Process runner (astartes-futures integration)
	Command    string   `json:"command,omitempty"`    // e.g. "python"
	Args       []string `json:"args,omitempty"`       // e.g. ["run_strategy.py", "--config", "es_momentum.yaml"]
	WorkDir    string   `json:"work_dir,omitempty"`   // e.g. "/home/user/astartes-futures"

	// Remote runner
	Endpoint   string `json:"endpoint,omitempty"`     // e.g. "http://localhost:9001"
	HealthPath string `json:"health_path,omitempty"`  // e.g. "/health"
}

// MarineCycle records a single wake-sleep execution cycle.
type MarineCycle struct {
	ID               string       `json:"id"`
	MarineID         string       `json:"marine_id"`
	WakeAt           time.Time    `json:"wake_at"`
	SleepAt          *time.Time   `json:"sleep_at,omitempty"`
	Status           MarineStatus `json:"status"`
	SignalsGenerated int          `json:"signals_generated"`
	OrdersSubmitted  int          `json:"orders_submitted"`
	DurationMs       int64        `json:"duration_ms"`
	Error            string       `json:"error,omitempty"`
}

// SystemEvent represents a lifecycle or health event for the Vox bus.
type SystemEvent struct {
	ID        string                 `json:"id"`
	Service   string                 `json:"service"`
	Event     string                 `json:"event"`
	MarineID  string                 `json:"marine_id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// ═══════════════════════════════════════════════════════════
// COUNCIL — Financial War Room & Career Progression
// ═══════════════════════════════════════════════════════════
//
// The Council manages the business side of trading:
// - Career phase progression (Initiate → Chapter Master)
// - Account tracking (prop, personal, paper)
// - Payout management and withdrawal advice
// - Budget integration and cash flow allocation
// - Business metrics and goal tracking

// Phase represents a stage in the trader's career progression.
// Modeled after Space Marine rank advancement.
type Phase string

const (
	// PhaseInitiate: Prop trading only. Prove you can be consistently profitable.
	// Goal: Generate regular payouts from prop accounts.
	PhaseInitiate Phase = "initiate"

	// PhaseNeophyte: Prop profits fund a personal MES account ($5-10K).
	// Run same strategies on personal account alongside prop.
	PhaseNeophyte Phase = "neophyte"

	// PhaseBattleBrother: Personal account hits $25K. PDT rule eliminated.
	// Options trading unlocked (Fortress Secundus comes online).
	PhaseBattleBrother Phase = "battle_brother"

	// PhaseVeteran: Graduate MES to ES. 10x position size on personal.
	// Personal income begins exceeding prop income.
	PhaseVeteran Phase = "veteran"

	// PhaseCaptain: Drop prop trading entirely. Full autonomy.
	// No profit splits, no rules, no restrictions.
	PhaseCaptain Phase = "captain"

	// PhaseChapterMaster: Full business operation. LLC, tax optimization,
	// multiple asset classes, scaling strategies.
	PhaseChapterMaster Phase = "chapter_master"
)

// PhaseConfig defines a phase's goals, milestones, and unlock criteria.
type PhaseConfig struct {
	Phase       Phase       `json:"phase"`
	Name        string      `json:"name"`
	Title       string      `json:"title"`       // WH40K rank
	Description string      `json:"description"`
	Milestones  []Milestone `json:"milestones"`
	UnlockWhen  []Condition `json:"unlock_when"` // ALL must be true to advance
	Active      bool        `json:"active"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
}

// Milestone is a trackable goal within a phase.
type Milestone struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Target      float64 `json:"target"`
	Current     float64 `json:"current"`
	Unit        string  `json:"unit"` // "usd", "pct", "count", "days"
	Completed   bool    `json:"completed"`
}

// Condition defines a requirement for phase advancement.
type Condition struct {
	Type    string  `json:"type"` // "balance_gte", "payout_count_gte", "win_rate_gte", "profitable_days_gte"
	Target  float64 `json:"target"`
	Current float64 `json:"current"`
	Met     bool    `json:"met"`
	Label   string  `json:"label"`
}

// Roadmap holds the full career progression state.
type Roadmap struct {
	CurrentPhase Phase         `json:"current_phase"`
	Phases       []PhaseConfig `json:"phases"`
	StartedAt    time.Time     `json:"started_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// ─── Accounts & Finances ────────────────────────────────────

// AccountType categorizes trading accounts.
type AccountType string

const (
	AccountProp     AccountType = "prop"     // Funded/prop firm (Apex, ProjectX)
	AccountPersonal AccountType = "personal" // Personal brokerage
	AccountPaper    AccountType = "paper"    // Paper/sim
)

// TradingAccount represents a broker account with financial tracking.
type TradingAccount struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Broker         string      `json:"broker"` // "apex", "projectx", "ibkr", "tastytrade"
	Type           AccountType `json:"type"`
	AccountNumber  string      `json:"account_number,omitempty"`
	InitialBalance float64     `json:"initial_balance"`
	CurrentBalance float64     `json:"current_balance"`
	TotalPnL       float64     `json:"total_pnl"`
	TotalPayouts   float64     `json:"total_payouts"`
	PayoutCount    int         `json:"payout_count"`
	ProfitSplit    float64     `json:"profit_split"` // e.g. 0.90 = you keep 90%
	Status         string      `json:"status"`       // "active", "blown", "graduated", "closed"
	Instruments    []string    `json:"instruments"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// Payout records a withdrawal from a trading account.
type Payout struct {
	ID          string     `json:"id"`
	AccountID   string     `json:"account_id"`
	GrossAmount float64    `json:"gross_amount"` // Before prop firm split
	NetAmount   float64    `json:"net_amount"`   // After split (what you receive)
	Destination string     `json:"destination"`  // "bank", "personal_trading", "savings", "bills"
	Status      string     `json:"status"`       // "pending", "processing", "completed"
	RequestedAt time.Time  `json:"requested_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Note        string     `json:"note,omitempty"`
}

// ─── Budget Integration ─────────────────────────────────────

// BudgetCategory tracks spending/income in a category.
type BudgetCategory struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`    // "Bills", "Trading Capital", "Taxes", "Savings"
	Type    string  `json:"type"`    // "expense", "income", "transfer"
	Monthly float64 `json:"monthly"` // Monthly target/budget
	Current float64 `json:"current"` // Current month spend/income
	Color   string  `json:"color"`
}

// BudgetSummary is the monthly financial overview.
type BudgetSummary struct {
	Month            string           `json:"month"` // "2026-03"
	TradingIncome    float64          `json:"trading_income"`
	PropPayouts      float64          `json:"prop_payouts"`
	PersonalPnL      float64          `json:"personal_pnl"`
	TotalExpenses    float64          `json:"total_expenses"`
	NetCashFlow      float64          `json:"net_cash_flow"`
	SavingsRate      float64          `json:"savings_rate"`
	TradingCapitalIn float64          `json:"trading_capital_in"`
	Categories       []BudgetCategory `json:"categories"`
	Allocations      []Allocation     `json:"allocations"`
}

// Allocation defines how trading income should be distributed.
type Allocation struct {
	Category   string  `json:"category"`   // "bills", "trading_capital", "taxes", "savings", "personal"
	Percentage float64 `json:"percentage"` // Target % of net income
	Amount     float64 `json:"amount"`     // Actual amount this period
}

// ─── Withdrawal Advisor ─────────────────────────────────────

// WithdrawalAdvice is the Council's recommendation on when/how much to withdraw.
type WithdrawalAdvice struct {
	AccountID       string       `json:"account_id"`
	AccountName     string       `json:"account_name"`
	CurrentBalance  float64      `json:"current_balance"`
	AvailableProfit float64      `json:"available_profit"`
	RecommendedAmt  float64      `json:"recommended_amount"`
	Urgency         string       `json:"urgency"` // "now", "soon", "hold", "wait"
	Reason          string       `json:"reason"`
	Allocations     []Allocation `json:"allocations"`
	NextReviewAt    time.Time    `json:"next_review_at"`
}

// ─── Goals & Big Purchases ──────────────────────────────────

// GoalCategory categorizes personal financial goals.
type GoalCategory string

const (
	GoalHomeImprovement GoalCategory = "home_improvement" // Garage, shed, porch
	GoalVehicle         GoalCategory = "vehicle"          // Corvette, truck, etc.
	GoalSavings         GoalCategory = "savings"          // Emergency fund, etc.
	GoalTrading         GoalCategory = "trading"          // Account funding
	GoalDebt            GoalCategory = "debt"             // Paying off debt
	GoalLifestyle       GoalCategory = "lifestyle"        // Vacation, gear, etc.
	GoalBusiness        GoalCategory = "business"         // LLC fees, equipment
)

// GoalStatus tracks the lifecycle of a goal.
type GoalStatus string

const (
	GoalActive    GoalStatus = "active"
	GoalCompleted GoalStatus = "completed"
	GoalPaused    GoalStatus = "paused"
)

// Goal represents a personal financial goal funded by trading income.
type Goal struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Description   string       `json:"description,omitempty"`
	Category      GoalCategory `json:"category"`
	TargetAmount  float64      `json:"target_amount"`
	CurrentAmount float64      `json:"current_amount"`
	Priority      int          `json:"priority"` // 1 = highest, 5 = lowest
	TargetDate    *time.Time   `json:"target_date,omitempty"`
	Status        GoalStatus   `json:"status"`
	Icon          string       `json:"icon,omitempty"` // Emoji or symbol
	PayoutsNeeded int          `json:"payouts_needed"` // Calculated: how many avg payouts to reach goal
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	CompletedAt   *time.Time   `json:"completed_at,omitempty"`
}

// GoalContribution records money allocated toward a goal.
type GoalContribution struct {
	ID        string    `json:"id"`
	GoalID    string    `json:"goal_id"`
	Amount    float64   `json:"amount"`
	Source    string    `json:"source"` // "payout", "manual", "allocation"
	PayoutID  string    `json:"payout_id,omitempty"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ─── Billing & Expenses ─────────────────────────────────────

// ExpenseFrequency defines how often an expense recurs.
type ExpenseFrequency string

const (
	FreqMonthly  ExpenseFrequency = "monthly"
	FreqWeekly   ExpenseFrequency = "weekly"
	FreqBiweekly ExpenseFrequency = "biweekly"
	FreqAnnual   ExpenseFrequency = "annual"
	FreqOneTime  ExpenseFrequency = "one_time"
)

// Expense represents a recurring or one-time bill/expense.
type Expense struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Category   string           `json:"category"` // "rent", "utilities", "subscriptions", "insurance", "trading_fees", "data_feeds"
	Amount     float64          `json:"amount"`
	Frequency  ExpenseFrequency `json:"frequency"`
	DueDay     int              `json:"due_day,omitempty"` // Day of month (1-31)
	AutoPay    bool             `json:"auto_pay"`
	Status     string           `json:"status"` // "active", "paused", "cancelled"
	NextDue    *time.Time       `json:"next_due,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

// Payment records a payment made for an expense.
type Payment struct {
	ID        string    `json:"id"`
	ExpenseID string    `json:"expense_id"`
	Amount    float64   `json:"amount"`
	PaidAt    time.Time `json:"paid_at"`
	Method    string    `json:"method"` // "bank", "trading_income", "manual"
	Note      string    `json:"note,omitempty"`
}

// BillingSummary aggregates expense and payment data for a period.
type BillingSummary struct {
	Month           string    `json:"month"` // "2026-03"
	TotalExpenses   float64   `json:"total_expenses"`
	TotalPaid       float64   `json:"total_paid"`
	TotalPending    float64   `json:"total_pending"`
	TotalOverdue    float64   `json:"total_overdue"`
	TradingCoverage float64   `json:"trading_coverage"` // % of expenses covered by trading income
	Expenses        []Expense `json:"expenses"`
	Payments        []Payment `json:"recent_payments"`
}

// ─── Business Metrics ───────────────────────────────────────

// BusinessMetrics tracks overall business health.
type BusinessMetrics struct {
	LifetimePnL          float64 `json:"lifetime_pnl"`
	LifetimePayouts      float64 `json:"lifetime_payouts"`
	AccountsBlown        int     `json:"accounts_blown"`
	AccountsGraduated    int     `json:"accounts_graduated"`
	TotalTradingDays     int     `json:"total_trading_days"`
	ProfitableDays       int     `json:"profitable_days"`
	MonthlyPnL           float64 `json:"monthly_pnl"`
	MonthlyPayouts       float64 `json:"monthly_payouts"`
	MonthlyExpenses      float64 `json:"monthly_expenses"`
	MonthlyNetIncome     float64 `json:"monthly_net_income"`
	PersonalAccountValue float64 `json:"personal_account_value"`
	PersonalAccountGoal  float64 `json:"personal_account_goal"`
	GoalProgress         float64 `json:"goal_progress"`
	CurrentPhase         Phase   `json:"current_phase"`
	PhaseProgress        float64 `json:"phase_progress"`
	DaysInPhase          int     `json:"days_in_phase"`
}
