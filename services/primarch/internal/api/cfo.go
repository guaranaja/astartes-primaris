package api

import (
	"net/http"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/cfo"
)

// registerCFORoutes adds unified finance endpoints.
func (s *Server) registerCFORoutes() {
	s.mux.HandleFunc("GET /api/v1/council/finance/overview", s.handleFinanceOverview)
	s.mux.HandleFunc("GET /api/v1/council/finance/accounts", s.handleFinanceAccounts)
	s.mux.HandleFunc("GET /api/v1/council/finance/status", s.handleFinanceStatus)
}

func (s *Server) handleFinanceOverview(w http.ResponseWriter, r *http.Request) {
	if s.cfo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFO integration not configured",
		})
		return
	}
	overview, err := s.cfo.GetOverview()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Inject trading data from Council's existing store
	// Only funded (fxt/live) accounts have real cash value
	accounts := s.store.ListAccounts()
	for _, a := range accounts {
		overview.Accounts = append(overview.Accounts, cfo.UnifiedAccount{
			ID:       "tr-" + a.ID,
			Name:     a.Name,
			Source:   cfo.SourceTrading,
			Type:     string(a.Type),
			Balance:  a.CurrentBalance,
			Currency: "USD",
		})
		hasCashValue := (a.AccountPhase == "fxt" || a.AccountPhase == "live") && a.Status == "active"
		if hasCashValue {
			overview.TradingValue += a.CurrentBalance
			overview.TotalNetWorth += a.CurrentBalance
		}
	}

	// Trading P&L from metrics
	metrics := s.store.GetBusinessMetrics()
	overview.TradingPnL = metrics.LifetimePnL

	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) handleFinanceAccounts(w http.ResponseWriter, r *http.Request) {
	if s.cfo == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "CFO integration not configured",
		})
		return
	}
	overview, err := s.cfo.GetOverview()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Add trading accounts with cash-value annotation
	for _, a := range s.store.ListAccounts() {
		acctType := string(a.Type)
		if a.AccountPhase == "combine" || a.Type == "paper" {
			acctType = acctType + ":sim" // downstream knows this has no cash value
		}
		overview.Accounts = append(overview.Accounts, cfo.UnifiedAccount{
			ID:       "tr-" + a.ID,
			Name:     a.Name,
			Source:   cfo.SourceTrading,
			Type:     acctType,
			Balance:  a.CurrentBalance,
			Currency: "USD",
		})
	}

	writeJSON(w, http.StatusOK, overview.Accounts)
}

func (s *Server) handleFinanceStatus(w http.ResponseWriter, r *http.Request) {
	if s.cfo == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"cfo_engine": "not configured",
			"monarch":    "not configured",
			"trading":    "connected",
		})
		return
	}
	status := s.cfo.PingAll()
	status["trading"] = "connected"
	writeJSON(w, http.StatusOK, status)
}
