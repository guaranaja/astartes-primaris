// Package api provides the HTTP REST API for the Primarch control plane.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/scheduler"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/store"
)

// Server is the HTTP API server for Primarch.
type Server struct {
	store     store.DataStore
	scheduler *scheduler.Scheduler
	logger    *slog.Logger
	hub       *WSHub
	mux       *http.ServeMux
}

// NewServer creates the API server with a new WebSocket hub.
func NewServer(s store.DataStore, sched *scheduler.Scheduler, logger *slog.Logger) *Server {
	return NewServerWithHub(s, sched, logger, NewWSHub())
}

// NewServerWithHub creates the API server with an externally provided WebSocket hub.
func NewServerWithHub(s store.DataStore, sched *scheduler.Scheduler, logger *slog.Logger, hub *WSHub) *Server {
	srv := &Server{
		store:     s,
		scheduler: sched,
		logger:    logger,
		hub:       hub,
		mux:       http.NewServeMux(),
	}
	srv.routes()
	return srv
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return corsMiddleware(authMiddleware(logMiddleware(s.mux, s.logger)))
}

// Hub returns the WebSocket hub for broadcasting events.
func (s *Server) Hub() *WSHub {
	return s.hub
}

func (s *Server) routes() {
	// Auth
	s.mux.HandleFunc("POST /api/v1/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/v1/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/v1/auth-check", s.handleAuthCheck)

	// Health
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /api/v1/status", s.handleStatus)

	// Fortresses
	s.mux.HandleFunc("GET /api/v1/fortresses", s.handleListFortresses)
	s.mux.HandleFunc("POST /api/v1/fortresses", s.handleCreateFortress)
	s.mux.HandleFunc("GET /api/v1/fortresses/{id}", s.handleGetFortress)
	s.mux.HandleFunc("PUT /api/v1/fortresses/{id}", s.handleUpdateFortress)
	s.mux.HandleFunc("DELETE /api/v1/fortresses/{id}", s.handleDeleteFortress)

	// Companies
	s.mux.HandleFunc("GET /api/v1/fortresses/{fortressID}/companies", s.handleListCompanies)
	s.mux.HandleFunc("POST /api/v1/companies", s.handleCreateCompany)
	s.mux.HandleFunc("GET /api/v1/companies/{id}", s.handleGetCompany)
	s.mux.HandleFunc("PUT /api/v1/companies/{id}", s.handleUpdateCompany)
	s.mux.HandleFunc("DELETE /api/v1/companies/{id}", s.handleDeleteCompany)

	// Marines
	s.mux.HandleFunc("GET /api/v1/companies/{companyID}/marines", s.handleListMarines)
	s.mux.HandleFunc("GET /api/v1/marines", s.handleListAllMarines)
	s.mux.HandleFunc("POST /api/v1/marines", s.handleCreateMarine)
	s.mux.HandleFunc("GET /api/v1/marines/{id}", s.handleGetMarine)
	s.mux.HandleFunc("PUT /api/v1/marines/{id}", s.handleUpdateMarine)
	s.mux.HandleFunc("DELETE /api/v1/marines/{id}", s.handleDeleteMarine)

	// Marine lifecycle
	s.mux.HandleFunc("POST /api/v1/marines/{id}/wake", s.handleWakeMarine)
	s.mux.HandleFunc("POST /api/v1/marines/{id}/sleep", s.handleSleepMarine)
	s.mux.HandleFunc("POST /api/v1/marines/{id}/enable", s.handleEnableMarine)
	s.mux.HandleFunc("POST /api/v1/marines/{id}/disable", s.handleDisableMarine)
	s.mux.HandleFunc("GET /api/v1/marines/{id}/cycles", s.handleGetCycles)

	// Kill switch
	s.mux.HandleFunc("POST /api/v1/kill-switch/{scope}", s.handleKillSwitch)

	// WebSocket
	s.mux.HandleFunc("GET /ws", s.hub.HandleWebSocket)

	// Holdings
	s.mux.HandleFunc("GET /api/v1/holdings", s.handleListHoldings)
	s.mux.HandleFunc("POST /api/v1/holdings", s.handleCreateHolding)
	s.mux.HandleFunc("PUT /api/v1/holdings/{id}", s.handleUpdateHolding)
	s.mux.HandleFunc("DELETE /api/v1/holdings/{id}", s.handleDeleteHolding)

	// Wheel Analysis
	s.mux.HandleFunc("GET /api/v1/wheel-analysis", s.handleWheelAnalysis)

	// Council
	s.registerCouncilRoutes()
}

