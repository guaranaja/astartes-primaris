// Package store defines the DataStore interface for Primarch persistence.
package store

import (
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// DataStore is the persistence interface for all Primarch data.
// Implementations: MemStore (in-memory) and PGStore (PostgreSQL).
type DataStore interface {
	// ─── Fortress ───────────────────────────────────────
	ListFortresses() []domain.Fortress
	GetFortress(id string) (*domain.Fortress, error)
	CreateFortress(f *domain.Fortress) error
	UpdateFortress(f *domain.Fortress) error
	DeleteFortress(id string) error

	// ─── Company ────────────────────────────────────────
	ListCompanies(fortressID string) []domain.Company
	GetCompany(id string) (*domain.Company, error)
	CreateCompany(c *domain.Company) error
	UpdateCompany(c *domain.Company) error
	DeleteCompany(id string) error

	// ─── Marine ─────────────────────────────────────────
	ListMarines(companyID string) []domain.Marine
	ListAllMarines() []domain.Marine
	GetMarine(id string) (*domain.Marine, error)
	CreateMarine(m *domain.Marine) error
	UpdateMarine(m *domain.Marine) error
	UpdateMarineStatus(id string, status domain.MarineStatus) error
	DeleteMarine(id string) error

	// ─── Cycles ─────────────────────────────────────────
	RecordCycle(c domain.MarineCycle)
	GetCycles(marineID string, limit int) []domain.MarineCycle

	// ─── Kill Switch ────────────────────────────────────
	ActivateKillSwitch(scope string)

	// ─── Accounts ───────────────────────────────────────
	ListAccounts() []domain.TradingAccount
	GetAccount(id string) (*domain.TradingAccount, error)
	CreateAccount(a *domain.TradingAccount) error
	UpdateAccount(a *domain.TradingAccount) error
	DeleteAccount(id string) error

	// ─── Payouts ────────────────────────────────────────
	ListPayouts(accountID string) []domain.Payout
	GetPayout(id string) *domain.Payout
	RecordPayout(p domain.Payout) error
	DeletePayout(id string) error

	// ─── Budget & Allocations ───────────────────────────
	GetBudget() *domain.BudgetSummary
	UpdateBudget(b *domain.BudgetSummary)
	GetAllocations() []domain.Allocation
	SetAllocations(allocs []domain.Allocation)

	// ─── Roadmap ────────────────────────────────────────
	GetRoadmap() *domain.Roadmap
	UpdateRoadmap(r *domain.Roadmap)

	// ─── Withdrawal & Metrics ───────────────────────────
	GetWithdrawalAdvice() []domain.WithdrawalAdvice
	GetBusinessMetrics() domain.BusinessMetrics

	// ─── Goals ──────────────────────────────────────────
	ListGoals() []domain.Goal
	GetGoal(id string) (*domain.Goal, error)
	CreateGoal(g *domain.Goal) error
	UpdateGoal(g *domain.Goal) error
	DeleteGoal(id string) error
	ContributeToGoal(c domain.GoalContribution) error
	ListContributions(goalID string) []domain.GoalContribution

	// ─── Expenses & Billing ─────────────────────────────
	ListExpenses() []domain.Expense
	CreateExpense(e *domain.Expense) error
	UpdateExpense(e *domain.Expense) error
	DeleteExpense(id string) error
	RecordPayment(p domain.Payment) error
	ListPayments(expenseID string) []domain.Payment
	GetBillingSummary() domain.BillingSummary

	// ─── Payout Allocations ────────────────────────────
	RecordAllocation(a domain.PayoutAllocation) error
	ListAllocationsForMonth(year int, month int) []domain.PayoutAllocation

	// ─── Prop Fees ─────────────────────────────────────
	RecordPropFee(f domain.PropFee) error
	ListPropFees(accountID string) []domain.PropFee

	// ─── Holdings ──────────────────────────────────────
	ListHoldings() []domain.Holding
	CreateHolding(h *domain.Holding) error
	UpdateHolding(h *domain.Holding) error
	DeleteHolding(id string) error

	// ─── Wheel Cycles ──────────────────────────────────
	ListWheelCycles() []domain.WheelCycle
	CreateWheelCycle(c *domain.WheelCycle) error
	UpdateWheelCycle(c *domain.WheelCycle) error
	ListWheelLegs(cycleID string) []domain.WheelLeg
	CreateWheelLeg(l *domain.WheelLeg) error
	UpdateWheelLeg(l *domain.WheelLeg) error

	// ─── Commands (Engine Protocol) ────────────────────────
	CreateCommand(c *domain.Command) error
	GetCommand(id string) (*domain.Command, error)
	ListPendingCommands(engineID string) []domain.Command
	UpdateCommand(c *domain.Command) error

	// ─── Trades ────────────────────────────────────────────
	UpsertTrade(t *domain.Trade) (created bool, err error)
	ListTrades(marineID string, since *time.Time, limit int) []domain.Trade

	// ─── Positions ─────────────────────────────────────────
	UpsertPosition(p *domain.Position) error
	ListPositions(marineID string) []domain.Position

	// ─── Account Snapshots ─────────────────────────────────
	RecordAccountSnapshot(s *domain.AccountSnapshot) error

	// ─── Market Bars ───────────────────────────────────────
	UpsertBar(b *domain.MarketBar) (created bool, err error)
}
