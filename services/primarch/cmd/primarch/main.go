// Primarch — Control Plane for the Astartes Primaris trading platform.
//
// Usage:
//
//	PRIMARCH_PORT=8401 PRIMARCH_SEED=true ./primarch
//
// API:
//
//	GET  /health                              — Health check
//	GET  /api/v1/status                       — Imperium status overview
//	GET  /api/v1/fortresses                   — List all fortresses
//	POST /api/v1/fortresses                   — Create fortress
//	GET  /api/v1/marines                      — List all marines
//	POST /api/v1/marines                      — Register a marine
//	POST /api/v1/marines/{id}/wake            — Trigger immediate wake cycle
//	POST /api/v1/marines/{id}/enable          — Enable scheduled execution
//	POST /api/v1/marines/{id}/disable         — Disable marine
//	POST /api/v1/kill-switch/{scope}          — Emergency halt
//	GET  /ws                                  — WebSocket for live events
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/api"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/config"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/domain"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/runner"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/scheduler"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/store"
)

func main() {
	cfg := config.Load()

	// Logger
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	fmt.Println(`
    ╔═══════════════════════════════════════════╗
    ║          PRIMARCH — CONTROL PLANE         ║
    ║       Astartes Primaris v0.1.0            ║
    ╚═══════════════════════════════════════════╝`)

	// Initialize components
	dataStore := store.New()
	runnerMgr := runner.NewManager(logger)

	// Create WebSocket hub first (shared by API server and scheduler events)
	hub := api.NewWSHub()

	// Event sink broadcasts to WebSocket clients
	eventSink := scheduler.EventSink(func(event domain.SystemEvent) {
		logger.Info("event", "service", event.Service, "event", event.Event, "marine", event.MarineID)
		hub.Broadcast(event)
	})

	sched := scheduler.New(dataStore, runnerMgr, eventSink, logger)
	srv := api.NewServerWithHub(dataStore, sched, logger, hub)

	// Seed initial data
	if cfg.Seed {
		fmt.Println("\n  Seeding Imperium hierarchy...")
		api.SeedFuturesFortress(dataStore)
	}

	// Start scheduler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(ctx)

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Printf("\n  Primarch listening on %s\n", addr)
	fmt.Printf("  Dashboard API:  http://localhost%s/api/v1/status\n", addr)
	fmt.Printf("  WebSocket:      ws://localhost%s/ws\n", addr)
	fmt.Printf("  Health:         http://localhost%s/health\n\n", addr)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logger.Info("shutdown signal received", "signal", sig)
		fmt.Println("\n  Primarch shutting down...")
		sched.Stop()
		cancel()
		httpServer.Shutdown(context.Background())
	}()

	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	fmt.Println("  Primarch offline.")
}
