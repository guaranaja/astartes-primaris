package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// handleEngineTrades receives bulk trade data from an engine.
func (s *Server) handleEngineTrades(w http.ResponseWriter, r *http.Request) {
	var req domain.EngineTradesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.EngineID == "" {
		writeError(w, http.StatusBadRequest, "engine_id is required")
		return
	}
	if len(req.Trades) > 500 {
		writeError(w, http.StatusBadRequest, "max 500 trades per request")
		return
	}

	resp := domain.EngineTradesResponse{}
	for i := range req.Trades {
		t := &req.Trades[i]
		created, err := s.store.UpsertTrade(t)
		if err != nil {
			s.logger.Warn("upsert trade failed", "id", t.ID, "error", err)
			resp.TradesSkipped++
			continue
		}
		if created {
			resp.TradesCreated++
		} else {
			resp.TradesUpdated++
		}
	}

	s.logger.Info("engine trades ingested",
		"engine_id", req.EngineID,
		"created", resp.TradesCreated,
		"updated", resp.TradesUpdated,
		"skipped", resp.TradesSkipped,
	)
	writeJSON(w, http.StatusOK, resp)
}

// handleEnginePositions receives a position snapshot from an engine.
func (s *Server) handleEnginePositions(w http.ResponseWriter, r *http.Request) {
	var req domain.EnginePositionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.EngineID == "" {
		writeError(w, http.StatusBadRequest, "engine_id is required")
		return
	}

	for i := range req.Positions {
		if err := s.store.UpsertPosition(&req.Positions[i]); err != nil {
			s.logger.Warn("upsert position failed", "error", err)
		}
	}

	s.logger.Info("engine positions updated", "engine_id", req.EngineID, "count", len(req.Positions))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "ok",
		"positions_updated": len(req.Positions),
	})
}

// handleEngineAccountSnapshot receives account balance data from an engine.
func (s *Server) handleEngineAccountSnapshot(w http.ResponseWriter, r *http.Request) {
	var req domain.EngineAccountSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.EngineID == "" {
		writeError(w, http.StatusBadRequest, "engine_id is required")
		return
	}

	for i := range req.Accounts {
		snap := &req.Accounts[i]
		if snap.Timestamp.IsZero() {
			snap.Timestamp = time.Now()
		}
		if err := s.store.RecordAccountSnapshot(snap); err != nil {
			s.logger.Warn("record account snapshot failed", "error", err)
		}

		// Auto-upsert Council trading account from snapshot data
		acctType := domain.AccountProp
		if snap.AccountType == "personal" {
			acctType = domain.AccountPersonal
		} else if snap.AccountType == "paper" {
			acctType = domain.AccountPaper
		}
		status := snap.Status
		if status == "" {
			status = "active"
		}
		name := snap.Name
		if name == "" {
			name = snap.BrokerAccountID
		}
		broker := snap.Broker
		if broker == "" {
			broker = "unknown"
		}
		profitSplit := snap.ProfitSplit
		if profitSplit == 0 {
			profitSplit = 0.90 // default prop split
		}

		acct := &domain.TradingAccount{
			ID:                 snap.BrokerAccountID,
			Name:               name,
			Broker:             broker,
			Type:               acctType,
			InitialBalance:     snap.InitialBalance,
			CurrentBalance:     snap.Balance,
			TotalPnL:           snap.TotalPnL,
			TotalPayouts:       snap.TotalPayouts,
			PayoutCount:        snap.PayoutCount,
			ProfitSplit:        profitSplit,
			Status:             status,
			Instruments:        snap.Instruments,
			MaxLossLimit:       snap.MaxLossLimit,
			ProfitTarget:       snap.ProfitTarget,
			TrailingDD:         snap.TrailingDD,
			MllHeadroom:        snap.MllHeadroom,
			MllUsagePct:        snap.MllUsagePct,
			CombineProgressPct: snap.CombineProgressPct,
			AccountPhase:       snap.AccountPhase,
			WinningDays:        snap.WinningDays,
			TotalTradingDays:   snap.TotalTradingDays,
			BestDayPnL:         snap.BestDayPnL,
			CombineNumber:      snap.CombineNumber,
			CombineStartDate:   snap.CombineStartDate,
		}
		if err := s.store.CreateAccount(acct); err != nil {
			// Already exists — update
			s.store.UpdateAccount(acct)
		}
	}

	s.logger.Info("engine account snapshots recorded", "engine_id", req.EngineID, "count", len(req.Accounts))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"accounts_synced": len(req.Accounts),
	})
}

