// Package scheduler manages the wake/sleep lifecycle of Marines.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/runner"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/store"
)

// EventSink receives lifecycle events (for WebSocket broadcast, Vox, etc).
type EventSink func(event domain.SystemEvent)

// Scheduler orchestrates marine wake/sleep cycles on their configured schedules.
type Scheduler struct {
	store     *store.Store
	runner    *runner.Manager
	eventSink EventSink
	logger    *slog.Logger

	mu       sync.Mutex
	timers   map[string]*time.Timer   // marine ID → next wake timer
	cancels  map[string]context.CancelFunc
	running  bool
}

// New creates a scheduler.
func New(s *store.Store, r *runner.Manager, sink EventSink, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		store:     s,
		runner:    r,
		eventSink: sink,
		logger:    logger,
		timers:    make(map[string]*time.Timer),
		cancels:   make(map[string]context.CancelFunc),
	}
}

// Start begins scheduling all enabled marines.
func (sc *Scheduler) Start(ctx context.Context) {
	sc.mu.Lock()
	sc.running = true
	sc.mu.Unlock()

	sc.logger.Info("scheduler started")
	sc.emit("scheduler", "started", "", nil)

	// Schedule all currently registered marines
	for _, m := range sc.store.ListAllMarines() {
		if m.Schedule.Enabled && m.Status != domain.StatusDisabled {
			sc.ScheduleMarine(ctx, m.ID)
		}
	}
}

// Stop cancels all scheduled marines.
func (sc *Scheduler) Stop() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.running = false
	for id, cancel := range sc.cancels {
		cancel()
		delete(sc.cancels, id)
	}
	for id, timer := range sc.timers {
		timer.Stop()
		delete(sc.timers, id)
	}
	sc.logger.Info("scheduler stopped")
}

// ScheduleMarine sets up the recurring wake timer for a marine.
func (sc *Scheduler) ScheduleMarine(ctx context.Context, marineID string) error {
	m, err := sc.store.GetMarine(marineID)
	if err != nil {
		return err
	}

	if !m.Schedule.Enabled {
		return fmt.Errorf("marine %q schedule is disabled", marineID)
	}

	interval, err := sc.resolveInterval(m)
	if err != nil {
		return fmt.Errorf("invalid schedule for marine %q: %w", marineID, err)
	}

	sc.mu.Lock()
	// Cancel existing timer if any
	if cancel, ok := sc.cancels[marineID]; ok {
		cancel()
	}
	if timer, ok := sc.timers[marineID]; ok {
		timer.Stop()
	}

	mCtx, cancel := context.WithCancel(ctx)
	sc.cancels[marineID] = cancel
	sc.mu.Unlock()

	sc.logger.Info("marine scheduled", "marine", marineID, "interval", interval)
	sc.emit("scheduler", "marine_scheduled", marineID, map[string]interface{}{
		"interval": interval.String(),
	})

	// Start the recurring timer
	go sc.runLoop(mCtx, marineID, interval)
	return nil
}

// UnscheduleMarine stops the recurring wake timer for a marine.
func (sc *Scheduler) UnscheduleMarine(marineID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if cancel, ok := sc.cancels[marineID]; ok {
		cancel()
		delete(sc.cancels, marineID)
	}
	if timer, ok := sc.timers[marineID]; ok {
		timer.Stop()
		delete(sc.timers, marineID)
	}
	sc.logger.Info("marine unscheduled", "marine", marineID)
}

// WakeNow triggers an immediate wake cycle for a marine (manual trigger).
func (sc *Scheduler) WakeNow(ctx context.Context, marineID string) error {
	m, err := sc.store.GetMarine(marineID)
	if err != nil {
		return err
	}
	if m.Status == domain.StatusDisabled {
		return fmt.Errorf("marine %q is disabled (kill switch)", marineID)
	}
	go sc.executeCycle(ctx, marineID)
	return nil
}

func (sc *Scheduler) runLoop(ctx context.Context, marineID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Execute first cycle immediately
	sc.executeCycle(ctx, marineID)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.executeCycle(ctx, marineID)
		}
	}
}

