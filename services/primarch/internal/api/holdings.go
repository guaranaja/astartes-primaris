package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

func (s *Server) handleListHoldings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.ListHoldings())
}

func (s *Server) handleCreateHolding(w http.ResponseWriter, r *http.Request) {
	var h domain.Holding
	if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if h.Symbol == "" || h.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "symbol and quantity > 0 are required")
		return
	}
	if h.ID == "" {
		h.ID = fmt.Sprintf("hold-%s-%d", h.Symbol, time.Now().UnixMilli())
	}
	if err := s.store.CreateHolding(&h); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.logger.Info("holding created", "id", h.ID, "symbol", h.Symbol, "qty", h.Quantity)
	writeJSON(w, http.StatusCreated, h)
}

func (s *Server) handleUpdateHolding(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var h domain.Holding
	if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	h.ID = id
	if err := s.store.UpdateHolding(&h); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func (s *Server) handleDeleteHolding(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteHolding(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
