package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// getTaxWithholdPct returns the % of net payouts to reserve as a Taxes allocation.
// Default 0.30 (30%) for 1099 prop income; override with TRADING_TAX_WITHHOLD_PCT.
func getTaxWithholdPct() float64 {
	if v := os.Getenv("TRADING_TAX_WITHHOLD_PCT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f < 1 {
			return f
		}
	}
	return 0.30
}

// RegisterCouncilRoutes adds Council endpoints to the server.
func (s *Server) registerCouncilRoutes() {
	// Roadmap & phases
	s.mux.HandleFunc("GET /api/v1/council/roadmap", s.handleGetRoadmap)
	s.mux.HandleFunc("PUT /api/v1/council/roadmap/phase", s.handleAdvancePhase)
	s.mux.HandleFunc("PUT /api/v1/council/roadmap/milestone", s.handleUpdateMilestone)

	// Trading accounts
	s.mux.HandleFunc("GET /api/v1/council/accounts", s.handleListAccounts)
	s.mux.HandleFunc("POST /api/v1/council/accounts", s.handleCreateAccount)
	s.mux.HandleFunc("DELETE /api/v1/council/accounts", s.handleDeleteAllAccounts)
	s.mux.HandleFunc("GET /api/v1/council/accounts/{id}", s.handleGetAccount)
	s.mux.HandleFunc("PUT /api/v1/council/accounts/{id}", s.handleUpdateAccount)
	s.mux.HandleFunc("DELETE /api/v1/council/accounts/{id}", s.handleDeleteAccount)
	s.mux.HandleFunc("POST /api/v1/council/accounts/{id}/mark-passed", s.handleMarkCombinePassed)

	// Prop firm registry
	s.mux.HandleFunc("GET /api/v1/council/prop-firms", s.handleListPropFirms)

	// Prop fees (eval / activation / reset — post to Firefly as withdrawals)
	s.mux.HandleFunc("GET /api/v1/council/prop-fees", s.handleListPropFees)
	s.mux.HandleFunc("POST /api/v1/council/prop-fees", s.handleRecordPropFee)

	// Payouts
	s.mux.HandleFunc("GET /api/v1/council/payouts", s.handleListPayouts)
	s.mux.HandleFunc("POST /api/v1/council/payouts", s.handleRecordPayout)

	// Budget
	s.mux.HandleFunc("GET /api/v1/council/budget", s.handleGetBudget)
	s.mux.HandleFunc("PUT /api/v1/council/budget", s.handleUpdateBudget)

	// Allocations
	s.mux.HandleFunc("GET /api/v1/council/allocations", s.handleGetAllocations)
	s.mux.HandleFunc("PUT /api/v1/council/allocations", s.handleSetAllocations)

	// Withdrawal advice
	s.mux.HandleFunc("GET /api/v1/council/withdrawal-advice", s.handleWithdrawalAdvice)

	// Business metrics
	s.mux.HandleFunc("GET /api/v1/council/metrics", s.handleBusinessMetrics)

	// Goals
	s.mux.HandleFunc("GET /api/v1/council/goals", s.handleListGoals)
	s.mux.HandleFunc("POST /api/v1/council/goals", s.handleCreateGoal)
	s.mux.HandleFunc("GET /api/v1/council/goals/{id}", s.handleGetGoal)
	s.mux.HandleFunc("PUT /api/v1/council/goals/{id}", s.handleUpdateGoal)
	s.mux.HandleFunc("DELETE /api/v1/council/goals/{id}", s.handleDeleteGoal)
	s.mux.HandleFunc("POST /api/v1/council/goals/{id}/contribute", s.handleContributeGoal)

	// Billing & Expenses
	s.mux.HandleFunc("GET /api/v1/council/expenses", s.handleListExpenses)
	s.mux.HandleFunc("POST /api/v1/council/expenses", s.handleCreateExpense)
	s.mux.HandleFunc("PUT /api/v1/council/expenses/{id}", s.handleUpdateExpense)
	s.mux.HandleFunc("DELETE /api/v1/council/expenses/{id}", s.handleDeleteExpense)
	s.mux.HandleFunc("POST /api/v1/council/expenses/{id}/pay", s.handlePayExpense)
	s.mux.HandleFunc("GET /api/v1/council/billing", s.handleBillingSummary)

	// Payout allocations
	s.mux.HandleFunc("POST /api/v1/council/allocations/record", s.handleRecordAllocation)
	s.mux.HandleFunc("GET /api/v1/council/allocations/history", s.handleListAllocations)

	// Unified budgets (Firefly III + Monarch)
	s.mux.HandleFunc("GET /api/v1/council/budgets", s.handleUnifiedBudgets)
}