func (sc *Scheduler) executeCycle(ctx context.Context, marineID string) {
	m, err := sc.store.GetMarine(marineID)
	if err != nil {
		sc.logger.Error("failed to get marine for cycle", "marine", marineID, "error", err)
		return
	}

	if m.Status == domain.StatusDisabled {
		sc.logger.Warn("skipping cycle for disabled marine", "marine", marineID)
		return
	}

	// Check trading hours
	if !sc.withinTradingHours(m) {
		sc.logger.Debug("outside trading hours", "marine", marineID)
		return
	}

	cycleStart := time.Now()
	cycleID := fmt.Sprintf("%s-%d", marineID, cycleStart.UnixMilli())

	// WAKE
	sc.store.UpdateMarineStatus(marineID, domain.StatusWaking)
	sc.emit("lifecycle", "wake", marineID, nil)
	sc.logger.Info("marine waking", "marine", marineID, "cycle", cycleID)

	// Execute via runner
	result, err := sc.runner.Execute(ctx, m)

	cycleDuration := time.Since(cycleStart)
	cycle := domain.MarineCycle{
		ID:         cycleID,
		MarineID:   marineID,
		WakeAt:     cycleStart,
		DurationMs: cycleDuration.Milliseconds(),
	}

	if err != nil {
		sc.store.UpdateMarineStatus(marineID, domain.StatusFailed)
		cycle.Status = domain.StatusFailed
		cycle.Error = err.Error()
		sc.logger.Error("marine cycle failed", "marine", marineID, "error", err, "duration", cycleDuration)
		sc.emit("lifecycle", "failed", marineID, map[string]interface{}{
			"error":       err.Error(),
			"duration_ms": cycleDuration.Milliseconds(),
		})
	} else {
		sc.store.UpdateMarineStatus(marineID, domain.StatusDormant)
		cycle.Status = domain.StatusDormant
		cycle.SignalsGenerated = result.SignalsGenerated
		cycle.OrdersSubmitted = result.OrdersSubmitted
		sc.logger.Info("marine cycle complete", "marine", marineID, "duration", cycleDuration,
			"signals", result.SignalsGenerated, "orders", result.OrdersSubmitted)
		sc.emit("lifecycle", "sleep", marineID, map[string]interface{}{
			"duration_ms": cycleDuration.Milliseconds(),
			"signals":     result.SignalsGenerated,
			"orders":      result.OrdersSubmitted,
		})
	}

	now := time.Now()
	cycle.SleepAt = &now
	sc.store.RecordCycle(cycle)
}

func (sc *Scheduler) resolveInterval(m *domain.Marine) (time.Duration, error) {
	switch m.Schedule.Type {
	case domain.ScheduleInterval:
		return time.ParseDuration(m.Schedule.Interval)
	case domain.ScheduleCron:
		// For MVP, treat cron as interval-based with common mappings
		// Full cron support to be added
		return 1 * time.Minute, nil
	case domain.ScheduleManual:
		// Manual marines don't have a recurring timer — return a large duration
		// They're triggered via WakeNow
		return 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported schedule type: %s", m.Schedule.Type)
	}
}

func (sc *Scheduler) withinTradingHours(m *domain.Marine) bool {
	th := m.Schedule.TradingHours
	if th == nil {
		return true // No trading hours restriction
	}

	loc, err := time.LoadLocation(th.Timezone)
	if err != nil {
		sc.logger.Warn("invalid timezone, allowing execution", "timezone", th.Timezone)
		return true
	}

	now := time.Now().In(loc)

	// Check day
	dayName := now.Weekday().String()[:3]
	dayAllowed := false
	for _, d := range th.ActiveDays {
		if d == dayName || d == now.Weekday().String() {
			dayAllowed = true
			break
		}
	}
	if !dayAllowed {
		return false
	}

	// Check time
	openH, openM := parseTime(th.MarketOpen)
	closeH, closeM := parseTime(th.MarketClose)
	nowMinutes := now.Hour()*60 + now.Minute()
	openMinutes := openH*60 + openM
	closeMinutes := closeH*60 + closeM

	return nowMinutes >= openMinutes && nowMinutes < closeMinutes
}

func parseTime(t string) (int, int) {
	var h, m int
	fmt.Sscanf(t, "%d:%d", &h, &m)
	return h, m
}

func (sc *Scheduler) emit(service, event, marineID string, data map[string]interface{}) {
	if sc.eventSink == nil {
		return
	}
	sc.eventSink(domain.SystemEvent{
		ID:        fmt.Sprintf("%s-%s-%d", service, event, time.Now().UnixMilli()),
		Service:   service,
		Event:     event,
		MarineID:  marineID,
		Timestamp: time.Now(),
		Data:      data,
	})
}
