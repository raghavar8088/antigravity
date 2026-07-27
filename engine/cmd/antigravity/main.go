package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"antigravity-engine/internal/admin"
	"antigravity-engine/internal/ai"
	"antigravity-engine/internal/aiscoring"
	"antigravity-engine/internal/alpha/funding"
	"antigravity-engine/internal/calibration"
	tconfig "antigravity-engine/internal/config"
	"antigravity-engine/internal/dataquality"
	"antigravity-engine/internal/delta"
	"antigravity-engine/internal/derivatives"
	"antigravity-engine/internal/dominance"
	"antigravity-engine/internal/etf"
	"antigravity-engine/internal/eventstore"
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/executiongateway"
	"antigravity-engine/internal/gateway"
	killswitchpkg "antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/learning"
	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/macro"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/ml"
	"antigravity-engine/internal/mongopersist"
	"antigravity-engine/internal/observability"
	_ "antigravity-engine/internal/observability" // registers Prometheus metrics at import time
	"antigravity-engine/internal/options"
	"antigravity-engine/internal/options_selling"
	"antigravity-engine/internal/orderbook"
	"antigravity-engine/internal/paperpersist"
	btpkg "antigravity-engine/internal/backtest"
	"antigravity-engine/internal/persistence"
	pmspkg "antigravity-engine/internal/pms"
	"antigravity-engine/internal/positions"
	reconciliationv2 "antigravity-engine/internal/reconciliationv2"
	"antigravity-engine/internal/regime"
	"antigravity-engine/internal/risk"
	"antigravity-engine/internal/secrets"
	"antigravity-engine/internal/security"
	"antigravity-engine/internal/security/vault"
	"antigravity-engine/internal/sentiment"
	"antigravity-engine/internal/shadow"
	"antigravity-engine/internal/strategy"
	"antigravity-engine/internal/strategy/scalpers"
	"antigravity-engine/internal/temporal"
	"antigravity-engine/internal/tracing"
	"antigravity-engine/internal/trading"
	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/production"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
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

// getInitialPaperBalanceUSD returns the configured starting balance for the
// live BTC futures paper account. Env: INITIAL_PAPER_BALANCE_USD. Falls back
// to $1000 (default) if unset, invalid, or non-positive. Floors at $100 — a
// paper account below that is not meaningfully tradable given fee/slippage
// assumptions baked into strategy sizing.
func getInitialPaperBalanceUSD() float64 {
	v := os.Getenv("INITIAL_PAPER_BALANCE_USD")
	if v == "" {
		return 1000.0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		log.Printf("[CONFIG] WARNING: invalid INITIAL_PAPER_BALANCE_USD=%q, using default $1000", v)
		return 1000.0
	}
	if f < 100 {
		log.Printf("[CONFIG] WARNING: INITIAL_PAPER_BALANCE_USD=%.2f below $100 floor, clamping to $100", f)
		return 100.0
	}
	return f
}

// getMaxPositionBTC returns the aggregate net BTC exposure ceiling for the
// futures paper account (RiskEngine.Validate rejects any order that would push
// |net exposure| above this). Sourced from MAX_POSITION_BTC (documented in
// CLAUDE.md, previously unwired) with a default of 5.0 BTC. The default was
// raised from the legacy 2.0 on 2026-07-07: with only two OOS-validated live
// strategies (down from ~100) FIXED_TRADE_SIZE_BTC was raised to 2.5 to use the
// risk budget, and a 5.0 cap lets both live shorts hold one position each
// (~$300k peak on a $1M book, the "moderate" sizing profile) while rejecting a
// third — keeping aggregate drawdown a small fraction of the 20% backtest limit.
func getMaxPositionBTC() float64 {
	v := os.Getenv("MAX_POSITION_BTC")
	if v == "" {
		return 5.0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		log.Printf("[CONFIG] WARNING: invalid MAX_POSITION_BTC=%q, using default 5.0 BTC", v)
		return 5.0
	}
	return f
}

// resetPaperBalanceOnBoot reports whether the operator asked the engine to
// discard the persisted paper balance/positions/trades on boot and start fresh
// from getInitialPaperBalanceUSD(). Env: RESET_PAPER_BALANCE_ON_BOOT (truthy:
// "1", "true", "yes", "on"). This is required to actually CHANGE the starting
// balance of an existing desk — otherwise boot restores the previously saved
// balance (e.g. a legacy $1,000,000) and the new INITIAL_PAPER_BALANCE_USD is
// ignored. Set it for ONE boot, then unset it so accumulated paper PnL persists
// again on subsequent restarts.
func resetPaperBalanceOnBoot() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("RESET_PAPER_BALANCE_ON_BOOT"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// configSource reports whether an env var is explicitly set ("env") or
// falling back to its built-in default ("default"). Used for startup logging
// so a misconfigured/missing env var is visible immediately, not discovered
// later from a balance that doesn't match expectations.
func configSource(envKey string) string {
	if os.Getenv(envKey) == "" {
		return "default"
	}
	return "env"
}

var (
	deltaProbeClient *marketdata.DeltaTickerClient

	// optionsEngineBTCSpot is updated by OptionsPriceFeed for GET /api/options/btc-feed (UI).
	optionsEngineBTCSpot struct {
		mu           sync.RWMutex
		Source       string // delta | binance | synthetic | unknown
		LastPrice    float64
		LastUpdated  time.Time
		TickerSymbol string
	}
)

func publishOptionsEngineBTCSpot(source string, price float64, ticker string) {
	optionsEngineBTCSpot.mu.Lock()
	defer optionsEngineBTCSpot.mu.Unlock()
	optionsEngineBTCSpot.Source = source
	optionsEngineBTCSpot.LastPrice = price
	optionsEngineBTCSpot.LastUpdated = time.Now().UTC()
	optionsEngineBTCSpot.TickerSymbol = ticker
}

// loadDotEnv reads a .env file from the repo root and sets any keys that are
// not already present in the environment. Safe to call on Render (where real
// env vars take precedence) and does nothing if the file is absent.
//
// Resolves the repo root from THIS SOURCE FILE's location (via runtime.Caller)
// rather than a cwd-relative "../.." path. The old cwd-relative path only
// resolved correctly if the process happened to be launched with cwd set to
// engine/cmd/antigravity specifically (three directories below repo root);
// running `go run ./cmd/antigravity` from engine/ (a natural, expected
// invocation) left it pointed one level too high and .env was silently never
// found, so every required env var appeared "missing" despite a correctly
// populated .env file at the repo root.
func loadDotEnv() {
	_, thisFile, _, ok := runtime.Caller(0)
	var envPath string
	if ok {
		// thisFile = .../engine/cmd/antigravity/main.go — repo root is 3 dirs up.
		envPath = filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".env")
	} else {
		envPath = "../../../.env" // fallback: only correct if cwd is repo root
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		return // no .env file — normal in production
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
			os.Setenv(key, val)
		}

	}
	log.Println("[ENV] Loaded local .env file")
}

// requireEnv returns the value of an environment variable or terminates the
// process with a descriptive fatal error if the variable is unset or empty.
// Call this after loadDotEnv() so .env files are already applied.
func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("FATAL: required environment variable %s is not set. Check your .env file.", key)
	}
	return val
}

// validatePortfolioViability warns at startup if the configured paper balance is
// too small to produce positions above the execution size floor. This is a soft
// warning — trading is not halted — but it surfaces the mismatch immediately.
func validatePortfolioViability() {
	balance := getInitialPaperBalanceUSD()
	minSizeBTC := 0.0001
	if v := os.Getenv("MIN_EXECUTION_SIZE_BTC"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			minSizeBTC = f
		}
	}
	// Approximate BTC price; if env not set we use a conservative $50,000.
	btcPriceUSD := 50000.0
	kellyProbationary := 0.05 // 5% of portfolio — worst case (all strategies probationary)
	smallestPositionUSD := balance * kellyProbationary
	smallestPositionBTC := smallestPositionUSD / btcPriceUSD
	if smallestPositionBTC < minSizeBTC {
		log.Printf("[STARTUP] WARNING: portfolio viability check FAIL — balance=%.2f USD, "+
			"smallest Kelly position=%.6f BTC (5%% × %.2f / %.0f), "+
			"minExecutionSizeBTC=%.6f — ALL live signals will be rejected by size floor. "+
			"Raise INITIAL_PAPER_BALANCE_USD or lower MIN_EXECUTION_SIZE_BTC.",
			balance, smallestPositionBTC, balance, btcPriceUSD, minSizeBTC)
	} else {
		log.Printf("[STARTUP] Portfolio viability check PASS — balance=%.2f USD, "+
			"smallest Kelly position=%.6f BTC >= floor=%.6f BTC",
			balance, smallestPositionBTC, minSizeBTC)
	}
}

// validateRequiredEnv checks all required environment variables in a single pass
// so operators see every missing variable at once instead of restarting for each one.
func validateRequiredEnv() {
	required := []string{
		"DATABASE_URL",
		"MONGODB_URI",
		"BINANCE_API_KEY",
		"BINANCE_API_SECRET",
		"ENGINE_ADMIN_SECRET",
		"AUTH_JWT_SECRET",
	}
	var missing []string
	for _, key := range required {
		if os.Getenv(key) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		log.Fatalf("FATAL: missing required environment variables: %s — check your .env file", strings.Join(missing, ", "))
	}
}

func saveOptionsSnapshot(ctx context.Context, store persistence.OptionsBuyPaperPersistence, snapshot options.PersistedState) error {
	priceHistJSON, err := json.Marshal(snapshot.PriceHist)
	if err != nil {
		return fmt.Errorf("marshal options price history: %w", err)
	}
	minuteBarsJSON, err := json.Marshal(snapshot.MinuteBars)
	if err != nil {
		return fmt.Errorf("marshal options minute bars: %w", err)
	}
	tradesJSON, err := json.Marshal(snapshot.Trades)
	if err != nil {
		return fmt.Errorf("marshal options trades: %w", err)
	}
	strategiesJSON, err := json.Marshal(snapshot.Strategies)
	if err != nil {
		return fmt.Errorf("marshal options strategies: %w", err)
	}

	return store.SaveOptionsState(ctx, &persistence.OptionsState{
		Balance:    snapshot.Balance,
		LastPrice:  snapshot.LastPrice,
		LastMinute: snapshot.LastMinute,
		TradeSeq:   snapshot.TradeSeq,
		PriceHist:  priceHistJSON,
		MinuteBars: minuteBarsJSON,
		Trades:     tradesJSON,
		Strategies: strategiesJSON,
		SavedAt:    snapshot.SavedAt,
	})
}

func loadOptionsSnapshot(state *persistence.OptionsState) (options.PersistedState, error) {
	snapshot := options.PersistedState{
		Balance:    state.Balance,
		LastPrice:  state.LastPrice,
		LastMinute: state.LastMinute,
		TradeSeq:   state.TradeSeq,
		SavedAt:    state.SavedAt,
	}

	if len(state.PriceHist) > 0 && string(state.PriceHist) != "[]" {
		if err := json.Unmarshal(state.PriceHist, &snapshot.PriceHist); err != nil {
			return options.PersistedState{}, fmt.Errorf("unmarshal options price history: %w", err)
		}
	}
	if len(state.MinuteBars) > 0 && string(state.MinuteBars) != "[]" {
		if err := json.Unmarshal(state.MinuteBars, &snapshot.MinuteBars); err != nil {
			return options.PersistedState{}, fmt.Errorf("unmarshal options minute bars: %w", err)
		}
	}
	if len(state.Trades) > 0 && string(state.Trades) != "[]" {
		if err := json.Unmarshal(state.Trades, &snapshot.Trades); err != nil {
			return options.PersistedState{}, fmt.Errorf("unmarshal options trades: %w", err)
		}
	}
	if len(state.Strategies) > 0 && string(state.Strategies) != "[]" {
		if err := json.Unmarshal(state.Strategies, &snapshot.Strategies); err != nil {
			return options.PersistedState{}, fmt.Errorf("unmarshal options strategies: %w", err)
		}
	}

	return snapshot, nil
}

func saveOptionsSellingSnapshot(ctx context.Context, store persistence.OptionsSellPaperPersistence, snapshot options_selling.PersistedState) error {
	priceHistJSON, err := json.Marshal(snapshot.PriceHist)
	if err != nil {
		return fmt.Errorf("marshal options selling price history: %w", err)
	}
	minuteBarsJSON, err := json.Marshal(snapshot.MinuteBars)
	if err != nil {
		return fmt.Errorf("marshal options selling minute bars: %w", err)
	}
	tradesJSON, err := json.Marshal(snapshot.Trades)
	if err != nil {
		return fmt.Errorf("marshal options selling trades: %w", err)
	}
	strategiesJSON, err := json.Marshal(snapshot.Strategies)
	if err != nil {
		return fmt.Errorf("marshal options selling strategies: %w", err)
	}

	return store.SaveOptionsSellingState(ctx, &persistence.OptionsSellingState{
		Balance:         snapshot.Balance,
		DayStartBalance: snapshot.DayStartBalance,
		DayStartDate:    snapshot.DayStartDate,
		LastPrice:       snapshot.LastPrice,
		LastMinute:      snapshot.LastMinute,
		TradeSeq:        snapshot.TradeSeq,
		PriceHist:       priceHistJSON,
		MinuteBars:      minuteBarsJSON,
		Trades:          tradesJSON,
		Strategies:      strategiesJSON,
		SavedAt:         snapshot.SavedAt,
	})
}