// ─── Roadmap ────────────────────────────────────────────────

func (s *Server) handleGetRoadmap(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetRoadmap())
}

func (s *Server) handleAdvancePhase(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phase domain.Phase `json:"phase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	rm := s.store.GetRoadmap()
	rm.CurrentPhase = req.Phase
	for i := range rm.Phases {
		rm.Phases[i].Active = rm.Phases[i].Phase == req.Phase
	}
	s.store.UpdateRoadmap(rm)
	s.logger.Info("phase advanced", "phase", req.Phase)
	writeJSON(w, http.StatusOK, rm)
}

func (s *Server) handleUpdateMilestone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phase       domain.Phase `json:"phase"`
		MilestoneID string       `json:"milestone_id"`
		Current     float64      `json:"current"`
		Completed   bool         `json:"completed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	rm := s.store.GetRoadmap()
	for i, p := range rm.Phases {
		if p.Phase == req.Phase {
			for j, m := range p.Milestones {
				if m.ID == req.MilestoneID {
					rm.Phases[i].Milestones[j].Current = req.Current
					rm.Phases[i].Milestones[j].Completed = req.Completed
					break
				}
			}
			break
		}
	}
	s.store.UpdateRoadmap(rm)
	writeJSON(w, http.StatusOK, rm)
}

// ─── Accounts ───────────────────────────────────────────────

func (s *Server) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListAccounts())
}