// ─── Health & Status ────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "operational", "service": "primarch"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	fortresses := s.store.ListFortresses()
	marines := s.store.ListAllMarines()

	active := 0
	dormant := 0
	failed := 0
	disabled := 0
	for _, m := range marines {
		switch m.Status {
		case domain.StatusDormant:
			dormant++
		case domain.StatusDisabled:
			disabled++
		case domain.StatusFailed:
			failed++
		default:
			active++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service":    "primarch",
		"version":    "0.1.0",
		"uptime":     time.Since(startTime).String(),
		"fortresses": len(fortresses),
		"marines": map[string]int{
			"total":    len(marines),
			"active":   active,
			"dormant":  dormant,
			"failed":   failed,
			"disabled": disabled,
		},
		"ws_clients": s.hub.ClientCount(),
	})
}

var startTime = time.Now()

// ─── Fortresses ─────────────────────────────────────────────

func (s *Server) handleListFortresses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListFortresses())
}

func (s *Server) handleCreateFortress(w http.ResponseWriter, r *http.Request) {
	var f domain.Fortress
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if f.ID == "" || f.Name == "" || f.AssetClass == "" {
		writeError(w, http.StatusBadRequest, "id, name, and asset_class are required")
		return
	}
	if err := s.store.CreateFortress(&f); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.logger.Info("fortress created", "id", f.ID, "name", f.Name)
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleGetFortress(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := s.store.GetFortress(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleUpdateFortress(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var f domain.Fortress
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	f.ID = id
	if err := s.store.UpdateFortress(&f); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleDeleteFortress(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteFortress(id); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Companies ──────────────────────────────────────────────

func (s *Server) handleListCompanies(w http.ResponseWriter, r *http.Request) {
	fortressID := r.PathValue("fortressID")
	writeJSON(w, http.StatusOK, s.store.ListCompanies(fortressID))
}

func (s *Server) handleCreateCompany(w http.ResponseWriter, r *http.Request) {
	var c domain.Company
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if c.ID == "" || c.Name == "" || c.FortressID == "" {
		writeError(w, http.StatusBadRequest, "id, name, and fortress_id are required")
		return
	}
	if err := s.store.CreateCompany(&c); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.logger.Info("company created", "id", c.ID, "fortress", c.FortressID)
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleGetCompany(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := s.store.GetCompany(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleUpdateCompany(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var c domain.Company
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	c.ID = id
	if err := s.store.UpdateCompany(&c); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteCompany(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteCompany(id); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Marines ────────────────────────────────────────────────

func (s *Server) handleListAllMarines(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListAllMarines())
}

func (s *Server) handleListMarines(w http.ResponseWriter, r *http.Request) {
	companyID := r.PathValue("companyID")
	writeJSON(w, http.StatusOK, s.store.ListMarines(companyID))
}

func (s *Server) handleCreateMarine(w http.ResponseWriter, r *http.Request) {
	var m domain.Marine
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if m.ID == "" || m.Name == "" || m.CompanyID == "" || m.StrategyName == "" {
		writeError(w, http.StatusBadRequest, "id, name, company_id, and strategy_name are required")
		return
	}
	// Defaults
	if m.Resources.MemoryMB == 0 {
		m.Resources.MemoryMB = 256
	}
	if m.Resources.CPUMillicores == 0 {
		m.Resources.CPUMillicores = 250
	}
	if m.Resources.TimeoutSeconds == 0 {
		m.Resources.TimeoutSeconds = 30
	}

	if err := s.store.CreateMarine(&m); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	s.logger.Info("marine created", "id", m.ID, "strategy", m.StrategyName, "company", m.CompanyID)

	// Auto-schedule if enabled
	if m.Schedule.Enabled {
		if err := s.scheduler.ScheduleMarine(context.Background(), m.ID); err != nil {
			s.logger.Warn("failed to auto-schedule marine", "id", m.ID, "error", err)
		}
	}

	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleGetMarine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.store.GetMarine(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleUpdateMarine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var m domain.Marine
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	m.ID = id
	if err := s.store.UpdateMarine(&m); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteMarine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.scheduler.UnscheduleMarine(id)
	if err := s.store.DeleteMarine(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Marine Lifecycle ───────────────────────────────────────

func (s *Server) handleWakeMarine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.scheduler.WakeNow(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "waking", "marine_id": id})
}

func (s *Server) handleSleepMarine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.scheduler.UnscheduleMarine(id)
	s.store.UpdateMarineStatus(id, domain.StatusDormant)
	writeJSON(w, http.StatusOK, map[string]string{"status": "dormant", "marine_id": id})
}

func (s *Server) handleEnableMarine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.store.GetMarine(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	m.Status = domain.StatusDormant
	m.Schedule.Enabled = true
	s.store.UpdateMarine(m)
	s.scheduler.ScheduleMarine(context.Background(), id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled", "marine_id": id})
}

func (s *Server) handleDisableMarine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.scheduler.UnscheduleMarine(id)
	m, err := s.store.GetMarine(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	m.Status = domain.StatusDisabled
	m.Schedule.Enabled = false
	s.store.UpdateMarine(m)
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled", "marine_id": id})
}

func (s *Server) handleGetCycles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cycles := s.store.GetCycles(id, 100)
	writeJSON(w, http.StatusOK, cycles)
}

// ─── Kill Switch ────────────────────────────────────────────

func (s *Server) handleKillSwitch(w http.ResponseWriter, r *http.Request) {
	scope := r.PathValue("scope")
	s.store.ActivateKillSwitch(scope)
	// Unschedule all affected marines
	for _, m := range s.store.ListAllMarines() {
		if m.Status == domain.StatusDisabled {
			s.scheduler.UnscheduleMarine(m.ID)
		}
	}
	s.logger.Warn("KILL SWITCH ACTIVATED", "scope", scope)
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "kill_switch_active",
		"scope":  scope,
	})
}

// ─── Helpers ────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(wrapped, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", time.Since(start),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// ─── Seed ───────────────────────────────────────────────────

// SeedFuturesFortress creates the initial Fortress Primus hierarchy for futures.
// This makes it easy to register your existing astartes-futures strategies.
func SeedFuturesFortress(s store.DataStore) {
	_ = s.CreateFortress(&domain.Fortress{
		ID:         "fortress-primus",
		Name:       "Fortress Primus",
		AssetClass: "futures",
		Metadata: map[string]string{
			"description": "All futures trading operations",
		},
	})

	_ = s.CreateCompany(&domain.Company{
		ID:         "first-company",
		FortressID: "fortress-primus",
		Name:       "1st Company (Veterans)",
		Type:       domain.CompanyVeteran,
		RiskLimits: domain.RiskLimits{
			MaxPositionSize: 5,
			MaxDailyLoss:    1000,
			MaxDrawdownPct:  5,
		},
	})

	_ = s.CreateCompany(&domain.Company{
		ID:         "scout-company",
		FortressID: "fortress-primus",
		Name:       "Scout Company",
		Type:       domain.CompanyScout,
		RiskLimits: domain.RiskLimits{
			MaxPositionSize: 1,
			MaxDailyLoss:    200,
			MaxDrawdownPct:  3,
		},
	})

	fmt.Println("  Fortress Primus (Futures) seeded with 1st Company + Scout Company")
}
