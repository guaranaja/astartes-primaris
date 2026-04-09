package store

import (
	"fmt"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// ─── Trading Accounts ───────────────────────────────────────

func (s *Store) ListAccounts() []domain.TradingAccount {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.TradingAccount, 0, len(s.accounts))
	for _, a := range s.accounts {
		out = append(out, *a)
	}
	return out
}

func (s *Store) GetAccount(id string) (*domain.TradingAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account %q not found", id)
	}
	ac := *a
	return &ac, nil
}

func (s *Store) CreateAccount(a *domain.TradingAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.accounts[a.ID]; exists {
		return fmt.Errorf("account %q already exists", a.ID)
	}
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = "active"
	}
	if a.ProfitSplit == 0 && a.Type == domain.AccountProp {
		a.ProfitSplit = 0.90 // Default 90/10 split
	}
	if a.ProfitSplit == 0 && a.Type == domain.AccountPersonal {
		a.ProfitSplit = 1.0 // You keep 100%
	}
	s.accounts[a.ID] = a
	return nil
}

func (s *Store) UpdateAccount(a *domain.TradingAccount) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.accounts[a.ID]; !exists {
		return fmt.Errorf("account %q not found", a.ID)
	}
	a.UpdatedAt = time.Now()
	s.accounts[a.ID] = a
	return nil
}

func (s *Store) DeleteAccount(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.accounts[id]; !exists {
		return fmt.Errorf("account %q not found", id)
	}
	delete(s.accounts, id)
	return nil
}

// ─── Payouts ────────────────────────────────────────────────

func (s *Store) ListPayouts(accountID string) []domain.Payout {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Payout
	for _, p := range s.payouts {
		if accountID == "" || p.AccountID == accountID {
			out = append(out, p)
		}
	}
	return out
}

func (s *Store) RecordPayout(p domain.Payout) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[p.AccountID]
	if !ok {
		return fmt.Errorf("account %q not found", p.AccountID)
	}
	// Calculate net amount after split
	if p.NetAmount == 0 {
		p.NetAmount = p.GrossAmount * a.ProfitSplit
	}
	if p.Status == "" {
		p.Status = "completed"
	}
	p.RequestedAt = time.Now()
	s.payouts = append(s.payouts, p)

	// Update account totals
	a.TotalPayouts += p.NetAmount
	a.PayoutCount++
	a.UpdatedAt = time.Now()
	return nil
}

// ─── Budget ─────────────────────────────────────────────────

func (s *Store) GetBudget() *domain.BudgetSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := *s.budget
	return &b
}

func (s *Store) UpdateBudget(b *domain.BudgetSummary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.budget = b
}

func (s *Store) GetAllocations() []domain.Allocation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.Allocation{}, s.allocations...)
}

func (s *Store) SetAllocations(allocs []domain.Allocation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allocations = allocs
}

// ─── Roadmap ────────────────────────────────────────────────

func (s *Store) GetRoadmap() *domain.Roadmap {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r := *s.roadmap
	return &r
}

func (s *Store) UpdateRoadmap(r *domain.Roadmap) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.UpdatedAt = time.Now()
	s.roadmap = r
}

// ─── Withdrawal Advice ──────────────────────────────────────