func (s *Server) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	var a domain.TradingAccount
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if a.ID == "" || a.Name == "" || a.Broker == "" {
		writeError(w, http.StatusBadRequest, "id, name, and broker are required")
		return
	}
	if err := s.store.CreateAccount(&a); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.logger.Info("trading account created", "id", a.ID, "broker", a.Broker, "type", a.Type)
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, err := s.store.GetAccount(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleUpdateAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var a domain.TradingAccount
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	a.ID = id
	if err := s.store.UpdateAccount(&a); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteAccount(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.logger.Info("trading account deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteAllAccounts(w http.ResponseWriter, r *http.Request) {
	accounts := s.store.ListAccounts()
	for _, a := range accounts {
		s.store.DeleteAccount(a.ID)
	}
	s.logger.Info("all trading accounts deleted", "count", len(accounts))
	writeJSON(w, http.StatusOK, map[string]int{"deleted": len(accounts)})
}

// ─── Payouts ────────────────────────────────────────────────

func (s *Server) handleListPayouts(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	writeJSON(w, http.StatusOK, s.store.ListPayouts(accountID))
}

func (s *Server) handleRecordPayout(w http.ResponseWriter, r *http.Request) {
	var p domain.Payout
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if p.ID == "" || p.AccountID == "" || p.GrossAmount <= 0 {
		writeError(w, http.StatusBadRequest, "id, account_id, and gross_amount > 0 required")
		return
	}
	if err := s.store.RecordPayout(p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Re-read the payout so we have the store-enriched net_amount and status.
	enriched := s.store.GetPayout(p.ID)
	if enriched != nil {
		p = *enriched
	}

	s.logger.Info("payout recorded", "account", p.AccountID, "gross", p.GrossAmount, "net", p.NetAmount, "destination", p.Destination)

	// Dual-write: create deposit transaction in Firefly III
	if s.cfo != nil && s.cfo.Available() {
		acct, _ := s.store.GetAccount(p.AccountID)
		acctName := p.AccountID
		if acct != nil {
			acctName = acct.Name
		}
		dest := p.Destination
		if dest == "" {
			dest = "Personal Checking"
		}
		if err := s.cfo.RecordPayoutTransaction(acctName, p.NetAmount, dest); err != nil {
			s.logger.Warn("failed to record payout in Firefly III", "error", err)
		} else {
			s.logger.Info("payout synced to Firefly III", "account", acctName, "amount", p.NetAmount)
		}
	}

	// Auto-reserve a tax allocation (ledger-only, no money movement).
	// Only applies to prop payouts (1099 income), not personal-account withdrawals.
	if acct, _ := s.store.GetAccount(p.AccountID); acct != nil && acct.Type == domain.AccountProp {
		taxPct := getTaxWithholdPct()
		if taxPct > 0 && p.NetAmount > 0 {
			taxAmount := p.NetAmount * taxPct
			alloc := domain.PayoutAllocation{
				ID:        fmt.Sprintf("alloc-tax-%d", time.Now().UnixNano()),
				PayoutID:  p.ID,
				Category:  "taxes",
				Amount:    taxAmount,
				Note:      fmt.Sprintf("Auto tax reserve (%.0f%%) for prop payout %s", taxPct*100, p.ID),
				CreatedAt: time.Now(),
			}
			if err := s.store.RecordAllocation(alloc); err != nil {
				s.logger.Warn("failed to record tax allocation", "error", err)
			} else {
				s.logger.Info("tax allocation reserved", "payout", p.ID, "amount", taxAmount, "pct", taxPct)
			}
		}
	}

	writeJSON(w, http.StatusCreated, p)
}

// ─── Budget ─────────────────────────────────────────────────

func (s *Server) handleGetBudget(w http.ResponseWriter, r *http.Request) {
	// Merge Firefly III budgets with store allocations
	if s.cfo != nil && s.cfo.Available() {
		budgets, err := s.cfo.GetBudgets()
		if err == nil {
			storeBudget := s.store.GetBudget()
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"month":       storeBudget.Month,
				"budgets":     budgets,
				"allocations": storeBudget.Allocations,
				"source":      "firefly",
			})
			return
		}
		s.logger.Warn("firefly budgets unavailable, falling back to store", "error", err)
	}
	writeJSON(w, http.StatusOK, s.store.GetBudget())
}

func (s *Server) handleUpdateBudget(w http.ResponseWriter, r *http.Request) {
	var b domain.BudgetSummary
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	s.store.UpdateBudget(&b)
	writeJSON(w, http.StatusOK, b)
}

// ─── Allocations ────────────────────────────────────────────

func (s *Server) handleGetAllocations(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetAllocations())
}

func (s *Server) handleSetAllocations(w http.ResponseWriter, r *http.Request) {
	var allocs []domain.Allocation
	if err := json.NewDecoder(r.Body).Decode(&allocs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	// Validate percentages sum to 100
	total := 0.0
	for _, a := range allocs {
		total += a.Percentage
	}
	if total < 99 || total > 101 {
		writeError(w, http.StatusBadRequest, "allocation percentages must sum to 100")
		return
	}
	s.store.SetAllocations(allocs)
	writeJSON(w, http.StatusOK, allocs)
}

// ─── Withdrawal Advice ──────────────────────────────────────

func (s *Server) handleWithdrawalAdvice(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.GetWithdrawalAdvice())
}

// ─── Business Metrics ───────────────────────────────────────

func (s *Server) handleBusinessMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.store.GetBusinessMetrics()
	// Enrich with Firefly III monthly data
	if s.cfo != nil && s.cfo.Available() {
		fm, err := s.cfo.GetMonthlyMetrics()
		if err == nil {
			metrics.MonthlyExpenses = fm.Expenses
			metrics.MonthlyNetIncome = fm.Net
		} else {
			s.logger.Warn("firefly metrics unavailable", "error", err)
		}
	}
	writeJSON(w, http.StatusOK, metrics)
}

// ─── Goals ──────────────────────────────────────────────────

func (s *Server) handleListGoals(w http.ResponseWriter, r *http.Request) {
	// Prefer Firefly III piggy banks for goals
	if s.cfo != nil && s.cfo.Available() {
		goals, err := s.cfo.GetGoals()
		if err == nil {
			writeJSON(w, http.StatusOK, goals)
			return
		}
		s.logger.Warn("firefly goals unavailable, falling back to store", "error", err)
	}
	writeJSON(w, http.StatusOK, s.store.ListGoals())
}

