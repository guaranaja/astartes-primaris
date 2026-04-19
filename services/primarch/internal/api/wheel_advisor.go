package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/advisor/wheel"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// registerWheelAdvisorRoutes wires the wheel strategy advisor endpoints.
func (s *Server) registerWheelAdvisorRoutes() {
	s.mux.HandleFunc("GET /api/v1/wheel/status", s.handleWheelStatus)
	s.mux.HandleFunc("GET /api/v1/wheel/config", s.handleGetWheelConfig)
	s.mux.HandleFunc("PUT /api/v1/wheel/config", s.handleUpdateWheelConfig)
	s.mux.HandleFunc("GET /api/v1/wheel/watchlist", s.handleListWheelWatchlist)
	s.mux.HandleFunc("POST /api/v1/wheel/watchlist", s.handleUpsertWheelWatchlist)
	s.mux.HandleFunc("DELETE /api/v1/wheel/watchlist/{symbol}", s.handleDeleteWheelWatchlist)
	s.mux.HandleFunc("GET /api/v1/wheel/recommendations", s.handleListWheelRecommendations)
	s.mux.HandleFunc("POST /api/v1/wheel/recommendations/{id}/take", s.handleTakeWheelRec)
	s.mux.HandleFunc("POST /api/v1/wheel/recommendations/{id}/dismiss", s.handleDismissWheelRec)
	s.mux.HandleFunc("POST /api/v1/wheel/scan", s.handleTriggerWheelScan)
}

func (s *Server) handleWheelStatus(w http.ResponseWriter, r *http.Request) {
	available := s.wheel != nil && s.wheel.Available()
	out := map[string]interface{}{
		"available":     available,
		"claude_review": s.advisor != nil && s.advisor.Available(),
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetWheelConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.GetWheelConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleUpdateWheelConfig(w http.ResponseWriter, r *http.Request) {
	var c domain.WheelConfig
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if err := s.store.UpdateWheelConfig(&c); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleListWheelWatchlist(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListWheelWatchlist())
}

func (s *Server) handleUpsertWheelWatchlist(w http.ResponseWriter, r *http.Request) {
	var e domain.WheelWatchlistEntry
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if e.Symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol required")
		return
	}
	// Default to active on creation.
	if !e.Active {
		e.Active = true
	}
	if err := s.store.UpsertWheelWatchlistEntry(&e); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (s *Server) handleDeleteWheelWatchlist(w http.ResponseWriter, r *http.Request) {
	sym := r.PathValue("symbol")
	if err := s.store.DeleteWheelWatchlistEntry(sym); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListWheelRecommendations(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "fresh"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}
	writeJSON(w, http.StatusOK, s.store.ListWheelRecommendations(status, limit))
}

func (s *Server) handleTakeWheelRec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.UpdateWheelRecommendationStatus(id, domain.WheelRecTaken); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDismissWheelRec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.UpdateWheelRecommendationStatus(id, domain.WheelRecDismissed); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTriggerWheelScan(w http.ResponseWriter, r *http.Request) {
	if s.wheel == nil || !s.wheel.Available() {
		writeError(w, http.StatusServiceUnavailable, "wheel advisor not configured")
		return
	}
	s.wheel.TriggerNow()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "scan triggered"})
}

// Compile-time guard to keep the wheel service import live.
var _ = (*wheel.Service)(nil)