func loadOptionsSellingSnapshot(state *persistence.OptionsSellingState) (options_selling.PersistedState, error) {
	snapshot := options_selling.PersistedState{
		Balance:         state.Balance,
		DayStartBalance: state.DayStartBalance,
		DayStartDate:    state.DayStartDate,
		LastPrice:       state.LastPrice,
		LastMinute:      state.LastMinute,
		TradeSeq:        state.TradeSeq,
		SavedAt:         state.SavedAt,
	}

	if len(state.PriceHist) > 0 && string(state.PriceHist) != "[]" {
		if err := json.Unmarshal(state.PriceHist, &snapshot.PriceHist); err != nil {
			return options_selling.PersistedState{}, fmt.Errorf("unmarshal options selling price history: %w", err)
		}
	}
	if len(state.MinuteBars) > 0 && string(state.MinuteBars) != "[]" {
		if err := json.Unmarshal(state.MinuteBars, &snapshot.MinuteBars); err != nil {
			return options_selling.PersistedState{}, fmt.Errorf("unmarshal options selling minute bars: %w", err)
		}
	}
	if len(state.Trades) > 0 && string(state.Trades) != "[]" {
		if err := json.Unmarshal(state.Trades, &snapshot.Trades); err != nil {
			return options_selling.PersistedState{}, fmt.Errorf("unmarshal options selling trades: %w", err)
		}
	}
	if len(state.Strategies) > 0 && string(state.Strategies) != "[]" {
		if err := json.Unmarshal(state.Strategies, &snapshot.Strategies); err != nil {
			return options_selling.PersistedState{}, fmt.Errorf("unmarshal options selling strategies: %w", err)
		}
	}

	return snapshot, nil
}