func (s *Server) handleGetGoal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	g, err := s.store.GetGoal(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleCreateGoal(w http.ResponseWriter, r *http.Request) {
	var g domain.Goal
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if g.ID == "" || g.Name == "" || g.TargetAmount <= 0 {
		writeError(w, http.StatusBadRequest, "id, name, and target_amount > 0 required")
		return
	}
	if err := s.store.CreateGoal(&g); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.logger.Info("goal created", "id", g.ID, "name", g.Name, "target", g.TargetAmount)
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) handleUpdateGoal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var g domain.Goal
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	g.ID = id
	if err := s.store.UpdateGoal(&g); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) handleDeleteGoal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteGoal(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handleContributeGoal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var c domain.GoalContribution
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if c.ID == "" || c.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "id and amount > 0 required")
		return
	}
	c.GoalID = id
	if err := s.store.ContributeToGoal(c); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("goal contribution", "goal", id, "amount", c.Amount, "source", c.Source)
	writeJSON(w, http.StatusCreated, c)
}

// ─── Billing & Expenses ─────────────────────────────────────

func (s *Server) handleListExpenses(w http.ResponseWriter, r *http.Request) {
	// Prefer Firefly III bills over in-memory expenses
	if s.cfo != nil && s.cfo.Available() {
		bills, err := s.cfo.GetBills()
		if err == nil {
			writeJSON(w, http.StatusOK, bills)
			return
		}
		s.logger.Warn("firefly bills unavailable, falling back to store", "error", err)
	}
	writeJSON(w, http.StatusOK, s.store.ListExpenses())
}

func (s *Server) handleCreateExpense(w http.ResponseWriter, r *http.Request) {
	var e domain.Expense
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if e.ID == "" || e.Name == "" || e.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "id, name, and amount > 0 required")
		return
	}
	if err := s.store.CreateExpense(&e); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.logger.Info("expense created", "id", e.ID, "name", e.Name, "amount", e.Amount)
	writeJSON(w, http.StatusCreated, e)
}

func (s *Server) handleUpdateExpense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var e domain.Expense
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	e.ID = id
	if err := s.store.UpdateExpense(&e); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) handleDeleteExpense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteExpense(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) handlePayExpense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var p domain.Payment
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if p.ID == "" || p.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "id and amount > 0 required")
		return
	}
	p.ExpenseID = id
	if err := s.store.RecordPayment(p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("payment recorded", "expense", id, "amount", p.Amount)
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleBillingSummary(w http.ResponseWriter, r *http.Request) {
	// Prefer Firefly III billing summary
	if s.cfo != nil && s.cfo.Available() {
		tradingIncome := s.store.GetBudget().TradingIncome
		summary, err := s.cfo.GetBillingSummary(tradingIncome)
		if err == nil {
			writeJSON(w, http.StatusOK, summary)
			return
		}
		s.logger.Warn("firefly billing unavailable, falling back to store", "error", err)
	}
	writeJSON(w, http.StatusOK, s.store.GetBillingSummary())
}

// ─── Payout Allocations ────────────────────────────────────

// handleRecordAllocation logs where funds were sent ("I sent $X to family").
func (s *Server) handleRecordAllocation(w http.ResponseWriter, r *http.Request) {
	var a domain.PayoutAllocation
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if a.Category == "" || a.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "category and amount > 0 required")
		return
	}
	if a.ID == "" {
		a.ID = fmt.Sprintf("alloc-%d", time.Now().UnixNano())
	}
	if err := s.store.RecordAllocation(a); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("allocation recorded", "category", a.Category, "amount", a.Amount, "note", a.Note)
	writeJSON(w, http.StatusCreated, a)
}

// handleListAllocations returns allocations for the current month (or specified year/month).
func (s *Server) handleListAllocations(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	allocations := s.store.ListAllocationsForMonth(year, month)

	// Compute per-category totals
	totals := make(map[string]float64)
	for _, a := range allocations {
		totals[a.Category] += a.Amount
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"year":        year,
		"month":       month,
		"allocations": allocations,
		"totals":      totals,
	})
}

