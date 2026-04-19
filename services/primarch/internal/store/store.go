// Package store provides in-memory storage for the Primarch registry.
// This will be replaced with PostgreSQL (Librarium) once the data layer is wired up.
package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
)

// Store is a thread-safe in-memory registry for the Imperium hierarchy.
type Store struct {
	mu          sync.RWMutex
	fortresses  map[string]*domain.Fortress
	companies   map[string]*domain.Company
	marines     map[string]*domain.Marine
	cycles      []domain.MarineCycle
	// Council
	accounts      map[string]*domain.TradingAccount
	payouts       []domain.Payout
	budget        *domain.BudgetSummary
	allocations   []domain.Allocation
	roadmap       *domain.Roadmap
	// Goals & Billing
	goals         map[string]*domain.Goal
	contributions []domain.GoalContribution
	expenses      map[string]*domain.Expense
	payments      []domain.Payment
	holdings      map[string]*domain.Holding
	wheelCycles   map[string]*domain.WheelCycle
	wheelLegs     map[string]*domain.WheelLeg
	commands      map[string]*domain.Command
	trades        map[string]*domain.Trade
	positions     map[string]*domain.Position // key: marine_id:broker:symbol
	snapshots     []domain.AccountSnapshot
	bars                map[string]*domain.MarketBar // key: symbol:timeframe:time
	payoutAllocations   []domain.PayoutAllocation
	propFees            []domain.PropFee
	advisorThreads      map[string]*domain.AdvisorThread
	advisorMessages     []domain.AdvisorMessage
	ffTxns              map[string]*domain.FFTransaction
	mnTxns              map[string]*domain.MNTransaction
	syncStates          map[string]*domain.FinanceSyncState
}

// New creates a new empty store.
func New() *Store {
	return &Store{
		fortresses:  make(map[string]*domain.Fortress),
		companies:   make(map[string]*domain.Company),
		marines:     make(map[string]*domain.Marine),
		accounts:    make(map[string]*domain.TradingAccount),
		allocations: DefaultAllocations(),
		budget:      DefaultBudget(),
		roadmap:     DefaultRoadmap(),
		goals:       make(map[string]*domain.Goal),
		expenses:    make(map[string]*domain.Expense),
		holdings:    make(map[string]*domain.Holding),
		wheelCycles: make(map[string]*domain.WheelCycle),
		wheelLegs:   make(map[string]*domain.WheelLeg),
		commands:    make(map[string]*domain.Command),
		trades:      make(map[string]*domain.Trade),
		positions:   make(map[string]*domain.Position),
		bars:           make(map[string]*domain.MarketBar),
		advisorThreads: make(map[string]*domain.AdvisorThread),
		ffTxns:         make(map[string]*domain.FFTransaction),
		mnTxns:         make(map[string]*domain.MNTransaction),
		syncStates:     make(map[string]*domain.FinanceSyncState),
	}
}

// ─── Fortress ───────────────────────────────────────────────

func (s *Store) ListFortresses() []domain.Fortress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Fortress, 0, len(s.fortresses))
	for _, f := range s.fortresses {
		fc := *f
		fc.Companies = s.companiesForFortress(f.ID)
		out = append(out, fc)
	}
	return out
}

func (s *Store) GetFortress(id string) (*domain.Fortress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.fortresses[id]
	if !ok {
		return nil, fmt.Errorf("fortress %q not found", id)
	}
	fc := *f
	fc.Companies = s.companiesForFortress(id)
	return &fc, nil
}

func (s *Store) CreateFortress(f *domain.Fortress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.fortresses[f.ID]; exists {
		return fmt.Errorf("fortress %q already exists", f.ID)
	}
	now := time.Now()
	f.CreatedAt = now
	f.UpdatedAt = now
	s.fortresses[f.ID] = f
	return nil
}

func (s *Store) UpdateFortress(f *domain.Fortress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.fortresses[f.ID]; !exists {
		return fmt.Errorf("fortress %q not found", f.ID)
	}
	f.UpdatedAt = time.Now()
	s.fortresses[f.ID] = f
	return nil
}