func (s *Store) GetWithdrawalAdvice() []domain.WithdrawalAdvice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var advice []domain.WithdrawalAdvice
	for _, a := range s.accounts {
		if a.Status != "active" || a.Type != domain.AccountProp {
			continue
		}
		// Only funded accounts (Rubicon crossed) are withdrawal-eligible
		if a.AccountPhase != "fxt" && a.AccountPhase != "live" {
			continue
		}
		profit := a.CurrentBalance - a.InitialBalance
		if profit <= 0 {
			continue
		}

		wa := domain.WithdrawalAdvice{
			AccountID:       a.ID,
			AccountName:     a.Name,
			CurrentBalance:  a.CurrentBalance,
			AvailableProfit: profit,
			NextReviewAt:    time.Now().AddDate(0, 0, 7),
		}

		// Withdrawal logic
		net := profit * a.ProfitSplit
		switch {
		case net >= 2000:
			wa.Urgency = "now"
			wa.RecommendedAmt = net
			wa.Reason = fmt.Sprintf("$%.0f available — withdraw to protect gains and fund goals", net)
		case net >= 1000:
			wa.Urgency = "soon"
			wa.RecommendedAmt = net
			wa.Reason = fmt.Sprintf("$%.0f available — good time to take profits", net)
		case net >= 500:
			wa.Urgency = "hold"
			wa.RecommendedAmt = 0
			wa.Reason = "Building cushion — let it grow unless you need cash flow"
		default:
			wa.Urgency = "wait"
			wa.RecommendedAmt = 0
			wa.Reason = "Keep trading — not enough profit to justify withdrawal fees"
		}

		// Distribute recommended withdrawal across allocations
		if wa.RecommendedAmt > 0 {
			wa.Allocations = s.distributeWithdrawal(wa.RecommendedAmt)
		}

		// Add goal context to reason
		if topGoal := s.topActiveGoal(); topGoal != nil {
			remaining := topGoal.TargetAmount - topGoal.CurrentAmount
			if remaining > 0 {
				needed := s.calcPayoutsNeeded(remaining)
				if needed > 0 {
					wa.Reason += fmt.Sprintf(" (%d payouts to %s)", needed, topGoal.Name)
				}
			}
		}

		advice = append(advice, wa)
	}
	return advice
}

// topActiveGoal returns the highest-priority active goal. Must hold read lock.
func (s *Store) topActiveGoal() *domain.Goal {
	var top *domain.Goal
	for _, g := range s.goals {
		if g.Status != domain.GoalActive {
			continue
		}
		if top == nil || g.Priority < top.Priority {
			top = g
		}
	}
	return top
}

func (s *Store) distributeWithdrawal(amount float64) []domain.Allocation {
	allocs := s.allocations
	if len(allocs) == 0 {
		// Default allocation if none set
		allocs = DefaultAllocations()
	}
	result := make([]domain.Allocation, len(allocs))
	for i, a := range allocs {
		result[i] = domain.Allocation{
			Category:   a.Category,
			Percentage: a.Percentage,
			Amount:     amount * (a.Percentage / 100),
		}
	}
	return result
}

// ─── Business Metrics ───────────────────────────────────────

func (s *Store) GetBusinessMetrics() domain.BusinessMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m := domain.BusinessMetrics{
		CurrentPhase: s.roadmap.CurrentPhase,
	}

	for _, a := range s.accounts {
		m.LifetimePnL += a.TotalPnL
		m.LifetimePayouts += a.TotalPayouts
		if a.Status == "blown" {
			m.AccountsBlown++
		}
		if a.Status == "graduated" {
			m.AccountsGraduated++
		}
		if a.Type == domain.AccountPersonal {
			m.PersonalAccountValue += a.CurrentBalance
		}
		// Cash-value split: funded (fxt/live) vs sim (combine/paper)
		isFunded := (a.AccountPhase == "fxt" || a.AccountPhase == "live") && a.Status == "active"
		if isFunded {
			m.FundedPnL += a.TotalPnL
			m.FundedCapital += a.CurrentBalance
			m.AccountsFunded++
		} else if a.Status != "blown" && a.Status != "graduated" && a.Status != "closed" {
			m.SimPnL += a.TotalPnL
			if a.AccountPhase == "combine" || a.AccountPhase == "" {
				m.AccountsInCombine++
			}
		}
	}

	m.MonthlyPnL = s.budget.TradingIncome
	m.MonthlyPayouts = s.budget.PropPayouts
	m.MonthlyExpenses = s.budget.TotalExpenses
	m.MonthlyNetIncome = s.budget.NetCashFlow

	// Goal based on current phase
	switch s.roadmap.CurrentPhase {
	case domain.PhaseInitiate:
		m.PersonalAccountGoal = 5000
	case domain.PhaseNeophyte:
		m.PersonalAccountGoal = 25000
	case domain.PhaseBattleBrother:
		m.PersonalAccountGoal = 50000
	case domain.PhaseVeteran:
		m.PersonalAccountGoal = 100000
	default:
		m.PersonalAccountGoal = 250000
	}

	if m.PersonalAccountGoal > 0 {
		m.GoalProgress = m.PersonalAccountValue / m.PersonalAccountGoal
		if m.GoalProgress > 1 {
			m.GoalProgress = 1
		}
	}

	// Phase progress
	rm := s.roadmap
	for _, p := range rm.Phases {
		if p.Phase == rm.CurrentPhase && len(p.UnlockWhen) > 0 {
			met := 0
			for _, c := range p.UnlockWhen {
				if c.Met {
					met++
				}
			}
			m.PhaseProgress = float64(met) / float64(len(p.UnlockWhen))
			break
		}
	}

	if s.roadmap.StartedAt.Year() > 2000 {
		m.DaysInPhase = int(time.Since(s.roadmap.StartedAt).Hours() / 24)
	}

	return m
}

