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

	// Inject trading data from Council's existing store.
	// Only XFT/live accounts have real monetary value.
	// Combine accounts track progress but aren't real assets.
	// Practice accounts are purely simulated.
	accounts := s.store.ListAccounts()
	for _, a := range accounts {
		phase := a.AccountPhase // "practice", "combine", "fxt", "live", "blown"

		// Determine real asset value based on account phase
		var realValue float64
		switch phase {
		case "live":
			realValue = a.CurrentBalance
		case "fxt":
			// Only profit after split is yours; the base capital is the firm's
			profit := a.CurrentBalance - float64(a.InitialBalance)
			if profit > 0 {
				realValue = profit * a.ProfitSplit
			}
		default:
			// practice, combine, blown — no real asset value
			realValue = 0
		}

		overview.Accounts = append(overview.Accounts, cfo.UnifiedAccount{
			ID:       "tr-" + a.ID,
			Name:     a.Name,
			Source:   cfo.SourceTrading,
			Type:     phase, // use phase instead of generic "prop"
			Balance:  a.CurrentBalance,
			Currency: "USD",
		})
		overview.TradingValue += realValue
		overview.TotalNetWorth += realValue
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

	// Add trading accounts with phase as type
	for _, a := range s.store.ListAccounts() {
		acctType := a.AccountPhase
		if acctType == "" {
			acctType = string(a.Type)
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