// runHealthcheck is invoked via `antigravity --healthcheck` as the Docker
// HEALTHCHECK command. The runtime image is built FROM scratch (no shell,
// no wget/curl — see engine/Dockerfile), so an external-binary healthcheck
// can never work; this does the same /health GET in-process instead and
// maps the result to a process exit code Docker understands.
func runHealthcheck() {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck: request failed:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: HTTP", resp.StatusCode)
		os.Exit(1)
	}
	os.Exit(0)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		runHealthcheck()
		return
	}

	log.SetOutput(globalLogs)
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║   RAIG ENGINE v6.0 — IMMORTAL EDITION                  ║")
	fmt.Println("║   30 Curated Strategies | Full State Restore | Panic Recovery  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")

	loadDotEnv()

	// Validate required environment variables before any DB or exchange connections.
	// A missing variable here is the #1 cause of exit status 0xffffffff on Windows
	// because the process panics deep in a driver before logging anything useful.
	validateRequiredEnv()
	validatePortfolioViability()

	// Log key env config at startup so misconfiguration is immediately visible.
	log.Printf("[CONFIG] SQLITE_ENABLED=%s OTEL_ENABLED=%s ML_SCORER_ENDPOINT=%s MAX_POSITION_BTC=%s MAX_DAILY_LOSS_PCT=%s",
		os.Getenv("SQLITE_ENABLED"),
		os.Getenv("OTEL_ENABLED"),
		os.Getenv("ML_SCORER_ENDPOINT"),
		os.Getenv("MAX_POSITION_BTC"),
		os.Getenv("MAX_DAILY_LOSS_PCT"),
	)

	// Initialise distributed tracing — must be first after env load.
	tracingCfg := tracing.ConfigFromEnv()
	tracingShutdown, tracingErr := tracing.InitTracer(tracingCfg)
	if tracingErr != nil {
		log.Printf("[TRACING] ⚠️  OpenTelemetry initialisation failed — continuing without traces: %v", tracingErr)
	} else {
		defer tracingShutdown()
		log.Printf("[TRACING] ✅ OpenTelemetry tracer initialised jaeger_endpoint=%s enabled=%v",
			tracingCfg.JaegerEndpoint, tracingCfg.Enabled)
	}

	// ── Wiring 1: AWS Secrets Manager (Phase 5) ──────────────────────────────
	// When USE_LOCAL_SECRETS=true (dev) or AWS is unavailable, falls back to env vars.
	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion == "" {
		awsRegion = "ap-south-1"
	}
	useLocalSecrets := os.Getenv("USE_LOCAL_SECRETS") == "true"
	secretClient, secretErr := secrets.NewSecretClient(awsRegion, useLocalSecrets)
	if secretErr != nil {
		log.Printf("[SECRETS] ⚠️  AWS Secrets Manager unavailable — using env fallback: %v", secretErr)
	} else {
		log.Printf("[SECRETS] ✅ Secret client ready (region=%s local_fallback=%v)", awsRegion, useLocalSecrets)
	}
	_ = secretClient // available for callers that switch from os.Getenv to secretClient.Get()

	bootGate := production.RunBootGate(production.DefaultBootGateConfig())
	if !bootGate.Passed {
		for _, b := range bootGate.Blockers {
			log.Fatalf("[BOOT GATE] %s", b)
		}
	}
	for _, w := range bootGate.Warnings {
		log.Printf("[BOOT GATE] WARNING: %s", w)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bootStart := time.Now()
	var reconciliationComplete atomic.Bool // set to true after ReconcileOnRestart completes

	// ═══════════════════════════════════════════════════
	// 1. WebSocket Live Stream (Coinbase)
	// ═══════════════════════════════════════════════════
	coinbaseClient := marketdata.NewCoinbaseClient()
	deltaProbeClient = marketdata.NewDeltaTickerClient()
	go func() {
		err := coinbaseClient.Connect(ctx, []string{"BTC-USD"})
		if err != nil {
			log.Fatalf("Fatal error connecting to Coinbase: %v", err)
		}
	}()

	// ═══════════════════════════════════════════════════
	// 2. Build curated strategies (full BTC Equity roster; no silent truncation)
	// ═══════════════════════════════════════════════════
	const btcEquityStrategyCapacity = 600
	allStrategies := strategy.BuildCuratedScalpers()
	if len(allStrategies) == 0 {
		log.Fatalf("[INIT] FATAL: zero strategies loaded — check strategy registry (engine/internal/strategy/scalpers/)")
	}
	strategyNames := make([]string, len(allStrategies))
	for i, e := range allStrategies {
		strategyNames[i] = e.Strategy.Name()
	}
	log.Printf("[STRATEGY] Loaded %d strategies: %v", len(allStrategies), strategyNames)
	// Emit initial PROBATIONARY (0) status for all strategies at startup.
	for _, e := range allStrategies {
		observability.ScalersStrategyStatus.WithLabelValues(e.Strategy.Name()).Set(0)
	}
	// Record current STRATEGY_ROLLOUT_PHASE for observability.
	observability.SetStrategyRolloutPhase()
	if len(allStrategies) > btcEquityStrategyCapacity {
		log.Printf("[INIT] Loaded %d curated live strategies (capacity %d exceeded; no truncation applied)", len(allStrategies), btcEquityStrategyCapacity)
	} else {
		log.Printf("[INIT] Loaded %d curated live strategies (capacity %d; no truncation applied)", len(allStrategies), btcEquityStrategyCapacity)
	}

	// Extract names, categories, timeframes for tracker
	names := make([]string, len(allStrategies))
	categories := make([]string, len(allStrategies))
	timeframes := make([]string, len(allStrategies))
	for i, e := range allStrategies {
		names[i] = e.Strategy.Name()
		categories[i] = strategy.NormalizeCategory(e.Category, e.Strategy.Name())
		timeframes[i] = e.Timeframe
	}

	// ═══════════════════════════════════════════════════
	// 2b. Funding Alpha — live collection loop
	// Fetches BTC perpetual funding rates from Binance every 8 hours and
	// injects snapshots directly into every InstitutionalAlphaScalper so
	// the FundingMeanReversion and Confluence modules have live data.
	// ═══════════════════════════════════════════════════
	go safeGo("FundingCollector", func() {
		collector := funding.NewCollector()
		collect := func() {
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			snap, err := collector.Fetch(cctx, "binance", "BTCUSDT")
			if err != nil {
				log.Printf("[FUNDING] collection error: %v", err)
				return
			}
			snap2, _ := collector.Fetch(cctx, "bybit", "BTCUSDT")
			for _, entry := range allStrategies {
				if inj, ok := entry.Strategy.(interface {
					InjectFunding(funding.FundingSnapshot)
				}); ok {
					inj.InjectFunding(snap)
					if snap2.Exchange != "" {
						inj.InjectFunding(snap2)
					}
				}
			}
			log.Printf("[FUNDING] collected rate=%.6f (Binance BTCUSDT)", snap.FundingRate)
		}
		collect() // immediate on startup
		ticker := time.NewTicker(8 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				collect()
			}
		}
	})

	// ═══════════════════════════════════════════════════
	// 3. Risk Engine (configured for the futures paper account; default $100)
	// ═══════════════════════════════════════════════════
	riskProfile := risk.RiskProfile{
		MaxPositionBTC:  getMaxPositionBTC(),         // aggregate net BTC exposure cap (env MAX_POSITION_BTC, default 5.0)
		MaxCapitalUSD:   getInitialPaperBalanceUSD(), // configured paper balance
		MaxDailyLossPct: 0.05,                        // 5% daily loss circuit breaker
	}
	riskEngine := risk.NewRiskEngine(riskProfile)
	// BUG 2: schedule daily P&L reset at midnight UTC so daily-loss circuit breaker
	// resets automatically without a process restart.
	go riskEngine.ScheduleDailyReset(ctx)
	log.Println("[RISK] Daily loss reset scheduled (fires at 00:00 UTC)")

	// ═══════════════════════════════════════════════════
	// 4. Strategy Tracker (Per-Strategy Performance)
	// ═══════════════════════════════════════════════════
	tracker := risk.NewStrategyTracker(names, categories, timeframes, getInitialPaperBalanceUSD())

	// ═══════════════════════════════════════════════════
	// 5. Paper Executor (futures paper account; default $100)
	// ═══════════════════════════════════════════════════
	log.Printf("[CONFIG] Initial paper balance: $%.2f (source: %s)",
		getInitialPaperBalanceUSD(), configSource("INITIAL_PAPER_BALANCE_USD"))
	paperExecute := execution.NewPaperClient(getInitialPaperBalanceUSD())

	// ═══════════════════════════════════════════════════
	// 5b. Paper OMS — canonical execution (Epic 1)
	// ═══════════════════════════════════════════════════
	paperOMS := execution.NewPaperOMS(getInitialPaperBalanceUSD())

	// ═══════════════════════════════════════════════════
	// 6. Position Manager (Trailing SL/TP)
	// ═══════════════════════════════════════════════════
	posMgr := positions.NewManager()

	// ═══════════════════════════════════════════════════
	// 7. Signal Aggregator (per-timeframe cooldown per strategy)
	// ═══════════════════════════════════════════════════
	aggregator := trading.NewSignalAggregator(15) // 15s default for tick/unknown strategies
	for _, e := range allStrategies {
		aggregator.SetStrategyCooldown(e.Strategy.Name(), trading.CooldownForTimeframe(e.Timeframe))
	}

	// ═══════════════════════════════════════════════════
	// 8. Trade Journal
	// ═══════════════════════════════════════════════════
	// 8. Trade Journal (Expanded to 5,000 for full session history)
	journal := execution.NewTradeJournal(5000)

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
		// ── UNLIMITED MODE HOOK ──
		// Register a real-time save hook so every trade is persisted to the relational table immediately.
		journal.OnTrade = func(entry execution.JournalEntry) {
			// Convert to map for store interface
			tradeMap := map[string]interface{}{
				"id":           entry.ID,
				"strategyName": entry.StrategyName,
				"category":     entry.Category,
				"side":         entry.Side,
				"entryPrice":   entry.EntryPrice,
				"exitPrice":    entry.ExitPrice,
				"size":         entry.Size,
				"grossPnl":     entry.GrossPnL,
				"fees":         entry.Fees,
				"netPnl":       entry.NetPnL,
				"reason":       entry.Reason,
				"entryTime":    entry.EntryTime,
				"exitTime":     entry.ExitTime,
				"duration":     entry.Duration,
				"aiDecisionId": entry.AIDecisionID,
				"aiProvider":   entry.AIProvider,
				"aiReasoning":  entry.AIReasoning,
				"aiConfidence": entry.AIConfidence,
				"aiBullThesis": entry.AIBullThesis,
				"aiBearThesis": entry.AIBearThesis,
			}
			if err := dbStore.SaveTrade(ctx, tradeMap); err != nil {
				log.Printf("[DB] ⚠️  Failed to save trade %s to relational table: %v", entry.ID, err)
			}
		}

		// ── Restore ALL state on boot ──
		state, loadErr := dbStore.LoadState(ctx)
		if resetPaperBalanceOnBoot() {
			// Operator forced a fresh start: discard persisted balance/positions/
			// trades and keep the in-memory paperExecute at getInitialPaperBalanceUSD().
			if loadErr == nil {
				if rErr := dbStore.ResetState(ctx); rErr != nil {
					log.Printf("[DB] ⚠️  RESET_PAPER_BALANCE_ON_BOOT: failed to reset persisted state: %v", rErr)
				}
			}
			log.Printf("[DB] 🧹 RESET_PAPER_BALANCE_ON_BOOT set — paper desk starting fresh at $%.2f (persisted balance/positions/trades discarded)",
				getInitialPaperBalanceUSD())
		} else if loadErr == nil && state.Balance != getInitialPaperBalanceUSD() {
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

			// ── MIGRATION ON BOOT ──
			// If we range through existing restored trades and save them one-by-one,
			// the ON CONFLICT clause in SaveTrade ensures we migrate old BLOB data to the new table safely.
			if len(restoredTrades) > 0 {
				log.Printf("[DB] 🚚 Migrating %d trades to relational table...", len(restoredTrades))
				for _, t := range restoredTrades {
					journal.OnTrade(t)
				}
			}
		} else {
			log.Println("[DB] Fresh start — no previous state to restore")
		}
	}

	// ═══════════════════════════════════════════════════
	// 9c. PHASE 31B — MongoDB Atlas paper-trading persistence
	//
	// MongoManager is the single shared handle for all paperpersist writers.
	// If MongoDB is unavailable the engine degrades gracefully (PostgreSQL only).
	// ═══════════════════════════════════════════════════

	// Environment validation — logs a clear report before attempting connection.
	// If MONGODB_URI is missing or malformed the engine continues in SQLite-only
	// mode rather than fatally exiting, but the report makes the issue obvious.
	envReport := paperpersist.LogEnvReport()
	paperpersist.LogAccountKeyAlignment(true)
	if !envReport.MongoReady {
		log.Printf("[Phase31B] ⛔  MongoDB env not ready — skipping MongoDB persistence. Paper Desk will show empty data until MONGODB_URI is configured and engine is restarted.")
	}

	var ppBundle *trading.PaperPersistBundle
	var recoveryReport paperpersist.RecoveryReport
	// Bounded retry: a transient Atlas hiccup at boot (e.g. ReplicaSetNoPrimary
	// during a primary election, observed 2026-07-07) must not condemn the whole
	// run to in-memory-only persistence — the engine never reconnects mid-run.
	mongoMgr, mongoErr := paperpersist.NewMongoManager(ctx)
	for attempt := 2; mongoErr != nil && attempt <= 4; attempt++ {
		log.Printf("[Phase31B] MongoDB connect failed (attempt %d/4 in 15s): %v", attempt-1, mongoErr)
		select {
		case <-ctx.Done():
			attempt = 5
		case <-time.After(15 * time.Second):
			mongoMgr, mongoErr = paperpersist.NewMongoManager(ctx)
		}
	}
	if mongoErr != nil {
		log.Printf("[Phase31B] ❌  MongoDB connect failed — running without MongoDB persistence: %v", mongoErr)
		log.Printf("[Phase31B]     Account key : %s", envReport.AccountKey)
		log.Printf("[Phase31B]     Database    : %s", envReport.DatabaseName)
		log.Printf("[Phase31B]     Check Atlas IP whitelist and credentials.")
	} else {
		if idxErr := mongoMgr.EnsureIndexes(ctx); idxErr != nil {
			log.Printf("[Phase31B] index creation warning: %v", idxErr)
		}
		// Wiring 2: ensure TTL + compound indexes created by Phase 1 EnsureIndexes.
		if db := mongoMgr.DB(); db != nil {
			indexCtx, indexCancel := context.WithTimeout(ctx, 30*time.Second)
			if indexErr := mongopersist.EnsureIndexes(indexCtx, db); indexErr != nil {
				log.Printf("[INDEXES] TTL index creation warning: %v", indexErr)
			}
			indexCancel()
		}
		// Startup diagnostics: logs connectivity, account_key, URI, and collection presence.
		mongoMgr.StartupReport(ctx)

		// Crash recovery: restore balance + open positions from MongoDB Atlas.
		// This is authoritative when PostgreSQL is stale or unavailable.
		recoveryReport = paperpersist.Recover(ctx, mongoMgr)
		if recoveryReport.AccountRestored {
			restoredBalance := recoveryReport.AccountState.Balance
			if restoredBalance < 0 {
				log.Printf("[Phase31B] CRITICAL: MongoDB recovery: negative balance %.4f clamped to 0 — bookkeeping inconsistency detected", restoredBalance)
				observability.NegativeBalanceRecoveries.Inc()
				restoredBalance = 0
			}
			if !resetPaperBalanceOnBoot() && restoredBalance != getInitialPaperBalanceUSD() {
				log.Printf("[Phase31B] ♻️  MongoDB recovery: balance=%.2f age=%s — overriding PostgreSQL state",
					restoredBalance, recoveryReport.AccountDataAge.Round(time.Second))
				paperExecute.RestoreBalance(restoredBalance, recoveryReport.AccountState.TotalFees)
			}
		}
		if resetPaperBalanceOnBoot() {
			log.Printf("[Phase31B] 🧹 RESET_PAPER_BALANCE_ON_BOOT set — skipping MongoDB balance/journal/position recovery; starting fresh at $%.2f",
				getInitialPaperBalanceUSD())
			// Purge persisted engine state so discarded open positions cannot be
			// resurrected as zombies on the next normal (non-reset) boot.
			if purgeErr := paperpersist.PurgeEngineState(ctx, mongoMgr); purgeErr != nil {
				log.Printf("[Phase31B] ⚠️  reset purge failed (stale positions may reappear next boot): %v", purgeErr)
			}
		}

		// Bootstrap in-memory journal from MongoDB paper_trades so strategy health
		// has trade history immediately after restart (not only after new closes).
		if !resetPaperBalanceOnBoot() {
			if booted, bootErr := paperpersist.BootstrapJournalFromMongo(ctx, mongoMgr, journal); bootErr != nil {
				log.Printf("[Phase31B] journal bootstrap warning: %v", bootErr)
			} else if booted > 0 {
				log.Printf("[Phase31B] bootstrapped %d trades from paper_trades into journal", booted)
			}
		}
		// Restore open positions from MongoDB if PostgreSQL didn't restore any.
		if !resetPaperBalanceOnBoot() && len(recoveryReport.OpenPositions) > 0 && posMgr.GetPositionCount() == 0 {
			log.Printf("[Phase31B] restoring %d open positions from MongoDB", len(recoveryReport.OpenPositions))
			var mongoPositions []positions.Position
			for _, rp := range recoveryReport.OpenPositions {
				side := strategy.ActionBuy
				if rp.Side == "SHORT" || rp.Side == "SELL" {
					side = strategy.ActionSell
				}
				mongoPositions = append(mongoPositions, positions.Position{
					ID:           rp.PositionID,
					Symbol:       rp.Symbol,
					Side:         side,
					EntryPrice:   rp.EntryPrice,
					Size:         rp.Size,
					StopLoss:     rp.StopLoss,
					TakeProfit:   rp.TakeProfit,
					StrategyName: rp.StrategyID,
					OpenedAt:     rp.OpenedAt,
					Status:       "OPEN",
				})
				// Sync PaperClient's signed BTC position so mark-to-market equity
				// and SettlePosition are accurate for positions open at restart.
				paperExecute.RestoreOpenPosition(side, rp.Size)
			}
			posMgr.RestorePositions(mongoPositions)
		}

		log.Printf("[Phase31B] recovery complete — success=%v positions=%d inconsistencies=%d",
			recoveryReport.Success, recoveryReport.PositionsRestored, len(recoveryReport.Inconsistencies))

		// Wire writers.
		tradeWriter := paperpersist.NewTradeWriter(mongoMgr)
		orderWriter := paperpersist.NewOrderWriter(mongoMgr)
		ppBundle = trading.NewPaperPersistBundle(mongoMgr, tradeWriter, orderWriter)

		// Ping monitor: reconnects MongoDB on transient failures.
		go mongoMgr.RunPingMonitor(ctx, 30*time.Second)

		log.Printf("[Phase31B] MongoDB persistence active — account_key=%s", paperpersist.AccountKey())
	}

	// ── Wiring 4: Reconcile open trades that may have hit SL/TP during downtime ─
	// This is BLOCKING and fatal on error — runs before any trading starts.
	if mongoMgr != nil && mongoMgr.IsConnected() && mongoMgr.DB() != nil {
		reconCtx, reconCancel := context.WithTimeout(ctx, 30*time.Second)
		reconPrice, reconPriceErr := fetchBinanceBTCSpot(reconCtx)
		if reconPriceErr != nil {
			log.Printf("[RECON RESTART] ⚠️  BTC price fetch failed (using 0): %v", reconPriceErr)
			reconPrice = 0
		}
		reconCancel()

		const maxReconAttempts = 3
		var reconReport *reconciliationv2.ReconciliationReport
		var reconErr error
		for attempt := 1; attempt <= maxReconAttempts; attempt++ {
			attemptCtx, attemptCancel := context.WithTimeout(ctx, 30*time.Second)
			reconReport, reconErr = reconciliationv2.ReconcileOnRestart(attemptCtx, mongoMgr.DB(), reconPrice)
			attemptCancel()
			if reconErr == nil {
				break
			}
			log.Printf("[RECON RESTART] attempt %d/%d failed: %v", attempt, maxReconAttempts, reconErr)
			if attempt < maxReconAttempts {
				time.Sleep(5 * time.Second)
			}
		}
		if reconErr != nil {
			log.Printf("[RECON RESTART] ⚠️  all %d attempts failed — continuing without full reconciliation: %v", maxReconAttempts, reconErr)
		} else {
			log.Printf("[RECON RESTART] ✅ complete — found=%d reconciled=%d closed_retroactively=%d discrepancies=%d price=%.0f",
				reconReport.TradesFound, reconReport.TradesReconciled,
				reconReport.TradesClosedRetroactively, len(reconReport.DiscrepanciesFound), reconPrice)
		}
		reconciliationComplete.Store(true)
	} else {
		// No MongoDB — reconciliation not needed; mark complete so /ready doesn't block.
		reconciliationComplete.Store(true)
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

	// ── Institutional Kill Switch (P1-B) ─────────────────────────────────────
	// Wire the killswitch.Service into the orchestrator so PreTradeRiskPipeline
	// can gate every new order submission without requiring a process shutdown.
	// Three modes supported:
	//   Mode A: block new orders (IsActive=true) — engine keeps running
	//   Mode B: flatten positions (FlattenPositions action) — no process kill
	//   Mode C: context cancellation via admin.KillSwitchController (nuclear stop)
	ksExecutor := trading.NewKillSwitchExecutor(paperExecute, posMgr)

	// ── Durable kill switch ledger ────────────────────────────────────────────
	// Prefer PostgresStore so kill-switch state (trigger, reason, activatedAt)
	// survives engine restarts. Falls back to MongoLedgerStore (reusing the
	// MongoDB connection already established above for trade persistence —
	// no new infrastructure) when Postgres is unavailable, and only falls
	// back further to the non-durable in-memory store as a last resort.
	//
	// Jun 2026 incident: DATABASE_URL was set but Postgres was unreachable
	// (never actually provisioned), so this silently ran on MemoryStore in
	// production — an unbounded in-process store that leaked memory until
	// the engine got OOM-killed roughly every 24-36 hours, which also wiped
	// kill-switch state on every restart since it lived only in that memory.
	var ksLedger ledger.Store = ledger.NewMemoryStore()
	var durableLedger ledger.Store
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		if pgStore, pgErr := ledger.NewPostgresStore(ctx, dbURL); pgErr != nil {
			log.Printf("[LEDGER] ⚠️  PostgresStore unavailable (%v) — trying MongoDB fallback", pgErr)
		} else if schemaErr := pgStore.CreateSchema(ctx); schemaErr != nil {
			log.Printf("[LEDGER] ⚠️  Postgres schema init failed (%v) — trying MongoDB fallback", schemaErr)
		} else {
			durableLedger = pgStore
			ksLedger = pgStore
			log.Println("[LEDGER] ✅ Durable PostgresStore wired — kill switch and PMS state will survive restarts")
		}
	} else {
		log.Println("[LEDGER] DATABASE_URL not set — trying MongoDB fallback")
	}
	if durableLedger == nil && mongoMgr != nil && mongoMgr.IsConnected() {
		// Pass mongoMgr.DB (the method value, NOT mongoMgr.DB()) so the ledger
		// resolves the LIVE database on every op and survives reconnects that
		// swap the client. See NewMongoLedgerStore's doc comment.
		if mongoStore, mongoErr := ledger.NewMongoLedgerStore(ctx, mongoMgr.DB); mongoErr != nil {
			log.Printf("[LEDGER] ⚠️  MongoLedgerStore unavailable (%v) — kill switch state is non-durable", mongoErr)
		} else {
			durableLedger = mongoStore
			ksLedger = mongoStore
			log.Println("[LEDGER] ✅ Durable MongoLedgerStore wired — kill switch and PMS state will survive restarts")
		}
	}
	if durableLedger == nil {
		log.Println("[LEDGER] ⚠️  No durable store available (Postgres and MongoDB both unavailable) — kill switch ledger is in-memory only")
	}

	ksSvc := killswitchpkg.NewService(ksLedger, ksExecutor, "btc-paper-1")
	// RISK 2: wire reconciler into kill switch so OMS_DESYNC auto-release is
	// blocked when reconciliation finds live mismatches.
	if mongoMgr != nil && mongoMgr.IsConnected() {
		ksSvc.SetReconciler(func(rctx context.Context) (int, error) {
			price := paperExecute.GetLastPrice()
			report, err := reconciliationv2.ReconcileOnRestart(rctx, mongoMgr.DB(), price)
			if err != nil {
				return 0, err
			}
			return len(report.DiscrepanciesFound), nil
		})
		log.Println("[KILL SWITCH] Reconciler wired — OMS_DESYNC auto-release validates trade reconciliation")
	}
	wasHalted := ksSvc.RestoreStateOnStartup(ctx)
	if !ksSvc.IsEnabled() {
		log.Println("[KILL SWITCH] DISABLED — trading will not halt. Set KILL_SWITCH_ENABLED=true on engine to re-arm.")
		if err := ksSvc.DisableAndRelease(ctx); err != nil {
			log.Printf("[KILL SWITCH] disable release error: %v", err)
		}
		wasHalted = false
	}
	if wasHalted {
		log.Printf("[KILL SWITCH] ⚠️  engine starting in HALTED state — kill switch was active from prior session")
		log.Printf("[KILL SWITCH] ⚠️  action: POST /api/admin/ks/release with body {confirm:RESUME} to resume trading")
	}
	orchestrator.SetKillSwitch(ksSvc)
	orchestrator.SetEventLedger(ksLedger)
	execWatchdog := trading.NewExecutionWatchdog(coinbaseClient, ksSvc)
	orchestrator.SetExecutionWatchdog(execWatchdog)
	go safeGo("ExecutionWatchdog", func() { execWatchdog.Run(ctx) })
	ksExecutor.SetOrchestrator(orchestrator)
	log.Println("[KILL SWITCH] Institutional kill switch wired — PreTradeRiskPipeline gated")
	log.Println("[LEDGER] Event ledger shared with orchestrator — OMS events durable for reconciliation")

	// ── Portfolio Management System (P3-A) ───────────────────────────────────
	// Activate PMS as the portfolio-level pre-trade gate. It runs before the
	// per-strategy institutional pipeline and enforces heat, VaR, drawdown, and
	// daily-loss limits at the aggregate portfolio level.
	var pmsLedger ledger.Store = ledger.NewMemoryStore()
	if durableLedger != nil {
		pmsLedger = durableLedger
	}
	pmsBudget := pmspkg.NewPortfolioRiskBudget(pmsLedger)
	pmsBudget.InitPortfolio("btc-paper-1", pmspkg.RiskBudget{
		MaxHeatPct:      8,
		MaxVaR95Pct:     6,
		MaxCVaR95Pct:    9,
		MaxDrawdownPct:  10,
		MaxDailyLossPct: 3,
		MaxGrossExpPct:  250,
		MaxNetExpPct:    150,
	})
	orchestrator.SetPMSBudget(pmsBudget)
	log.Println("[PMS] Portfolio risk budget gate active — btc-paper-1 initialized")

	// ── Shadow Trading (strategy incubation) ─────────────────────────────────
	// Lets every curated strategy fire real, fully-evaluated signals — those
	// gated below the current STRATEGY_ROLLOUT_PHASE are routed to a shadow
	// ledger instead of the live paper OMS, so they accumulate a real
	// performance track record without risking paper account balance. See
	// engine/internal/shadow and rollout_phase.go.
	// Pass mongoMgr.DB (the method value, resolved live per call) so shadow
	// persistence survives reconnects; a snapshot handle would silently stop
	// persisting after an Atlas primary election. getDB stays nil when there is
	// no manager, keeping in-memory-only mode.
	var shadowGetDB func() *mongo.Database
	if mongoMgr != nil {
		shadowGetDB = mongoMgr.DB
	}
	shadowLedger := shadow.NewShadowLedger(shadowGetDB)
	shadowPersisted := shadowGetDB != nil && shadowGetDB() != nil
	if shadowPersisted {
		if idxErr := shadowLedger.EnsureIndexes(ctx); idxErr != nil {
			log.Printf("[SHADOW] index creation warning: %v", idxErr)
		}
		recovered, recErr := shadowLedger.RecoverOpenTrades(ctx)
		if recErr != nil {
			log.Printf("[SHADOW] recovery warning: %v", recErr)
		}
		log.Printf("[SHADOW] Recovered %d open shadow positions", recovered)
	}
	shadowPromoter := shadow.NewShadowPromoter(shadowLedger, orchestrator.WalkForwardValidator())
	orchestrator.SetShadowLedger(shadowLedger, shadowPromoter)
	log.Printf("[SHADOW] Shadow ledger initialized (mongo_persisted=%v)", shadowPersisted)

	// Bootstrap portfolio ledger from MongoDB paper_trades (authoritative accounting).
	if mongoMgr != nil && mongoMgr.IsConnected() {
		if err := paperpersist.BootstrapPortfolioLedgerFromMongo(
			ctx, mongoMgr, orchestrator.PortfolioLedger(), paperExecute.GetBalanceUSD(),
		); err != nil {
			log.Printf("[Phase31B] portfolio ledger bootstrap warning: %v", err)
		} else {
			log.Printf("[Phase31B] portfolio ledger bootstrapped from paper_trades")
		}
	}

	// Phase 31B: register recovered position→order mappings so processCloseEvents
	// can emit correct OMS transitions when recovered positions hit SL/TP.
	if len(recoveryReport.OpenPositions) > 0 {
		orchestrator.RegisterRecoveredPositions(recoveryReport.OpenPositions)
	}

	// Phase 31B: attach MongoDB persistence bundle + start state snapshotter.
	if ppBundle != nil {
		orchestrator.SetPaperPersist(ppBundle)
		// Atlas M0 write-pressure note (2026-07-10): 10s snapshots + 1m equity
		// contributed to free-tier throttling. 60s snapshots keep RPO<5min
		// (Phase 14 recovery spec) at 1/6th the write volume.
		snapshotter := paperpersist.NewStateSnapshotter(ppBundle.Mgr(), orchestrator, 60*time.Second)
		go snapshotter.Run(ctx)
		log.Printf("[Phase31B] StateSnapshotter started (60s interval)")

		// Phase 31D: EquityRecorder — 5-minute equity curve snapshots + daily PnL seal.
		equityRecorder := paperpersist.NewEquityRecorder(ppBundle.Mgr(), orchestrator, orchestrator, 5*time.Minute)
		go safeGo("EquityRecorder", func() { equityRecorder.Run(ctx) })
		log.Printf("[Phase31D] EquityRecorder started (5m interval)")

		// Portfolio metrics writer — persists authoritative snapshot every 30 minutes.
		go safeGo("PortfolioMetricsWriter", func() {
			ticker := time.NewTicker(30 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					snap := orchestrator.GetAccountSnapshot()
					_ = paperpersist.WritePortfolioMetrics(ctx, ppBundle.Mgr(), paperpersist.PortfolioMetricsDoc{
						RealizedPnL:      snap.RealizedPnL,
						UnrealizedPnL:    snap.UnrealizedPnL,
						Equity:           snap.Equity,
						Balance:          snap.Balance,
						PeakEquity:       snap.PeakEquity,
						CurrentDrawdown:  snap.CurrentDrawdown,
						MaxDrawdown:      snap.MaxDrawdown,
						TotalTrades:      snap.TotalTrades,
						WinRate:          snap.WinRate,
						TotalFees:        snap.TotalFees,
						LongExposureBTC:  snap.LongExposureBTC,
						ShortExposureBTC: snap.ShortExposureBTC,
						GrossExposureBTC: snap.TotalExposureBTC,
						OpenPositions:    snap.OpenPositionCount,
					})
				}
			}
		})

		// Phase 31D: StrategyHealthMonitor — compute + persist health every 15 min.
		healthMonitor := paperpersist.NewStrategyHealthMonitor(ppBundle.Mgr(), orchestrator, 15*time.Minute)
		go safeGo("StrategyHealthMonitor", func() { healthMonitor.Run(ctx) })
		log.Printf("[Phase31D] StrategyHealthMonitor started (15m interval)")
	}

	// ═══════════════════════════════════════════════════
	// 10b. AI MULTI-AGENT SYSTEM — Claude-powered trading
	// ═══════════════════════════════════════════════════
	openAIClient := ai.NewOpenAIClient()
	geminiClient := ai.NewGeminiClient()
	groqClient := ai.NewGroqClient()
	openRouterClient := ai.NewOpenRouterClient()
	mistralClient := ai.NewMistralClient()
	huggingFaceClient := ai.NewHuggingFaceClient()
	cloudflareClient := ai.NewCloudflareClient()
	var aiOrchestrator *ai.MultiAgentOrchestrator

	if openAIClient.IsAvailable() || groqClient.IsAvailable() || openRouterClient.IsAvailable() ||
		geminiClient.IsAvailable() || mistralClient.IsAvailable() || huggingFaceClient.IsAvailable() ||
		cloudflareClient.IsAvailable() {
		aiOrchestrator = ai.NewMultiAgentOrchestrator(openAIClient, geminiClient, groqClient, openRouterClient, mistralClient, huggingFaceClient, cloudflareClient, dbStore)
		orchestrator.SetAIOrchestrator(aiOrchestrator)

		// Restore AI History from DB
		if dbStore != nil {
			hist, _ := dbStore.LoadAuditLogs(ctx, 50)
			for _, h := range hist {
				aiOrchestrator.AddHistoricalAudit(h)
			}
		}

		aiSystem := "AI Supreme Court [Technicals + Macro]"
		if !openAIClient.IsAvailable() && (groqClient.IsAvailable() || openRouterClient.IsAvailable()) {
			aiSystem = "AI Supreme Court — 100% FREE RESILIENCE MODE (Groq/OpenRouter)"
		}
		log.Printf("[AI] ✅ %s initialized (History restored)", aiSystem)
	} else {
		log.Println("[AI] ⚠️  AI Keys not set — running rules-only mode")
	}

	// ═══════════════════════════════════════════════════
	// 11. WARMUP — Pre-fill strategy buffers from Coinbase REST (with retry)
	// ═══════════════════════════════════════════════════
	log.Println("[WARMUP] Fetching historical candles to pre-fill strategy buffers...")
	var warmupData *marketdata.WarmupData
	for attempt := 1; attempt <= 3; attempt++ {
		data, fetchErr := marketdata.FetchWarmupCandles("BTC-USD")
		if fetchErr == nil && data != nil && len(data.Candles1m) >= 30 {
			warmupData = data
			break
		}
		log.Printf("[WARMUP] ⚠️  Attempt %d/3 failed (got %d candles): %v", attempt, func() int {
			if data != nil {
				return len(data.Candles1m)
			}
			return 0
		}(), fetchErr)
		if attempt < 3 {
			time.Sleep(2 * time.Second)
		}
	}
	if warmupData != nil {
		orchestrator.WarmupStrategies(warmupData)
	} else {
		log.Println("[WARMUP] ⚠️  All warmup attempts failed — will warm up from live data")
	}

	log.Printf("[BOOT] Engine fully initialized in %s", time.Since(bootStart).Round(time.Millisecond))

	// ── Wirings 3 + 5 + 6 + 7: Phase 1–5 LoopDeps construction ─────────────────
	// Wiring 3: data quality validator
	dataValidator := dataquality.NewValidator()

	// Wiring 5: regime classifier, strategy gate, cycle guard, async scorer
	regimeClassifier := regime.NewClassifier()
	strategyGate := regime.NewStrategyGate(regimeClassifier, &strategyNamesAdapter{strategies: allStrategies})
	cycleGuard := trading.NewCycleGuard()
	fallbackScorer := aiscoring.NewFallbackScorer()
	asyncScorer := aiscoring.NewAsyncScorer(nil, 3) // no AI client; uses fallback scorer internally
	asyncScorer.Start()

	// Wiring 6: funding rate + open interest fetchers
	fundingFetcher := derivatives.NewFundingFetcher()
	oiFetcher := derivatives.NewOIFetcher()
	go fundingFetcher.StartPolling(ctx, 15*time.Minute)
	go oiFetcher.StartPolling(ctx, 15*time.Minute)
	log.Println("[DEPS] Funding rate + OI fetchers polling every 15m")

	// Wiring 6b: Deribit BTC DVOL volatility index (used by S10-S13 vol family)
	dvolHolder := marketdata.NewDeribitDVOLHolder()
	dvolHolder.StartPolling(ctx)
	log.Println("[DEPS] Deribit BTC DVOL feed polling every 5m")

	// Wiring 6c: Binance BTC perpetual liquidation feed (used by S14)
	liquidationHolder := marketdata.NewBinanceLiquidationHolder()
	liquidationHolder.StartStreaming(ctx)
	log.Println("[DEPS] Binance BTC liquidation feed streaming (!forceOrder@arr)")

	// Wiring 6d: Binance BTC perpetual mark price (used by S16 basis calc)
	perpPriceHolder := marketdata.NewBinancePerpPriceHolder()
	perpPriceHolder.StartPolling(ctx)
	log.Println("[DEPS] Binance BTC perp mark price feed polling every 30s")

	// Wiring 6e: Macro cross-asset feed — Nasdaq futures proxy + DXY (used by S18-S21 macro family)
	macroFeedHolder := marketdata.NewMacroFeedHolder()
	macroFeedHolder.StartPolling(ctx)
	log.Println("[DEPS] Macro cross-asset feed (Nasdaq proxy + DXY) polling every 10m")

	// Wiring 7: L2 order book depth subscriber
	depthSubscriber := orderbook.NewDepthSubscriber()
	go safeGo("DepthSubscriber", func() {
		if err := depthSubscriber.Connect(ctx); err != nil {
			log.Printf("[DEPTH] subscriber exited: %v", err)
		}
	})
	log.Println("[DEPS] L2 depth subscriber connecting to Binance BTCUSDT@depth20")

	// Wiring 8: BTC ETF flow fetcher (daily, via Python yfinance script)
	pythonPath := os.Getenv("PYTHON_PATH")
	if pythonPath == "" {
		pythonPath = "python3"
	}
	etfScriptPath := os.Getenv("ETF_SCRIPT_PATH")
	if etfScriptPath == "" {
		etfScriptPath = "infrastructure/ai/etf_fetcher.py"
	}
	etfFetcher := etf.NewETFFetcher(pythonPath, etfScriptPath)
	etfFetcher.StartDailyPoll(ctx)
	log.Printf("[DEPS] ETF flow tracker started (09:30 UTC daily, script=%s)", etfScriptPath)

	// Wiring 9: BTC dominance tracker (hourly, CoinGecko public API)
	dominanceFetcher := dominance.NewDominanceFetcher(nil)
	dominanceFetcher.StartPolling(ctx, time.Hour)
	log.Println("[DEPS] BTC dominance tracker started (1h interval)")
	// Wire the same dominance fetcher into the scalpers package so S25
	// (Dominance_Relative_Strength) can read its latest BTC.D reading.
	scalpers.SetDominanceFetcher(dominanceFetcher)

	// Wiring 10: Macro correlation fetcher (hourly, via Python yfinance script)
	macroScriptPath := os.Getenv("MACRO_SCRIPT_PATH")
	if macroScriptPath == "" {
		macroScriptPath = "infrastructure/ai/macro_fetcher.py"
	}
	macroFetcher := macro.NewMacroFetcher(pythonPath, macroScriptPath)
	macroFetcher.StartHourlyPoll(ctx)
	log.Printf("[DEPS] Macro correlation tracker started (1h interval, script=%s)", macroScriptPath)

	// Wiring 11: News sentiment fetcher (30m interval, local sentiment server)
	sentimentServerURL := os.Getenv("SENTIMENT_SERVER_URL")
	if sentimentServerURL == "" {
		sentimentServerURL = "http://localhost:8001"
	}
	sentimentFetcher := sentiment.NewSentimentFetcher(nil, sentimentServerURL)
	sentimentFetcher.StartPolling(ctx, 30*time.Minute)
	log.Printf("[DEPS] Sentiment fetcher started (30m interval, server=%s)", sentimentServerURL)

	// Wiring 12: Temporal pattern analyser
	temporalAnalyser := temporal.NewTemporalAnalyser()
	temporalPatternsPath := os.Getenv("TEMPORAL_PATTERNS_PATH")
	if temporalPatternsPath == "" {
		temporalPatternsPath = "data/temporal_patterns.json"
	}
	if err := temporalAnalyser.LoadPatterns(temporalPatternsPath); err != nil {
		log.Printf("[DEPS] Temporal patterns not found — will build from trade history: %v", err)
		if mongoMgr != nil && mongoMgr.DB() != nil {
			go temporal.BuildAndSavePatterns(ctx, mongoMgr.DB(), temporalPatternsPath)
		}
	} else {
		log.Printf("[DEPS] Temporal patterns loaded from %s", temporalPatternsPath)
	}

	loopDeps := &trading.LoopDeps{
		DataValidator:     dataValidator,
		AsyncScorer:       asyncScorer,
		FallbackScorer:    fallbackScorer,
		RegimeClassifier:  regimeClassifier,
		StrategyGate:      strategyGate,
		CycleGuard:        cycleGuard,
		FundingFetcher:    fundingFetcher,
		OIFetcher:         oiFetcher,
		DepthSubscriber:   depthSubscriber,
		DVOLHolder:        dvolHolder,
		LiquidationHolder: liquidationHolder,
		PerpPriceHolder:   perpPriceHolder,
		MacroFeedHolder:   macroFeedHolder,
		PortfolioValue:    getInitialPaperBalanceUSD(),
		// Kelly ledger — PortfolioLedger implements kelly.LedgerInterface via
		// its ClosedTrades() method, which returns the per-trade PnL% ring buffer.
		// Kelly sizing requires at least 30 closed trades before activating.
		Ledger: orchestrator.PortfolioLedger(),
		// Phase C signals (optional — nil = score 0)
		ETFFetcher:       etfFetcher,
		DominanceFetcher: dominanceFetcher,
		MacroFetcher:     macroFetcher,
		SentimentFetcher: sentimentFetcher,
		TemporalAnalyser: temporalAnalyser,
	}
	if err := loopDeps.Validate(); err != nil {
		log.Printf("[DEPS] ⚠️  LoopDeps validation warning: %v", err)
	}
	orchestrator.SetDeps(loopDeps)
	log.Println("[DEPS] ✅ Phase 1–5 + Phase C LoopDeps wired into orchestrator")

	// Wiring 13: Confidence calibration (Phase D.2)
	// Loads the most recent calibration from MongoDB and schedules monthly recalibration.
	if mongoMgr != nil && mongoMgr.DB() != nil {
		if calResult, err := calibration.LoadLatest(ctx, mongoMgr.DB()); err == nil {
			if calResult != nil {
				loopDeps.UpdateCalibration(calResult)
				observability.CalibrationFactor.Set(calResult.CalibrationFactor)
				observability.CalibrationTradesAnalysed.Set(float64(calResult.TradesAnalysed))
				log.Printf("[CALIBRATION] Loaded from MongoDB: factor=%.3f overconfident=%v trades=%d",
					calResult.CalibrationFactor, calResult.IsOverconfident, calResult.TradesAnalysed)
			} else {
				log.Println("[CALIBRATION] No prior calibration result — will compute after 60 trades accumulate")
			}
		} else {
			log.Printf("[CALIBRATION] Load error (non-fatal): %v", err)
		}
		calibration.ScheduleRecalibration(ctx, mongoMgr.DB(), 30*24*time.Hour, func(r *calibration.CalibrationResult) {
			loopDeps.UpdateCalibration(r)
			observability.CalibrationFactor.Set(r.CalibrationFactor)
			observability.CalibrationTradesAnalysed.Set(float64(r.TradesAnalysed))
			log.Printf("[CALIBRATION] Recalibrated: factor=%.3f overconfident=%v trades=%d",
				r.CalibrationFactor, r.IsOverconfident, r.TradesAnalysed)
		})
		log.Println("[CALIBRATION] Monthly recalibration scheduler started")
	}

	// Wiring 14: Post-trade self-learning loop (Phase D.3)
	if mongoMgr != nil && mongoMgr.DB() != nil {
		lessonGen := learning.NewLessonGenerator(mongoMgr.DB())
		loopDeps.LessonGenerator = lessonGen
		lessonGen.StartPeriodicLearning(ctx, 24*time.Hour)
		log.Println("[LEARNING] Post-trade lesson generator started (24h interval)")
	}

	// Wiring 15: Event store dual-write (Phase E.2) — optional, non-blocking
	{
		pgPool, pgErr := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
		if pgErr != nil || os.Getenv("DATABASE_URL") == "" {
			log.Println("[EVENTSTORE] ⚠️  DATABASE_URL unavailable — events not persisted to TimescaleDB")
		} else {
			slogLogger := slog.Default()
			evWriter := eventstore.NewEventWriter(pgPool, slogLogger)
			evWriter.Start(ctx)
			loopDeps.EventStore = evWriter
			log.Println("[EVENTSTORE] ✅ Event store writer initialised")
		}
	}

	// Wiring 16: Local ML pre-scorer (Phase E.6) — fully optional
	{
		mlEndpoint := getEnvOrDefault("ML_SCORER_ENDPOINT", "http://localhost:8002")
		mlPrescorer := ml.NewMLPrescorer(mlEndpoint, 0.55, slog.Default())
		mlPrescorer.StartHealthPoller(ctx)
		loopDeps.MLPrescorer = mlPrescorer
		log.Printf("[ML] ✅ ML pre-scorer health poller started (endpoint=%s threshold=0.55)", mlEndpoint)
	}

	// Start the orchestrator with panic recovery
	go safeGo("Orchestrator", func() { orchestrator.Run(ctx) })

	// Upgrade 6: Binance kline WebSocket feed for live 15m/1h candles.
	// Falls back to 5m synthesis automatically on disconnect.
	klineClient := marketdata.NewBinanceKlineClient([]string{"15m", "1h"})
	go safeGo("BinanceKlines", func() {
		if err := klineClient.Start(
			ctx,
			func(c marketdata.Candle) {
				orchestrator.Push15mKlineCandle(c)
				orchestrator.SetKlineFeedActive("15m", true)
			},
			func(c marketdata.Candle) {
				orchestrator.Push1hKlineCandle(c)
				orchestrator.SetKlineFeedActive("1h", true)
			},
		); err != nil {
			log.Printf("[KLINES] BinanceKlineClient stopped: %v", err)
		}
		orchestrator.SetKlineFeedActive("15m", false)
		orchestrator.SetKlineFeedActive("1h", false)
	})

	// Upgrade 2: Binance aggTrade WebSocket feed for real CVD (taker-side
	// classification) instead of the price-direction proxy. Falls back to
	// the proxy automatically on disconnect via SetAggTradeFeedActive(false).
	aggTradeClient := marketdata.NewBinanceAggTradeClient("btcusdt", func(t marketdata.AggTrade) {
		orchestrator.PushAggTrade(t)
	})
	go safeGo("BinanceAggTrade", func() {
		if err := aggTradeClient.Start(ctx); err != nil {
			log.Printf("[AGGTRADE] feed error: %v", err)
		}
		orchestrator.SetAggTradeFeedActive(false)
	})

	// ── Reconciliation Authority v2 ───────────────────────────────────────────
	// Compares ledger OMS projections against:
	//   1) live position manager runtime (always)
	//   2) Delta Exchange REST snapshots (when DELTA_API_KEY/SECRET are set)
	// CRITICAL drift triggers institutional kill switch (OMS_DESYNC).
	if _, err := reconciliationv2.WireProduction(
		ctx,
		ksLedger,
		posMgr,
		paperExecute.GetEquityUSD,
		ksSvc,
		"btc-paper-1",
		&reconciliationv2.WireProductionConfig{
			InitialBalanceUSD: getInitialPaperBalanceUSD(),
			MarkPriceUSD:      paperExecute.GetLastPrice,
		},
	); err != nil {
		log.Printf("[RECON-V2] ⚠️  reconciliation bootstrap failed: %v", err)
	}

	// ═══════════════════════════════════════════════════
	// 11c. BTC OPTIONS SCALPER — 50 strategies, separate paper account (default $100)
	// ═══════════════════════════════════════════════════
	optionsEngine := options.NewEngine()
	optionsSellingEngine := options_selling.NewEngine()

	// Delta Exchange live bridge — mirrors BTC option signals to Delta when enabled.
	// StartMonitor polls live positions every 5 min and auto-closes at profit/stop targets.
	deltaBridge := delta.NewBridge()
	orchestrator.WireDeltaBridge(deltaBridge)
	// Custody: reload positions this app opened before the restart, then adopt any
	// untracked real option position on the exchange. A position the app opened
	// stays the app's responsibility until SL/TP/expiry closes it — it must never
	// be orphaned by a restart or a disarm.
	deltaBridge.RestoreTrades()
	go func() {
		adoptCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if n, err := deltaBridge.AdoptUntrackedPositions(adoptCtx); err != nil {
			log.Printf("[DELTA BRIDGE] custody: adoption sweep failed: %v", err)
		} else if n > 0 {
			log.Printf("[DELTA BRIDGE] custody: adopted %d untracked live position(s) into SL/TP management", n)
		}
	}()
	deltaBridge.StartMonitor(ctx)

	// Live Engine control plane (real-money option BUYING module). Ships DISARMED
	// with a $100 server-enforced ceiling; arming is a human action only.
	wireLiveEngine(ctx, deltaBridge, ksSvc, optionsEngine)
	// LIVE ENGINE is buying-only: real orders are mirrored from the BUYING engine
	// (long calls/puts, bounded risk), NOT the selling engine (naked shorts). The
	// bridge is forced to buying+native mode and gated by a per-strategy allow-list
	// in wireLiveEngine. The selling desk stays paper-only — it no longer feeds
	// live orders.
	optionsEngine.SetOnOpenHook(func(posID string, stratID int, stratName string, optType string, strike float64, expiry time.Time, premiumUSD float64, btcSpot float64) {
		deltaBridge.OnOpen(delta.OpenSignal{
			PaperTradeID: posID,
			StrategyID:   stratID,
			StrategyName: stratName,
			OptionType:   optType,
			Strike:       strike,
			ExpiryTime:   expiry,
			PremiumUSD:   premiumUSD,
			BTCPrice:     btcSpot,
		})
	})
	optionsEngine.SetOnCloseHook(func(posID string, stratID int, optType string, strike float64, exitReason string) {
		// Mark the source: this exit is the STRATEGY's decision, measured on the
		// synthetic paper chain, not the real position's own risk levels. The two
		// can disagree — a paper "SL" has closed a real leg that was in profit —
		// so labelling it plainly avoids reading a strategy exit as a real stop.
		deltaBridge.OnClose(delta.CloseSignal{
			PaperTradeID: posID,
			StrategyID:   stratID,
			OptionType:   optType,
			Strike:       strike,
			ExitReason:   "strategy_" + exitReason,
		})
	})
	var btcBuy persistence.OptionsBuyPaperPersistence
	var btcSell persistence.OptionsSellPaperPersistence
	if dbStore != nil {
		btcBuy, btcSell = dbStore, dbStore
	} else if fs, ferr := persistence.NewFileSnapshotStore(); ferr == nil {
		btcBuy, btcSell = fs, fs
		log.Printf("[SNAPSHOT] ✅ BTC options paper state → files under %s (set ENGINE_DATA_DIR to a mounted disk so redeploys keep history)", fs.Dir)
	} else {
		log.Printf("[SNAPSHOT] ⚠️  BTC options not persisted: no DATABASE_URL and file store failed: %v", ferr)
	}

	if btcBuy != nil {
		optionsEngine.SetStateSaveHook(func(snapshot options.PersistedState) {
			if err := saveOptionsSnapshot(context.Background(), btcBuy, snapshot); err != nil {
				log.Printf("[OPTIONS PERSIST] ⚠️  Failed to save options (buy) state: %v", err)
			}
		})

		optionsSellingEngine.SetStateSaveHook(func(snapshot options_selling.PersistedState) {
			if err := saveOptionsSellingSnapshot(context.Background(), btcSell, snapshot); err != nil {
				log.Printf("[OPTIONS PERSIST] ⚠️  Failed to save options selling state: %v", err)
			}
		})

		optState, loadErr := btcBuy.LoadOptionsState(ctx)
		if loadErr != nil {
			log.Printf("[OPTIONS PERSIST] ⚠️  Failed to load options (buy) state: %v", loadErr)
		} else {
			snapshot, snapshotErr := loadOptionsSnapshot(optState)
			if snapshotErr != nil {
				log.Printf("[OPTIONS PERSIST] ⚠️  Failed to decode options state: %v", snapshotErr)
			} else {
				optionsEngine.RestoreState(snapshot)
				restoredOpen := 0
				for _, strategyState := range snapshot.Strategies {
					if strategyState.Position != nil {
						restoredOpen++
					}
				}
				savedAt := "new"
				if !optState.SavedAt.IsZero() {
					savedAt = optState.SavedAt.Format(time.RFC3339)
				}
				log.Printf(
					"[OPTIONS PERSIST] ♻️  Options (buy) restored | savedAt=%s | Balance: $%.2f | Open: %d | Trades: %d",
					savedAt, snapshot.Balance, restoredOpen, len(snapshot.Trades),
				)
			}
		}

		sellOptState, loadErr := btcSell.LoadOptionsSellingState(ctx)
		if loadErr != nil {
			log.Printf("[OPTIONS PERSIST] ⚠️  Failed to load options selling state: %v", loadErr)
		} else {
			snapshot, snapshotErr := loadOptionsSellingSnapshot(sellOptState)
			if snapshotErr != nil {
				log.Printf("[OPTIONS PERSIST] ⚠️  Failed to decode options selling state: %v", snapshotErr)
			} else {
				optionsSellingEngine.RestoreState(snapshot)
				restoredOpen := 0
				for _, strategyState := range snapshot.Strategies {
					if strategyState.Position != nil {
						restoredOpen++
					}
				}
				savedAt := "new"
				if !sellOptState.SavedAt.IsZero() {
					savedAt = sellOptState.SavedAt.Format(time.RFC3339)
				}
				log.Printf(
					"[OPTIONS PERSIST] ♻️  Options SELLING restored | savedAt=%s | Balance: $%.2f | Open: %d | Trades: %d",
					savedAt, snapshot.Balance, restoredOpen, len(snapshot.Trades),
				)
			}
		}
	}

	// Pre-fill BTC options engines with historical 1m bars — eliminates the 55-min wait.
	if warmupData != nil && len(warmupData.Candles1m) > 0 {
		closes := make([]float64, len(warmupData.Candles1m))
		for i, c := range warmupData.Candles1m {
			closes[i] = c.Close
		}
		optionsEngine.InjectMinuteBars(closes)
		optionsSellingEngine.InjectMinuteBars(closes)
	}

	// Feed live BTC spot into the BTC options engines: Delta Exchange public ticker first,
	// then Binance REST if Delta is unavailable (no API key required for ticker).
	go safeGo("OptionsPriceFeed", func() {
		lastBTCPrice := 0.0
		lastRESTFetch := time.Time{}
		lastGoodSource := "unknown"
		symDefault := strings.TrimSpace(os.Getenv("DELTA_OPTIONS_BTC_TICKER"))
		if symDefault == "" {
			symDefault = "BTCUSD"
		}
		var syntheticSpotLogged sync.Once
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				if lastRESTFetch.IsZero() || time.Since(lastRESTFetch) >= 10*time.Second {
					lastRESTFetch = time.Now()
					if p, err := fetchDeltaBTCSpotForOptions(ctx); err == nil && p > 5000 && p < 1000000 {
						lastBTCPrice = p
						lastGoodSource = "delta"
					} else {
						if err != nil {
							log.Printf("[OPTIONS FEED] Delta spot fetch failed: %v", err)
						}
						if p, err2 := fetchBinanceBTCSpot(ctx); err2 == nil && p > 5000 && p < 1000000 {
							lastBTCPrice = p
							lastGoodSource = "binance"
							log.Printf("[OPTIONS FEED] using Binance fallback BTC %.0f", p)
						} else if err2 != nil {
							log.Printf("[OPTIONS FEED] Binance spot fetch failed: %v", err2)
						}
					}
				}
				p := lastBTCPrice
				src := lastGoodSource
				if p <= 0 {
					p = options.PaperBTCFallbackSpot()
					src = "synthetic"
					syntheticSpotLogged.Do(func() {
						log.Printf("[OPTIONS FEED] using synthetic BTC spot %.0f until Delta/Binance feed is available", p)
					})
				}
				tickerDisp := symDefault
				if src == "binance" {
					tickerDisp = "BTCUSDT"
				} else if src == "synthetic" {
					tickerDisp = "PAPER"
				}
				publishOptionsEngineBTCSpot(src, p, tickerDisp)
				optionsEngine.UpdatePrice(p)
				optionsSellingEngine.UpdatePrice(p)
			}
		}
	})

	go safeGo("OptionsScalper", func() { optionsEngine.Run(ctx.Done()) })
	go safeGo("OptionsSellingScalper", func() { optionsSellingEngine.Run(ctx.Done()) })

	// ═══════════════════════════════════════════════════
	// 11b. STATE SAVER — Periodic DB snapshots
	// ═══════════════════════════════════════════════════
	if dbStore != nil {
		saver := persistence.NewStateSaver(dbStore, paperExecute, posMgr, journal)
		go safeGo("StateSaver", func() { saver.Run(ctx) })
	}

	// ── P3-C: Automated daily loss reset ─────────────────────────────────────
	// Resets risk engine daily counters and strategy tracker daily PnL at
	// 00:00:00 UTC every day. This eliminates the dependency on manual
	// /api/admin/clear-history calls and ensures circuit breakers reset reliably.
	// Timezone: UTC. BTC trades 24/7 so UTC midnight is the canonical reset point.
	go safeGo("DailyLossReset", func() {
		for {
			now := time.Now().UTC()
			// Next midnight UTC
			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
			sleepDur := time.Until(nextMidnight)
			log.Printf("[DAILY RESET] Next daily loss reset scheduled at %s (in %s)",
				nextMidnight.Format("2006-01-02T15:04:05Z"), sleepDur.Truncate(time.Second))
			select {
			case <-ctx.Done():
				return
			case <-time.After(sleepDur):
				riskEngine.ResetDaily()
				tracker.ResetDaily()
				log.Printf("[DAILY RESET] Daily loss counters reset at %s UTC", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
			}
		}
	})

	// ═══════════════════════════════════════════════════
	// 12. HTTP API Server
	// ═══════════════════════════════════════════════════
	killswitch := admin.NewKillSwitch(ctx, cancel, paperExecute, paperExecute, journal, posMgr, dbStore, riskEngine, tracker)
	killswitch.SetEmergencyFlatten(orchestrator.ExecuteEmergencyFlatten)

	// ── Phase 15J: Vault Secret Provider ─────────────────────────────────────
	// Loads from HashiCorp Vault when VAULT_ADDR+VAULT_TOKEN are set;
	// falls back to environment variables for local development.
	secretProvider := vault.LoadFromEnv()
	log.Printf("[VAULT] Secret provider: %s", secretProvider.Source())

	// Start secret rotation engine — zero-downtime rotation via cache invalidation.
	rotationEngine := vault.NewRotationEngine(secretProvider, vault.DefaultRotationPolicies())
	rotationEngine.Start(ctx)
	defer rotationEngine.Stop()
	log.Printf("[VAULT] Rotation engine started — %d policies registered", len(vault.DefaultRotationPolicies()))

	// ── Phase 15G: Zero Trust Security Gate ──────────────────────────────────
	secPolicy := security.LoadPolicy()
	secGate := security.NewGate(secPolicy, nil)
	log.Printf("[SECURITY] Zero Trust Gate active — enforce_auth=%v source=%s",
		secPolicy.EnforceAuth, secretProvider.Source())

	// ═══════════════════════════════════════════════════
	// Phase 30 — MongoDB Persistence Layer
	// ═══════════════════════════════════════════════════
	mongoPhase30 := mongopersist.StartAndRestore(ctx)
	if mongoPhase30 != nil {
		defer mongoPhase30.Close(context.Background())
		http.Handle("/phase30/", http.StripPrefix("/phase30", mongopersist.NewHandler(mongoPhase30)))
		log.Println("[PHASE30] ✅ MongoDB persistence layer active — /phase30/* endpoints registered")
	}

	// ── Phase 31D: diagnostics ────────────────────────────────────────────────────
	// GET /api/paper-desk/diagnostics — live MongoDB health, collection presence,
	// write metrics, account_key, and persistence lag. No auth required (read-only).
	http.HandleFunc("/api/paper-desk/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if mongoMgr == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"connected":   false,
				"error":       "MongoDB not initialised",
				"account_key": paperpersist.AccountKey(),
			})
			return
		}
		report := mongoMgr.Diagnostics(r.Context())
		json.NewEncoder(w).Encode(report)
	})

	// Prometheus metrics — expose institutional metrics registry alongside default.
	// observability.MetricsHandler() combines the default gatherer (Go runtime,
	// process stats) with the custom trading metrics registry registered at import time.
	http.Handle("/metrics", observability.MetricsHandler())

	// Options Scalper endpoints
	http.HandleFunc("/api/options/positions", optionsEngine.HandlePositions)
	http.HandleFunc("/api/options/trades", optionsEngine.HandleTrades)
	http.HandleFunc("/api/options/strategies", optionsEngine.HandleStrategies)
	http.HandleFunc("/api/options/stats", optionsEngine.HandleStats)
	http.HandleFunc("/api/options/reset", optionsEngine.HandleReset)
	http.HandleFunc("/api/options/clear-history", optionsEngine.HandleClearHistory)
	http.HandleFunc("/api/options/btc-feed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		optionsEngineBTCSpot.mu.RLock()
		src := optionsEngineBTCSpot.Source
		if src == "" {
			src = "unknown"
		}
		out := map[string]interface{}{
			"source":       src,
			"lastPrice":    optionsEngineBTCSpot.LastPrice,
			"tickerSymbol": optionsEngineBTCSpot.TickerSymbol,
		}
		if !optionsEngineBTCSpot.LastUpdated.IsZero() {
			out["lastUpdated"] = optionsEngineBTCSpot.LastUpdated.UTC().Format(time.RFC3339)
		}
		optionsEngineBTCSpot.mu.RUnlock()
		_ = json.NewEncoder(w).Encode(out)
	})

	// Options Selling Scalper endpoints
	http.HandleFunc("/api/options-selling/positions", optionsSellingEngine.HandlePositions)
	http.HandleFunc("/api/options-selling/trades", optionsSellingEngine.HandleTrades)
	http.HandleFunc("/api/options-selling/strategies", optionsSellingEngine.HandleStrategies)
	http.HandleFunc("/api/options-selling/stats", optionsSellingEngine.HandleStats)
	http.HandleFunc("/api/options-selling/reset", optionsSellingEngine.HandleReset)
	http.HandleFunc("/api/options-selling/clear-history", optionsSellingEngine.HandleClearHistory)

	// Paper OMS — canonical execution endpoints (Epic 1)
	http.Handle("/paper/", &execution.PaperOMSHandler{OMS: paperOMS, Symbol: "BTCUSDT"})

	// Delta Exchange Live Bridge endpoints
	http.HandleFunc("/api/delta-live/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		stats := deltaBridge.Stats(r.Context())
		_ = json.NewEncoder(w).Encode(stats)
	})
	http.HandleFunc("/api/delta-live/trades", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(deltaBridge.Trades())
	})
	http.HandleFunc("/api/delta-live/open", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(deltaBridge.OpenTrades())
	})
	http.HandleFunc("/api/delta-live/enable", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		deltaBridge.SetEnabled(body.Enabled)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "enabled": body.Enabled})
	})

	http.HandleFunc("/api/delta-live/mode", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			BuyingMode bool `json:"buyingMode"` // true = buy options, false = sell options
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"need {buyingMode: true|false}"}`, http.StatusBadRequest)
			return
		}
		deltaBridge.SetBuyingMode(body.BuyingMode)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "buyingMode": body.BuyingMode})
	})

	http.HandleFunc("/api/delta-live/order", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Symbol string `json:"symbol"`
			Side   string `json:"side"`
			Size   int    `json:"size"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Symbol == "" || body.Side == "" || body.Size < 1 {
			http.Error(w, `{"error":"need symbol, side (buy/sell), size (>=1)"}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		resp, err := orchestrator.ProcessExecutionRequest(ctx, executiongateway.Request{
			Venue: "delta", Symbol: body.Symbol, Side: body.Side,
			Contracts: body.Size, StrategyName: "MANUAL_DELTA",
		})
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if !resp.OK {
			w.WriteHeader(http.StatusUnprocessableEntity)
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	// Institutional execution gateway — sole human-facing execution API on the engine.
	execGatewayHandler := executiongateway.NewHandler(orchestrator)
	http.Handle("/api/execution/request", execGatewayHandler)

	// ── Backtest API (Tasks 5–9) ──────────────────────────────────────────────
	{
		btDataDir := os.Getenv("ENGINE_DATA_DIR")
		if btDataDir == "" {
			btDataDir = "./data"
		}
		btStore, btErr := btpkg.OpenResultsStore(btpkg.DBPath(btDataDir))
		if btErr != nil {
			log.Printf("[BACKTEST] WARNING: could not open results store: %v — backtest endpoints disabled", btErr)
		} else {
			var promoteFn func(string)
			if orchestrator != nil && orchestrator.ScalerBundle() != nil {
				sb := orchestrator.ScalerBundle()
				promoteFn = sb.RequestPromotion
			}
			btAPI := btpkg.NewBacktestAPI(btStore, btDataDir+"/historical", promoteFn)
			btAPI.Register()
			log.Printf("[BACKTEST] API registered (DB: %s)", btpkg.DBPath(btDataDir))
		}
	}

	// Health check — used by load balancers, Docker HEALTHCHECK, and uptime monitors
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":        true,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"uptime":    time.Since(bootStart).Round(time.Second).String(),
		})
	})

	// Regime endpoint — current market regime for BTC engines
	http.HandleFunc("/api/regime", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"btc":       optionsSellingEngine.RegimeInfo(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Probe endpoint — connectivity test for Delta Exchange BTC ticker
	http.HandleFunc("/api/probe/delta-btc", handleDeltaBTCProbe)

	// BTC Option Chain endpoint
	http.HandleFunc("/api/option-chain", optionsEngine.HandleOptionChain)

	// Admin endpoints
	// Trade Threshold Configuration module:
	//   GET  /api/engine/config          — full ThresholdConfig list (live registry + static metadata)
	//   POST /api/engine/config          — X-Engine-Admin-Secret required; validates [min,max], applies
	//                                       immediately via the registry, audit-logs to config_changes
	//   GET  /api/engine/config/history  — audit trail (optional ?key=, defaults to last 50, all keys)
	// Initialize the registry eagerly so the first request doesn't pay env-scan latency.
	_ = tconfig.Default()
	http.HandleFunc("/api/engine/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			tconfig.HandleGetConfig(w, r)
		case http.MethodPost:
			tconfig.HandleSetConfig(w, r)
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/api/engine/config/history", tconfig.HandleConfigHistory)
	// Master Strictness Dial — one 0-100 control that proportionally scales
	// every signal-quality-relevant threshold at once (see strictness.go).
	//   GET  /api/engine/config/strictness — live profiles + drift-aware dial position
	//   POST /api/engine/config/strictness — X-Engine-Admin-Secret required; applies
	//                                        the dial in one batch, one audit entry
	http.HandleFunc("/api/engine/config/strictness", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			tconfig.HandleGetStrictness(w, r)
		case http.MethodPost:
			tconfig.HandleSetStrictness(w, r)
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/admin/kill", killswitch.HandleTrigger)
	http.HandleFunc("/api/admin/close-all", killswitch.HandleCloseAll)
	http.HandleFunc("/api/admin/reset", killswitch.HandleReset)
	http.HandleFunc("/api/admin/clear-history", killswitch.HandleClearHistory)

	// Institutional kill switch endpoints (P1-B):
	//   POST /api/admin/ks/block  — Mode A: block new orders, engine stays alive
	//   POST /api/admin/ks/release — release graceful block, resume order flow
	//   GET  /api/admin/ks/status  — query kill switch state
	http.HandleFunc("/api/admin/ks/block", func(w http.ResponseWriter, r *http.Request) {
		// No CORS wildcard — this is an admin-only endpoint.
		// The security gate enforces ENGINE_ADMIN_SECRET before this handler runs.
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if !ksSvc.IsEnabled() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "ignored",
				"message": "Kill switch is disabled (KILL_SWITCH_ENABLED=false). Set KILL_SWITCH_ENABLED=true to arm.",
			})
			return
		}
		if err := ksSvc.Trigger(r.Context(), killswitchpkg.Activation{
			Trigger: killswitchpkg.TriggerManualOperator,
			Reason:  "manual operator block via /api/admin/ks/block",
			Actions: []killswitchpkg.Action{killswitchpkg.ActionBlockNewOrders, killswitchpkg.ActionSendAlerts},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Println("[KILL SWITCH] Mode A activated: new order flow blocked, engine running")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "blocked", "message": "New order submissions blocked. Engine running. Use /api/admin/ks/release to resume."})
	})
	http.HandleFunc("/api/admin/ks/release", func(w http.ResponseWriter, r *http.Request) {
		// No CORS wildcard — admin-only endpoint protected by security gate.
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := ksSvc.Release(r.Context(), killswitchpkg.TriggerManualOperator, "operator", "manual release via /api/admin/ks/release"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Println("[KILL SWITCH] Released: order flow resumed")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "released", "message": "Kill switch released. Order flow resumed."})
	})
	http.HandleFunc("/api/admin/ks/status", func(w http.ResponseWriter, r *http.Request) {
		// Status is a safe read but still should not leak CORS wildcard.
		w.Header().Set("Content-Type", "application/json")
		payload := map[string]interface{}{
			"active":  ksSvc.IsActive(),
			"enabled": ksSvc.IsEnabled(),
			"reason":  ksSvc.Reason(),
		}
		if at := ksSvc.ActivatedAt(); !at.IsZero() {
			payload["triggeredAt"] = at.UTC().Format(time.RFC3339)
		}
		json.NewEncoder(w).Encode(payload)
	})
	// /api/system/resume — session-gated operator resume (no ENGINE_ADMIN_SECRET required).
	// The caller (Next.js /api/killswitch/resume) validates the raig_session JWT before
	// forwarding here, so this endpoint only needs the body confirmation guard.
	http.HandleFunc("/api/system/resume", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Confirm string `json:"confirm"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Confirm != "RESUME" {
			http.Error(w, `{"error":"confirm must equal RESUME"}`, http.StatusBadRequest)
			return
		}
		if err := ksSvc.Release(r.Context(), killswitchpkg.TriggerManualOperator, "operator", "manual release via /api/system/resume"); err != nil {
			log.Printf("[KILL SWITCH] release error: %v", err)
			http.Error(w, `{"error":"release failed"}`, http.StatusInternalServerError)
			return
		}
		log.Println("[KILL SWITCH] Released via /api/system/resume: order flow resumed")
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"resumed": true,
			"message": "Trading resumed. Kill switch cleared.",
		})
	})

	// Mock trading health — post-fix Sev-1 recovery certification endpoint.
	http.HandleFunc("/api/health/mock-trading", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		wh := execWatchdog.Health()
		tradingAllowed := !wh.KillSwitchActive && !wh.StaleMarketData
		status := "healthy"
		if wh.KillSwitchActive {
			status = "blocked_kill_switch"
		} else if wh.StaleMarketData {
			status = "stale_market_data"
		} else if wh.NoTradeAlertLevel != "" {
			status = "no_trades_" + wh.NoTradeAlertLevel
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":              tradingAllowed,
			"status":          status,
			"trading_allowed": tradingAllowed,
			"kill_switch": map[string]interface{}{
				"active": wh.KillSwitchActive,
				"reason": wh.KillSwitchReason,
			},
			"last_tick_at":         formatHealthTime(wh.LastTickAt),
			"last_signal_at":       formatHealthTime(wh.LastSignalAt),
			"last_fill_at":         formatHealthTime(wh.LastFillAt),
			"no_trade_since_fill":  wh.NoTradeSinceFill.String(),
			"no_trade_alert_level": wh.NoTradeAlertLevel,
			"stale_market_data":    wh.StaleMarketData,
			"account_key":          paperpersist.FrontendAccountKey,
			"execution_authority":  "go_engine",
			"timestamp":            time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Security status endpoint — SUPER_ADMIN only (gate enforces RBAC).
	http.HandleFunc("/api/security/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		snap := secGate.Projection().Snapshot()
		json.NewEncoder(w).Encode(snap) //nolint:errcheck
	})
	http.HandleFunc("/api/security/audit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(secGate.AuditLog(200)) //nolint:errcheck
	})
	http.HandleFunc("/api/security/incidents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(secGate.Monitor().OpenIncidents()) //nolint:errcheck
	})

	// /health — liveness probe (K8s + load balancer).
	// Returns 200 as long as the process is alive.
	// Returns 500 only if the kill switch is active (engine should restart).
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		ksActive := ksSvc.IsActive()
		lastCycleAgo := int64(-1)
		if lt := orchestrator.LastCycleTime(); !lt.IsZero() {
			lastCycleAgo = int64(time.Since(lt).Seconds())
		}
		regime := "UNKNOWN"
		if rc := orchestrator.CurrentRegime(); rc != "" {
			regime = rc
		}
		body := map[string]interface{}{
			"status":                 "alive",
			"service":                "btc-pilot-engine",
			"uptime_seconds":         int64(time.Since(bootStart).Seconds()),
			"kill_switch_active":     ksActive,
			"regime":                 regime,
			"last_cycle_ago_seconds": lastCycleAgo,
			"strategies":             len(allStrategies),
		}
		w.Header().Set("Content-Type", "application/json")
		if ksActive {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		json.NewEncoder(w).Encode(body) //nolint:errcheck
	})

	// /ready — readiness probe (K8s). Returns 503 if engine is not ready to trade.
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		mongoOK := mongoMgr != nil && mongoMgr.IsConnected()
		fundingOK := loopDeps != nil && loopDeps.FundingFetcher != nil && loopDeps.FundingFetcher.GetLatest() != nil
		oiOK := loopDeps != nil && loopDeps.OIFetcher != nil && loopDeps.OIFetcher.GetLatest() != nil
		depthOK := loopDeps != nil && loopDeps.DepthSubscriber != nil
		dqScore := 100.0
		if loopDeps != nil && loopDeps.DataValidator != nil {
			dqScore = loopDeps.DataValidator.LatestScore()
		}
		reconOK := reconciliationComplete.Load()
		lastCycleAgo := int64(-1)
		if lt := orchestrator.LastCycleTime(); !lt.IsZero() {
			lastCycleAgo = int64(time.Since(lt).Seconds())
		}
		body := map[string]interface{}{
			"status":                     "ready",
			"mongodb_connected":          mongoOK,
			"funding_fetcher_ok":         fundingOK,
			"oi_fetcher_ok":              oiOK,
			"depth_subscriber_connected": depthOK,
			"data_quality_score":         dqScore,
			"reconciliation_complete":    reconOK,
			"last_cycle_ago_seconds":     lastCycleAgo,
		}
		w.Header().Set("Content-Type", "application/json")
		notReady := !mongoOK || dqScore < 60 || !reconOK
		if notReady {
			body["status"] = "not_ready"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		json.NewEncoder(w).Encode(body) //nolint:errcheck
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

	// GET /api/strategies/walkforward — Walk-forward validator status per strategy
	http.HandleFunc("/api/strategies/walkforward", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orchestrator.WalkForwardSummary())
	})

	// GET /api/strategies/performance — Per-strategy performance metrics
	http.HandleFunc("/api/strategies/performance", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orchestrator.StrategyPerformanceSummary())
	})

	// ── Shadow Trading endpoints ──────────────────────────────────────────────
	// Admin-secret protected by the global security gate (same as
	// /api/admin/ks/status) — no per-handler auth needed here.

	// GET /api/shadow/performance — leaderboard of ALL shadow strategies,
	// sorted by win rate descending.
	http.HandleFunc("/api/shadow/performance", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		sl := orchestrator.ShadowLedger()
		if sl == nil {
			json.NewEncoder(w).Encode([]shadow.ShadowPerformance{}) //nolint:errcheck
			return
		}
		perfs := sl.AllPerformance()
		sort.Slice(perfs, func(i, j int) bool { return perfs[i].WinRate > perfs[j].WinRate })
		json.NewEncoder(w).Encode(perfs) //nolint:errcheck
	})

	// GET /api/shadow/open — all currently open (not yet closed) shadow positions.
	http.HandleFunc("/api/shadow/open", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		sl := orchestrator.ShadowLedger()
		if sl == nil {
			json.NewEncoder(w).Encode([]interface{}{}) //nolint:errcheck
			return
		}
		json.NewEncoder(w).Encode(sl.AllOpenTrades()) //nolint:errcheck
	})

	// GET /api/shadow/performance/{strategyName} — single strategy detail
	// including its closed shadow trades.
	http.HandleFunc("/api/shadow/performance/", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/shadow/performance/")
		if name == "" {
			http.Error(w, "strategy name required", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		sl := orchestrator.ShadowLedger()
		if sl == nil {
			http.Error(w, "shadow ledger not initialized", http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"performance":   sl.GetPerformance(name),
			"closed_trades": sl.GetClosedTrades(name, 50),
		})
	})

	// POST /api/shadow/promote — body: {"strategyName": "..."}
	http.HandleFunc("/api/shadow/promote", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			StrategyName string `json:"strategyName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.StrategyName == "" {
			http.Error(w, "strategyName required", http.StatusBadRequest)
			return
		}
		promoter := orchestrator.ShadowPromoter()
		if promoter == nil {
			http.Error(w, "shadow promoter not initialized", http.StatusServiceUnavailable)
			return
		}
		if err := promoter.Promote(r.Context(), body.StrategyName); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"status":  "promoted",
			"message": fmt.Sprintf("%s will trade live from the next 15m eval cycle", body.StrategyName),
		})
	})

	// POST /api/shadow/demote — body: {"strategyName": "..."}
	http.HandleFunc("/api/shadow/demote", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			StrategyName string `json:"strategyName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.StrategyName == "" {
			http.Error(w, "strategyName required", http.StatusBadRequest)
			return
		}
		promoter := orchestrator.ShadowPromoter()
		if promoter == nil {
			http.Error(w, "shadow promoter not initialized", http.StatusServiceUnavailable)
			return
		}
		if err := promoter.Demote(r.Context(), body.StrategyName); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
			"status":  "demoted",
			"message": fmt.Sprintf("%s moved back to shadow mode", body.StrategyName),
		})
	})

	// GET /api/system/confidence-floor — Adaptive confidence floor status
	http.HandleFunc("/api/system/confidence-floor", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orchestrator.ConfidenceFloorSnapshot())
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

	// GET /api/trades — Completed trade journal (UNLIMITED DB MODE)
	http.HandleFunc("/api/trades", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}

		// If DB is available, fetch the latest 5,000 trades from the relational table.
		if dbStore != nil {
			trades, err := dbStore.GetTrades(context.Background(), 5000)
			if err == nil {
				json.NewEncoder(w).Encode(trades)
				return
			}
			log.Printf("[API] ⚠️  Failed to fetch history from DB: %v", err)
		}

		// Fallback to in-memory summary if DB query fails.
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
		realizedBalance := paperExecute.GetBalanceUSD()

		ticks, candles := candleAgg.GetStats()
		response := map[string]interface{}{
			"aggregate":      aggStats,
			"balance":        realizedBalance,
			"equity":         paperExecute.GetEquityUSD(),
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

	// GET /api/scalers/stats — Live monitoring for the 7 curated scalper strategies.
	// Returns per-strategy win/loss stats, current regime, CVD, and eval cycle count.
	// Use this to detect which strategies are trading and tune selectivity per regime.
	http.HandleFunc("/api/scalers/stats", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		json.NewEncoder(w).Encode(orchestrator.GetScalersStats()) //nolint:errcheck
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

	// GET /api/ai/insights — Recent Claude multi-agent decisions
	http.HandleFunc("/api/ai/insights", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		if aiOrchestrator == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled":       false,
				"geminiEnabled": false,
				"message":       "AI agents disabled — set GROQ_API_KEY (free) or OPENAI_API_KEY to enable AI trading",
				"insights":      []interface{}{},
			})
			return
		}
		latest := aiOrchestrator.GetInsights().Latest()
		recent := aiOrchestrator.GetInsights().GetRecent(20)
		audits := aiOrchestrator.GetInsights().GetAuditLogs(10)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":       true,
			"geminiEnabled": aiOrchestrator.GeminiEnabled(),
			"latest":        latest,
			"recent":        recent,
			"auditLogs":     audits,
		})
	})

	// GET /api/ai/strategies — Structured AI strategy library and support summary
	http.HandleFunc("/api/ai/strategies", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total":      len(ai.GetAIStrategyLibrary()),
			"summary":    ai.SummarizeAIStrategyLibrary(),
			"categories": ai.GetAIStrategyCategories(),
			"strategies": ai.GetAIStrategyLibrary(),
		})
	})

	// GET /api/ai/pending — Parked signals waiting for UI Command Center
	http.HandleFunc("/api/ai/pending", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		pending := orchestrator.GetPendingSignals()
		json.NewEncoder(w).Encode(pending)
	})

	// POST /api/ai/submit — Final submission from UI Command Center (ChatGPT Arbitrator)
	http.HandleFunc("/api/ai/submit", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID     string `json:"id"`
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Run in background but with a context that won't die immediately
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := orchestrator.ConfirmSignal(ctx, req.ID, req.Prompt); err != nil {
				log.Printf("[AI SUBMIT] confirm failed for %s: %v", req.ID, err)
			}
		}()

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "processing"})
	})

	// POST /api/ai/bridge-result — Structured verdict from ChatGPT browser bridge
	http.HandleFunc("/api/ai/bridge-result", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			ID         string  `json:"id"`
			Approved   bool    `json:"approved"`
			Action     string  `json:"action"`
			Confidence float64 `json:"confidence"`
			Reason     string  `json:"reason"`
			RawReply   string  `json:"rawReply"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if req.ID == "" {
			http.Error(w, "Missing signal id", http.StatusBadRequest)
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := orchestrator.ConfirmSignalFromBridge(ctx, req.ID, trading.BridgeDecision{
				Approved:   req.Approved,
				Action:     req.Action,
				Confidence: req.Confidence,
				Reason:     req.Reason,
				RawReply:   req.RawReply,
			})
			if err != nil {
				log.Printf("[BRIDGE] ⚠️  Failed to process browser verdict for %s: %v", req.ID, err)
			}
		}()

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "processing"})
	})

	// GET /api/ai/bridge-status — Check if the browser bridge is online
	http.HandleFunc("/api/ai/bridge-heartbeat", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		orchestrator.RecordBridgeHeartbeat()
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/api/ai/bridge-event", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Message string `json:"message"`
			Level   string `json:"level"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if req.Message != "" {
			orchestrator.RecordBridgeEvent(req.Message, req.Level)
		}
		w.WriteHeader(http.StatusOK)
	})

	// POST /api/ai/test-signal — Trigger a fake signal for testing the Robot
	http.HandleFunc("/api/ai/test-signal", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		orchestrator.AddTestSignal()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "RAIG: TEST SIGNAL INJECTED. WATCH YOUR ROBOT!")
	})

	http.HandleFunc("/api/ai/bridge-status", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		json.NewEncoder(w).Encode(orchestrator.GetBridgeStatus())
	})

	// GET /api/execution/intelligence — Phase 22D execution-intelligence report:
	// trade conversion, missed entries, latency percentiles, slippage, TP-override
	// impact, bottleneck ranking, and the composite execution quality score.
	http.HandleFunc("/api/execution/intelligence", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(orchestrator.ExecIntelSnapshot()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// GET /api/phase22e/certification — Phase 22E profitability validation.
	// Runs the full certification pipeline on the in-memory paper trade journal
	// and returns the ValidationResult plus Monte Carlo, Alpha, Correlation,
	// Retirement, and Deployment tier data as JSON.
	http.HandleFunc("/api/phase22e/certification", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		w.Header().Set("Content-Type", "application/json")

		// collect closed trades from the paper trade journal
		var trades []phase22e.TradeRecord
		for _, entry := range journal.GetAllTrades() {
			if entry.ExitTime.IsZero() {
				continue
			}
			trades = append(trades, phase22e.TradeRecord{
				TradeID:      entry.ID,
				StrategyID:   entry.StrategyName,
				StrategyName: entry.StrategyName,
				Family:       entry.Category,
				Symbol:       "BTC-USD",
				Side:         entry.Side,
				EntryPrice:   entry.EntryPrice,
				ExitPrice:    entry.ExitPrice,
				Quantity:     entry.Size,
				GrossPnLUSD:  entry.GrossPnL,
				NetPnLUSD:    entry.NetPnL,
				FeesUSD:      entry.Fees,
				HoldMinutes:  entry.Duration.Minutes(),
				EntryTime:    entry.EntryTime,
				ExitTime:     entry.ExitTime,
				IsLive:       false,
			})
		}

		totalCapital := getInitialPaperBalanceUSD()
		v := phase22e.NewValidator(totalCapital)
		result := v.Run(trades)

		portfolioMC := phase22e.RunMonteCarlo(trades, totalCapital, 500)

		stratTrades := make(map[string][]phase22e.TradeRecord)
		for _, t := range trades {
			stratTrades[t.StrategyID] = append(stratTrades[t.StrategyID], t)
		}
		nStrats := len(stratTrades)
		if nStrats == 0 {
			nStrats = 1
		}
		perStratNAV := totalCapital / float64(nStrats)
		stratMC := make(map[string]phase22e.MonteCarloResult)
		for sid, sts := range stratTrades {
			stratMC[sid] = phase22e.RunMonteCarlo(sts, perStratNAV, 200)
		}

		alphas := phase22e.CertifyAlphaEngines(trades, totalCapital)
		corrMatrix := phase22e.ComputeCorrelation(trades)
		retirementCandidates := phase22e.IdentifyRetirementCandidates(result.Strategies, stratMC)
		deployments := phase22e.ClassifyAllStrategies(result.Strategies)

		resp := map[string]interface{}{
			"certification":         result,
			"monte_carlo_portfolio": portfolioMC,
			"alpha_engines":         alphas,
			"correlation": map[string]interface{}{
				"diversification_score": corrMatrix.DiversScore,
				"cluster_count":         len(corrMatrix.Clusters),
				"clusters":              corrMatrix.Clusters,
			},
			"retirement_candidates": retirementCandidates,
			"deployment_tiers":      deployments,
			"tier_counts":           phase22e.TierCounts(deployments),
			"generated_at":          result.GeneratedAt,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// ── Pre-Live Engine transparent proxy (/prelive/* → localhost:8082/*) ────────
	// Port 8082 is internal-only; this route exposes it through port 80 so
	// Vercel can reach the pre-live engine without opening a second firewall port.
	preLiveHost := os.Getenv("PRE_LIVE_HOST")
	if preLiveHost == "" {
		preLiveHost = "127.0.0.1"
	}
	preLivePort := os.Getenv("PRE_LIVE_PORT")
	if preLivePort == "" {
		preLivePort = "8082"
	}
	http.HandleFunc("/prelive/", func(w http.ResponseWriter, r *http.Request) {
		target := "http://" + preLiveHost + ":" + preLivePort + r.URL.RequestURI()[len("/prelive"):]
		req, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
		if err != nil {
			http.Error(w, "proxy error: "+err.Error(), http.StatusBadGateway)
			return
		}
		req.Header = r.Header.Clone()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "pre-live engine unreachable: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body) //nolint:errcheck
	})

	// Use PORT env var so the server and keepAlive both bind to the same port.
	// Render sets PORT=10000; locally defaults to 8080.
	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	// ── Phase 15J: API Gateway wraps the security gate ───────────────────────
	// Traffic flow: Gateway (tracing + access log + panic recovery)
	//               → Security Gate (authn + RBAC + rate limit + audit)
	//               → Handler
	apiGateway := gateway.New(secGate.Wrap(http.DefaultServeMux), gateway.Config{
		ServiceName: "raig-engine-v3",
	})

	server := &http.Server{
		Addr:              ":" + httpPort,
		Handler:           apiGateway,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	go func() {
		fmt.Printf("═══════════════════════════════════════════\n")
		fmt.Printf("   RAIG AUTONOMOUS TRADING ENGINE ONLINE\n")
		fmt.Printf("   Listening on :%s\n", httpPort)
		fmt.Printf("═══════════════════════════════════════════\n")
		fmt.Println("  [RAIG CORE PROTOCOLS ACTIVE]")
		fmt.Println("    GET    /health          — System Vital Check")
		fmt.Println("    GET    /api/strategies   — Strategy Intelligence")
		fmt.Println("    GET    /api/positions    — Active Engagements")
		fmt.Println("    GET    /api/stats        — Performance Data")
		fmt.Println("    POST   /api/admin/kill   — Global Kill Switch")
		fmt.Printf("═══════════════════════════════════════════\n")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Println("[RAIG] Server error:", err)
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[RAIG] HTTP shutdown warning: %v", err)
	}
	coinbaseClient.Close()
	if dbStore != nil {
		dbStore.Close()
	}
	// Phase 31B: flush retry queue + disconnect MongoDB.
	if ppBundle != nil {
		ppBundle.TradeWriter().Stop()
		if err := ppBundle.Mgr().Shutdown(shutdownCtx); err != nil {
			log.Printf("[Phase31B] MongoDB shutdown warning: %v", err)
		}
		log.Printf("[Phase31B] MongoDB persistence shut down cleanly")
	}
	// Stop async scorer workers cleanly.
	asyncScorer.Stop()
	time.Sleep(2 * time.Second) // Allow state saver final flush
	log.Println("Systems offline.")
}

// spotFromDeltaQuote picks a USD BTC reference from a Delta ticker (spot index, last, or mark).
func spotFromDeltaQuote(q marketdata.DeltaTickerQuote) float64 {
	if q.SpotPrice > 5000 && q.SpotPrice < 1000000 {
		return q.SpotPrice
	}
	if q.Price > 5000 && q.Price < 1000000 {
		return q.Price
	}
	if q.MarkPrice > 5000 && q.MarkPrice < 1000000 {
		return q.MarkPrice
	}
	return 0
}

// fetchDeltaBTCSpotForOptions reads Delta's public GET /v2/tickers/{symbol} (no auth).
// Override symbol with DELTA_OPTIONS_BTC_TICKER (default BTCUSD). Base URL: DELTA_API_BASE_URL or India default.
func fetchDeltaBTCSpotForOptions(ctx context.Context) (float64, error) {
	if deltaProbeClient == nil {
		return 0, fmt.Errorf("delta ticker client not initialized")
	}
	sym := strings.TrimSpace(os.Getenv("DELTA_OPTIONS_BTC_TICKER"))
	if sym == "" {
		sym = "BTCUSD"
	}
	q, err := deltaProbeClient.FetchTicker(ctx, sym)
	if err != nil {
		return 0, err
	}
	p := spotFromDeltaQuote(q)
	if p <= 0 {
		return 0, fmt.Errorf("delta ticker %s: no usable spot/close/mark price", sym)
	}
	return p, nil
}

func fetchBinanceBTCSpot(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.binance.com/api/v3/ticker/price?symbol=BTCUSDT", nil)
	if err != nil {
		return 0, fmt.Errorf("build fallback request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fallback request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("fallback status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("fallback body read failed: %w", err)
	}
	var payload struct {
		Price string `json:"price"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, fmt.Errorf("fallback decode failed: %w", err)
	}
	price, err := strconv.ParseFloat(payload.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("fallback parse failed: %w", err)
	}
	return price, nil
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

// handleDeltaBTCProbe fetches a live BTC ticker from Delta Exchange and returns it as JSON.
func handleDeltaBTCProbe(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}

	quote, err := deltaProbeClient.FetchTicker(r.Context(), "BTCUSD")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":     false,
			"error":  err.Error(),
			"source": "delta_exchange",
			"symbol": "BTCUSD",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"source":        "delta_exchange",
		"symbol":        quote.Symbol,
		"price":         quote.Price,
		"open":          quote.Open,
		"high":          quote.High,
		"low":           quote.Low,
		"mark_price":    quote.MarkPrice,
		"spot_price":    quote.SpotPrice,
		"volume":        quote.Volume,
		"contract_type": quote.ContractType,
		"exchange_time": quote.ExchangeTime,
		"fetched_at":    quote.FetchedAt.Format(time.RFC3339),
		"ok":            true,
		"error":         "",
	})
}

// safeGo wraps a goroutine function with panic recovery.
// If the goroutine panics, it logs the error and restarts after 5 seconds.
// If fn returns normally (context cancelled), safeGo exits without restarting.
func safeGo(name string, fn func()) {
	for {
		panicked := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[⚠️ PANIC RECOVERED] %s crashed: %v — restarting in 5s...", name, r)
					panicked = true
				}
			}()
			fn()
		}()
		if !panicked {
			log.Printf("[%s] Goroutine exited normally", name)
			return
		}
		time.Sleep(5 * time.Second)
	}
}

// strategyNamesAdapter wraps []strategy.RegistryEntry so it satisfies
// regime.StrategyRegistry without importing the regime package at the call-site.
type strategyNamesAdapter struct {
	strategies []strategy.RegistryEntry
}

func (a *strategyNamesAdapter) Names() []string {
	names := make([]string, len(a.strategies))
	for i, e := range a.strategies {
		names[i] = e.Strategy.Name()
	}
	return names
}

func formatHealthTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// getEnvOrDefault returns the env var value or fallback when the var is unset.
func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
