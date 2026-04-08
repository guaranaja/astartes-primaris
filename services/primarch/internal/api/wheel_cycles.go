package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// ─── Wheel Cycles ──────────────────────────────────────────

func (s *Server) handleListWheelCycles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListWheelCycles())
}

func (s *Server) handleCreateWheelCycle(w http.ResponseWriter, r *http.Request) {
	var c domain.WheelCycle
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if c.Underlying == "" {
		writeError(w, http.StatusBadRequest, "underlying is required")
		return
	}
	if c.ID == "" {
		c.ID = fmt.Sprintf("wc-%s-%d", c.Underlying, time.Now().UnixMilli())
	}
	if c.Status == "" {
		c.Status = "selling_puts"
	}
	if c.Mode == "" {
		c.Mode = "manual"
	}
	if err := s.store.CreateWheelCycle(&c); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.logger.Info("wheel cycle created", "id", c.ID, "underlying", c.Underlying, "status", c.Status)
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleUpdateWheelCycle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var c domain.WheelCycle
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	c.ID = id
	if err := s.store.UpdateWheelCycle(&c); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.logger.Info("wheel cycle updated", "id", c.ID, "status", c.Status)
	writeJSON(w, http.StatusOK, c)
}

// ─── Wheel Legs ────────────────────────────────────────────

func (s *Server) handleListWheelLegs(w http.ResponseWriter, r *http.Request) {
	cycleID := r.PathValue("id")
	writeJSON(w, http.StatusOK, s.store.ListWheelLegs(cycleID))
}

func (s *Server) handleCreateWheelLeg(w http.ResponseWriter, r *http.Request) {
	cycleID := r.PathValue("id")
	var l domain.WheelLeg
	if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if l.LegType == "" || l.Symbol == "" {
		writeError(w, http.StatusBadRequest, "leg_type and symbol are required")
		return
	}
	l.CycleID = cycleID
	if l.ID == "" {
		l.ID = fmt.Sprintf("wl-%s-%d", l.LegType, time.Now().UnixMilli())
	}
	if l.Status == "" {
		l.Status = "open"
	}
	if err := s.store.CreateWheelLeg(&l); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.logger.Info("wheel leg created", "id", l.ID, "cycle", cycleID, "type", l.LegType, "symbol", l.Symbol)
	writeJSON(w, http.StatusCreated, l)
}

func (s *Server) handleUpdateWheelLeg(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var l domain.WheelLeg
	if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	l.ID = id
	if err := s.store.UpdateWheelLeg(&l); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.logger.Info("wheel leg updated", "id", l.ID, "status", l.Status)
	writeJSON(w, http.StatusOK, l)
}