// ─── Default Seed Data ──────────────────────────────────────

// DefaultAllocations returns the recommended income allocation.
func DefaultAllocations() []domain.Allocation {
	return []domain.Allocation{
		{Category: "bills", Percentage: 30},
		{Category: "trading_capital", Percentage: 35},
		{Category: "taxes", Percentage: 15},
		{Category: "savings", Percentage: 10},
		{Category: "personal", Percentage: 10},
	}
}

// DefaultRoadmap creates the career progression plan.
func DefaultRoadmap() *domain.Roadmap {
	return &domain.Roadmap{
		CurrentPhase: domain.PhaseInitiate,
		StartedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Phases: []domain.PhaseConfig{
			{
				Phase:       domain.PhaseInitiate,
				Name:        "Initiate",
				Title:       "Scout Marine",
				Description: "Prove yourself in the proving grounds. Trade prop accounts, generate consistent payouts, build your war chest.",
				Active:      true,
				Milestones: []domain.Milestone{
					{ID: "first-payout", Name: "First Payout", Description: "Complete your first successful withdrawal", Target: 1, Unit: "count"},
					{ID: "consistent-payouts", Name: "3 Consecutive Payouts", Description: "Withdraw from prop accounts 3 months in a row", Target: 3, Unit: "count"},
					{ID: "war-chest", Name: "War Chest $5K", Description: "Save $5,000 for personal trading account", Target: 5000, Unit: "usd"},
				},
				UnlockWhen: []domain.Condition{
					{Type: "payout_count_gte", Target: 3, Label: "3+ payouts completed"},
					{Type: "savings_gte", Target: 5000, Label: "$5,000 saved for personal account"},
				},
			},
			{
				Phase:       domain.PhaseNeophyte,
				Name:        "Neophyte",
				Title:       "Battle Brother (MES)",
				Description: "Open your personal MES account. Run the same strategies that proved themselves in prop. Grow to $25K.",
				Milestones: []domain.Milestone{
					{ID: "open-personal", Name: "Open Personal Account", Description: "Fund personal futures account with MES access", Target: 1, Unit: "count"},
					{ID: "first-personal-trade", Name: "First Personal Trade", Description: "Execute your first trade on personal capital", Target: 1, Unit: "count"},
					{ID: "personal-10k", Name: "Personal $10K", Description: "Grow personal account to $10,000", Target: 10000, Unit: "usd"},
					{ID: "personal-25k", Name: "PDT Threshold $25K", Description: "Reach $25,000 — Pattern Day Trader rule eliminated", Target: 25000, Unit: "usd"},
				},
				UnlockWhen: []domain.Condition{
					{Type: "personal_balance_gte", Target: 25000, Label: "Personal account $25K+"},
					{Type: "profitable_months_gte", Target: 3, Label: "3+ profitable months on personal"},
				},
			},
			{
				Phase:       domain.PhaseBattleBrother,
				Name:        "Battle Brother",
				Title:       "Battle Brother (Options)",
				Description: "PDT eliminated. Options trading unlocked. Fortress Secundus comes online. Diversify income streams.",
				Milestones: []domain.Milestone{
					{ID: "options-account", Name: "Options Account", Description: "Open and fund options trading account", Target: 1, Unit: "count"},
					{ID: "first-options-trade", Name: "First Options Trade", Description: "Execute first options strategy", Target: 1, Unit: "count"},
					{ID: "options-profitable", Name: "Options Profitable Month", Description: "First profitable month trading options", Target: 1, Unit: "count"},
					{ID: "personal-50k", Name: "Personal $50K", Description: "Combined personal accounts reach $50K", Target: 50000, Unit: "usd"},
				},
				UnlockWhen: []domain.Condition{
					{Type: "personal_balance_gte", Target: 50000, Label: "Personal accounts $50K+"},
					{Type: "options_profitable_months_gte", Target: 2, Label: "2+ profitable months in options"},
				},
			},
			{
				Phase:       domain.PhaseVeteran,
				Name:        "Veteran",
				Title:       "Veteran Sergeant",
				Description: "Graduate MES to full ES contracts. 10x your position sizing. Personal income exceeds prop income.",
				Milestones: []domain.Milestone{
					{ID: "first-es-trade", Name: "First ES Trade", Description: "Execute first full E-mini S&P contract", Target: 1, Unit: "count"},
					{ID: "es-consistent", Name: "ES Consistency", Description: "3 consecutive profitable months on ES", Target: 3, Unit: "count"},
					{ID: "personal-exceeds-prop", Name: "Personal > Prop Income", Description: "Personal trading income exceeds prop trading income", Target: 1, Unit: "count"},
					{ID: "personal-100k", Name: "Personal $100K", Description: "Personal accounts reach $100K", Target: 100000, Unit: "usd"},
				},
				UnlockWhen: []domain.Condition{
					{Type: "personal_income_exceeds_prop", Target: 1, Label: "Personal income > prop income"},
					{Type: "personal_balance_gte", Target: 100000, Label: "Personal accounts $100K+"},
				},
			},
			{
				Phase:       domain.PhaseCaptain,
				Name:        "Captain",
				Title:       "Company Captain",
				Description: "Drop prop trading. Full autonomy. No splits, no rules, no restrictions. You are the firm.",
				Milestones: []domain.Milestone{
					{ID: "close-prop", Name: "Close Prop Accounts", Description: "Close all prop firm accounts — you don't need them anymore", Target: 1, Unit: "count"},
					{ID: "full-independence", Name: "Full Independence", Description: "All trading on personal accounts for 3+ months", Target: 3, Unit: "count"},
					{ID: "monthly-target", Name: "Monthly Income Target", Description: "Hit your monthly income target from personal trading alone", Target: 1, Unit: "count"},
				},
				UnlockWhen: []domain.Condition{
					{Type: "no_active_prop_accounts", Target: 1, Label: "All prop accounts closed"},
					{Type: "personal_monthly_income_gte", Target: 5000, Label: "$5K+/month from personal trading"},
				},
			},
			{
				Phase:       domain.PhaseChapterMaster,
				Name:        "Chapter Master",
				Title:       "Chapter Master",
				Description: "Full business operation. LLC formed, tax optimized, multiple asset classes, scaling strategies. You are the Imperium.",
				Milestones: []domain.Milestone{
					{ID: "llc-formed", Name: "Form LLC", Description: "Establish business entity for trading operations", Target: 1, Unit: "count"},
					{ID: "tax-optimized", Name: "Tax Strategy", Description: "Implement mark-to-market or entity-level tax optimization", Target: 1, Unit: "count"},
					{ID: "multi-asset", Name: "Multi-Asset", Description: "Profitable in 2+ asset classes (futures + options + equities)", Target: 2, Unit: "count"},
					{ID: "personal-250k", Name: "Quarter Million", Description: "Trading accounts reach $250K", Target: 250000, Unit: "usd"},
				},
				UnlockWhen: []domain.Condition{
					{Type: "balance_gte", Target: 250000, Label: "Accounts reach $250K"},
				},
			},
		},
	}
}