func (s *Store) DeleteFortress(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.fortresses[id]; !exists {
		return fmt.Errorf("fortress %q not found", id)
	}
	// Check for child companies
	for _, c := range s.companies {
		if c.FortressID == id {
			return fmt.Errorf("cannot delete fortress %q: has companies", id)
		}
	}
	delete(s.fortresses, id)
	return nil
}

// ─── Company ────────────────────────────────────────────────

func (s *Store) ListCompanies(fortressID string) []domain.Company {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.companiesForFortress(fortressID)
}

// must hold at least a read lock
func (s *Store) companiesForFortress(fortressID string) []domain.Company {
	var out []domain.Company
	for _, c := range s.companies {
		if fortressID == "" || c.FortressID == fortressID {
			cc := *c
			cc.Marines = s.marinesForCompany(c.ID)
			out = append(out, cc)
		}
	}
	return out
}

func (s *Store) GetCompany(id string) (*domain.Company, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.companies[id]
	if !ok {
		return nil, fmt.Errorf("company %q not found", id)
	}
	cc := *c
	cc.Marines = s.marinesForCompany(id)
	return &cc, nil
}

func (s *Store) CreateCompany(c *domain.Company) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.companies[c.ID]; exists {
		return fmt.Errorf("company %q already exists", c.ID)
	}
	if _, exists := s.fortresses[c.FortressID]; !exists {
		return fmt.Errorf("fortress %q not found", c.FortressID)
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	s.companies[c.ID] = c
	return nil
}

func (s *Store) UpdateCompany(c *domain.Company) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.companies[c.ID]; !exists {
		return fmt.Errorf("company %q not found", c.ID)
	}
	c.UpdatedAt = time.Now()
	s.companies[c.ID] = c
	return nil
}

func (s *Store) DeleteCompany(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.companies[id]; !exists {
		return fmt.Errorf("company %q not found", id)
	}
	for _, m := range s.marines {
		if m.CompanyID == id {
			return fmt.Errorf("cannot delete company %q: has marines", id)
		}
	}
	delete(s.companies, id)
	return nil
}

// ─── Marine ─────────────────────────────────────────────────

func (s *Store) ListMarines(companyID string) []domain.Marine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.marinesForCompany(companyID)
}

func (s *Store) ListAllMarines() []domain.Marine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Marine, 0, len(s.marines))
	for _, m := range s.marines {
		out = append(out, *m)
	}
	return out
}

// must hold at least a read lock
func (s *Store) marinesForCompany(companyID string) []domain.Marine {
	var out []domain.Marine
	for _, m := range s.marines {
		if companyID == "" || m.CompanyID == companyID {
			out = append(out, *m)
		}
	}
	return out
}

func (s *Store) GetMarine(id string) (*domain.Marine, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.marines[id]
	if !ok {
		return nil, fmt.Errorf("marine %q not found", id)
	}
	mc := *m
	return &mc, nil
}

func (s *Store) CreateMarine(m *domain.Marine) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.marines[m.ID]; exists {
		return fmt.Errorf("marine %q already exists", m.ID)
	}
	if _, exists := s.companies[m.CompanyID]; !exists {
		return fmt.Errorf("company %q not found", m.CompanyID)
	}
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	if m.Status == "" {
		m.Status = domain.StatusDormant
	}
	s.marines[m.ID] = m
	return nil
}

func (s *Store) UpdateMarine(m *domain.Marine) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.marines[m.ID]; !exists {
		return fmt.Errorf("marine %q not found", m.ID)
	}
	m.UpdatedAt = time.Now()
	s.marines[m.ID] = m
	return nil
}

func (s *Store) UpdateMarineStatus(id string, status domain.MarineStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.marines[id]
	if !ok {
		return fmt.Errorf("marine %q not found", id)
	}
	m.Status = status
	m.UpdatedAt = time.Now()
	now := time.Now()
	switch status {
	case domain.StatusWaking:
		m.LastWake = &now
	case domain.StatusDormant, domain.StatusSleeping:
		m.LastSleep = &now
	}
	return nil
}

func (s *Store) DeleteMarine(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.marines[id]; !exists {
		return fmt.Errorf("marine %q not found", id)
	}
	delete(s.marines, id)
	return nil
}

// ─── Cycles ─────────────────────────────────────────────────

