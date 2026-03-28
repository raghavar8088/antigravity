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
	fmt.Println("║   ANTIGRAVITY ENGINE v2.0 — MULTI-STRATEGY EDITION     ║")
	fmt.Println("║   40 Parallel Strategies | Trailing SL/TP | $100K      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	// 7. Signal Aggregator (30s cooldown per strategy)
	// ═══════════════════════════════════════════════════
	aggregator := trading.NewSignalAggregator(30)

	// ═══════════════════════════════════════════════════
	// 8. Trade Journal
	// ═══════════════════════════════════════════════════
	journal := execution.NewTradeJournal(500)

	// ═══════════════════════════════════════════════════
	// 9. Multi-Strategy Orchestrator
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
	)
	go orchestrator.Run(ctx)

	// ═══════════════════════════════════════════════════
	// 10. HTTP API Server
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
			"service":    "antigravity-engine-v2",
			"strategies": len(allStrategies),
			"uptime":     time.Since(time.Now()).String(),
		})
	})

	// ───── NEW API ENDPOINTS ─────

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

		response := map[string]interface{}{
			"aggregate":    aggStats,
			"balance":      paperExecute.GetBalanceUSD(),
			"exposure":     riskEngine.GetExposure(),
			"dailyPnl":     riskEngine.GetDailyPnL(),
			"totalFees":    paperExecute.GetTotalFees(),
			"lastPrice":    paperExecute.GetLastPrice(),
			"openPositions": len(posMgr.GetOpenPositions()),
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
