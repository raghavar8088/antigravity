// Pre-Live Trade Engine — 100 Backtested-Qualified Strategies
//
// Connects to real brokers (Coinbase price feed, Binance klines, Delta Exchange
// for live order execution). Enforces ONLY the strategy-level SL/TP thresholds
// baked into each strategy. No position count limits, no max-trades-per-strategy,
// no daily-loss circuit breaker, no correlation gate.
//
// Run:
//
//	go run ./cmd/pre_live/main.go
//
// Port: PRE_LIVE_PORT env var (default 8082)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"antigravity-engine/internal/aiscoring"
	"antigravity-engine/internal/dataquality"
	"antigravity-engine/internal/derivatives"
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/orderbook"
	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/regime"
	"antigravity-engine/internal/risk"
	"antigravity-engine/internal/strategy"
	"antigravity-engine/internal/temporal"
	"antigravity-engine/internal/trading"
)

// warmupCandles fetches the last 200 1h candles from Binance REST and pushes
// them into the orchestrator so 4h synthetic candles are available immediately
// (4h candles are built by accumulating 4×1h bars). Without this, strategies
// that require 4h indicators would be silent for the first 4+ hours.
func warmupCandles(orch *trading.Orchestrator) {
	fetcher := marketdata.NewBinanceHistoricalFetcher(os.TempDir())
	endMs := time.Now().UnixMilli()
	startMs := endMs - int64(200*time.Hour/time.Millisecond)
	candles, err := fetcher.FetchKlines("BTCUSDT", "1h", startMs, endMs)
	if err != nil {
		log.Printf("[PRE-LIVE] warmup fetch failed (will wait for live 1h bars): %v", err)
		return
	}
	for _, c := range candles {
		orch.Push1hKlineCandle(marketdata.Candle{
			OpenTime:  c.OpenTime,
			CloseTime: c.CloseTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
		})
	}
	log.Printf("[PRE-LIVE] warmup complete: pushed %d 1h candles → ~%d synthetic 4h candles ready", len(candles), len(candles)/4)
}

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║   PRE-LIVE TRADE ENGINE — 100 Qualified Strategies       ║")
	fmt.Println("║   Real Broker · Real Money · No Position Limits          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")

	loadDotEnv()

	// Set pre-live account key before any paperpersist calls so MongoDB data is
	// segregated from the main paper desk (account_key="pre_live_engine").
	applyPreLiveAccountKey()

	port := os.Getenv("PRE_LIVE_PORT")
	if port == "" {
		port = "8082"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── 1. Market data feeds ─────────────────────────────────────────────────
	coinbase := marketdata.NewCoinbaseClient()
	go func() {
		if err := coinbase.Connect(ctx, []string{"BTC-USD"}); err != nil {
			log.Fatalf("[PRE-LIVE] Coinbase feed error: %v", err)
		}
	}()

	// ── 2. Load 100 qualified strategies (no shadow overlay, no winners filter) ──
	qualified := strategy.BuildPreLiveStrategies()
	log.Printf("[PRE-LIVE] Loaded %d qualified strategies", len(qualified))

	names := make([]string, len(qualified))
	cats := make([]string, len(qualified))
	tfs := make([]string, len(qualified))
	for i, e := range qualified {
		names[i] = e.Strategy.Name()
		cats[i] = strategy.NormalizeCategory(e.Category, e.Strategy.Name())
		tfs[i] = e.Timeframe
	}

	// ── 3. Risk engine — unlimited positions, no daily loss circuit breaker ──
	riskProfile := risk.RiskProfile{
		MaxPositionBTC:  99999,
		MaxCapitalUSD:   preLiveBalance(),
		MaxDailyLossPct: 0,
	}
	riskEngine := risk.NewRiskEngine(riskProfile)

	// ── 4. Core components ───────────────────────────────────────────────────
	tracker := risk.NewStrategyTracker(names, cats, tfs, preLiveBalance())
	exec := execution.NewPaperClient(preLiveBalance())
	posMgr := positions.NewManagerWithConfig(positions.ManagerConfig{
		TrailingStopPct:    0.18,
		BreakEvenThreshold: 0.00,
		PartialTPRatio:     1.0,
		MinTakeProfitPct:   0.30,
		MaxPerStrategy:     2,
		ReverseTargets:     false,
		MaxPositionAgeMins: 45,
		Leverage:           preLiveLeverage(),
	})

	// Zero cooldown — all 100 strategies fire in parallel every tick.
	agg := trading.NewSignalAggregator(0)

	journal := execution.NewTradeJournal(10000)
	candleAgg := marketdata.NewCandleAggregator()

	// ── 5. Orchestrator ──────────────────────────────────────────────────────
	orch := trading.NewOrchestrator(coinbase, qualified, riskEngine, exec, agg, posMgr, tracker, journal, candleAgg)

	// ── 6. LoopDeps (required minimal set) ───────────────────────────────────
	dataValidator := dataquality.NewValidator()
	cycleGuard := trading.NewCycleGuard()
	fallbackScorer := aiscoring.NewFallbackScorer()
	asyncScorer := aiscoring.NewAsyncScorer(nil, 3)
	asyncScorer.Start()

	regimeClassifier := regime.NewClassifier()
	strategyGate := regime.NewStrategyGate(regimeClassifier, &strategyNamesAdapter{strategies: qualified})

	fundingFetcher := derivatives.NewFundingFetcher()
	oiFetcher := derivatives.NewOIFetcher()
	go fundingFetcher.StartPolling(ctx, 15*time.Minute)
	go oiFetcher.StartPolling(ctx, 15*time.Minute)

	depthSub := orderbook.NewDepthSubscriber()
	go func() {
		if err := depthSub.Connect(ctx); err != nil {
			log.Printf("[PRE-LIVE] depth subscriber exited: %v", err)
		}
	}()

	temporalAnalyser := temporal.NewTemporalAnalyser()

	loopDeps := &trading.LoopDeps{
		DataValidator:    dataValidator,
		AsyncScorer:      asyncScorer,
		FallbackScorer:   fallbackScorer,
		RegimeClassifier: regimeClassifier,
		StrategyGate:     strategyGate,
		CycleGuard:       cycleGuard,
		FundingFetcher:   fundingFetcher,
		OIFetcher:        oiFetcher,
		DepthSubscriber:  depthSub,
		PortfolioValue:   preLiveBalance(),
		Ledger:           orch.PortfolioLedger(),
		TemporalAnalyser: temporalAnalyser,
	}
	if err := loopDeps.Validate(); err != nil {
		log.Fatalf("[PRE-LIVE] LoopDeps invalid: %v", err)
	}
	orch.SetDeps(loopDeps)

	// ── 7. Binance kline feed for 15m/1h candles ─────────────────────────────
	klineClient := marketdata.NewBinanceKlineClient([]string{"15m", "1h"})
	go func() {
		if err := klineClient.Start(
			ctx,
			func(c marketdata.Candle) {
				orch.Push15mKlineCandle(c)
				orch.SetKlineFeedActive("15m", true)
			},
			func(c marketdata.Candle) {
				orch.Push1hKlineCandle(c)
				orch.SetKlineFeedActive("1h", true)
			},
		); err != nil {
			log.Printf("[PRE-LIVE] kline feed stopped: %v", err)
		}
		orch.SetKlineFeedActive("15m", false)
		orch.SetKlineFeedActive("1h", false)
	}()

	// ── 8. Binance aggTrade feed for CVD ─────────────────────────────────────
	aggTradeClient := marketdata.NewBinanceAggTradeClient("btcusdt", func(t marketdata.AggTrade) {
		orch.PushAggTrade(t)
	})
	go func() {
		if err := aggTradeClient.Start(ctx); err != nil {
			log.Printf("[PRE-LIVE] aggTrade feed error: %v", err)
		}
		orch.SetAggTradeFeedActive(false)
	}()

	// ── 9. Pre-warm 4h candles then start trading loop ───────────────────────
	// Fetch 200 historical 1h bars so 4h synthetic candles are ready immediately.
	warmupCandles(orch)
	go orch.Run(ctx)
	log.Printf("[PRE-LIVE] Trading loop started with %d strategies (no limits)", len(qualified))

	// ── 10. MongoDB persistence ───────────────────────────────────────────────
	// Wire all paperpersist collections (trades, positions, equity, health, etc.)
	// and seed backtest_results.json into pre_live_strategies.
	// Runs in-memory only when MONGODB_URI is not configured (graceful degradation).
	qualifiedNames := names // reuse the names slice built at step 2
	mongoBundle := initPreLiveMongo(ctx, orch, journal, preLiveBalance(), qualifiedNames)

	// ── 11. HTTP API ──────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	setCORS := func(w http.ResponseWriter) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	}

	handle := func(pattern string, fn func(http.ResponseWriter, *http.Request)) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			setCORS(w)
			if r.Method == http.MethodOptions {
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fn(w, r)
		})
	}

	handle("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"status":            "ok",
			"engine":            "pre-live",
			"strategies":        len(qualified),
			"mongodb_connected": mongoBundle.IsConnected(),
		})
	})

	handle("/ready", func(w http.ResponseWriter, r *http.Request) {
		mongoOK := mongoBundle.IsConnected()
		status := "ready"
		if !mongoOK {
			status = "degraded"
		}
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"status":            status,
			"engine":            "pre-live",
			"mongodb_connected": mongoOK,
			"strategies":        len(qualified),
		})
	})

	handle("/api/positions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(posMgr.GetOpenPositions()) //nolint:errcheck
	})

	handle("/api/trades", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(journal.GetRecentTrades(1000)) //nolint:errcheck
	})

	handle("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		ticks, candles := candleAgg.GetStats()
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"aggregate":      journal.GetAggregateStats(),
			"balance":        exec.GetBalanceUSD(),
			"equity":         exec.GetEquityUSD(),
			"cashBalance":    exec.GetBalanceUSD(),
			"initialBalance": preLiveBalance(),
			"exposure":       riskEngine.GetAbsoluteExposure(),
			"netPosition":    riskEngine.GetExposure(),
			"dailyPnl":       riskEngine.GetDailyPnL(),
			"lastPrice":      exec.GetLastPrice(),
			"openPositions":  len(posMgr.GetOpenPositions()),
			"ticksProcessed": ticks,
			"candlesClosed":  candles,
			"strategies":     len(qualified),
		})
	})

	handle("/api/scalers/stats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(orch.GetScalersStats()) //nolint:errcheck
	})

	// POST /api/admin/reset — wipe all trade history and restore starting balance.
	// Forces all open positions closed at the last known price first.
	handle("/api/admin/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		lastPrice := exec.GetLastPrice()
		if lastPrice <= 0 {
			http.Error(w, "no price available — cannot close positions", http.StatusServiceUnavailable)
			return
		}
		posMgr.CloseAllPositions(lastPrice)
		journal.Reset()
		if err := exec.ResetAccount(); err != nil {
			http.Error(w, "reset failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("[PRE-LIVE RESET] account reset at price $%.2f — fresh start", lastPrice)
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"ok":             true,
			"message":        "pre-live engine reset — balance restored, all trades cleared",
			"initialBalance": preLiveBalance(),
		})
	})

	handle("/api/regime", func(w http.ResponseWriter, r *http.Request) {
		reg := orch.GetScalersStats().Regime
		if reg == "" {
			reg = "UNKNOWN"
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"regime": reg}) //nolint:errcheck
	})

	handle("/api/strategies", func(w http.ResponseWriter, r *http.Request) {
		rows := make([]map[string]interface{}, 0, len(qualified))
		for _, e := range qualified {
			rows = append(rows, map[string]interface{}{
				"name":      e.Strategy.Name(),
				"category":  e.Category,
				"timeframe": e.Timeframe,
			})
		}
		json.NewEncoder(w).Encode(rows) //nolint:errcheck
	})

	handle("/api/strategies/stats", func(w http.ResponseWriter, r *http.Request) {
		allStats := tracker.GetAllStats()
		rows := make([]map[string]interface{}, 0, len(qualified))
		for _, e := range qualified {
			name := e.Strategy.Name()
			s := allStats[name]
			var winRate float64
			if s.TotalTrades > 0 {
				winRate = float64(s.Wins) / float64(s.TotalTrades) * 100
			}
			rows = append(rows, map[string]interface{}{
				"name":          name,
				"category":      e.Category,
				"timeframe":     e.Timeframe,
				"trades":        s.TotalTrades,
				"wins":          s.Wins,
				"losses":        s.Losses,
				"winRate":       winRate,
				"pnl":           s.TotalPnL,
				"signalsFired":  s.SignalsFired,
			})
		}
		json.NewEncoder(w).Encode(rows) //nolint:errcheck
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("[PRE-LIVE] HTTP API listening on :%s", port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("[PRE-LIVE] Shutdown signal received")
		cancel()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		srv.Shutdown(shutCtx) //nolint:errcheck
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[PRE-LIVE] HTTP server error: %v", err)
	}
}

func preLiveLeverage() float64 {
	v := os.Getenv("PRE_LIVE_LEVERAGE")
	if v == "" {
		return 10.0
	}
	var f float64
	fmt.Sscanf(v, "%f", &f)
	if f <= 0 {
		return 10.0
	}
	return f
}

func preLiveBalance() float64 {
	v := os.Getenv("PRE_LIVE_INITIAL_BALANCE_USD")
	if v == "" {
		return 100
	}
	var f float64
	fmt.Sscanf(v, "%f", &f)
	if f <= 0 {
		return 100
	}
	return f
}

// loadDotEnv reads the .env at the repo root and injects any missing env vars.
func loadDotEnv() {
	_, thisFile, _, ok := runtime.Caller(0)
	var envPath string
	if ok {
		// thisFile = .../engine/cmd/pre_live/main.go — repo root is 3 dirs up.
		envPath = filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".env")
	} else {
		envPath = "../../../.env"
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if os.Getenv(key) == "" {
			os.Setenv(key, val) //nolint:errcheck
		}
	}
}

// strategyNamesAdapter implements regime.StrategyRegistry so StrategyGate can
// classify strategies by name.
type strategyNamesAdapter struct {
	mu         sync.RWMutex
	strategies []strategy.RegistryEntry
}

func (a *strategyNamesAdapter) Names() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	names := make([]string, len(a.strategies))
	for i, e := range a.strategies {
		names[i] = e.Strategy.Name()
	}
	return names
}