// ─── Goals ──────────────────────────────────────────────────

func (s *Store) ListGoals() []domain.Goal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Goal, 0, len(s.goals))
	for _, g := range s.goals {
		gc := *g
		gc.PayoutsNeeded = s.calcPayoutsNeeded(gc.TargetAmount - gc.CurrentAmount)
		out = append(out, gc)
	}
	return out
}

func (s *Store) GetGoal(id string) (*domain.Goal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.goals[id]
	if !ok {
		return nil, fmt.Errorf("goal %q not found", id)
	}
	gc := *g
	gc.PayoutsNeeded = s.calcPayoutsNeeded(gc.TargetAmount - gc.CurrentAmount)
	return &gc, nil
}

func (s *Store) CreateGoal(g *domain.Goal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.goals[g.ID]; exists {
		return fmt.Errorf("goal %q already exists", g.ID)
	}
	now := time.Now()
	g.CreatedAt = now
	g.UpdatedAt = now
	if g.Status == "" {
		g.Status = domain.GoalActive
	}
	if g.Priority == 0 {
		g.Priority = 3
	}
	s.goals[g.ID] = g
	return nil
}

func (s *Store) UpdateGoal(g *domain.Goal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.goals[g.ID]; !exists {
		return fmt.Errorf("goal %q not found", g.ID)
	}
	g.UpdatedAt = time.Now()
	if g.CurrentAmount >= g.TargetAmount && g.Status == domain.GoalActive {
		g.Status = domain.GoalCompleted
		now := time.Now()
		g.CompletedAt = &now
	}
	s.goals[g.ID] = g
	return nil
}