func (s *Store) RecordCycle(c domain.MarineCycle) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cycles = append(s.cycles, c)
	// Keep last 10000 cycles in memory
	if len(s.cycles) > 10000 {
		s.cycles = s.cycles[len(s.cycles)-10000:]
	}
}

func (s *Store) GetCycles(marineID string, limit int) []domain.MarineCycle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.MarineCycle
	for i := len(s.cycles) - 1; i >= 0 && len(out) < limit; i-- {
		if s.cycles[i].MarineID == marineID {
			out = append(out, s.cycles[i])
		}
	}
	return out
}

// ─── Kill Switch ────────────────────────────────────────────

func (s *Store) ActivateKillSwitch(scope string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case scope == "imperium":
		for _, m := range s.marines {
			m.Status = domain.StatusDisabled
			m.Schedule.Enabled = false
		}
	default:
		// Try as fortress
		if _, ok := s.fortresses[scope]; ok {
			for _, c := range s.companies {
				if c.FortressID == scope {
					for _, m := range s.marines {
						if m.CompanyID == c.ID {
							m.Status = domain.StatusDisabled
							m.Schedule.Enabled = false
						}
					}
				}
			}
			return
		}
		// Try as company
		if _, ok := s.companies[scope]; ok {
			for _, m := range s.marines {
				if m.CompanyID == scope {
					m.Status = domain.StatusDisabled
					m.Schedule.Enabled = false
				}
			}
			return
		}
		// Try as marine
		if m, ok := s.marines[scope]; ok {
			m.Status = domain.StatusDisabled
			m.Schedule.Enabled = false
		}
	}
}

// ─── Payout Allocations ────────────────────────────────────

func (s *Store) RecordAllocation(a domain.PayoutAllocation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	s.payoutAllocations = append(s.payoutAllocations, a)
	return nil
}

func (s *Store) ListAllocationsForMonth(year int, month int) []domain.PayoutAllocation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.PayoutAllocation
	for _, a := range s.payoutAllocations {
		if a.CreatedAt.Year() == year && int(a.CreatedAt.Month()) == month {
			out = append(out, a)
		}
	}
	return out
}

func (s *Store) ListAllocationsForPayout(payoutID string) []domain.PayoutAllocation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.PayoutAllocation
	for _, a := range s.payoutAllocations {
		if a.PayoutID == payoutID {
			out = append(out, a)
		}
	}
	return out
}

// ─── Prop Fees ──────────────────────────────────────────────

func (s *Store) RecordPropFee(f domain.PropFee) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}
	s.propFees = append(s.propFees, f)
	return nil
}

func (s *Store) ListPropFees(accountID string) []domain.PropFee {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.PropFee
	for _, f := range s.propFees {
		if accountID == "" || f.AccountID == accountID {
			out = append(out, f)
		}
	}
	return out
}

// ─── Holdings ───────────────────────────────────────────────

func (s *Store) ListHoldings() []domain.Holding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Holding, 0, len(s.holdings))
	for _, h := range s.holdings {
		out = append(out, *h)
	}
	return out
}

func (s *Store) CreateHolding(h *domain.Holding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.holdings[h.ID]; exists {
		return fmt.Errorf("holding %q already exists", h.ID)
	}
	now := time.Now()
	h.CreatedAt = now
	h.UpdatedAt = now
	cp := *h
	s.holdings[h.ID] = &cp
	return nil
}

func (s *Store) UpdateHolding(h *domain.Holding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.holdings[h.ID]; !ok {
		return fmt.Errorf("holding %q not found", h.ID)
	}
	h.UpdatedAt = time.Now()
	cp := *h
	s.holdings[h.ID] = &cp
	return nil
}

func (s *Store) DeleteHolding(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.holdings[id]; !ok {
		return fmt.Errorf("holding %q not found", id)
	}
	delete(s.holdings, id)
	return nil
}

// ─── Wheel Cycles ──────────────────────────────────────────

func (s *Store) ListWheelCycles() []domain.WheelCycle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.WheelCycle, 0, len(s.wheelCycles))
	for _, c := range s.wheelCycles {
		out = append(out, *c)
	}
	return out
}

func (s *Store) CreateWheelCycle(c *domain.WheelCycle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.wheelCycles[c.ID]; exists {
		return fmt.Errorf("wheel cycle %q already exists", c.ID)
	}
	if c.StartedAt.IsZero() {
		c.StartedAt = time.Now()
	}
	cp := *c
	s.wheelCycles[c.ID] = &cp
	return nil
}

