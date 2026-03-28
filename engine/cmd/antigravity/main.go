package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"antigravity-engine/internal/admin"
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/risk"
	"antigravity-engine/internal/strategy"
	"antigravity-engine/internal/trading"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║   ANTIGRAVITY ENGINE v3.0 — CANDLE-AWARE EDITION       ║")
	fmt.Println("║   40 Strategies | 1m/5m Candles | Warmup | $100K       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bootStart := time.Now()

	// ═══════════════════════════════════════════════════
	// 1. WebSocket Live Stream
	// ═══════════════════════════════════════════════════
	binanceClient := marketdata.NewBinanceClient()
	go func() {
		err := binanceClient.Connect(ctx, []string{"btcusdt"})
		if err != nil {
			log.Fatalf("Fatal error connecting to Binance: %v", err)
		}
	}()

	// ═══════════════════════════════════════════════════
	// 2. Build ALL 40 Strategies
	// ═══════════════════════════════════════════════════
	allStrategies := strategy.BuildAllScalpers()
	log.Printf("[INIT] Loaded %d scalping strategies", len(allStrategies))

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
	aggregator := trading.NewSignalAggregator(15) // Reduced from 30s for faster trading

	// ═══════════════════════════════════════════════════
	// 8. Trade Journal
	// ═══════════════════════════════════════════════════
	journal := execution.NewTradeJournal(500)

	// ═══════════════════════════════════════════════════
	// 9. Candle Aggregator (NEW — converts ticks → 1m/5m candles)
	// ═══════════════════════════════════════════════════
	candleAgg := marketdata.NewCandleAggregator()
	log.Println("[INIT] ✅ Candle Aggregator ready (1m + 5m intervals)")

	// ═══════════════════════════════════════════════════
	// 10. Multi-Strategy Orchestrator (UPGRADED)
	// ═══════════════════════════════════════════════════
	orchestrator := trading.NewOrchestrator(
		binanceClient,
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
	// 11. WARMUP — Pre-fill strategy buffers from Binance REST
	// ═══════════════════════════════════════════════════
	log.Println("[WARMUP] Fetching historical candles to pre-fill strategy buffers...")
	warmupData, err := marketdata.FetchWarmupCandles("BTCUSDT", 500, 100)
	if err != nil {
		log.Printf("[WARMUP] ⚠️  Warmup failed (will warm up from live data): %v", err)
	} else {
		orchestrator.WarmupStrategies(warmupData)
	}

	log.Printf("[BOOT] Engine fully initialized in %s", time.Since(bootStart).Round(time.Millisecond))

	// Start the orchestrator
	go orchestrator.Run(ctx)

	// ═══════════════════════════════════════════════════
	// 12. HTTP API Server
	// ═══════════════════════════════════════════════════
	killswitch := admin.NewKillSwitch(cancel, paperExecute)

	// Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())

	// Admin endpoints
	http.HandleFunc("/api/admin/kill", killswitch.HandleTrigger)
	http.HandleFunc("/api/admin/reset", killswitch.HandleReset)

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions { return }
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
		if r.Method == http.MethodOptions { return }
		stats := tracker.GetAllStats()
		json.NewEncoder(w).Encode(stats)
	})

	// GET /api/positions — Open positions with live SL/TP
	http.HandleFunc("/api/positions", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions { return }
		openPositions := posMgr.GetOpenPositions()
		json.NewEncoder(w).Encode(openPositions)
	})

	// GET /api/trades — Completed trade journal
	http.HandleFunc("/api/trades", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions { return }
		trades := journal.GetRecentTrades(100)
		json.NewEncoder(w).Encode(trades)
	})

	// GET /api/stats — Aggregate performance statistics
	http.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions { return }
		aggStats := journal.GetAggregateStats()

		ticks, candles := candleAgg.GetStats()
		response := map[string]interface{}{
			"aggregate":      aggStats,
			"balance":        paperExecute.GetBalanceUSD(),
			"exposure":       riskEngine.GetExposure(),
			"dailyPnl":       riskEngine.GetDailyPnL(),
			"totalFees":      paperExecute.GetTotalFees(),
			"lastPrice":      paperExecute.GetLastPrice(),
			"openPositions":  len(posMgr.GetOpenPositions()),
			"ticksProcessed": ticks,
			"candlesClosed":  candles,
		}
		json.NewEncoder(w).Encode(response)
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
	binanceClient.Close()
	time.Sleep(1 * time.Second)
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
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	healthURL := fmt.Sprintf("http://localhost:%s/health", port)

	log.Printf("[KEEP-ALIVE] Self-ping enabled every 10m → %s", healthURL)

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