func (s *Store) DeleteGoal(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.goals[id]; !exists {
		return fmt.Errorf("goal %q not found", id)
	}
	delete(s.goals, id)
	return nil
}

func (s *Store) ContributeToGoal(c domain.GoalContribution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.goals[c.GoalID]
	if !ok {
		return fmt.Errorf("goal %q not found", c.GoalID)
	}
	c.CreatedAt = time.Now()
	s.contributions = append(s.contributions, c)
	g.CurrentAmount += c.Amount
	g.UpdatedAt = time.Now()
	if g.CurrentAmount >= g.TargetAmount && g.Status == domain.GoalActive {
		g.Status = domain.GoalCompleted
		now := time.Now()
		g.CompletedAt = &now
	}
	return nil
}

func (s *Store) ListContributions(goalID string) []domain.GoalContribution {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.GoalContribution
	for _, c := range s.contributions {
		if goalID == "" || c.GoalID == goalID {
			out = append(out, c)
		}
	}
	return out
}

// calcPayoutsNeeded estimates how many payouts to reach a goal amount.
// Must hold at least a read lock.
func (s *Store) calcPayoutsNeeded(remaining float64) int {
	if remaining <= 0 {
		return 0
	}
	// Calculate average payout from history
	if len(s.payouts) == 0 {
		return -1 // Unknown
	}
	total := 0.0
	for _, p := range s.payouts {
		total += p.NetAmount
	}
	avg := total / float64(len(s.payouts))
	if avg <= 0 {
		return -1
	}
	// Only the personal allocation % goes toward goals
	personalPct := 0.10 // Default: 10% personal allocation
	for _, a := range s.allocations {
		if a.Category == "personal" || a.Category == "savings" {
			personalPct += a.Percentage / 100
		}
	}
	if personalPct > 0 {
		perPayout := avg * personalPct
		if perPayout > 0 {
			needed := int(remaining/perPayout) + 1
			return needed
		}
	}
	return -1
}

// ─── Expenses & Billing ─────────────────────────────────────