func (s *Store) UpdateWheelCycle(c *domain.WheelCycle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.wheelCycles[c.ID]; !exists {
		return fmt.Errorf("wheel cycle %q not found", c.ID)
	}
	cp := *c
	s.wheelCycles[c.ID] = &cp
	return nil
}

func (s *Store) ListWheelLegs(cycleID string) []domain.WheelLeg {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.WheelLeg
	for _, l := range s.wheelLegs {
		if l.CycleID == cycleID {
			out = append(out, *l)
		}
	}
	return out
}

func (s *Store) CreateWheelLeg(l *domain.WheelLeg) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.wheelLegs[l.ID]; exists {
		return fmt.Errorf("wheel leg %q already exists", l.ID)
	}
	if l.OpenedAt.IsZero() {
		l.OpenedAt = time.Now()
	}
	cp := *l
	s.wheelLegs[l.ID] = &cp
	return nil
}

func (s *Store) UpdateWheelLeg(l *domain.WheelLeg) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.wheelLegs[l.ID]; !exists {
		return fmt.Errorf("wheel leg %q not found", l.ID)
	}
	cp := *l
	s.wheelLegs[l.ID] = &cp
	return nil
}

// ─── Commands (Engine Protocol) ────────────────────────────

func (s *Store) CreateCommand(c *domain.Command) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.commands[c.ID]; exists {
		return fmt.Errorf("command %q already exists", c.ID)
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	cp := *c
	s.commands[c.ID] = &cp
	return nil
}

func (s *Store) GetCommand(id string) (*domain.Command, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.commands[id]
	if !ok {
		return nil, fmt.Errorf("command %q not found", id)
	}
	cp := *c
	return &cp, nil
}

func (s *Store) ListPendingCommands(engineID string) []domain.Command {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Command
	for _, c := range s.commands {
		if c.EngineID == engineID && (c.Status == domain.CommandPending || c.Status == domain.CommandAcked) {
			out = append(out, *c)
		}
	}
	return out
}

func (s *Store) UpdateCommand(c *domain.Command) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.commands[c.ID]; !exists {
		return fmt.Errorf("command %q not found", c.ID)
	}
	c.UpdatedAt = time.Now()
	cp := *c
	s.commands[c.ID] = &cp
	return nil
}

// ─── Trades ────────────────────────────────────────────────

func (s *Store) UpsertTrade(t *domain.Trade) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.trades[t.ID]
	cp := *t
	s.trades[t.ID] = &cp
	return !exists, nil
}

func (s *Store) ListTrades(marineID string, since *time.Time, limit int) []domain.Trade {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Trade
	for _, t := range s.trades {
		if marineID != "" && t.MarineID != marineID {
			continue
		}
		if since != nil && t.ExitTime.Before(*since) {
			continue
		}
		out = append(out, *t)
	}
	// Simple sort by exit time descending
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].ExitTime.After(out[i].ExitTime) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ─── Positions ─────────────────────────────────────────────

func (s *Store) UpsertPosition(p *domain.Position) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := p.MarineID + ":" + p.BrokerAccountID + ":" + p.Symbol
	cp := *p
	s.positions[key] = &cp
	return nil
}

func (s *Store) ListPositions(marineID string) []domain.Position {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Position
	for _, p := range s.positions {
		if marineID == "" || p.MarineID == marineID {
			out = append(out, *p)
		}
	}
	return out
}

// ─── Account Snapshots ─────────────────────────────────────

func (s *Store) RecordAccountSnapshot(snap *domain.AccountSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = append(s.snapshots, *snap)
	if len(s.snapshots) > 10000 {
		s.snapshots = s.snapshots[len(s.snapshots)-10000:]
	}
	return nil
}

// ─── Market Bars ───────────────────────────────────────────

func (s *Store) UpsertBar(b *domain.MarketBar) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := b.Symbol + ":" + b.Timeframe + ":" + b.Time.Format(time.RFC3339Nano)
	_, exists := s.bars[key]
	cp := *b
	s.bars[key] = &cp
	return !exists, nil
}

