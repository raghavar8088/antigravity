// Pre-Live Trade Engine — Backtested-Qualified Strategies
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
	"antigravity-engine/internal/backtest"
	tconfig "antigravity-engine/internal/config"
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
	fmt.Println("║   PRE-LIVE TRADE ENGINE — Qualified Strategies           ║")
	fmt.Println("║   Real Broker · Paper Money · Validated Whitelist Only   ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")

	loadDotEnv()

	// ── Threshold overrides: applied to the ThresholdRegistry BEFORE
	// trading.init() runs its RefreshThresholdsFromRegistry call.
	// Because Go package init() has already run by this point, we call
	// trading.RefreshThresholdsFromRegistry() again after setting all keys
	// so the package-level vars pick up the pre-live values.
	//
	// RC-5 fix: MIN_SIGNAL_TAKE_PROFIT_PCT — the env var approach is a no-op
	// because loop.go's init() has already read the default 0.15 before main()
	// runs. We set it directly in the ThresholdRegistry instead.
	//
	// RC-3 fix: SCALER_RR_MINIMUM raised to 2.5 so that at SL=0.25% the TP
	// floor becomes 0.625%, making breakeven 40% instead of 63.6%.
	//
	// RC-2 fix: FIXED_TRADE_SIZE_BTC reduced to 0.001 BTC (~$100 notional at
	// $100k BTC) so it fits within the $100 default starting balance.
	reg := tconfig.Default()
	if reg.GetWithDefault("MIN_SIGNAL_TAKE_PROFIT_PCT", 0.0) < 0.50 {
		reg.Set("MIN_SIGNAL_TAKE_PROFIT_PCT", 0.50, "pre_live_rc5") //nolint:errcheck
	}
	if reg.GetWithDefault("SCALER_RR_MINIMUM", 0.0) < 2.5 {
		reg.Set("SCALER_RR_MINIMUM", 2.5, "pre_live_rc3") //nolint:errcheck
	}
	if os.Getenv("FIXED_TRADE_SIZE_BTC") == "" {
		reg.Set("FIXED_TRADE_SIZE_BTC", 0.001, "pre_live_rc2") //nolint:errcheck
	}
	// Re-read all thresholds now that the registry has the pre-live values.
	// This overwrites the defaults set during package init().
	trading.RefreshThresholdsFromRegistry()
	log.Printf("[PRE-LIVE] Thresholds applied: MIN_SIGNAL_TAKE_PROFIT_PCT=%.2f%% SCALER_RR_MINIMUM=%.1f FIXED_TRADE_SIZE_BTC=%.4f",
		reg.GetWithDefault("MIN_SIGNAL_TAKE_PROFIT_PCT", 0.50),
		reg.GetWithDefault("SCALER_RR_MINIMUM", 2.5),
		reg.GetWithDefault("FIXED_TRADE_SIZE_BTC", 0.001),
	)

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

	// ── 2. Load qualified strategies (no shadow overlay, no winners filter) ──
	qualified := strategy.BuildPreLiveStrategies()
	log.Printf("[PRE-LIVE] Loaded %d qualified strategies", len(qualified))
	// RC-4 assertion: warn if the whitelist has more names than strategies found
	// in the source pool. Silent drops here indicate a naming mismatch between
	// the whitelist and the builder functions (the dropped names never trade).
	if wl := strategy.PreLiveWhitelistSize(); wl > len(qualified) {
		log.Printf("[PRE-LIVE] WARNING: whitelist has %d entries but only %d strategies found in source pool — %d names are unmatched and will never trade",
			wl, len(qualified), wl-len(qualified))
	}

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

	// Seed the v2 drawdown high-water mark from the actual current equity so the
	// 10% drawdown circuit breaker measures from today's starting balance, not from
	// the original configured capital. Without this, any cumulative loss that already
	// exceeded 10% of initial capital would permanently halt all trading on restart.
	riskEngine.V2().ResetHighWatermark(exec.GetEquityUSD())
	posMgr := positions.NewManagerWithConfig(positions.ManagerConfig{
		TrailingStopPct:    0.18,
		BreakEvenThreshold: 0.00,
		PartialTPRatio:     1.0,
		MinTakeProfitPct:   0.625, // RC-3: raised from 0.50 to match SCALER_RR_MINIMUM=2.5 × SL_floor(0.25%)
		MaxPerStrategy:     2,
		ReverseTargets:     false,
		// RC-13 fix: raised from 20 to 90 minutes, then to 240 (4h).
		// The old 20-minute hard expiry was cutting 1h-timeframe strategies
		// before they reached their TP, locking in guaranteed fee-drag losses
		// (every EXPIRED exit costs 0.10% in fees with zero price movement).
		// 240 minutes gives 1h-timeframe strategies up to four full bars to play out.
		MaxPositionAgeMins: 240,
		Leverage:           preLiveLeverage(),
		FeeRatePct:         0.00050, // Binance futures taker 0.05% per leg
	})

	// RC-8 fix: Use per-strategy timeframe-based cooldowns so 15m signals get
	// the 5-minute cooldown that CooldownForTimeframe("15m") prescribes, rather
	// than the 30-second default. The constructor default is only the fallback
	// for strategies where SetStrategyCooldown is not called.
	agg := trading.NewSignalAggregator(300) // 5-minute default (matches 15m timeframe floor)
	for _, e := range qualified {
		cd := trading.CooldownForTimeframe(e.Timeframe)
		agg.SetStrategyCooldown(e.Strategy.Name(), cd)
	}

	// RC-7 fix: raise cap from 5 to 20 (≈10% of 42 validated strategies, 2026-07-02 rebuild).
	// 5 was too low: correlated strategies always claimed all 5 slots, destroying
	// diversification. 20 allows more signal variety while still preventing floods.
	maxPerCycle := 20
	if n := len(qualified); n < 20 {
		maxPerCycle = n
	}
	agg.SetMaxPerCycle(maxPerCycle)
	log.Printf("[PRE-LIVE] Aggregator: maxPerCycle=%d per-strategy cooldowns wired for %d strategies", maxPerCycle, len(qualified))

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

	// ── RC2: Auto-demotion ticker (DemotionCriteria.MaxWinRate now 0.40) ─────
	go func() {
		tick := time.NewTicker(15 * time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				backtest.ApplyAutoDemotion()
			case <-ctx.Done():
				return
			}
		}
	}()

	// ── Task 7: Regime-aware PnL tracker ────────────────────────────────────
	regimePnL := newRegimePnLTracker()

	// RC4 + Task 7: Wire OnClose callback — captures regime at close time.
	// Safe: o.mu is not held when CheckStopLossAndTakeProfit/CheckExpiredPositions
	// call emitClose, so orch.CurrentRegime() (which takes o.mu.RLock) is deadlock-free.
	posMgr.SetOnCloseCallback(func(pos *positions.Position, reason positions.CloseReason, exitPrice, pnl float64) {
		regime := orch.CurrentRegime()
		feeDrag := 0.0
		if 0.00050 > 0 {
			feeDrag = (pos.EntryPrice + exitPrice) * pos.Size * 0.00050
		}
		regimePnL.record(regime, reason, pnl, feeDrag)
		if reason == positions.ReasonExpired {
			log.Printf("[EXPIRED-REGIME] strategy=%s pnl=%.4f regime=%s", pos.StrategyName, pnl, regime)
		}
	})

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
	// Forces all open positions closed at the last known price first, then deletes
	// all MongoDB documents for the pre-live account key so data doesn't reappear
	// on the next page refresh or engine restart.
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
		regimePnL = newRegimePnLTracker() // reset in-memory regime PnL tracker
		if err := exec.ResetAccount(); err != nil {
			http.Error(w, "reset failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Wipe MongoDB so trades/positions don't come back on next load.
		if wipeErr := mongoBundle.WipePreLiveData(r.Context()); wipeErr != nil {
			log.Printf("[PRE-LIVE RESET] mongo wipe warn: %v", wipeErr)
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

	handle("/api/regime-pnl", func(w http.ResponseWriter, r *http.Request) {
		current := orch.CurrentRegime()
		if current == "" {
			current = "UNKNOWN"
		}
		json.NewEncoder(w).Encode(regimePnL.snapshot(current)) //nolint:errcheck
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
		// RC-2 fix: raised default from $100 to $10,000.
		// At FIXED_TRADE_SIZE_BTC=0.001 (~$100 notional) the round-trip fee is
		// ~$0.10, which is 0.001% of $10,000 — a viable fee-to-balance ratio.
		// The old $100 default with 0.1 BTC trades caused every BUY to fail
		// with INSUFFICIENT FUNDS before the first position could open.
		return 10000
	}
	var f float64
	fmt.Sscanf(v, "%f", &f)
	if f <= 0 {
		return 10000
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

// ── Task 7: Regime-aware PnL tracker ─────────────────────────────────────────

type regimeBucket struct {
	Trades  int     `json:"trades"`
	Wins    int     `json:"wins"`
	Losses  int     `json:"losses"`
	WinRate float64 `json:"win_rate"`
	NetPnL  float64 `json:"net_pnl"`
}

type regimePnLSnapshot struct {
	CurrentRegime string                   `json:"current_regime"`
	Breakdown     map[string]*regimeBucket `json:"breakdown"`
	TotalTrades   int                      `json:"total_trades"`
	TotalPnL      float64                  `json:"total_pnl"`
	ExpiredPnL    float64                  `json:"expired_pnl"`
	FeeDrag       float64                  `json:"fee_drag"`
}

type regimePnLTracker struct {
	mu         sync.Mutex
	buckets    map[string]*regimeBucket
	expiredPnL float64
	feeDrag    float64
}

func newRegimePnLTracker() *regimePnLTracker {
	return &regimePnLTracker{buckets: make(map[string]*regimeBucket)}
}

func (r *regimePnLTracker) record(reg string, reason positions.CloseReason, pnl, feeDrag float64) {
	if reg == "" {
		reg = "UNKNOWN"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.buckets[reg]
	if !ok {
		b = &regimeBucket{}
		r.buckets[reg] = b
	}
	b.Trades++
	b.NetPnL += pnl
	if pnl > 0 {
		b.Wins++
	} else {
		b.Losses++
	}
	if b.Trades > 0 {
		b.WinRate = float64(b.Wins) / float64(b.Trades)
	}
	if reason == positions.ReasonExpired {
		r.expiredPnL += pnl
	}
	r.feeDrag += feeDrag
}

func (r *regimePnLTracker) snapshot(currentRegime string) regimePnLSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	total := 0
	totalPnL := 0.0
	bd := make(map[string]*regimeBucket, len(r.buckets))
	for k, v := range r.buckets {
		cp := *v
		bd[k] = &cp
		total += v.Trades
		totalPnL += v.NetPnL
	}
	return regimePnLSnapshot{
		CurrentRegime: currentRegime,
		Breakdown:     bd,
		TotalTrades:   total,
		TotalPnL:      totalPnL,
		ExpiredPnL:    r.expiredPnL,
		FeeDrag:       r.feeDrag,
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
