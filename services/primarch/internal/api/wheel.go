package api

import (
	"database/sql"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

func (s *Server) handleWheelAnalysis(w http.ResponseWriter, r *http.Request) {
	holdings := s.store.ListHoldings()
	if len(holdings) == 0 {
		writeJSON(w, http.StatusOK, []domain.WheelAnalysis{})
		return
	}

	// Get the DB connection from PGStore if available
	db := s.getDB()
	if db == nil {
		writeError(w, http.StatusServiceUnavailable, "no database connection for options data")
		return
	}

	var results []domain.WheelAnalysis
	now := time.Now()

	for _, h := range holdings {
		analysis := domain.WheelAnalysis{
			Symbol:   h.Symbol,
			Quantity: h.Quantity,
			AvgCost:  h.AvgCost,
			DataAsOf: now,
		}

		// Get latest underlying price from option_chains mark (ATM options)
		var markPrice sql.NullFloat64
		db.QueryRow(`SELECT mark FROM option_chains
			WHERE underlying=$1 AND option_type='C' AND mark > 0
			ORDER BY ABS(strike - $2), time DESC LIMIT 1`,
			h.Symbol, h.AvgCost).Scan(&markPrice)

		// Get covered calls: OTM calls above avg cost, 14-45 DTE, sorted by premium/day
		lots := int(h.Quantity / 100)
		if lots > 0 {
			analysis.CoveredCalls = s.queryOptions(db, h.Symbol, "C", h.AvgCost, true, now)
		}

		// Get cash-secured puts: OTM puts below avg cost (or current price), 14-45 DTE
		analysis.CashSecuredPuts = s.queryOptions(db, h.Symbol, "P", h.AvgCost, false, now)

		results = append(results, analysis)
	}

	writeJSON(w, http.StatusOK, results)
}

func (s *Server) queryOptions(db *sql.DB, symbol, optType string, refPrice float64, otmAbove bool, now time.Time) []domain.OptionRecommendation {
	// Get the latest snapshot time for this symbol
	var latestTime time.Time
	err := db.QueryRow(`SELECT MAX(time) FROM option_chains WHERE underlying=$1`, symbol).Scan(&latestTime)
	if err != nil {
		return nil
	}

	// Query options from the latest snapshot, filtered for wheel strategy
	// OTM calls: strike > refPrice; OTM puts: strike < refPrice
	var strikeFilter string
	if otmAbove {
		strikeFilter = "AND strike > $3"
	} else {
		strikeFilter = "AND strike <= $3"
	}

	query := `SELECT DISTINCT ON (expiration, strike)
		expiration, strike, option_type, bid, ask, mark, volume, open_interest
		FROM option_chains
		WHERE underlying = $1
		AND option_type = $2
		` + strikeFilter + `
		AND time = $4
		AND expiration >= CURRENT_DATE + INTERVAL '7 days'
		AND expiration <= CURRENT_DATE + INTERVAL '60 days'
		AND bid > 0
		ORDER BY expiration, strike, time DESC`

	rows, err := db.Query(query, symbol, optType, refPrice, latestTime)
	if err != nil {
		s.logger.Error("wheel query", "error", err, "symbol", symbol)
		return nil
	}
	defer rows.Close()

	var recs []domain.OptionRecommendation
	today := time.Now().Truncate(24 * time.Hour)

	for rows.Next() {
		var r domain.OptionRecommendation
		var exp time.Time
		var bid, ask, mark sql.NullFloat64
		var vol, oi sql.NullInt64

		if err := rows.Scan(&exp, &r.Strike, &r.OptionType, &bid, &ask, &mark, &vol, &oi); err != nil {
			continue
		}

		r.Expiration = exp.Format("2006-01-02")
		r.Bid = bid.Float64
		r.Ask = ask.Float64
		r.Mark = mark.Float64
		if vol.Valid {
			r.Volume = int(vol.Int64)
		}
		if oi.Valid {
			r.OpenInterest = int(oi.Int64)
		}

		r.DTE = int(exp.Sub(today).Hours() / 24)
		if r.DTE <= 0 {
			continue
		}

		// Premium per day based on mark price
		r.PremiumPerDay = r.Mark / float64(r.DTE)

		// Annualized return: (premium / strike) * (365 / DTE) * 100
		if r.Strike > 0 {
			r.AnnualReturn = (r.Mark / r.Strike) * (365.0 / float64(r.DTE)) * 100
		}

		recs = append(recs, r)
	}

	// Sort by premium per day descending, take top 10
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].PremiumPerDay > recs[j].PremiumPerDay
	})

	// Filter: keep reasonable strikes (within 20% of ref price)
	var filtered []domain.OptionRecommendation
	for _, r := range recs {
		if math.Abs(r.Strike-refPrice)/refPrice > 0.20 {
			continue
		}
		filtered = append(filtered, r)
		if len(filtered) >= 10 {
			break
		}
	}

	return filtered
}

// getDB extracts the *sql.DB from the store if it's a PGStore.
func (s *Server) getDB() *sql.DB {
	type dbGetter interface {
		DB() *sql.DB
	}
	if pg, ok := s.store.(dbGetter); ok {
		return pg.DB()
	}
	return nil
}
