package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"antigravity-engine/internal/admin"
	"antigravity-engine/internal/ai"
	"antigravity-engine/internal/delta"
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/niftystocks"
	"antigravity-engine/internal/options"
	"antigravity-engine/internal/options_selling"
	"antigravity-engine/internal/persistence"
	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/risk"
	"antigravity-engine/internal/security"
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

const initialPaperBalanceUSD = 1000000.0

var (
	deltaProbeClient    *marketdata.DeltaTickerClient
	angelOneProbeClient *marketdata.AngelOneClient
	niftyOptionsEngine  *options.Engine

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
func loadDotEnv() {
	root := "../.." // relative to engine/cmd/antigravity
	data, err := os.ReadFile(root + "/.env")
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

func saveNiftyOptionsSnapshot(ctx context.Context, store *persistence.Store, snapshot options.PersistedState) error {
	priceHistJSON, err := json.Marshal(snapshot.PriceHist)
	if err != nil {
		return fmt.Errorf("marshal nifty options price history: %w", err)
	}
	minuteBarsJSON, err := json.Marshal(snapshot.MinuteBars)
	if err != nil {
		return fmt.Errorf("marshal nifty options minute bars: %w", err)
	}
	tradesJSON, err := json.Marshal(snapshot.Trades)
	if err != nil {
		return fmt.Errorf("marshal nifty options trades: %w", err)
	}
	strategiesJSON, err := json.Marshal(snapshot.Strategies)
	if err != nil {
		return fmt.Errorf("marshal nifty options strategies: %w", err)
	}

	return store.SaveNiftyOptionsState(ctx, &persistence.NiftyOptionsState{
		Balance:    snapshot.Balance,
		LastPrice:  snapshot.LastPrice,
		LastMinute: snapshot.LastMinute,
		TradeSeq:   snapshot.TradeSeq,
		PriceHist:  priceHistJSON,
		MinuteBars: minuteBarsJSON,
		Trades:     tradesJSON,
		Strategies: strategiesJSON,
	})
}

func loadNiftyOptionsSnapshot(state *persistence.NiftyOptionsState) (options.PersistedState, error) {
	snapshot := options.PersistedState{
		Balance:    state.Balance,
		LastPrice:  state.LastPrice,
		LastMinute: state.LastMinute,
		TradeSeq:   state.TradeSeq,
		SavedAt:    state.SavedAt,
	}

	if len(state.PriceHist) > 0 && string(state.PriceHist) != "[]" {
		if err := json.Unmarshal(state.PriceHist, &snapshot.PriceHist); err != nil {
			return options.PersistedState{}, fmt.Errorf("unmarshal nifty options price history: %w", err)
		}
	}
	if len(state.MinuteBars) > 0 && string(state.MinuteBars) != "[]" {
		if err := json.Unmarshal(state.MinuteBars, &snapshot.MinuteBars); err != nil {
			return options.PersistedState{}, fmt.Errorf("unmarshal nifty options minute bars: %w", err)
		}
	}
	if len(state.Trades) > 0 && string(state.Trades) != "[]" {
		if err := json.Unmarshal(state.Trades, &snapshot.Trades); err != nil {
			return options.PersistedState{}, fmt.Errorf("unmarshal nifty options trades: %w", err)
		}
	}
	if len(state.Strategies) > 0 && string(state.Strategies) != "[]" {
		if err := json.Unmarshal(state.Strategies, &snapshot.Strategies); err != nil {
			return options.PersistedState{}, fmt.Errorf("unmarshal nifty options strategies: %w", err)
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

func saveNiftyOptionsSellingSnapshot(ctx context.Context, store *persistence.Store, snapshot options_selling.PersistedState) error {
	priceHistJSON, err := json.Marshal(snapshot.PriceHist)
	if err != nil {
		return fmt.Errorf("marshal nifty options selling price history: %w", err)
	}
	minuteBarsJSON, err := json.Marshal(snapshot.MinuteBars)
	if err != nil {
		return fmt.Errorf("marshal nifty options selling minute bars: %w", err)
	}
	tradesJSON, err := json.Marshal(snapshot.Trades)
	if err != nil {
		return fmt.Errorf("marshal nifty options selling trades: %w", err)
	}
	strategiesJSON, err := json.Marshal(snapshot.Strategies)
	if err != nil {
		return fmt.Errorf("marshal nifty options selling strategies: %w", err)
	}

	return store.SaveNiftyOptionsSellingState(ctx, &persistence.NiftyOptionsSellingState{
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
	})
}

func loadNiftyOptionsSellingSnapshot(state *persistence.NiftyOptionsSellingState) (options_selling.PersistedState, error) {
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
			return options_selling.PersistedState{}, fmt.Errorf("unmarshal nifty options selling price history: %w", err)
		}
	}
	if len(state.MinuteBars) > 0 && string(state.MinuteBars) != "[]" {
		if err := json.Unmarshal(state.MinuteBars, &snapshot.MinuteBars); err != nil {
			return options_selling.PersistedState{}, fmt.Errorf("unmarshal nifty options selling minute bars: %w", err)
		}
	}
	if len(state.Trades) > 0 && string(state.Trades) != "[]" {
		if err := json.Unmarshal(state.Trades, &snapshot.Trades); err != nil {
			return options_selling.PersistedState{}, fmt.Errorf("unmarshal nifty options selling trades: %w", err)
		}
	}
	if len(state.Strategies) > 0 && string(state.Strategies) != "[]" {
		if err := json.Unmarshal(state.Strategies, &snapshot.Strategies); err != nil {
			return options_selling.PersistedState{}, fmt.Errorf("unmarshal nifty options selling strategies: %w", err)
		}
	}

	return snapshot, nil
}

func main() {
	log.SetOutput(globalLogs)
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║   RAIG ENGINE v6.0 — IMMORTAL EDITION                  ║")
	fmt.Println("║   30 Curated Strategies | Full State Restore | Panic Recovery  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")

	loadDotEnv()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bootStart := time.Now()

	// ═══════════════════════════════════════════════════
	// 1. WebSocket Live Stream (Coinbase)
	// ═══════════════════════════════════════════════════
	coinbaseClient := marketdata.NewCoinbaseClient()
	nseIndexClient := marketdata.NewNSEIndexClient()
	niftyMarketCache := NewNiftyMarketCache(240)
	deltaProbeClient = marketdata.NewDeltaTickerClient()
	angelOneProbeClient = marketdata.NewAngelOneClient()
	go func() {
		err := coinbaseClient.Connect(ctx, []string{"BTC-USD"})
		if err != nil {
			log.Fatalf("Fatal error connecting to Coinbase: %v", err)
		}
	}()

	// ═══════════════════════════════════════════════════
	// 2. Build curated strategies (BTC Equity roster capped at 30)
	// ═══════════════════════════════════════════════════
	const btcEquityStrategyCount = 30
	allStrategies := strategy.BuildCuratedScalpers()
	if len(allStrategies) > btcEquityStrategyCount {
		allStrategies = allStrategies[:btcEquityStrategyCount]
	}
	log.Printf("[INIT] Loaded %d curated live strategies (cap %d)", len(allStrategies), btcEquityStrategyCount)

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
	// 3. Risk Engine (configured for the $1,000,000 futures paper account)
	// ═══════════════════════════════════════════════════
	riskProfile := risk.RiskProfile{
		MaxPositionBTC:  2.0,                    // Max 2 BTC total exposure
		MaxCapitalUSD:   initialPaperBalanceUSD, // $1,000,000 paper balance
		MaxDailyLossPct: 0.05,                   // 5% daily loss circuit breaker ($50,000)
	}
	riskEngine := risk.NewRiskEngine(riskProfile)

	// ═══════════════════════════════════════════════════
	// 4. Strategy Tracker (Per-Strategy Performance)
	// ═══════════════════════════════════════════════════
	tracker := risk.NewStrategyTracker(names, categories, timeframes, initialPaperBalanceUSD)

	// ═══════════════════════════════════════════════════
	// 5. Paper Executor ($1,000,000 futures paper account)
	// ═══════════════════════════════════════════════════
	paperExecute := execution.NewPaperClient(initialPaperBalanceUSD)

	// ═══════════════════════════════════════════════════
	// 5b. Paper OMS — canonical execution (Epic 1)
	// ═══════════════════════════════════════════════════
	paperOMS := execution.NewPaperOMS(initialPaperBalanceUSD)

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
		if loadErr == nil && state.Balance != initialPaperBalanceUSD {
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

	// Start the orchestrator with panic recovery
	go safeGo("Orchestrator", func() { orchestrator.Run(ctx) })

	// ═══════════════════════════════════════════════════
	// 11c. BTC OPTIONS SCALPER — 50 strategies, separate $1,000,000 paper account
	// ═══════════════════════════════════════════════════
	optionsEngine := options.NewEngine()
	optionsSellingEngine := options_selling.NewEngine()
	niftyOptionsEngine = options.NewNiftyEngine()
	niftyOptionsSellingEngine := options_selling.NewNiftyEngine()

	// Delta Exchange live bridge — mirrors BTC option signals to Delta when enabled.
	// StartMonitor polls live positions every 5 min and auto-closes at profit/stop targets.
	deltaBridge := delta.NewBridge()
	deltaBridge.StartMonitor(ctx)
	optionsSellingEngine.SetOnOpenHook(func(posID string, stratID int, stratName string, optType string, strike float64, expiry time.Time, premiumUSD float64, btcSpot float64) {
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
	optionsSellingEngine.SetOnCloseHook(func(posID string, stratID int, optType string, strike float64, exitReason string) {
		deltaBridge.OnClose(delta.CloseSignal{
			PaperTradeID: posID,
			StrategyID:   stratID,
			OptionType:   optType,
			Strike:       strike,
			ExitReason:   exitReason,
		})
	})
	niftyStocksEngine := niftystocks.NewEngine()

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

	if dbStore != nil {
		niftyOptionsEngine.SetStateSaveHook(func(snapshot options.PersistedState) {
			if err := saveNiftyOptionsSnapshot(context.Background(), dbStore, snapshot); err != nil {
				log.Printf("[DB] WARN Failed to save NIFTY options state: %v", err)
			}
		})

		niftyState, loadErr := dbStore.LoadNiftyOptionsState(ctx)
		if loadErr != nil {
			log.Printf("[DB] WARN Failed to load NIFTY options state: %v", loadErr)
		} else {
			snapshot, snapshotErr := loadNiftyOptionsSnapshot(niftyState)
			if snapshotErr != nil {
				log.Printf("[DB] WARN Failed to decode NIFTY options state: %v", snapshotErr)
			} else {
				niftyOptionsEngine.RestoreState(snapshot)
				restoredOpen := 0
				for _, strategyState := range snapshot.Strategies {
					if strategyState.Position != nil {
						restoredOpen++
					}
				}
				log.Printf(
					"[DB] RESTORE NIFTY options state from %s | Balance: INR %.2f | Open Positions: %d | Trades: %d",
					niftyState.SavedAt.Format(time.RFC3339), snapshot.Balance, restoredOpen, len(snapshot.Trades),
				)
			}
		}

		niftyOptionsSellingEngine.SetStateSaveHook(func(snapshot options_selling.PersistedState) {
			if err := saveNiftyOptionsSellingSnapshot(context.Background(), dbStore, snapshot); err != nil {
				log.Printf("[DB] WARN Failed to save NIFTY options selling state: %v", err)
			}
		})

		niftySellingState, loadErr := dbStore.LoadNiftyOptionsSellingState(ctx)
		if loadErr != nil {
			log.Printf("[DB] WARN Failed to load NIFTY options selling state: %v", loadErr)
		} else {
			snapshot, snapshotErr := loadNiftyOptionsSellingSnapshot(niftySellingState)
			if snapshotErr != nil {
				log.Printf("[DB] WARN Failed to decode NIFTY options selling state: %v", snapshotErr)
			} else {
				niftyOptionsSellingEngine.RestoreState(snapshot)
				restoredOpen := 0
				for _, strategyState := range snapshot.Strategies {
					if strategyState.Position != nil {
						restoredOpen++
					}
				}
				log.Printf(
					"[DB] RESTORE NIFTY options SELLING state from %s | Balance: INR %.2f | Open Positions: %d | Trades: %d",
					niftySellingState.SavedAt.Format(time.RFC3339), snapshot.Balance, restoredOpen, len(snapshot.Trades),
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

	// Pre-fill NIFTY options engines with today's 1m bars from Yahoo Finance.
	go safeGo("NiftyOptionsWarmup", func() {
		niftyCloses, err := marketdata.FetchNiftyWarmupBars(ctx)
		if err != nil {
			log.Printf("[WARMUP] NIFTY warmup failed: %v", err)
			return
		}
		niftyOptionsEngine.InjectMinuteBars(niftyCloses)
		niftyOptionsSellingEngine.InjectMinuteBars(niftyCloses)
		log.Printf("[WARMUP] ✅ Injected %d NIFTY 1m bars into NIFTY options engines", len(niftyCloses))
	})

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

	// isNSEHoliday returns true when the IST date is a declared NSE exchange holiday.
	isNSEHoliday := func(t time.Time) bool {
		type md struct{ m time.Month; d int }
		holidays := map[int][]md{
			2025: {
				{time.January, 26}, {time.February, 19}, {time.March, 14},
				{time.March, 31}, {time.April, 14}, {time.April, 18},
				{time.May, 1}, {time.August, 15}, {time.August, 27},
				{time.October, 2}, {time.October, 24}, {time.November, 5},
				{time.December, 25},
			},
			2026: {
				{time.January, 26}, {time.March, 14}, {time.April, 6},
				{time.April, 14}, {time.April, 18}, {time.May, 1},
				{time.August, 15}, {time.October, 2}, {time.October, 21},
				{time.November, 5}, {time.December, 25},
			},
		}
		for _, h := range holidays[t.Year()] {
			if t.Month() == h.m && t.Day() == h.d {
				return true
			}
		}
		return false
	}

	// indianMarketOpen returns true when NSE cash session is live (9:15–15:30 IST,
	// weekdays only, excluding declared NSE holidays).
	indianMarketOpen := func() bool {
		ist := time.FixedZone("IST", 5*3600+30*60)
		now := time.Now().In(ist)
		if wd := now.Weekday(); wd == time.Saturday || wd == time.Sunday {
			return false
		}
		if isNSEHoliday(now) {
			return false
		}
		open := time.Date(now.Year(), now.Month(), now.Day(), 9, 15, 0, 0, ist)
		close := time.Date(now.Year(), now.Month(), now.Day(), 15, 30, 0, 0, ist)
		return now.After(open) && now.Before(close)
	}

	// Feed NIFTY 50 spot into NIFTY modules.
	// During session (9:15–15:30 IST): polls NSE every 15s, Angel One as fallback.
	// Outside session: 60s cadence, synthetic price only (engines not updated).
	go safeGo("Nifty50PriceFeed", func() {
		var feedPrimed bool
		var lastLive bool
		var lastErr string

		pullQuote := func() {
			var quote marketdata.NSEIndexQuote
			live := false

			if indianMarketOpen() {
				q, nseErr := nseIndexClient.FetchNifty50Quote(ctx)
				if nseErr == nil && q.Price > 0 {
					quote = q
					live = true
					lastErr = ""
				} else {
					// Angel One as fallback when NSE API fails
					if angelOneProbeClient.Enabled() {
						if ao, _ := angelOneProbeClient.FetchNiftyQuote(ctx); ao.Price > 0 {
							quote = marketdata.NSEIndexQuote{
								Index:     "NIFTY 50",
								Price:     ao.Price,
								FetchedAt: time.Now(),
							}
							live = true
							lastErr = ""
							if nseErr != nil {
								log.Printf("[NSE] Falling back to Angel One (NSE err: %v)", nseErr)
							}
						}
					}
					if !live && nseErr != nil {
						msg := nseErr.Error()
						if msg != lastErr {
							log.Printf("[NSE] WARN live quote failed: %v", nseErr)
							lastErr = msg
						}
					}
				}
			}

			if !live || quote.Price <= 0 {
				quote = marketdata.NSEIndexQuote{
					Index:        "NIFTY 50",
					Price:        options.PaperNiftyFallbackSpot(),
					ExchangeTime: "SYNTHETIC",
					FetchedAt:    time.Now(),
				}
			}

			// Log session boundary transitions
			if !feedPrimed {
				feedPrimed = true
				lastLive = live
				if live {
					log.Printf("[NSE] 🟢 Session OPEN — NIFTY 50 live at %.2f (%s)", quote.Price, quote.ExchangeTime)
				} else {
					log.Printf("[NIFTY FEED] Session closed — synthetic NIFTY spot %.0f", quote.Price)
				}
			} else if live && !lastLive {
				log.Printf("[NSE] 🟢 Session OPEN — NIFTY 50 at %.2f", quote.Price)
				lastLive = true
			} else if !live && lastLive {
				log.Printf("[NSE] 🔴 Session CLOSED — last NIFTY 50 at %.2f", quote.Price)
				lastLive = false
			} else if live && lastErr != "" {
				log.Printf("[NSE] Feed recovered at %.2f", quote.Price)
			}

			niftyMarketCache.Update(quote)
			if live {
				niftyOptionsEngine.UpdatePrice(quote.Price)
				niftyOptionsSellingEngine.UpdatePrice(quote.Price)
				niftyStocksEngine.UpdatePrice(quote.Price)
			}
		}

		pullQuote()
		for {
			// 15s during session for fresh signals; 60s outside to reduce API pressure
			interval := 60 * time.Second
			if indianMarketOpen() {
				interval = 15 * time.Second
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				pullQuote()
			}
		}
	})

	go safeGo("OptionsScalper", func() { optionsEngine.Run(ctx.Done()) })
	go safeGo("OptionsSellingScalper", func() { optionsSellingEngine.Run(ctx.Done()) })
	// NIFTY engines re-enabled — price updates are session-gated in Nifty50PriceFeed,
	// so engines only receive real prices during 9:15–15:30 IST on trading days.
	go safeGo("NiftyOptionsScalper", func() { niftyOptionsEngine.Run(ctx.Done()) })
	go safeGo("NiftyOptionsSellingScalper", func() { niftyOptionsSellingEngine.Run(ctx.Done()) })

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
	killswitch := admin.NewKillSwitch(ctx, cancel, paperExecute, paperExecute, journal, posMgr, dbStore, riskEngine, tracker)

	// ── Phase 15G: Zero Trust Security Gate ──────────────────────────────────
	secPolicy := security.LoadPolicy()
	secGate := security.NewGate(secPolicy, nil)
	log.Printf("[SECURITY] Zero Trust Gate active — enforce_auth=%v source=%s",
		secPolicy.EnforceAuth, "env")

	// Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())

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
			Symbol string `json:"symbol"` // e.g. "C-BTC-76000-290426"
			Side   string `json:"side"`   // "buy" or "sell"
			Size   int    `json:"size"`   // number of contracts
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Symbol == "" || body.Side == "" || body.Size < 1 {
			http.Error(w, `{"error":"need symbol, side (buy/sell), size (>=1)"}`, http.StatusBadRequest)
			return
		}
		side := delta.SideBuy
		if strings.ToLower(body.Side) == "sell" {
			side = delta.SideSell
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		result, err := deltaBridge.PlaceManualOrder(ctx, body.Symbol, side, body.Size)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})

	// NIFTY 50 Options Scalper endpoints
	http.HandleFunc("/api/nifty-options/positions", niftyOptionsEngine.HandlePositions)
	http.HandleFunc("/api/nifty-options/trades", niftyOptionsEngine.HandleTrades)
	http.HandleFunc("/api/nifty-options/strategies", niftyOptionsEngine.HandleStrategies)
	http.HandleFunc("/api/nifty-options/stats", niftyOptionsEngine.HandleStats)
	http.HandleFunc("/api/nifty-options/reset", niftyOptionsEngine.HandleReset)
	http.HandleFunc("/api/nifty-options/clear-history", niftyOptionsEngine.HandleClearHistory)
	http.HandleFunc("/api/nifty-options-selling/positions", niftyOptionsSellingEngine.HandlePositions)
	http.HandleFunc("/api/nifty-options-selling/trades", niftyOptionsSellingEngine.HandleTrades)
	http.HandleFunc("/api/nifty-options-selling/strategies", niftyOptionsSellingEngine.HandleStrategies)
	http.HandleFunc("/api/nifty-options-selling/stats", niftyOptionsSellingEngine.HandleStats)
	http.HandleFunc("/api/nifty-options-selling/reset", niftyOptionsSellingEngine.HandleReset)
	http.HandleFunc("/api/nifty-options-selling/clear-history", niftyOptionsSellingEngine.HandleClearHistory)
	http.HandleFunc("/api/nifty-option-chain", niftyOptionsEngine.HandleOptionChain)
	http.HandleFunc("/api/nifty-options/inject-candles", handleNiftyInjectCandles)

	http.HandleFunc("/api/nifty-stocks/positions", niftyStocksEngine.HandlePositions)
	http.HandleFunc("/api/nifty-stocks/trades", niftyStocksEngine.HandleTrades)
	http.HandleFunc("/api/nifty-stocks/strategies", niftyStocksEngine.HandleStrategies)
	http.HandleFunc("/api/nifty-stocks/stats", niftyStocksEngine.HandleStats)
	http.HandleFunc("/api/nifty-stocks/reset", niftyStocksEngine.HandleReset)
	http.HandleFunc("/api/nifty-stocks/clear-history", niftyStocksEngine.HandleClearHistory)
	http.HandleFunc("/api/nifty-market", niftyMarketCache.HandleQuote)

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

	// Regime endpoint — current market regime for BTC and NIFTY engines
	http.HandleFunc("/api/regime", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"btc":       optionsSellingEngine.RegimeInfo(),
			"nifty":     niftyOptionsSellingEngine.RegimeInfo(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Probe endpoints — connectivity tests for Delta Exchange and Angel One
	http.HandleFunc("/api/probe/delta-btc", handleDeltaBTCProbe)
	http.HandleFunc("/api/probe/angelone-nifty", handleAngelOneNiftyProbe)

	// Angel One proxy — routes MCX/NSE API calls from Vercel through this whitelisted IP
	http.HandleFunc("/api/angel-proxy", handleAngelOneProxy)

	// BTC Option Chain endpoint
	http.HandleFunc("/api/option-chain", optionsEngine.HandleOptionChain)

	// Admin endpoints
	http.HandleFunc("/api/admin/kill", killswitch.HandleTrigger)
	http.HandleFunc("/api/admin/close-all", killswitch.HandleCloseAll)
	http.HandleFunc("/api/admin/reset", killswitch.HandleReset)
	http.HandleFunc("/api/admin/clear-history", killswitch.HandleClearHistory)

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

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		setCORS(w)
		if r.Method == http.MethodOptions {
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":     "ok",
			"service":    "raig-engine-v3",
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

	// Use PORT env var so the server and keepAlive both bind to the same port.
	// Render sets PORT=10000; locally defaults to 8080.
	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	server := &http.Server{
		Addr:              ":" + httpPort,
		Handler:           secGate.Wrap(http.DefaultServeMux),
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

// handleNiftyInjectCandles receives a JSON body with close prices and injects them into the NIFTY options engine.
func handleNiftyInjectCandles(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ClosePrices []float64 `json:"close_prices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.ClosePrices) == 0 {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	niftyOptionsEngine.InjectMinuteBars(body.ClosePrices)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":       true,
		"injected": len(body.ClosePrices),
	})
}

// handleAngelOneNiftyProbe fetches a live NIFTY 50 quote from Angel One and returns it as JSON.
func handleAngelOneNiftyProbe(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}

	if !angelOneProbeClient.Enabled() {
		missing := angelOneProbeClient.MissingEnv()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":         false,
			"configured": false,
			"error":      "missing env: " + strings.Join(missing, ", "),
			"source":     "angel_one",
		})
		return
	}

	quote, err := angelOneProbeClient.FetchNiftyQuote(r.Context())
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":         false,
			"configured": true,
			"error":      err.Error(),
			"source":     "angel_one",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"source":         "angel_one",
		"exchange":       quote.Exchange,
		"symbol":         quote.TradingSymbol,
		"symbol_token":   quote.SymbolToken,
		"price":          quote.Price,
		"open":           quote.Open,
		"high":           quote.High,
		"low":            quote.Low,
		"close":          quote.Close,
		"change":         quote.Change,
		"percent_change": quote.PercentChange,
		"exchange_time":  quote.ExchangeTime,
		"fetched_at":     quote.FetchedAt.Format(time.RFC3339),
		"configured":     true,
		"ok":             true,
		"error":          "",
	})
}

// handleAngelOneProxy proxies POST requests to Angel One API using the engine's whitelisted IP.
// The request body must be JSON: {"path": "/rest/secure/...", "body": {...}}.
func handleAngelOneProxy(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var proxyReq struct {
		Path string          `json:"path"`
		Body json.RawMessage `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&proxyReq); err != nil || proxyReq.Path == "" {
		http.Error(w, `{"error":"bad request: path required"}`, http.StatusBadRequest)
		return
	}

	// Allowlist: only permit specific Angel One API paths
	allowed := false
	for _, prefix := range []string{
		"/rest/secure/angelbroking/order/v1/searchScrip",
		"/rest/secure/angelbroking/market/v1/quote/",
		"/rest/secure/angelbroking/market/v1/optionChain",
		"/rest/secure/angelbroking/order/v1/getLtpData",
		"/rest/secure/angelbroking/historical/v1/getCandleData",
	} {
		if strings.HasPrefix(proxyReq.Path, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, `{"error":"path not permitted"}`, http.StatusForbidden)
		return
	}

	if !angelOneProbeClient.Enabled() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Angel One not configured"})
		return
	}

	body, status, err := angelOneProbeClient.ForwardRequest(r.Context(), proxyReq.Path, []byte(proxyReq.Body))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(status)
	w.Write(body)
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
