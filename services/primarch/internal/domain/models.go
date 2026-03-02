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
