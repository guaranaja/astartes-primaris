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
// ENGINE PROTOCOL — Service-to-Service Communication
// ═══════════════════════════════════════════════════════════

// CommandType identifies what action an engine should take.
type CommandType string

const (
	CmdKillSwitch    CommandType = "kill_switch"
	CmdDisableMarine CommandType = "disable_marine"
	CmdEnableMarine  CommandType = "enable_marine"
)

// CommandStatus tracks a command through its lifecycle.
type CommandStatus string

const (
	CommandPending   CommandStatus = "pending"
	CommandAcked     CommandStatus = "acked"
	CommandCompleted CommandStatus = "completed"
	CommandFailed    CommandStatus = "failed"
)

// Command is a directive from Primarch to an engine.
type Command struct {
	ID        string            `json:"id"`
	EngineID  string            `json:"engine_id"`
	Command   CommandType       `json:"command"`
	Scope     string            `json:"scope"`
	Params    map[string]string `json:"params"`
	Status    CommandStatus     `json:"status"`
	Error     string            `json:"error,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// ─── Engine Register Request/Response ───────────────────────

// EngineRegisterRequest is sent by an engine on startup.
type EngineRegisterRequest struct {
	EngineID   string                    `json:"engine_id"`
	EngineType string                    `json:"engine_type"`
	Version    string                    `json:"version"`
	Fortresses []EngineRegisterFortress  `json:"fortresses"`
}

// EngineRegisterFortress is a fortress in the register payload.
type EngineRegisterFortress struct {
	ID         string                   `json:"id"`
	Name       string                   `json:"name"`
	AssetClass string                   `json:"asset_class"`
	Companies  []EngineRegisterCompany  `json:"companies"`
}

// EngineRegisterCompany is a company in the register payload.
type EngineRegisterCompany struct {
	ID      string                  `json:"id"`
	Name    string                  `json:"name"`
	Marines []EngineRegisterMarine  `json:"marines"`
}

// EngineRegisterMarine is a marine in the register payload.
type EngineRegisterMarine struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	StrategyName    string            `json:"strategy_name"`
	StrategyVersion string            `json:"strategy_version"`
	BrokerAccountID string            `json:"broker_account_id"`
	Status          MarineStatus      `json:"status"`
	Schedule        ScheduleConfig    `json:"schedule"`
	Parameters      map[string]string `json:"parameters,omitempty"`
}

// EngineRegisterResponse is returned after a register call.
type EngineRegisterResponse struct {
	EngineID          string `json:"engine_id"`
	FortressesCreated int    `json:"fortresses_created"`
	FortressesUpdated int    `json:"fortresses_updated"`
	CompaniesCreated  int    `json:"companies_created"`
	CompaniesUpdated  int    `json:"companies_updated"`
	MarinesCreated    int    `json:"marines_created"`
	MarinesUpdated    int    `json:"marines_updated"`
}

// ─── Engine Heartbeat Request/Response ──────────────────────

// EngineHeartbeatRequest is sent periodically by an engine.
type EngineHeartbeatRequest struct {
	EngineID string                   `json:"engine_id"`
	Status   string                   `json:"status"`
	Uptime   string                   `json:"uptime"`
	Marines  []EngineHeartbeatMarine  `json:"marines"`
}

// EngineHeartbeatMarine is a marine status update in the heartbeat.
type EngineHeartbeatMarine struct {
	MarineID    string            `json:"marine_id"`
	Status      MarineStatus      `json:"status"`
	DailyPnL    float64           `json:"daily_pnl"`
	CyclesToday int               `json:"cycles_today"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

// EngineHeartbeatResponse is returned after a heartbeat call.
type EngineHeartbeatResponse struct {
	Status   string    `json:"status"`
	Commands []Command `json:"commands,omitempty"`
}

// EngineCommandCompleteRequest acknowledges a command.
type EngineCommandCompleteRequest struct {
	Status string `json:"status"` // "completed" or "failed"
	Error  string `json:"error,omitempty"`
}

// ─── Engine Data Sync ───────────────────────────────────────

// Trade represents a completed round-trip trade.
type Trade struct {
	ID              string            `json:"id"`
	MarineID        string            `json:"marine_id"`
	BrokerAccountID string            `json:"broker_account_id"`
	Symbol          string            `json:"symbol"`
	Side            string            `json:"side"` // "long", "short"
	Quantity        float64           `json:"quantity"`
	EntryPrice      float64           `json:"entry_price"`
	ExitPrice       float64           `json:"exit_price"`
	EntryTime       time.Time         `json:"entry_time"`
	ExitTime        time.Time         `json:"exit_time"`
	PnL             float64           `json:"pnl"`
	Fees            float64           `json:"fees"`
	DurationMs      int64             `json:"duration_ms"`
	Metadata        map[string]string `json:"metadata,omitempty"` // signal_type, regime, exit_reason, confidence, r_multiple, slippage, etc.
}

// Position represents a current open position.
type Position struct {
	MarineID        string  `json:"marine_id"`
	BrokerAccountID string  `json:"broker_account_id"`
	Symbol          string  `json:"symbol"`
	Quantity        float64 `json:"quantity"`
	AveragePrice    float64 `json:"average_price"`
	UnrealizedPnL   float64 `json:"unrealized_pnl"`
	RealizedPnL     float64 `json:"realized_pnl"`
}

// AccountSnapshot represents a point-in-time account balance.
type AccountSnapshot struct {
	BrokerAccountID string    `json:"broker_account_id"`
	Balance         float64   `json:"balance"`
	DailyPnL        float64   `json:"daily_pnl"`
	TotalPnL        float64   `json:"total_pnl"`
	Timestamp       time.Time `json:"timestamp"`
	// Optional fields for auto-creating Council trading accounts
	Name            string   `json:"name,omitempty"`
	Broker          string   `json:"broker,omitempty"`
	PropFirm        string   `json:"prop_firm,omitempty"`        // "topstep", "apex", "tpt", "tradeday", "mff"
	AccountType     string   `json:"account_type,omitempty"`     // "prop", "personal", "paper"
	InitialBalance  float64  `json:"initial_balance,omitempty"`
	ProfitSplit     float64  `json:"profit_split,omitempty"`     // e.g. 0.90
	Status          string   `json:"status,omitempty"`           // "active", "blown", "closed"
	Instruments     []string `json:"instruments,omitempty"`
	MaxLossLimit    float64  `json:"max_loss_limit,omitempty"`
	ProfitTarget    float64  `json:"profit_target,omitempty"`
	DailyLossLimit  float64  `json:"daily_loss_limit,omitempty"`
	TotalPayouts    float64  `json:"total_payouts,omitempty"`
	PayoutCount     int      `json:"payout_count,omitempty"`
	WinningDays     int      `json:"winning_days,omitempty"`
	TotalTradingDays int     `json:"total_trading_days,omitempty"`
	// Enhanced fields (from Fortress Primus v2)
	AccountPhase       string  `json:"account_phase,omitempty"`       // "combine", "fxt", "live", "blown"
	TrailingDD         float64 `json:"trailing_dd,omitempty"`
	SessionHighEquity  float64 `json:"session_high_equity,omitempty"`
	MLLHeadroom        float64 `json:"mll_headroom,omitempty"`
	MLLUsagePct        float64 `json:"mll_usage_pct,omitempty"`
	CombineProgressPct float64 `json:"combine_progress_pct,omitempty"`
	WithdrawalAvail    float64 `json:"withdrawal_available,omitempty"`
	IsLockedOut        bool    `json:"is_locked_out,omitempty"`
	LockoutReason      string  `json:"lockout_reason,omitempty"`
	BrokerBalance      float64 `json:"broker_balance,omitempty"`
	BrokerCanTrade     *bool   `json:"broker_can_trade,omitempty"`
	DailyTrades        int     `json:"daily_trades,omitempty"`
	DailyWins          int     `json:"daily_wins,omitempty"`
	DailyLosses        int     `json:"daily_losses,omitempty"`
	// Combine lifecycle
	CombineNumber    int    `json:"combine_number,omitempty"`
	CombineStartDate string `json:"combine_start_date,omitempty"` // ISO date
	CombinePassDate  string `json:"combine_pass_date,omitempty"`  // ISO date, null if active
	FundedDate       string `json:"funded_date,omitempty"`        // ISO date
	BlownDate        string `json:"blown_date,omitempty"`         // ISO date, null if active
	// Performance metrics
	BestDayPnL     float64 `json:"best_day_pnl,omitempty"`
	ConsistencyPct float64 `json:"consistency_pct,omitempty"` // best day as % of total (TopstepX < 50%)
	AvgDailyPnL    float64 `json:"avg_daily_pnl,omitempty"`
	OverallWinRate float64 `json:"overall_win_rate,omitempty"`
}

// MarketBar represents an OHLCV bar.
type MarketBar struct {
	Time       time.Time `json:"time"`
	Symbol     string    `json:"symbol"`
	Timeframe  string    `json:"timeframe"`
	Source     string    `json:"source,omitempty"`
	Open       float64   `json:"open"`
	High       float64   `json:"high"`
	Low        float64   `json:"low"`
	Close      float64   `json:"close"`
	Volume     int64     `json:"volume"`
	VWAP       float64   `json:"vwap,omitempty"`
	TradeCount int       `json:"trade_count,omitempty"`
}

// ─── Ingestion Requests/Responses ───────────────────────────

// EngineTradesRequest is a bulk trade upload from an engine.
type EngineTradesRequest struct {
	EngineID string  `json:"engine_id"`
	Trades   []Trade `json:"trades"`
}

// EngineTradesResponse reports upsert results.
type EngineTradesResponse struct {
	TradesCreated int `json:"trades_created"`
	TradesSkipped int `json:"trades_skipped"`
	TradesUpdated int `json:"trades_updated"`
}

// EnginePositionsRequest is a position snapshot from an engine.
type EnginePositionsRequest struct {
	EngineID  string     `json:"engine_id"`
	Positions []Position `json:"positions"`
}

// EngineAccountSnapshotRequest is an account balance update.
type EngineAccountSnapshotRequest struct {
	EngineID string            `json:"engine_id"`
	Accounts []AccountSnapshot `json:"accounts"`
}

// EngineBarsRequest is a bulk bar upload from an engine.
type EngineBarsRequest struct {
	EngineID string      `json:"engine_id"`
	Bars     []MarketBar `json:"bars"`
}

// EngineBarsResponse reports upsert results.
type EngineBarsResponse struct {
	BarsCreated int `json:"bars_created"`
	BarsSkipped int `json:"bars_skipped"`
}

// ─── Performance Query Types ────────────────────────────────

// PerformanceStats holds computed trading statistics.
type PerformanceStats struct {
	TotalTrades      int     `json:"total_trades"`
	WinRate          float64 `json:"win_rate"`
	ProfitFactor     float64 `json:"profit_factor"`
	AvgWin           float64 `json:"avg_win"`
	AvgLoss          float64 `json:"avg_loss"`
	BestTrade        float64 `json:"best_trade"`
	WorstTrade       float64 `json:"worst_trade"`
	TotalPnL         float64 `json:"total_pnl"`
	MaxDrawdown      float64 `json:"max_drawdown"`
	AvgDurationMs    int64   `json:"avg_duration_ms"`
	TotalLots        float64 `json:"total_lots"`
	LongPct          float64 `json:"long_pct"`
	DailyPnL         []DailyPnL `json:"daily_pnl,omitempty"`
}

// DailyPnL is a single day's P&L for calendar/chart views.
type DailyPnL struct {
	Date       string  `json:"date"` // "2026-04-07"
	PnL        float64 `json:"pnl"`
	TradeCount int     `json:"trade_count"`
}

// ═══════════════════════════════════════════════════════════
// HOLDINGS — Manual Stock Positions for Wheel Strategy
// ═══════════════════════════════════════════════════════════

// Holding represents a manually-entered stock position.
type Holding struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`
	Quantity    float64   `json:"quantity"`
	AvgCost     float64   `json:"avg_cost"`
	AcquiredAt  string    `json:"acquired_at,omitempty"` // YYYY-MM-DD
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WheelAnalysis contains wheel strategy recommendations for a holding.
type WheelAnalysis struct {
	Symbol         string              `json:"symbol"`
	Quantity       float64             `json:"quantity"`
	AvgCost        float64             `json:"avg_cost"`
	CurrentPrice   float64             `json:"current_price,omitempty"`
	CoveredCalls   []OptionRecommendation `json:"covered_calls"`
	CashSecuredPuts []OptionRecommendation `json:"cash_secured_puts"`
	DataAsOf       time.Time           `json:"data_as_of"`
}

// OptionRecommendation is a single option contract suggestion.
type OptionRecommendation struct {
	Expiration    string  `json:"expiration"`
	Strike        float64 `json:"strike"`
	OptionType    string  `json:"option_type"`
	Bid           float64 `json:"bid"`
	Ask           float64 `json:"ask"`
	Mark          float64 `json:"mark"`
	Volume        int     `json:"volume"`
	OpenInterest  int     `json:"open_interest"`
	DTE           int     `json:"dte"`
	PremiumPerDay float64 `json:"premium_per_day"`
	AnnualReturn  float64 `json:"annual_return_pct"`
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
	// Per-strategy progression tracks
	StrategyTracks []StrategyTrack `json:"strategy_tracks,omitempty"`
}

// StrategyTrack is an independent progression track for a strategy type.
// Each strategy type (prop futures, equities, options) advances on its own
// metrics and has its own Rubicon moment.
type StrategyTrack struct {
	ID           string       `json:"id"`            // e.g. "prop-futures", "equities-momentum"
	Name         string       `json:"name"`           // e.g. "Prop Futures"
	StrategyType string       `json:"strategy_type"`  // "prop_futures", "equities", "options"
	CurrentRank  StrategyRank `json:"current_rank"`
	RubiconDate  string       `json:"rubicon_date,omitempty"` // When they crossed — empty if not yet
	Metrics      TrackMetrics `json:"metrics"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// StrategyRank is the progression rank within a strategy track.
type StrategyRank string

const (
	// Pre-Rubicon ranks (no cash value)
	RankInitiate StrategyRank = "initiate" // Paper/practice only
	RankNeophyte StrategyRank = "neophyte" // In combine/evaluation

	// THE RUBICON — crossing from neophyte to astartes is the defining moment
	RankAstartes StrategyRank = "astartes" // Funded/FXT — crossed the Rubicon

	// Post-Rubicon ranks (earned through funded performance)
	RankVeteran      StrategyRank = "veteran"       // Consistent funded profitability
	RankCaptain      StrategyRank = "captain"        // Multiple funded accounts, scaling
	RankChapterMaster StrategyRank = "chapter_master" // Full autonomy, no prop needed
)

// TrackMetrics holds the success criteria for a strategy track.
// Different strategy types weight these differently.
type TrackMetrics struct {
	TotalTradingDays   int     `json:"total_trading_days"`
	ProfitableDays     int     `json:"profitable_days"`
	WinRate            float64 `json:"win_rate"`
	AvgDailyPnL        float64 `json:"avg_daily_pnl"`
	ConsistencyPct     float64 `json:"consistency_pct"`
	CombinesPassed     int     `json:"combines_passed"`
	AccountsBlown      int     `json:"accounts_blown"`
	AccountsFunded     int     `json:"accounts_funded"`
	FundedPnL          float64 `json:"funded_pnl"`
	TotalPayouts       float64 `json:"total_payouts"`
	ConsecutivePayouts int     `json:"consecutive_payouts"`
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
	PropFirm       string      `json:"prop_firm,omitempty"` // "topstep", "apex", "tpt", "tradeday", "mff" — empty for non-prop
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
	// Risk & drawdown (from engine snapshot)
	MaxLossLimit       float64 `json:"max_loss_limit,omitempty"`
	ProfitTarget       float64 `json:"profit_target,omitempty"`
	DailyPnL           float64 `json:"daily_pnl,omitempty"`
	TrailingDD         float64 `json:"trailing_dd,omitempty"`
	MLLHeadroom        float64 `json:"mll_headroom,omitempty"`
	MLLUsagePct        float64 `json:"mll_usage_pct,omitempty"`
	CombineProgressPct float64 `json:"combine_progress_pct,omitempty"`
	WithdrawalAvail    float64 `json:"withdrawal_available,omitempty"`
	AccountPhase       string  `json:"account_phase,omitempty"` // "combine", "fxt", "live", "blown"
	// Combine lifecycle
	CombineNumber    int    `json:"combine_number,omitempty"`
	CombineStartDate string `json:"combine_start_date,omitempty"`
	CombinePassDate  string `json:"combine_pass_date,omitempty"`
	FundedDate       string `json:"funded_date,omitempty"`
	BlownDate        string `json:"blown_date,omitempty"`
	// Performance metrics
	BestDayPnL       float64 `json:"best_day_pnl,omitempty"`
	ConsistencyPct   float64 `json:"consistency_pct,omitempty"`
	AvgDailyPnL      float64 `json:"avg_daily_pnl,omitempty"`
	OverallWinRate   float64 `json:"overall_win_rate,omitempty"`
	WinningDays      int     `json:"winning_days,omitempty"`
	TotalTradingDays int     `json:"total_trading_days,omitempty"`
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

// PropFee records a fee paid to a prop firm (combine purchase, activation, reset).
// Recording one also posts a withdrawal to Firefly tagged "prop-fee" so the cash
// outflow hits the budget picture.
type PropFee struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id,omitempty"` // optional — eval/reset fees may predate the account row
	PropFirm  string    `json:"prop_firm"`            // firm id (topstep/apex/...)
	FeeType   string    `json:"fee_type"`             // "eval", "activation", "reset", "data", "other"
	Amount    float64   `json:"amount"`
	PaidDate  string    `json:"paid_date"`            // ISO date
	Source    string    `json:"source,omitempty"`     // Firefly source account (e.g. "Personal Checking")
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// PayoutAllocation records where a payout was allocated (ledger entry, no money movement).
type PayoutAllocation struct {
	ID        string    `json:"id"`
	PayoutID  string    `json:"payout_id,omitempty"` // optional link to Payout
	Category  string    `json:"category"`            // "family", "bills", "savings", "trading_capital", "taxes"
	Amount    float64   `json:"amount"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
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

// ═══════════════════════════════════════════════════════════
// WHEEL CYCLE MANAGER — Options Wheel Strategy Tracking
// ═══════════════════════════════════════════════════════════

// WheelCycle tracks one full iteration of the wheel strategy on an underlying.
type WheelCycle struct {
	ID                    string            `json:"id"`
	Underlying            string            `json:"underlying"`
	Status                string            `json:"status"` // selling_puts, assigned, selling_calls, called_away, closed
	Mode                  string            `json:"mode"`   // manual, automated
	MarineID              string            `json:"marine_id,omitempty"`
	Broker                string            `json:"broker,omitempty"`
	StartedAt             time.Time         `json:"started_at"`
	ClosedAt              *time.Time        `json:"closed_at,omitempty"`
	TotalPremiumCollected float64           `json:"total_premium_collected"`
	CostBasis             float64           `json:"cost_basis"`
	SharesHeld            int               `json:"shares_held"`
	Metadata              map[string]string `json:"metadata,omitempty"`
}

// WheelLeg represents a single option trade within a wheel cycle.
type WheelLeg struct {
	ID         string     `json:"id"`
	CycleID    string     `json:"cycle_id"`
	LegType    string     `json:"leg_type"` // csp, covered_call, assignment, called_away, roll, close
	Symbol     string     `json:"symbol"`
	Strike     float64    `json:"strike"`
	Expiration string     `json:"expiration,omitempty"` // YYYY-MM-DD
	OptionType string     `json:"option_type,omitempty"` // P, C
	Quantity   int        `json:"quantity"`
	Premium    float64    `json:"premium"`
	FillPrice  float64    `json:"fill_price"`
	OpenedAt   time.Time  `json:"opened_at"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
	Status     string     `json:"status"` // open, expired, assigned, exercised, closed, rolled
	Notes      string     `json:"notes,omitempty"`
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
	FundedPnL            float64 `json:"funded_pnl"`          // P&L from FXT/live accounts only (real cash)
	SimPnL               float64 `json:"sim_pnl"`             // P&L from combine/paper (no cash value)
	FundedCapital        float64 `json:"funded_capital"`       // Current balance of funded accounts
	AccountsBlown        int     `json:"accounts_blown"`
	AccountsGraduated    int     `json:"accounts_graduated"`
	AccountsFunded       int     `json:"accounts_funded"`      // Currently in FXT or live
	AccountsInCombine    int     `json:"accounts_in_combine"`
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
