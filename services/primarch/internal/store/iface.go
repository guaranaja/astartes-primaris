// Package store defines the DataStore interface for Primarch persistence.
package store

import "github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"

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

	// ─── Payouts ────────────────────────────────────────
	ListPayouts(accountID string) []domain.Payout
	RecordPayout(p domain.Payout) error

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
}
