package api

import (
	"encoding/json"
	"net/http"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// RegisterCouncilRoutes adds Council endpoints to the server.
func (s *Server) registerCouncilRoutes() {
	// Roadmap & phases
	s.mux.HandleFunc("GET /api/v1/council/roadmap", s.handleGetRoadmap)
	s.mux.HandleFunc("PUT /api/v1/council/roadmap/phase", s.handleAdvancePhase)
	s.mux.HandleFunc("PUT /api/v1/council/roadmap/milestone", s.handleUpdateMilestone)

	// Trading accounts
	s.mux.HandleFunc("GET /api/v1/council/accounts", s.handleListAccounts)
	s.mux.HandleFunc("POST /api/v1/council/accounts", s.handleCreateAccount)
	s.mux.HandleFunc("GET /api/v1/council/accounts/{id}", s.handleGetAccount)
	s.mux.HandleFunc("PUT /api/v1/council/accounts/{id}", s.handleUpdateAccount)

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
	s.logger.Info("payout recorded", "account", p.AccountID, "gross", p.GrossAmount, "destination", p.Destination)
	writeJSON(w, http.StatusCreated, p)
}

// ─── Budget ─────────────────────────────────────────────────

func (s *Server) handleGetBudget(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, s.store.GetBusinessMetrics())
}