func (s *Store) ListExpenses() []domain.Expense {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Expense, 0, len(s.expenses))
	for _, e := range s.expenses {
		out = append(out, *e)
	}
	return out
}

func (s *Store) CreateExpense(e *domain.Expense) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.expenses[e.ID]; exists {
		return fmt.Errorf("expense %q already exists", e.ID)
	}
	now := time.Now()
	e.CreatedAt = now
	e.UpdatedAt = now
	if e.Status == "" {
		e.Status = "active"
	}
	s.expenses[e.ID] = e
	return nil
}

func (s *Store) UpdateExpense(e *domain.Expense) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.expenses[e.ID]; !exists {
		return fmt.Errorf("expense %q not found", e.ID)
	}
	e.UpdatedAt = time.Now()
	s.expenses[e.ID] = e
	return nil
}

func (s *Store) DeleteExpense(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.expenses[id]; !exists {
		return fmt.Errorf("expense %q not found", id)
	}
	delete(s.expenses, id)
	return nil
}

func (s *Store) RecordPayment(p domain.Payment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.expenses[p.ExpenseID]; !exists {
		return fmt.Errorf("expense %q not found", p.ExpenseID)
	}
	p.PaidAt = time.Now()
	s.payments = append(s.payments, p)
	return nil
}

func (s *Store) ListPayments(expenseID string) []domain.Payment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Payment
	for _, p := range s.payments {
		if expenseID == "" || p.ExpenseID == expenseID {
			out = append(out, p)
		}
	}
	return out
}

func (s *Store) GetBillingSummary() domain.BillingSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	month := now.Format("2006-01")
	summary := domain.BillingSummary{Month: month}

	// Calculate monthly expense totals
	for _, e := range s.expenses {
		if e.Status != "active" {
			continue
		}
		monthly := e.Amount
		switch e.Frequency {
		case domain.FreqWeekly:
			monthly = e.Amount * 4.33
		case domain.FreqBiweekly:
			monthly = e.Amount * 2.17
		case domain.FreqAnnual:
			monthly = e.Amount / 12
		case domain.FreqOneTime:
			// Only count if due this month
			if e.NextDue != nil && e.NextDue.Format("2006-01") != month {
				continue
			}
		}
		summary.TotalExpenses += monthly
		summary.Expenses = append(summary.Expenses, *e)
	}

	// Count payments this month
	for _, p := range s.payments {
		if p.PaidAt.Format("2006-01") == month {
			summary.TotalPaid += p.Amount
			summary.Payments = append(summary.Payments, p)
		}
	}

	summary.TotalPending = summary.TotalExpenses - summary.TotalPaid
	if summary.TotalPending < 0 {
		summary.TotalPending = 0
	}

	// Trading coverage: what % of bills can trading income cover
	if summary.TotalExpenses > 0 && s.budget != nil {
		summary.TradingCoverage = (s.budget.TradingIncome / summary.TotalExpenses)
		if summary.TradingCoverage > 1 {
			summary.TradingCoverage = 1
		}
	}

	return summary
}

// DefaultBudget creates an empty budget for the current month.
func DefaultBudget() *domain.BudgetSummary {
	now := time.Now()
	return &domain.BudgetSummary{
		Month: now.Format("2006-01"),
		Categories: []domain.BudgetCategory{
			{ID: "bills", Name: "Bills & Rent", Type: "expense", Monthly: 2000, Color: "#ef4444"},
			{ID: "trading-capital", Name: "Trading Capital", Type: "transfer", Monthly: 0, Color: "#60a5fa"},
			{ID: "taxes", Name: "Taxes (set aside)", Type: "expense", Monthly: 0, Color: "#fbbf24"},
			{ID: "savings", Name: "Savings", Type: "transfer", Monthly: 0, Color: "#2dd4a0"},
			{ID: "personal", Name: "Personal / Discretionary", Type: "expense", Monthly: 500, Color: "#a78bfa"},
		},
		Allocations: DefaultAllocations(),
	}
}
