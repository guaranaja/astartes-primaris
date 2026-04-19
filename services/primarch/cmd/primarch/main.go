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

	"github.com/guaranaja/astartes-primaris/services/primarch/internal/advisor"
	wheeladvisor "github.com/guaranaja/astartes-primaris/services/primarch/internal/advisor/wheel"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/api"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/banking"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/brokers/tastytrade"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/cfo"
	"github.com/guaranaja/astartes-primaris/services/primarch/internal/ingest"
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

	// Initialize store (PostgreSQL or in-memory)
	var dataStore store.DataStore
	if cfg.UseDB() {
		pg, err := store.NewPGStore(cfg.DBUrl, logger)
		if err != nil {
			logger.Error("failed to connect to database", "error", err)
			os.Exit(1)
		}
		defer pg.Close()
		dataStore = pg
		fmt.Println("  Storage: PostgreSQL (persistent)")
	} else {
		dataStore = store.New()
		fmt.Println("  Storage: In-memory (ephemeral)")
	}

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

	// Initialize CFO integration (Firefly III + Monarch Money)
	{
		var fireflyClient *cfo.FireflyClient
		var monarchClient *cfo.MonarchClient

		if cfg.CFOEngineURL != "" && cfg.CFOEngineToken != "" {
			fireflyClient = cfo.NewFireflyClient(cfg.CFOEngineURL, cfg.CFOEngineToken)
			fmt.Printf("  CFO Engine:     %s\n", cfg.CFOEngineURL)
		}
		if cfg.MonarchToken != "" {
			monarchClient = cfo.NewMonarchClient(cfg.MonarchToken)
			fmt.Println("  Monarch Money:  connected")
		}
		if fireflyClient != nil || monarchClient != nil {
			councilCFO := cfo.NewCouncilCFO(fireflyClient, monarchClient, logger)
			srv.SetCFO(councilCFO)

			// Background ingest worker: pulls Firefly + Monarch transactions
			// into Postgres on a 15-minute cadence for fast dashboard queries.
			financeWorker := ingest.NewFinanceWorker(dataStore, fireflyClient, monarchClient, logger)
			srv.SetFinanceWorker(financeWorker)
			ingestCtx, cancelIngest := context.WithCancel(context.Background())
			_ = cancelIngest // cancellation tied to process lifetime
			financeWorker.Start(ingestCtx)
			fmt.Println("  Finance ingest: running (15m interval)")

			// Banking — Plaid for now. Requires PLAID_CLIENT_ID, PLAID_SECRET,
			// PLAID_TOKEN_ENC_KEY (32-byte base64). Silently disables if any
			// are missing.
			plaidProvider := banking.NewPlaidProvider(logger)
			crypter, cryptErr := banking.NewTokenCrypter()
			if plaidProvider != nil && plaidProvider.Available() && cryptErr == nil {
				bankingSvc := banking.NewService(plaidProvider, crypter, dataStore, fireflyClient, logger)
				srv.SetBanking(bankingSvc)
				financeWorker.SetBankingSync(bankingSvc.SyncAll)
				fmt.Printf("  Banking:        connected (plaid %s)\n", plaidProvider.Env())
			} else if cryptErr != nil && plaidProvider != nil && plaidProvider.Available() {
				logger.Warn("banking disabled: token crypter failed", "error", cryptErr)
			}
		}
	}

	// Claude advisor — enabled only if CLAUDE_API_KEY is set
	var claudeClient *advisor.Client
	if c := advisor.NewClient(logger); c != nil {
		claudeClient = c
		srv.SetAdvisor(claudeClient)
		fmt.Println("  Advisor:        connected (Claude)")
	}

	// Wheel advisor — tastytrade OAuth + rules engine + optional Claude review.
	// Balance snapshots flow back into Firefly (added in the wheel service).
	if tasty := tastytrade.NewFromEnv(logger); tasty != nil && tasty.Available() {
		// Find the Firefly client (if it was set up above) via the server's CFO.
		// The wheel service takes the firefly client directly for balance sync.
		var fireflyForWheel *cfo.FireflyClient
		if cfg.CFOEngineURL != "" && cfg.CFOEngineToken != "" {
			fireflyForWheel = cfo.NewFireflyClient(cfg.CFOEngineURL, cfg.CFOEngineToken)
		}
		wheelSvc := wheeladvisor.NewService(dataStore, tasty, claudeClient, fireflyForWheel, logger)
		if wheelSvc != nil {
			srv.SetWheelAdvisor(wheelSvc)
			wheelCtx, cancelWheel := context.WithCancel(context.Background())
			_ = cancelWheel
			wheelSvc.Start(wheelCtx)
			fmt.Println("  Wheel advisor:  running (tastytrade, hourly during market hours)")
		}
	}

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