// handleEngineBars receives bulk market bar data from an engine.
func (s *Server) handleEngineBars(w http.ResponseWriter, r *http.Request) {
	var req domain.EngineBarsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.EngineID == "" {
		writeError(w, http.StatusBadRequest, "engine_id is required")
		return
	}
	if len(req.Bars) > 1000 {
		writeError(w, http.StatusBadRequest, "max 1000 bars per request")
		return
	}

	resp := domain.EngineBarsResponse{}
	for i := range req.Bars {
		b := &req.Bars[i]
		if b.Source == "" {
			b.Source = req.EngineID
		}
		created, err := s.store.UpsertBar(b)
		if err != nil {
			s.logger.Warn("upsert bar failed", "error", err)
			resp.BarsSkipped++
			continue
		}
		if created {
			resp.BarsCreated++
		} else {
			resp.BarsSkipped++
		}
	}

	s.logger.Info("engine bars ingested",
		"engine_id", req.EngineID,
		"created", resp.BarsCreated,
		"skipped", resp.BarsSkipped,
	)
	writeJSON(w, http.StatusOK, resp)
}

// ─── Query Endpoints ───────────────────────────────────────

// handleListTrades returns trades with optional filters.
func (s *Server) handleListTrades(w http.ResponseWriter, r *http.Request) {
	marineID := r.URL.Query().Get("marine_id")
	var since *time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = &t
		}
	}
	limit := 200
	trades := s.store.ListTrades(marineID, since, limit)
	if trades == nil {
		trades = []domain.Trade{}
	}
	writeJSON(w, http.StatusOK, trades)
}

// handleListPositions returns current open positions.
func (s *Server) handleListPositions(w http.ResponseWriter, r *http.Request) {
	marineID := r.URL.Query().Get("marine_id")
	positions := s.store.ListPositions(marineID)
	if positions == nil {
		positions = []domain.Position{}
	}
	writeJSON(w, http.StatusOK, positions)
}

// handlePerformance returns computed trading statistics.
func (s *Server) handlePerformance(w http.ResponseWriter, r *http.Request) {
	marineID := r.URL.Query().Get("marine_id")
	var since *time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = &t
		}
	}

	trades := s.store.ListTrades(marineID, since, 0)

	stats := domain.PerformanceStats{}
	if len(trades) == 0 {
		writeJSON(w, http.StatusOK, stats)
		return
	}

	stats.TotalTrades = len(trades)
	var wins, losses int
	var totalWin, totalLoss float64
	var totalDuration int64
	var longs int
	stats.BestTrade = trades[0].PnL
	stats.WorstTrade = trades[0].PnL

	// Daily P&L aggregation
	dailyMap := map[string]*domain.DailyPnL{}

	for _, t := range trades {
		stats.TotalPnL += t.PnL
		stats.TotalLots += t.Quantity
		totalDuration += t.DurationMs
		if t.Side == "long" {
			longs++
		}
		if t.PnL >= 0 {
			wins++
			totalWin += t.PnL
		} else {
			losses++
			totalLoss += t.PnL
		}
		if t.PnL > stats.BestTrade {
			stats.BestTrade = t.PnL
		}
		if t.PnL < stats.WorstTrade {
			stats.WorstTrade = t.PnL
		}

		day := t.ExitTime.Format("2006-01-02")
		if d, ok := dailyMap[day]; ok {
			d.PnL += t.PnL
			d.TradeCount++
		} else {
			dailyMap[day] = &domain.DailyPnL{Date: day, PnL: t.PnL, TradeCount: 1}
		}
	}

	if stats.TotalTrades > 0 {
		stats.WinRate = float64(wins) / float64(stats.TotalTrades)
		stats.AvgDurationMs = totalDuration / int64(stats.TotalTrades)
		stats.LongPct = float64(longs) / float64(stats.TotalTrades)
	}
	if wins > 0 {
		stats.AvgWin = totalWin / float64(wins)
	}
	if losses > 0 {
		stats.AvgLoss = totalLoss / float64(losses)
	}
	if totalLoss != 0 {
		stats.ProfitFactor = totalWin / -totalLoss
	}

	// Max drawdown
	var peak, dd, maxDD float64
	for i := len(trades) - 1; i >= 0; i-- {
		peak += trades[i].PnL
		if peak > dd {
			dd = peak
		}
		drawdown := dd - peak
		if drawdown > maxDD {
			maxDD = drawdown
		}
	}
	stats.MaxDrawdown = -maxDD

	// Daily P&L sorted
	for _, d := range dailyMap {
		stats.DailyPnL = append(stats.DailyPnL, *d)
	}

	writeJSON(w, http.StatusOK, stats)
}