func (s *Server) handleUnifiedBudgets(w http.ResponseWriter, r *http.Request) {
	if s.cfo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFO integration not configured",
		})
		return
	}
	budgets, err := s.cfo.GetBudgets()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, budgets)
}

// ─── Prop Firms Registry ───────────────────────────────────

// handleListPropFirms returns the registry of known prop firms and their rules.
// Also returns the current tax-withholding % so the UI can show estimates.
func (s *Server) handleListPropFirms(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"firms":            domain.PropFirmRegistry,
		"tax_withhold_pct": getTaxWithholdPct(),
	}
	if s.cfo != nil && s.cfo.Available() {
		if names, err := s.cfo.ListAssetAccountNames(); err == nil {
			resp["firefly_asset_accounts"] = names
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// ─── Prop Fees ──────────────────────────────────────────────

func (s *Server) handleListPropFees(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	writeJSON(w, http.StatusOK, s.store.ListPropFees(accountID))
}

// handleRecordPropFee records a prop-firm fee and posts a Firefly withdrawal.
// AccountID is optional (eval fees may be paid before the funded row exists).
func (s *Server) handleRecordPropFee(w http.ResponseWriter, r *http.Request) {
	var f domain.PropFee
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if f.PropFirm == "" || f.FeeType == "" || f.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "prop_firm, fee_type, and amount > 0 required")
		return
	}
	if f.ID == "" {
		f.ID = fmt.Sprintf("fee-%d", time.Now().UnixNano())
	}
	if f.PaidDate == "" {
		f.PaidDate = time.Now().Format("2006-01-02")
	}
	if err := s.store.RecordPropFee(f); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("prop fee recorded", "firm", f.PropFirm, "type", f.FeeType, "amount", f.Amount)

	// Post withdrawal to Firefly III
	if s.cfo != nil && s.cfo.Available() {
		firmName := f.PropFirm
		if firm := domain.GetPropFirm(f.PropFirm); firm != nil {
			firmName = firm.Name
		}
		if err := s.cfo.RecordFeeTransaction(firmName, f.FeeType, f.Source, f.Amount, f.PaidDate); err != nil {
			s.logger.Warn("failed to record prop fee in Firefly III", "error", err)
		} else {
			s.logger.Info("prop fee synced to Firefly III", "firm", firmName, "amount", f.Amount)
		}
	}

	writeJSON(w, http.StatusCreated, f)
}

// ─── Combine Rubicon ────────────────────────────────────────

// handleMarkCombinePassed transitions an account from combine → fxt (funded).
// Stamps combine_pass_date and funded_date (if not already set) and moves phase.
// Idempotent: calling again on an fxt/live account just returns it unchanged.
func (s *Server) handleMarkCombinePassed(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, err := s.store.GetAccount(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	today := time.Now().Format("2006-01-02")
	changed := false
	if a.AccountPhase == "combine" || a.AccountPhase == "" {
		a.AccountPhase = "fxt"
		changed = true
	}
	if a.CombinePassDate == "" {
		a.CombinePassDate = today
		changed = true
	}
	if a.FundedDate == "" {
		a.FundedDate = today
		changed = true
	}
	if a.Status == "blown" {
		a.Status = "active"
		a.BlownDate = ""
		changed = true
	}
	if !changed {
		writeJSON(w, http.StatusOK, a)
		return
	}
	if err := s.store.UpdateAccount(a); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("rubicon crossed — combine passed", "account", id, "firm", a.PropFirm, "pass_date", a.CombinePassDate)
	// Broadcast over SSE so Aurum can toast it
	if s.hub != nil {
		s.hub.Broadcast(domain.SystemEvent{
			ID:        fmt.Sprintf("rubicon-%s-%d", a.ID, time.Now().UnixNano()),
			Service:   "primarch",
			Event:     "combine.passed",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"account_id":   a.ID,
				"account_name": a.Name,
				"prop_firm":    a.PropFirm,
				"pass_date":    a.CombinePassDate,
			},
		})
	}
	writeJSON(w, http.StatusOK, a)
}
