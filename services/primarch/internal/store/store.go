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
	mu        sync.RWMutex
	fortresses map[string]*domain.Fortress
	companies  map[string]*domain.Company
	marines    map[string]*domain.Marine
	cycles     []domain.MarineCycle
}

// New creates a new empty store.
func New() *Store {
	return &Store{
		fortresses: make(map[string]*domain.Fortress),
		companies:  make(map[string]*domain.Company),
		marines:    make(map[string]*domain.Marine),
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