// ─── Advisor ────────────────────────────────────────────────

func (s *Store) CreateAdvisorThread(t *domain.AdvisorThread) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	t.UpdatedAt = t.CreatedAt
	cp := *t
	s.advisorThreads[t.ID] = &cp
	return nil
}

func (s *Store) GetAdvisorThread(id string) (*domain.AdvisorThread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.advisorThreads[id]
	if !ok {
		return nil, fmt.Errorf("advisor thread %s not found", id)
	}
	cp := *t
	for _, m := range s.advisorMessages {
		if m.ThreadID == id {
			cp.Messages = append(cp.Messages, m)
		}
	}
	return &cp, nil
}

func (s *Store) ListAdvisorThreads(status string) []domain.AdvisorThread {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.AdvisorThread, 0, len(s.advisorThreads))
	for _, t := range s.advisorThreads {
		if status != "" && t.Status != status {
			continue
		}
		out = append(out, *t)
	}
	return out
}

func (s *Store) UpdateAdvisorThread(t *domain.AdvisorThread) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.advisorThreads[t.ID]; !ok {
		return fmt.Errorf("advisor thread %s not found", t.ID)
	}
	t.UpdatedAt = time.Now()
	cp := *t
	s.advisorThreads[t.ID] = &cp
	return nil
}

func (s *Store) AppendAdvisorMessage(m *domain.AdvisorMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.advisorThreads[m.ThreadID]; !ok {
		return fmt.Errorf("advisor thread %s not found", m.ThreadID)
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	s.advisorMessages = append(s.advisorMessages, *m)
	now := m.CreatedAt
	s.advisorThreads[m.ThreadID].LastMessageAt = &now
	s.advisorThreads[m.ThreadID].UpdatedAt = now
	return nil
}

func (s *Store) ListAdvisorMessages(threadID string) []domain.AdvisorMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.AdvisorMessage
	for _, m := range s.advisorMessages {
		if m.ThreadID == threadID {
			out = append(out, m)
		}
	}
	return out
}

// ─── Finance Ingest Cache (in-memory) ───────────────────────

func (s *Store) UpsertFFTransaction(t *domain.FFTransaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	t.UpdatedAt = time.Now()
	cp := *t
	s.ffTxns[t.ID] = &cp
	return nil
}

func (s *Store) UpsertMNTransaction(t *domain.MNTransaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	t.UpdatedAt = time.Now()
	cp := *t
	s.mnTxns[t.ID] = &cp
	return nil
}

func (s *Store) QueryFFTransactions(f domain.ActivityFilter) []domain.FFTransaction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.FFTransaction
	for _, t := range s.ffTxns {
		if !matchesFFFilter(t, f) {
			continue
		}
		out = append(out, *t)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out
}

func (s *Store) QueryMNTransactions(f domain.ActivityFilter) []domain.MNTransaction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.MNTransaction
	for _, t := range s.mnTxns {
		if !matchesMNFilter(t, f) {
			continue
		}
		out = append(out, *t)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out
}

func (s *Store) UpsertFinanceSyncState(ss *domain.FinanceSyncState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss.UpdatedAt = time.Now()
	cp := *ss
	s.syncStates[ss.Source] = &cp
	return nil
}

func (s *Store) GetFinanceSyncState(source string) (*domain.FinanceSyncState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ss, ok := s.syncStates[source]; ok {
		cp := *ss
		return &cp, nil
	}
	return nil, fmt.Errorf("sync state %s not found", source)
}

func (s *Store) ListFinanceSyncState() []domain.FinanceSyncState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.FinanceSyncState, 0, len(s.syncStates))
	for _, ss := range s.syncStates {
		out = append(out, *ss)
	}
	return out
}

func matchesFFFilter(t *domain.FFTransaction, f domain.ActivityFilter) bool {
	if f.Category != "" && t.Category != f.Category {
		return false
	}
	if f.BudgetName != "" && t.BudgetName != f.BudgetName {
		return false
	}
	if f.Tag != "" {
		found := false
		for _, tag := range t.Tags {
			if tag == f.Tag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func matchesMNFilter(t *domain.MNTransaction, f domain.ActivityFilter) bool {
	if f.Category != "" && t.Category != f.Category {
		return false
	}
	return true
}
