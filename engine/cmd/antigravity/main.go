package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"antigravity-engine/internal/admin"
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/persistence"
	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/risk"
	"antigravity-engine/internal/strategy"
	"antigravity-engine/internal/trading"
)

// RingLogger stores the last N log lines in memory
type RingLogger struct {
	mu    sync.Mutex
	lines []string
	max   int
}

func (r *RingLogger) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, string(p))
	if len(r.lines) > r.max {
		r.lines = r.lines[1:]
	}
	fmt.Print(string(p)) // Also print to stdout for Render
	return len(p), nil
}

func (r *RingLogger) GetLogs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]string, len(r.lines))
	copy(cp, r.lines)
	return cp
}

var globalLogs = &RingLogger{max: 100}

func main() {
	log.SetOutput(globalLogs)
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║   ANTIGRAVITY ENGINE v6.0 — IMMORTAL EDITION           ║")
	fmt.Println("║   24 Curated Strategies | Full State Restore | Panic Recovery  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bootStart := time.Now()

	// ═══════════════════════════════════════════════════
	// 1. WebSocket Live Stream (Coinbase)
	// ═══════════════════════════════════════════════════
	coinbaseClient := marketdata.NewCoinbaseClient()
	go func() {
		err := coinbaseClient.Connect(ctx, []string{"BTC-USD"})
		if err != nil {
			log.Fatalf("Fatal error connecting to Coinbase: %v", err)
		}
	}()

	// ═══════════════════════════════════════════════════
	// 2. Build ALL 40 Strategies
	// ═══════════════════════════════════════════════════
	allStrategies := strategy.BuildCuratedScalpers()
	log.Printf("[INIT] Loaded %d curated live strategies", len(allStrategies))

	// Extract names, categories, timeframes for tracker
	names := make([]string, len(allStrategies))
	categories := make([]string, len(allStrategies))
	timeframes := make([]string, len(allStrategies))
	for i, e := range allStrategies {
		names[i] = e.Strategy.Name()
		categories[i] = e.Category
		timeframes[i] = e.Timeframe
	}

	// ═══════════════════════════════════════════════════
	// 3. Risk Engine (Expanded for $100K)
	// ═══════════════════════════════════════════════════
	riskProfile := risk.RiskProfile{
		MaxPositionBTC:  2.0,       // Max 2 BTC total exposure
		MaxCapitalUSD:   100000.00, // $100,000 paper balance
		MaxDailyLossPct: 0.05,      // 5% daily loss circuit breaker ($5,000)
	}
	riskEngine := risk.NewRiskEngine(riskProfile)

	// ═══════════════════════════════════════════════════
	// 4. Strategy Tracker (Per-Strategy Performance)
	// ═══════════════════════════════════════════════════
	tracker := risk.NewStrategyTracker(names, categories, timeframes, 100000.0)

	// ═══════════════════════════════════════════════════
	// 5. Paper Executor ($100K)
	// ═══════════════════════════════════════════════════
	paperExecute := execution.NewPaperClient(100000.0)

	// ═══════════════════════════════════════════════════
	// 6. Position Manager (Trailing SL/TP)
	// ═══════════════════════════════════════════════════
	posMgr := positions.NewManager()

	// ═══════════════════════════════════════════════════
	// 7. Signal Aggregator (15s cooldown per strategy)
	// ═══════════════════════════════════════════════════
	aggregator := trading.NewSignalAggregator(15)

	// ═══════════════════════════════════════════════════
	// 8. Trade Journal
	// ═══════════════════════════════════════════════════
	journal := execution.NewTradeJournal(500)

	// ═══════════════════════════════════════════════════
	// 9. Candle Aggregator
	// ═══════════════════════════════════════════════════
	candleAgg := marketdata.NewCandleAggregator()
	log.Println("[INIT] ✅ Candle Aggregator ready (1m + 5m intervals)")

	// ═══════════════════════════════════════════════════
	// 9b. DATABASE PERSISTENCE — FULL state restore from Neon PostgreSQL
	// ═══════════════════════════════════════════════════
	dbStore, err := persistence.NewStore(ctx)
	if err != nil {
		log.Printf("[DB] ⚠️  Database not available (will use fresh state): %v", err)
	} else {
		// ── Restore ALL state on boot ──
		state, loadErr := dbStore.LoadState(ctx)
		if loadErr == nil && state.Balance != 100000 {
			// 1. Restore paper balance + fees
			paperExecute.RestoreBalance(state.Balance, state.TotalFees)

			// 2. Restore open positions from DB
			var restoredPositions []positions.Position
			if len(state.Positions) > 2 { // Not empty "[]"
				if err := json.Unmarshal(state.Positions, &restoredPositions); err != nil {
					log.Printf("[DB] ⚠️  Failed to parse positions: %v", err)
				} else {
					posMgr.RestorePositions(restoredPositions)
				}
			}

			// 3. Restore trade journal from DB
			var restoredTrades []execution.JournalEntry
			if len(state.Trades) > 2 { // Not empty "[]"
				if err := json.Unmarshal(state.Trades, &restoredTrades); err != nil {
					log.Printf("[DB] ⚠️  Failed to parse trades: %v", err)
				} else {
					journal.RestoreTrades(restoredTrades,
						state.TotalTrades, state.TotalWins, state.TotalLosses, state.TotalPnL)
				}
			}

			log.Printf("[DB] ♻️  FULL state restored from %s | Balance: $%.2f | Positions: %d | Trades: %d",
				state.SavedAt.Format(time.RFC3339), state.Balance,
				posMgr.GetPositionCount(), state.TotalTrades)
		} else {
			log.Println("[DB] Fresh start — no previous state to restore")
		}
	}

	// ═══════════════════════════════════════════════════
	// 10. Multi-Strategy Orchestrator
	// ═══════════════════════════════════════════════════
	orchestrator := trading.NewOrchestrator(
		coinbaseClient,
		allStrategies,
		riskEngine,
		paperExecute,
		aggregator,
		posMgr,
		tracker,
		journal,
		candleAgg,
	)

	// ═══════════════════════════════════════════════════
	// 11. WARMUP — Pre-fill strategy buffers from Coinbase REST
	// ═══════════════════════════════════════════════════
	log.Println("[WARMUP] Fetching historical candles to pre-fill strategy buffers...")
	warmupData, err := marketdata.FetchWarmupCandles("BTC-USD")
	if err != nil {
		log.Printf("[WARMUP] ⚠️  Warmup failed (will warm up from live data): %v", err)
	} else {
		orchestrator.WarmupStrategies(warmupData)
	}

	log.Printf("[BOOT] Engine fully initialized in %s", time.Since(bootStart).Round(time.Millisecond))

	// Start the orchestrator with panic recovery
	go safeGo("Orchestrator", func() { orchestrator.Run(ctx) })

	// ═══════════════════════════════════════════════════
	// 11b. STATE SAVER — Periodic DB snapshots
	// ═══════════════════════════════════════════════════
	if dbStore != nil {
		saver := persistence.NewStateSaver(dbStore, paperExecute, posMgr, journal)
		go safeGo("StateSaver", func() { saver.Run(ctx) })
	}

	// ═══════════════════════════════════════════════════
	// 12. HTTP API Server
	// ═══════════════════════════════════════════════════
	killswitch := admin.NewKillSwitch(ctx, cancel, paperExecute, journal, posMgr, dbStore, riskEngine, tracker)

	// Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())

	// Admin endpoints
	http.HandleFunc("/api/admin/kill", killswitch.HandleTrigger)
	http.HandleFunc("/api/admin/reset", killswitch.HandleReset)

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "ok",
			"service":    "antigravity-engine-v3",
			"strategies": len(allStrategies),
			"uptime":     time.Since(bootStart).String(),
		})
	})

	// ───── API ENDPOINTS ─────

	// GET /api/strategies — Live strategy performance data
	http.HandleFunc("/api/strategies", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		stats := tracker.GetAllStats()
		json.NewEncoder(w).Encode(stats)
	})

	// GET /api/positions — Open positions with live SL/TP
	http.HandleFunc("/api/positions", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		openPositions := posMgr.GetOpenPositions()
		json.NewEncoder(w).Encode(openPositions)
	})

	// GET /api/trades — Completed trade journal
	http.HandleFunc("/api/trades", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		trades := journal.GetRecentTrades(100)
		json.NewEncoder(w).Encode(trades)
	})

	// GET /api/stats — Aggregate performance statistics
	http.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		aggStats := journal.GetAggregateStats()

		ticks, candles := candleAgg.GetStats()
		response := map[string]interface{}{
			"aggregate":      aggStats,
			"balance":        paperExecute.GetEquityUSD(),
			"cashBalance":    paperExecute.GetBalanceUSD(),
			"exposure":       riskEngine.GetAbsoluteExposure(),
			"netPosition":    riskEngine.GetExposure(),
			"dailyPnl":       riskEngine.GetDailyPnL(),
			"lastPrice":      paperExecute.GetLastPrice(),
			"openPositions":  len(posMgr.GetOpenPositions()),
			"ticksProcessed": ticks,
			"candlesClosed":  candles,
		}
		json.NewEncoder(w).Encode(response)
	})

	// GET /api/logs — Diagnostic memory buffer
	http.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"logs": globalLogs.GetLogs(),
		})
	})

	go func() {
		fmt.Println("═══════════════════════════════════════════")
		fmt.Println("  REST API Engine listening on :8080")
		fmt.Println("  Endpoints:")
		fmt.Println("    GET  /health          — Engine health")
		fmt.Println("    GET  /api/strategies   — Strategy stats")
		fmt.Println("    GET  /api/positions    — Open positions")
		fmt.Println("    GET  /api/trades       — Trade journal")
		fmt.Println("    GET  /api/stats        — Aggregate stats")
		fmt.Println("    GET  /api/logs         — Last 100 system logs")
		fmt.Println("    POST /api/admin/kill   — Kill switch")
		fmt.Println("    POST /api/admin/reset  — Reset account")
		fmt.Println("═══════════════════════════════════════════")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Println("Admin Server error:", err)
		}
	}()

	// ═══════════════════════════════════════════════════
	// 13. KEEP-ALIVE — Prevent Render free tier from sleeping
	// ═══════════════════════════════════════════════════
	go keepAlive(ctx)

	// Hardware Fallback Hook
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Hardware Kill Signal: Shutting down entire engine loop...")
	cancel()
	coinbaseClient.Close()
	if dbStore != nil {
		dbStore.Close()
	}
	time.Sleep(2 * time.Second) // Allow state saver final flush
	log.Println("Systems offline.")
}

// setCORS adds standard CORS headers for dashboard communication.
func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
}

// keepAlive pings the engine's own /health endpoint every 10 minutes
// to prevent Render free tier from spinning down the service.
// When the service sleeps, ALL strategy price buffers are lost.
func keepAlive(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	healthURL := fmt.Sprintf("http://localhost:%s/health", port)

	log.Printf("[KEEP-ALIVE] Self-ping enabled every 2m → %s", healthURL)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resp, err := http.Get(healthURL)
			if err != nil {
				log.Printf("[KEEP-ALIVE] Ping failed: %v", err)
			} else {
				resp.Body.Close()
				log.Println("[KEEP-ALIVE] ✅ Self-ping OK — engine stays warm")
			}
		}
	}
}

// safeGo wraps a goroutine function with panic recovery.
// If the goroutine panics, it logs the error and restarts automatically.
func safeGo(name string, fn func()) {
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[⚠️ PANIC RECOVERED] %s crashed: %v — restarting in 5s...", name, r)
				}
			}()
			fn()
		}()
		// If fn returned normally (context cancelled), don't restart
		log.Printf("[%s] Goroutine exited normally", name)
		return
	}
}
