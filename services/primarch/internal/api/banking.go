package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/banking"
)

// registerBankingRoutes wires the bank connectivity endpoints.
func (s *Server) registerBankingRoutes() {
	s.mux.HandleFunc("GET /api/v1/council/banking/status", s.handleBankingStatus)
	s.mux.HandleFunc("POST /api/v1/council/banking/link/token", s.handleCreateBankLinkToken)
	s.mux.HandleFunc("POST /api/v1/council/banking/link/exchange", s.handleExchangeBankLinkToken)
	s.mux.HandleFunc("GET /api/v1/council/banking/connections", s.handleListBankConnections)
	s.mux.HandleFunc("POST /api/v1/council/banking/connections/{id}/sync", s.handleSyncBankConnection)
	s.mux.HandleFunc("DELETE /api/v1/council/banking/connections/{id}", s.handleDeleteBankConnection)
}

func (s *Server) handleBankingStatus(w http.ResponseWriter, r *http.Request) {
	available := s.banking != nil && s.banking.Available()
	out := map[string]interface{}{
		"available": available,
	}
	if s.banking != nil {
		out["provider"] = s.banking.ProviderName()
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateBankLinkToken(w http.ResponseWriter, r *http.Request) {
	if s.banking == nil || !s.banking.Available() {
		writeError(w, http.StatusServiceUnavailable, "banking not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	// Single-user app — stable user id so Plaid can track the session.
	userID := "primaris-operator"
	token, exp, err := s.banking.CreateLinkToken(ctx, userID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "create link token: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"link_token": token,
		"expiration": exp.Format(time.RFC3339),
	})
}

type exchangeRequest struct {
	PublicToken string `json:"public_token"`
}

func (s *Server) handleExchangeBankLinkToken(w http.ResponseWriter, r *http.Request) {
	if s.banking == nil || !s.banking.Available() {
		writeError(w, http.StatusServiceUnavailable, "banking not configured")
		return
	}
	var req exchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.PublicToken == "" {
		writeError(w, http.StatusBadRequest, "public_token required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	conn, err := s.banking.ExchangeAndStore(ctx, req.PublicToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "exchange: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, conn)
}

func (s *Server) handleListBankConnections(w http.ResponseWriter, r *http.Request) {
	conns := s.store.ListBankConnections()
	// Never leak access token or sync cursor. Zero them defensively even though
	// BankConnection.JSON tags mark them "-" — belt-and-suspenders.
	for i := range conns {
		conns[i].AccessTokenCT = ""
		conns[i].SyncCursor = ""
	}
	writeJSON(w, http.StatusOK, conns)
}

func (s *Server) handleSyncBankConnection(w http.ResponseWriter, r *http.Request) {
	if s.banking == nil || !s.banking.Available() {
		writeError(w, http.StatusServiceUnavailable, "banking not configured")
		return
	}
	id := r.PathValue("id")
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	added, err := s.banking.SyncConnection(ctx, id)
	if err != nil {
		writeError(w, http.StatusBadGateway, "sync: "+err.Error())
		return
	}
	// Trigger a finance ingest so the new Firefly transactions show up in the
	// activity cache without waiting for the next 15-min tick.
	if s.financeWorker != nil {
		s.financeWorker.TriggerNow()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":    id,
		"added": added,
	})
}

func (s *Server) handleDeleteBankConnection(w http.ResponseWriter, r *http.Request) {
	if s.banking == nil {
		writeError(w, http.StatusServiceUnavailable, "banking not configured")
		return
	}
	id := r.PathValue("id")
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err := s.banking.RemoveConnection(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Compile-time guard: ensure banking.Service is imported (go vet checks).
var _ = (*banking.Service)(nil)
