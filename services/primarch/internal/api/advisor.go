package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/advisor"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// registerAdvisorRoutes wires the Council Advisor endpoints.
func (s *Server) registerAdvisorRoutes() {
	s.mux.HandleFunc("GET /api/v1/council/advisor/status", s.handleAdvisorStatus)
	s.mux.HandleFunc("GET /api/v1/council/advisor/playbooks", s.handleListPlaybooks)
	s.mux.HandleFunc("GET /api/v1/council/advisor/threads", s.handleListAdvisorThreads)
	s.mux.HandleFunc("POST /api/v1/council/advisor/threads", s.handleCreateAdvisorThread)
	s.mux.HandleFunc("GET /api/v1/council/advisor/threads/{id}", s.handleGetAdvisorThread)
	s.mux.HandleFunc("DELETE /api/v1/council/advisor/threads/{id}", s.handleArchiveAdvisorThread)
	s.mux.HandleFunc("POST /api/v1/council/advisor/threads/{id}/messages", s.handleSendAdvisorMessage)
}

func (s *Server) handleAdvisorStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"available": s.advisor != nil && s.advisor.Available(),
		"model":     advisor.DefaultModel,
		"cfo":       s.cfo != nil && s.cfo.Available(),
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleListPlaybooks(w http.ResponseWriter, r *http.Request) {
	out := make([]map[string]string, 0, len(advisor.Playbooks))
	for _, p := range advisor.Playbooks {
		out = append(out, map[string]string{
			"key":   p.Key,
			"title": p.Title,
			"brief": p.Brief,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListAdvisorThreads(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}
	threads := s.store.ListAdvisorThreads(status)
	writeJSON(w, http.StatusOK, threads)
}

func (s *Server) handleGetAdvisorThread(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := s.store.GetAdvisorThread(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// createThreadRequest is what the UI posts to start a new thread.
type createThreadRequest struct {
	Title       string `json:"title"`
	Topic       string `json:"topic"`        // "chat" | "playbook" | "milestone"
	PlaybookKey string `json:"playbook_key"` // required when topic=playbook
}

func (s *Server) handleCreateAdvisorThread(w http.ResponseWriter, r *http.Request) {
	if s.advisor == nil || !s.advisor.Available() {
		writeError(w, http.StatusServiceUnavailable, "advisor not configured (CLAUDE_API_KEY missing)")
		return
	}
	var req createThreadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.Topic == "" {
		req.Topic = "chat"
	}
	if req.Topic == "playbook" && req.PlaybookKey == "" {
		writeError(w, http.StatusBadRequest, "playbook_key required for playbook threads")
		return
	}

	// Title defaults: use playbook title, or timestamp for chat
	title := req.Title
	if title == "" {
		if pb := advisor.PlaybookByKey(req.PlaybookKey); pb != nil {
			title = pb.Title
		} else {
			title = "Advisor chat — " + time.Now().Format("Jan 2 3:04 PM")
		}
	}

	snapshot := advisor.BuildSnapshot(s.store, s.cfo)
	t := &domain.AdvisorThread{
		ID:              fmt.Sprintf("thread-%d", time.Now().UnixNano()),
		Title:           title,
		Topic:           req.Topic,
		PlaybookKey:     req.PlaybookKey,
		Status:          "active",
		ContextSnapshot: snapshot.AsJSON(),
	}
	if err := s.store.CreateAdvisorThread(t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("advisor thread created", "id", t.ID, "topic", t.Topic, "playbook", t.PlaybookKey)

	// Playbook threads auto-kick off with the canonical opening user turn,
	// so the user lands on a meaningful response instead of a blank box.
	if pb := advisor.PlaybookByKey(req.PlaybookKey); pb != nil && pb.OpeningUser != "" {
		go s.runAdvisorTurn(t.ID, pb.OpeningUser, snapshot)
	}

	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleArchiveAdvisorThread(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := s.store.GetAdvisorThread(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	t.Status = "archived"
	if err := s.store.UpdateAdvisorThread(t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleSendAdvisorMessage(w http.ResponseWriter, r *http.Request) {
	if s.advisor == nil || !s.advisor.Available() {
		writeError(w, http.StatusServiceUnavailable, "advisor not configured")
		return
	}
	id := r.PathValue("id")
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	t, err := s.store.GetAdvisorThread(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Use a fresh snapshot for this turn (advisor always sees current numbers,
	// even on a long-running thread). Thread's context_snapshot stays as the
	// "what was true when the thread started" audit record.
	snapshot := advisor.BuildSnapshot(s.store, s.cfo)
	reply, err := s.runAdvisorTurnSync(t.ID, req.Content, snapshot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "advisor call failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, reply)
}

// runAdvisorTurnSync appends the user message, calls Claude, appends the reply.
// Returns the assistant message so the API can respond with it directly.
func (s *Server) runAdvisorTurnSync(threadID, userContent string, snapshot advisor.Snapshot) (*domain.AdvisorMessage, error) {
	// Append the user turn first so the DB stays consistent even on API failure.
	userMsg := &domain.AdvisorMessage{
		ID:       fmt.Sprintf("msg-%d-u", time.Now().UnixNano()),
		ThreadID: threadID,
		Role:     "user",
		Content:  userContent,
	}
	if err := s.store.AppendAdvisorMessage(userMsg); err != nil {
		return nil, fmt.Errorf("append user message: %w", err)
	}

	t, err := s.store.GetAdvisorThread(threadID)
	if err != nil {
		return nil, err
	}
	system := advisor.BuildSystemPrompt(t.PlaybookKey, snapshot.AsMarkdown())
	history := toAnthropicHistory(t.Messages)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	res, err := s.advisor.Complete(ctx, system, history)
	if err != nil {
		return nil, err
	}

	assistantMsg := &domain.AdvisorMessage{
		ID:        fmt.Sprintf("msg-%d-a", time.Now().UnixNano()),
		ThreadID:  threadID,
		Role:      "assistant",
		Content:   res.Content,
		Model:     res.Model,
		TokensIn:  res.TokensIn,
		TokensOut: res.TokensOut,
	}
	if err := s.store.AppendAdvisorMessage(assistantMsg); err != nil {
		return nil, fmt.Errorf("append assistant message: %w", err)
	}
	s.logger.Info("advisor turn complete", "thread", threadID, "tokens_in", res.TokensIn, "tokens_out", res.TokensOut)
	return assistantMsg, nil
}

// runAdvisorTurn is the fire-and-forget variant used by playbook kickoff
// and milestone briefings. Errors log but don't propagate.
func (s *Server) runAdvisorTurn(threadID, userContent string, snapshot advisor.Snapshot) {
	if _, err := s.runAdvisorTurnSync(threadID, userContent, snapshot); err != nil {
		s.logger.Warn("advisor background turn failed", "thread", threadID, "error", err)
	}
}

// toAnthropicHistory converts stored messages to the format the Messages API wants.
// System messages are excluded (sent separately as the `system` param).
func toAnthropicHistory(msgs []domain.AdvisorMessage) []advisor.Message {
	out := make([]advisor.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "system" {
			continue
		}
		out = append(out, advisor.Message{Role: m.Role, Content: m.Content})
	}
	return out
}

// TriggerMilestone creates a briefing thread for a domain event (e.g.
// combine.passed, account.funded, first payout). Safe to call even when
// the advisor isn't configured — it just skips.
func (s *Server) TriggerMilestone(event string, data map[string]interface{}) {
	if s.advisor == nil || !s.advisor.Available() {
		return
	}
	title := milestoneTitle(event, data)
	snapshot := advisor.BuildSnapshot(s.store, s.cfo)
	t := &domain.AdvisorThread{
		ID:              fmt.Sprintf("thread-milestone-%d", time.Now().UnixNano()),
		Title:           title,
		Topic:           "milestone",
		Status:          "active",
		ContextSnapshot: snapshot.AsJSON(),
	}
	if md, err := json.Marshal(map[string]interface{}{"event": event, "data": data}); err == nil {
		t.Metadata = md
	}
	if err := s.store.CreateAdvisorThread(t); err != nil {
		s.logger.Warn("milestone thread create failed", "event", event, "error", err)
		return
	}
	s.logger.Info("advisor milestone thread created", "id", t.ID, "event", event)
	go s.runAdvisorTurn(t.ID, advisor.MilestoneBriefingPrompt(event, data), snapshot)
}

func milestoneTitle(event string, data map[string]interface{}) string {
	switch event {
	case "combine.passed":
		if name, ok := data["account_name"].(string); ok {
			return "Briefing: Combine passed (" + name + ")"
		}
		return "Briefing: Combine passed"
	case "account.funded":
		if name, ok := data["account_name"].(string); ok {
			return "Briefing: " + name + " crossed the Rubicon"
		}
		return "Briefing: Account funded"
	case "payout.recorded":
		if amt, ok := data["net_amount"].(float64); ok {
			return fmt.Sprintf("Briefing: $%.0f payout", amt)
		}
		return "Briefing: Payout"
	default:
		return "Briefing: " + event
	}
}
