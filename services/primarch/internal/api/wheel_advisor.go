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
	s.mux.HandleFunc("GET /api/v1/wheel/paper", s.handleListWheelPaper)
	s.mux.HandleFunc("POST /api/v1/wheel/paper", s.handleOpenWheelPaper)
	s.mux.HandleFunc("POST /api/v1/wheel/paper/{id}/close", s.handleCloseWheelPaper)
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

func (s *Server) handleListWheelPaper(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status") // "" | open | closed | expired
	writeJSON(w, http.StatusOK, s.store.ListWheelPaperPositions(status))
}

// handleOpenWheelPaper creates a paper position from either a recommendation
// id (the usual path: "Paper Take" on a candidate card) or an inline payload
// (for future manual-entry use). Defaults to 1 contract.
func (s *Server) handleOpenWheelPaper(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SourceRecID string `json:"source_rec_id"`
		Contracts   int    `json:"contracts"`
		Notes       string `json:"notes"`

		// Manual override path.
		Action       string  `json:"action"`
		Symbol       string  `json:"symbol"`
		OptionType   string  `json:"option_type"`
		OptionSymbol string  `json:"option_symbol"`
		Strike       float64 `json:"strike"`
		Expiration   string  `json:"expiration"`
		EntryPremium float64 `json:"entry_premium"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if body.Contracts <= 0 {
		body.Contracts = 1
	}
	p := &domain.WheelPaperPosition{Contracts: body.Contracts, Notes: body.Notes}
	if body.SourceRecID != "" {
		rec, err := s.store.GetWheelRecommendation(body.SourceRecID)
		if err != nil || rec == nil {
			writeError(w, http.StatusNotFound, "rec not found")
			return
		}
		p.SourceRecID = rec.ID
		p.Action = rec.Action
		p.Symbol = rec.Symbol
		p.OptionType = rec.OptionType
		p.OptionSymbol = rec.OptionSymbol
		p.Strike = rec.Strike
		p.Expiration = rec.Expiration
		p.EntryPremium = rec.Mid
		p.EntryVerdict = rec.Verdict
	} else {
		// Manual entry — require the essentials.
		if body.Symbol == "" || body.Expiration == "" || body.OptionType == "" || body.Strike <= 0 || body.EntryPremium <= 0 {
			writeError(w, http.StatusBadRequest, "symbol, option_type, strike, expiration, entry_premium required when source_rec_id is omitted")
			return
		}
		p.Action = body.Action
		p.Symbol = body.Symbol
		p.OptionType = body.OptionType
		p.OptionSymbol = body.OptionSymbol
		p.Strike = body.Strike
		p.Expiration = body.Expiration
		p.EntryPremium = body.EntryPremium
	}
	if err := s.store.InsertWheelPaperPosition(p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleCloseWheelPaper(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		ExitMark float64 `json:"exit_mark"`
		Notes    string  `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.ExitMark < 0 {
		writeError(w, http.StatusBadRequest, "exit_mark must be >= 0")
		return
	}
	if err := s.store.CloseWheelPaperPosition(id, body.ExitMark, body.Notes); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Compile-time guard to keep the wheel service import live.
var _ = (*wheel.Service)(nil)
