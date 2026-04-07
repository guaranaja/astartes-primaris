package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

func generateCommandID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "cmd-" + hex.EncodeToString(b)
}

// handleEngineRegister processes a full topology registration from an engine.
// Idempotent: creates on first call, updates on subsequent calls.
func (s *Server) handleEngineRegister(w http.ResponseWriter, r *http.Request) {
	var req domain.EngineRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.EngineID == "" {
		writeError(w, http.StatusBadRequest, "engine_id is required")
		return
	}

	resp := domain.EngineRegisterResponse{EngineID: req.EngineID}

	for _, rf := range req.Fortresses {
		// Upsert fortress
		f := &domain.Fortress{
			ID:         rf.ID,
			Name:       rf.Name,
			AssetClass: rf.AssetClass,
			Metadata:   map[string]string{"engine_id": req.EngineID},
		}
		if err := s.store.CreateFortress(f); err != nil {
			// Already exists — update
			s.store.UpdateFortress(f)
			resp.FortressesUpdated++
		} else {
			resp.FortressesCreated++
		}

		for _, rc := range rf.Companies {
			// Upsert company
			c := &domain.Company{
				ID:         rc.ID,
				FortressID: rf.ID,
				Name:       rc.Name,
				Type:       domain.CompanyVeteran,
			}
			if err := s.store.CreateCompany(c); err != nil {
				s.store.UpdateCompany(c)
				resp.CompaniesUpdated++
			} else {
				resp.CompaniesCreated++
			}

			for _, rm := range rc.Marines {
				// Upsert marine
				m := &domain.Marine{
					ID:              rm.ID,
					CompanyID:       rc.ID,
					Name:            rm.Name,
					StrategyName:    rm.StrategyName,
					StrategyVersion: rm.StrategyVersion,
					BrokerAccountID: rm.BrokerAccountID,
					Status:          rm.Status,
					Schedule:        rm.Schedule,
					Parameters:      rm.Parameters,
					RunnerType:      domain.RunnerRemote,
					Resources: domain.ResourceLimits{
						MemoryMB:       256,
						CPUMillicores:  250,
						TimeoutSeconds: 30,
					},
				}
				if m.Status == "" {
					m.Status = domain.StatusDormant
				}
				if err := s.store.CreateMarine(m); err != nil {
					s.store.UpdateMarine(m)
					resp.MarinesUpdated++
				} else {
					resp.MarinesCreated++
				}
			}
		}
	}

	s.logger.Info("engine registered",
		"engine_id", req.EngineID,
		"fortresses", resp.FortressesCreated+resp.FortressesUpdated,
		"companies", resp.CompaniesCreated+resp.CompaniesUpdated,
		"marines", resp.MarinesCreated+resp.MarinesUpdated,
	)

	writeJSON(w, http.StatusOK, resp)
}

// handleEngineHeartbeat processes a bulk status update from an engine
// and returns any pending commands.
func (s *Server) handleEngineHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req domain.EngineHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.EngineID == "" {
		writeError(w, http.StatusBadRequest, "engine_id is required")
		return
	}

	for _, hm := range req.Marines {
		m, err := s.store.GetMarine(hm.MarineID)
		if err != nil {
			s.logger.Warn("heartbeat for unknown marine", "marine_id", hm.MarineID, "engine_id", req.EngineID)
			continue
		}
		m.Status = hm.Status
		m.DailyPnL = hm.DailyPnL
		m.CyclesToday = hm.CyclesToday
		if hm.Parameters != nil {
			m.Parameters = hm.Parameters
		}
		m.UpdatedAt = time.Now()
		s.store.UpdateMarine(m)
	}

	// Get pending commands and mark them as acked
	commands := s.store.ListPendingCommands(req.EngineID)
	for i := range commands {
		if commands[i].Status == domain.CommandPending {
			commands[i].Status = domain.CommandAcked
			s.store.UpdateCommand(&commands[i])
		}
	}

	s.logger.Info("engine heartbeat",
		"engine_id", req.EngineID,
		"status", req.Status,
		"marines_updated", len(req.Marines),
		"commands_sent", len(commands),
	)

	writeJSON(w, http.StatusOK, domain.EngineHeartbeatResponse{
		Status:   "ok",
		Commands: commands,
	})
}

// handleEngineCommandComplete acknowledges a command from an engine.
func (s *Server) handleEngineCommandComplete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cmd, err := s.store.GetCommand(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req domain.EngineCommandCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	switch req.Status {
	case "completed":
		cmd.Status = domain.CommandCompleted
	case "failed":
		cmd.Status = domain.CommandFailed
		cmd.Error = req.Error
	default:
		writeError(w, http.StatusBadRequest, "status must be 'completed' or 'failed'")
		return
	}

	if err := s.store.UpdateCommand(cmd); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.logger.Info("command completed", "id", id, "status", req.Status)
	writeJSON(w, http.StatusOK, cmd)
}
