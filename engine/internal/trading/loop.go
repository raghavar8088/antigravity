package trading

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"antigravity-engine/internal/ai"
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/risk"
	"antigravity-engine/internal/strategy"
)

const (
	minExecutionSizeBTC  = 0.01
	maxAllocationUsage   = 0.60
	sizeChangeEpsilonBTC = 1e-9

	minExecutableConfidence     = 0.72 // Lowered: allow well-setup signals with tighter geometry
	minBridgeApprovalConfidence = 0.60 // Minimum ChatGPT confidence to honour a bridge approval
	minRewardToRiskRatio        = 1.25 // Lowered: achievable at 45%+ win rate (was 1.50)
	minSignalTakeProfitPct      = 0.18 // Ultra-tight TP — gets hit within 1-3 minutes on BTC
	maxSignalStopLossPct        = 0.22 // Ultra-tight SL — cut losses fast before they compound
	defaultSignalStopLossPct    = 0.15 // Default tight SL — noise filter, not a wide buffer

	minExecutionWeightToTrade = 0.25 // Lowered: allow newer/recovering strategies to trade (was 0.45)
	marketHistoryMaxSamples   = 320

	marketRegimeUnknown  = "UNKNOWN"
	marketRegimeTrend    = "TREND"
	marketRegimeRange    = "RANGE"
	marketRegimeVolatile = "VOLATILE"
	marketRegimeMixed    = "MIXED"
)

// Orchestrator is the multi-strategy parallel trading engine.
// It correctly separates tick-based strategies from candle-based strategies,
// ensuring each strategy type receives the data it was designed for.
type Orchestrator struct {
	client     marketdata.MarketDataClient
	strategies []strategy.RegistryEntry
	groups     strategy.StrategyGroups
	risk       *risk.RiskEngine
	exec       *execution.PaperClient
	aggregator *SignalAggregator
	posMgr     *positions.Manager
	tracker    *risk.StrategyTracker
	journal    *execution.TradeJournal
	candleAgg  *marketdata.CandleAggregator

	// AI multi-agent layer (nil when ANTHROPIC_API_KEY not set)
	aiAgent    *ai.MultiAgentOrchestrator
	aiCandleCh chan marketdata.Candle

	// Candle history for AI context (last 20 × 5m candles)
	candleHistory []ai.CandleSummary
	candleHistMu  sync.Mutex

	priceWindow  []float64
	volumeWindow []float64

	// Internal state
	lastPrice  float64
	m15Counter int // Counts 5m candles to simulate 15m (every 3rd 5m candle)
	h1Counter  int // Counts 5m candles to simulate 1h (every 12th)

	// Heartbeat for automated bridge failover
	lastBridgeHeartbeat time.Time
	bridgeHeartbeatMu   sync.RWMutex

	// Interactive AI: Pending signals waiting for UI submission
	pendingSignals map[string]PendingSignal
	pendingMu      sync.RWMutex

	// Replay protection for browser verdict submissions
	processedBridgeSignals map[string]time.Time
	processedBridgeMu      sync.Mutex

	lastBridgeEvent   string
	lastBridgeEventAt time.Time
	lastBridgeError   string
	lastBridgeErrorAt time.Time
	bridgeStateMu     sync.RWMutex

	mu sync.RWMutex
}

// PendingSignal represents a strategy signal waiting for AI/User approval.
type PendingSignal struct {
	ID           string           `json:"id"`
	Signal       strategy.Signal  `json:"signal"`
	StrategyName string           `json:"strategyName"`
	Category     string           `json:"category"`
	Context      ai.MarketContext `json:"context"`
	AutoPrompt   string           `json:"autoPrompt"`
	CreatedAt    time.Time        `json:"createdAt"`
}

// BridgeDecision is the structured verdict returned by the browser bridge.
type BridgeDecision struct {
	Approved   bool    `json:"approved"`
	Action     string  `json:"action"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
	RawReply   string  `json:"rawReply"`
}

type BridgeStatus struct {
	Online              bool      `json:"online"`
	LastHeartbeat       time.Time `json:"lastHeartbeat"`
	SecondsSinceBeat    int       `json:"secondsSinceBeat"`
	PendingSignals      int       `json:"pendingSignals"`
	ProcessedSignalKeys int       `json:"processedSignalKeys"`
	LastEvent           string    `json:"lastEvent"`
	LastEventAt         time.Time `json:"lastEventAt"`
	LastError           string    `json:"lastError"`
	LastErrorAt         time.Time `json:"lastErrorAt"`
}

func NewOrchestrator(
	c marketdata.MarketDataClient,
	strats []strategy.RegistryEntry,
	r *risk.RiskEngine,
	e *execution.PaperClient,
	agg *SignalAggregator,
	pm *positions.Manager,
	tracker *risk.StrategyTracker,
	journal *execution.TradeJournal,
	candleAgg *marketdata.CandleAggregator,
) *Orchestrator {
	groups := strategy.GroupByTimeframe(strats)
	log.Printf("[ORCHESTRATOR] Strategy groups: %d tick, %d 1m, %d 5m, %d 15m, %d 1h",
		len(groups.Tick), len(groups.M1), len(groups.M5), len(groups.M15), len(groups.H1))

	return &Orchestrator{
		client:                 c,
		strategies:             strats,
		groups:                 groups,
		risk:                   r,
		exec:                   e,
		aggregator:             agg,
		posMgr:                 pm,
		tracker:                tracker,
		journal:                journal,
		candleAgg:              candleAgg,
		priceWindow:            make([]float64, 0, marketHistoryMaxSamples),
		volumeWindow:           make([]float64, 0, marketHistoryMaxSamples),
		pendingSignals:         make(map[string]PendingSignal),
		processedBridgeSignals: make(map[string]time.Time),
		lastBridgeHeartbeat:    time.Now(),
	}
}

// SetAIOrchestrator attaches the multi-agent AI system to the orchestrator.
// Called after construction so the constructor signature stays unchanged.
func (o *Orchestrator) SetAIOrchestrator(agent *ai.MultiAgentOrchestrator) {
	o.aiAgent = agent
	o.aiCandleCh = make(chan marketdata.Candle, 10)
	log.Println("[AI] Multi-agent orchestrator attached — Claude will trade on every 5m candle")
}

// WarmupStrategies pre-fills strategy price buffers with historical candle data.
// This eliminates the warmup delay on cold start / Render restart.
func (o *Orchestrator) WarmupStrategies(warmup *marketdata.WarmupData) {
	if warmup == nil {
		log.Println("[WARMUP] No warmup data provided, strategies will warm up from live data")
		return
	}

	log.Printf("[WARMUP] Feeding %d historical 1m candles to %d strategies...",
		len(warmup.Candles1m), len(o.groups.M1))

	// Feed 1m candles to 1m strategies
	for _, candle := range warmup.Candles1m {
		tick := candle.ToTick()
		for _, entry := range o.groups.M1 {
			entry.Strategy.OnTick(tick)
		}
	}

	log.Printf("[WARMUP] Feeding %d historical 5m candles to %d 5m / %d 15m / %d 1h strategies...",
		len(warmup.Candles5m), len(o.groups.M5), len(o.groups.M15), len(o.groups.H1))

	// Feed 5m candles to 5m strategies and simulate 15m / 1h closes.
	for idx, candle := range warmup.Candles5m {
		tick := candle.ToTick()
		for _, entry := range o.groups.M5 {
			entry.Strategy.OnTick(tick)
		}
		if (idx+1)%3 == 0 {
			for _, entry := range o.groups.M15 {
				entry.Strategy.OnTick(tick)
			}
		}
		if (idx+1)%12 == 0 {
			for _, entry := range o.groups.H1 {
				entry.Strategy.OnTick(tick)
			}
		}
	}

	log.Println("[WARMUP] ✅ All strategy buffers pre-filled. Ready for live trading.")
}

// Run is the infinite heartbeat of RAIG Autonomous Trading.
// It processes ticks and candles through their respective strategy groups.
func (o *Orchestrator) Run(ctx context.Context) {
	log.Printf("[RAIG MASTER LOOP] 🛰️  Booting Protocols with %d active strategies...", len(o.strategies))
	ticks := o.client.GetTickChannel()

	// Background: process position close events (SL/TP/trailing)
	go o.processCloseEvents(ctx)

	// Background: re-enable cooled-down strategies every minute
	go o.strategyCooldownChecker(ctx)

	// Background: process 1m candle closes
	go o.process1mCandles(ctx)

	// Background: process 5m candle closes
	go o.process5mCandles(ctx)

	// Background: AI multi-agent decisions (only when API key is set)
	if o.aiAgent != nil && o.aiAgent.IsAvailable() {
		go o.processAIDecisions(ctx)
		log.Println("[AI] 🤖 Claude multi-agent trading loop ACTIVE")
	}

	// Background: Auto-fallback monitor for bridge failover
	go o.autoFallbackMonitor(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[MASTER LOOP] Gracefully halting execution loop...")
			return
		case t := <-ticks:
			o.processTickPipeline(ctx, t)
		}
	}
}

// processTickPipeline handles every raw tick:
// 1. Updates market state + position SL/TP
// 2. Feeds tick to candle aggregator (which emits candles on channels)
// 3. Runs ONLY tick-timeframe strategies on the raw tick
func (o *Orchestrator) processTickPipeline(ctx context.Context, t marketdata.Tick) {
	// 1. Update market state
	o.exec.UpdateMarketState(t.Price)
	o.mu.Lock()
	o.lastPrice = t.Price
	o.priceWindow = append(o.priceWindow, t.Price)
	if len(o.priceWindow) > marketHistoryMaxSamples {
		o.priceWindow = o.priceWindow[len(o.priceWindow)-marketHistoryMaxSamples:]
	}
	vol := t.Quantity
	if vol <= 0 {
		vol = 1
	}
	o.volumeWindow = append(o.volumeWindow, vol)
	if len(o.volumeWindow) > marketHistoryMaxSamples {
		o.volumeWindow = o.volumeWindow[len(o.volumeWindow)-marketHistoryMaxSamples:]
	}
	o.mu.Unlock()

	// 2. Check SL/TP/trailing on all open positions
	o.posMgr.CheckStopLossAndTakeProfit(t.Price)
	o.posMgr.CheckExpiredPositions(t.Price)

	// 3. Feed tick to candle aggregator (it emits 1m/5m candles on channels)
	o.candleAgg.Feed(t)

	// 4. Run ONLY tick-based strategies (OrderFlow, TickVelocity, VolumeSpike, GapFill)
	if len(o.groups.Tick) == 0 {
		return
	}
	o.processStrategyGroup(o.groups.Tick, t)
}

// process1mCandles listens for closed 1-minute candles and runs all 1m strategies.
func (o *Orchestrator) process1mCandles(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case candle := <-o.candleAgg.Candles1m:
			tick := candle.ToTick()
			log.Printf("[CANDLE 1m] Closed: O=%.2f H=%.2f L=%.2f C=%.2f Vol=%.4f Trades=%d",
				candle.Open, candle.High, candle.Low, candle.Close, candle.Volume, candle.Trades)
			o.processStrategyGroup(o.groups.M1, tick)
		}
	}
}

// process5mCandles listens for closed 5-minute candles and runs all 5m, 15m, and 1h strategies.
func (o *Orchestrator) process5mCandles(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case candle := <-o.candleAgg.Candles5m:
			tick := candle.ToTick()
			log.Printf("[CANDLE 5m] Closed: O=%.2f H=%.2f L=%.2f C=%.2f Vol=%.4f",
				candle.Open, candle.High, candle.Low, candle.Close, candle.Volume)
			o.processStrategyGroup(o.groups.M5, tick)

			// Simulate 15m candle: run 15m strategies every 3rd 5m candle.
			o.m15Counter++
			if o.m15Counter >= 3 {
				o.m15Counter = 0
				log.Println("[CANDLE 15m] Simulated 15m close — running 15m strategies")
				o.processStrategyGroup(o.groups.M15, tick)
			}

			// Simulate 1h candle: run 1h strategies every 12th 5m candle
			o.h1Counter++
			if o.h1Counter >= 12 {
				o.h1Counter = 0
				log.Println("[CANDLE 1h] Simulated 1h close — running hourly strategies")
				o.processStrategyGroup(o.groups.H1, tick)
			}

			// Record candle in history for AI context
			o.recordCandleHistory(candle)

			// Forward to AI channel (non-blocking — drop if AI is busy)
			if o.aiCandleCh != nil {
				select {
				case o.aiCandleCh <- candle:
				default:
					log.Println("[AI] Candle dropped — AI agent still processing previous candle")
				}
			}
		}
	}
}

// recordCandleHistory stores the last 20 × 5m candles for AI context.
func (o *Orchestrator) recordCandleHistory(candle marketdata.Candle) {
	o.candleHistMu.Lock()
	defer o.candleHistMu.Unlock()
	o.candleHistory = append(o.candleHistory, ai.CandleSummary{
		Open:   candle.Open,
		High:   candle.High,
		Low:    candle.Low,
		Close:  candle.Close,
		Volume: candle.Volume,
	})
	if len(o.candleHistory) > 20 {
		o.candleHistory = o.candleHistory[len(o.candleHistory)-20:]
	}
}

// processAIDecisions runs the Claude multi-agent debate on every 5m candle.
// This goroutine runs independently so it never blocks the main trading loop.
func (o *Orchestrator) processAIDecisions(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.aiCandleCh:
			o.runAIDecision(ctx)
		}
	}
}

// runAIDecision builds market context and calls the multi-agent orchestrator.
func (o *Orchestrator) runAIDecision(ctx context.Context) {
	o.mu.RLock()
	price := o.lastPrice
	prices := append([]float64(nil), o.priceWindow...)
	volumes := append([]float64(nil), o.volumeWindow...)
	o.mu.RUnlock()

	if price <= 0 || len(prices) < 20 {
		return
	}

	o.candleHistMu.Lock()
	candles := append([]ai.CandleSummary(nil), o.candleHistory...)
	o.candleHistMu.Unlock()

	// Compute indicators for AI context
	regime := o.classifyMarketRegime()
	rsi := computeRSI(prices, 14)
	atr := strategy.ATR(prices, 14)
	vwap := strategy.RollingVWAP(prices, volumes, 55)
	adx := strategy.ADX(prices, 14)
	emaFast := strategy.EMA(prices, 9)
	emaSlow := strategy.EMA(prices, 21)

	// Count open positions by direction
	openPos := o.posMgr.GetOpenPositions()
	longs, shorts := 0, 0
	for _, p := range openPos {
		if string(p.Side) == "BUY" {
			longs++
		} else {
			shorts++
		}
	}

	// Get account stats
	equityUSD := o.exec.GetEquityUSD()
	dailyPnL := o.risk.GetDailyPnL()

	market := ai.MarketContext{
		Symbol:         "BTC-USD",
		Price:          price,
		Regime:         regime,
		RSI:            rsi,
		ATR:            atr,
		VWAP:           vwap,
		ADX:            adx,
		EMAFast:        emaFast,
		EMASlow:        emaSlow,
		RecentCandles:  candles,
		OpenPositions:  len(openPos),
		LongPositions:  longs,
		ShortPositions: shorts,
		Balance:        equityUSD,
		DailyPnL:       dailyPnL,
	}

	decision := o.aiAgent.Decide(ctx, market)

	if !decision.RiskVerdict.Approved || decision.FinalAction == "HOLD" {
		log.Printf("[AI] 🤖 %s → HOLD | %s", decision.ID, decision.RiskVerdict.Reasoning)
		return
	}

	// Build a strategy.Signal from the AI decision and execute it
	riskSig := decision.RiskVerdict
	var activeSig AgentSignalForExec
	if decision.FinalAction == "BUY" {
		activeSig = AgentSignalForExec{
			action:        strategy.ActionBuy,
			size:          riskSig.AdjustedSize,
			stopLossPct:   decision.BullSignal.StopLossPct,
			takeProfitPct: decision.BullSignal.TakeProfitPct,
			confidence:    decision.BullSignal.Confidence,
		}
	} else {
		activeSig = AgentSignalForExec{
			action:        strategy.ActionSell,
			size:          riskSig.AdjustedSize,
			stopLossPct:   decision.BearSignal.StopLossPct,
			takeProfitPct: decision.BearSignal.TakeProfitPct,
			confidence:    decision.BearSignal.Confidence,
		}
	}

	// Sanitize size
	if activeSig.size < minExecutionSizeBTC {
		activeSig.size = minExecutionSizeBTC
	}
	if activeSig.size > 0.5 {
		activeSig.size = 0.5
	}

	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        activeSig.action,
		TargetSize:    activeSig.size,
		Confidence:    activeSig.confidence,
		StopLossPct:   activeSig.stopLossPct,
		TakeProfitPct: activeSig.takeProfitPct,
	}

	// Sanitize SL/TP
	sanitized, sanitizeReason, ok := sanitizeSignalForProfit(sig)
	if !ok {
		log.Printf("[AI] Signal sanitization failed — skipping: %s", sanitizeReason)
		return
	}
	sig = sanitized

	// Risk engine validation
	if err := o.risk.Validate(sig, price); err != nil {
		log.Printf("[AI] Risk engine rejected AI signal: %v", err)
		return
	}

	fill, err := o.exec.ExecuteSignal(sig, execution.OrderModeIOC)
	if err != nil {
		log.Printf("[AI] Execution failed: %v", err)
		return
	}

	o.risk.NotifyFill(sig)
	pos := o.posMgr.OpenPosition(sig, fill.ExecPrice, fmt.Sprintf("AI_%s", decision.ID))

	// Mark this decision as executed
	decision.Executed = true
	o.aiAgent.GetInsights().Add(decision)

	// Store AI reasoning in a special trade journal entry so it appears in history
	_ = pos
	log.Printf("[AI] ✅ %s EXECUTED %s %.4f BTC @ $%.2f | Bull: %s",
		decision.ID, decision.FinalAction, sig.TargetSize, fill.ExecPrice,
		truncate(decision.BullSignal.Thesis, 80))
}

// AgentSignalForExec holds execution parameters derived from the winning agent.
type AgentSignalForExec struct {
	action        strategy.Action
	size          float64
	stopLossPct   float64
	takeProfitPct float64
	confidence    float64
}

// computeRSI calculates RSI(n) from a price slice.
func computeRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50.0
	}
	gains, losses := 0.0, 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		delta := prices[i] - prices[i-1]
		if delta > 0 {
			gains += delta
		} else {
			losses -= delta
		}
	}
	if losses == 0 {
		return 100.0
	}
	rs := (gains / float64(period)) / (losses / float64(period))
	return 100 - (100 / (1 + rs))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// processStrategyGroup runs a group of strategies against a tick/candle and
// processes any resulting signals through aggregation, risk, and execution.
func (o *Orchestrator) processStrategyGroup(entries []strategy.RegistryEntry, t marketdata.Tick) {
	var wg sync.WaitGroup
	var sigMu sync.Mutex
	var rawSignals []AggregatedSignal

	for _, entry := range entries {
		wg.Add(1)
		go func(e strategy.RegistryEntry) {
			defer wg.Done()

			// Skip disabled strategies
			if !o.tracker.IsEnabled(e.Strategy.Name()) {
				return
			}

			signals := e.Strategy.OnTick(t)
			normalizedCategory := strategy.NormalizeCategory(e.Category, e.Strategy.Name())
			executionWeight := o.tracker.GetExecutionWeight(e.Strategy.Name())
			totalTrades := 0
			winRate := 0.5
			totalPnL := 0.0
			if stats, ok := o.tracker.GetStats(e.Strategy.Name()); ok {
				totalTrades = stats.TotalTrades
				totalPnL = stats.TotalPnL
				if stats.TotalTrades > 0 {
					winRate = float64(stats.Wins) / float64(stats.TotalTrades)
				}
			}

			for _, sig := range signals {
				if sig.Action == strategy.ActionHold {
					continue
				}
				sigMu.Lock()
				rawSignals = append(rawSignals, AggregatedSignal{
					Signal:          sig,
					StrategyName:    e.Strategy.Name(),
					Category:        normalizedCategory,
					ExecutionWeight: executionWeight,
					TotalTrades:     totalTrades,
					WinRate:         winRate,
					TotalPnL:        totalPnL,
				})
				sigMu.Unlock()
			}
		}(entry)
	}
	wg.Wait()

	// Aggregate and filter signals
	if len(rawSignals) == 0 {
		return
	}
	approved := o.aggregator.FilterSignalsSelective(rawSignals)
	regime := o.classifyMarketRegime()

	// Execute approved signals
	for _, aggSig := range approved {
		sig := aggSig.Signal

		// Record signal in tracker
		o.tracker.RecordSignal(aggSig.StrategyName)

		// Position limit check: prevent stacking too many positions per strategy
		if !o.posMgr.CanOpenPosition(aggSig.StrategyName) {
			log.Printf("[POSITION LIMIT] %s already at max positions — skipping", aggSig.StrategyName)
			continue
		}

		// Risk validation
		o.mu.RLock()
		currentPrice := o.lastPrice
		o.mu.RUnlock()

		if !isCategoryAlignedWithRegime(aggSig.Category, regime) {
			log.Printf("[REGIME FILTER] %s skipped in %s regime (%s category)",
				aggSig.StrategyName, regime, aggSig.Category)
			continue
		}

		// Dynamic sizing: reward stable winners, reduce weak performers.
		baseSize := sig.TargetSize
		sizeMultiplier := o.tracker.GetSizingMultiplier(aggSig.StrategyName)
		executionWeight := o.tracker.GetExecutionWeight(aggSig.StrategyName)
		if executionWeight < minExecutionWeightToTrade {
			log.Printf("[QUALITY FILTER] %s skipped due to weak execution weight %.2f",
				aggSig.StrategyName, executionWeight)
			continue
		}
		sig.TargetSize = baseSize * sizeMultiplier * executionWeight
		sig.Confidence = adjustConfidenceByExecutionWeight(sig.Confidence, executionWeight)

		// Capital cap: keep each strategy within a fraction of its allocation bucket.
		if currentPrice > 0 {
			if stats, ok := o.tracker.GetStats(aggSig.StrategyName); ok && stats.Allocation > 0 {
				maxSizeByAllocation := (stats.Allocation * maxAllocationUsage) / currentPrice
				if maxSizeByAllocation > 0 && sig.TargetSize > maxSizeByAllocation {
					sig.TargetSize = maxSizeByAllocation
				}
			}
		}

		if sig.TargetSize < minExecutionSizeBTC {
			log.Printf("[SIZE ENGINE] %s size too small after scaling (%.6f BTC) — skipping",
				aggSig.StrategyName, sig.TargetSize)
			continue
		}

		if sig.TargetSize-baseSize > sizeChangeEpsilonBTC || baseSize-sig.TargetSize > sizeChangeEpsilonBTC {
			log.Printf("[SIZE ENGINE] %s resized %.4f -> %.4f BTC (size x%.2f, quality x%.2f)",
				aggSig.StrategyName, baseSize, sig.TargetSize, sizeMultiplier, executionWeight)
		}

		baseStopLossPct := sig.StopLossPct
		baseTakeProfitPct := sig.TakeProfitPct
		sanitizedSig, sanitizeReason, allowed := sanitizeSignalForProfit(sig)
		if !allowed {
			log.Printf("[PROFIT FILTER] %s dropped: %s",
				aggSig.StrategyName, sanitizeReason)
			continue
		}
		sig = sanitizedSig
		if sig.StopLossPct != baseStopLossPct || sig.TakeProfitPct != baseTakeProfitPct {
			log.Printf("[PROFIT FILTER] %s adjusted SL/TP %.2f%%/%.2f%% -> %.2f%%/%.2f%%",
				aggSig.StrategyName, baseStopLossPct, baseTakeProfitPct, sig.StopLossPct, sig.TakeProfitPct)
		}

		err := o.risk.Validate(sig, currentPrice)
		if err != nil {
			log.Printf("[RISK DROPPED] %s from %s: %s", sig.Action, aggSig.StrategyName, err.Error())
			continue
		}

		orderMode := execution.RouteModeForCategory(aggSig.Category, regime)

		// ══════════════════════════════════════════════════════════════════════
		// AI SIGNAL AUDIT — GPT-4o/Gemini/Groq Veto Layer
		// ══════════════════════════════════════════════════════════════════════
		if (o.aiAgent != nil && o.aiAgent.IsAvailable()) || o.IsBridgeOnline() {
			// Build market context for the audit
			o.mu.RLock()
			prices := append([]float64(nil), o.priceWindow...)
			volumes := append([]float64(nil), o.volumeWindow...)
			o.mu.RUnlock()

			o.candleHistMu.Lock()
			candles := append([]ai.CandleSummary(nil), o.candleHistory...)
			o.candleHistMu.Unlock()

			market := ai.MarketContext{
				Symbol:        sig.Symbol,
				Price:         currentPrice,
				Regime:        regime,
				RSI:           computeRSI(prices, 14),
				ATR:           strategy.ATR(prices, 14),
				VWAP:          strategy.RollingVWAP(prices, volumes, 55),
				ADX:           strategy.ADX(prices, 14),
				EMAFast:       strategy.EMA(prices, 9),
				EMASlow:       strategy.EMA(prices, 21),
				RecentCandles: candles,
				OpenPositions: len(o.posMgr.GetOpenPositions()),
				Balance:       o.exec.GetEquityUSD(),
				DailyPnL:      o.risk.GetDailyPnL(),
			}

			// ══════════════════════════════════════════════════════════════════════
			// INTERACTIVE MODE: Park the signal for Dashabord Command Center
			// ══════════════════════════════════════════════════════════════════════
			pendingID := fmt.Sprintf("SIG-%d", time.Now().UnixNano()/1e6)

			o.pendingMu.Lock()
			o.pendingSignals[pendingID] = PendingSignal{
				ID:           pendingID,
				Signal:       sig,
				StrategyName: aggSig.StrategyName,
				Category:     aggSig.Category,
				Context:      market,
				AutoPrompt:   generateAutoPrompt(market, aggSig.StrategyName, string(sig.Action)),
				CreatedAt:    time.Now(),
			}
			o.pendingMu.Unlock()

			log.Printf("[COMMAND CENTER] 🛰️  Signal Parked: %s %s [%s] -> Waiting for UI submission",
				aggSig.StrategyName, sig.Action, pendingID)
			continue
		}

		// ══════════════════════════════════════════════════════════════════════
		// 13. EXECUTION — Fill via Coinbase Advanced Trad (Live/Paper)
		// ══════════════════════════════════════════════════════════════════════
		fill, err := o.exec.ExecuteSignal(sig, orderMode)
		if err != nil {
			log.Printf("[EXECUTION FAILED] %s from %s: %s", sig.Action, aggSig.StrategyName, err.Error())
			continue
		}
		execPrice := fill.ExecPrice

		// Notify risk engine
		o.risk.NotifyFill(sig)

		// Open tracked position with SL/TP
		o.posMgr.OpenPosition(sig, execPrice, aggSig.StrategyName)

		log.Printf("[EXECUTION ROUTE] %s used %s in %s regime", aggSig.StrategyName, fill.OrderMode, regime)

		log.Printf("[✅ TRADE EXECUTED] %s | %s %.4f BTC @ $%.2f | Strategy: %s",
			sig.Action, sig.Symbol, sig.TargetSize, execPrice, aggSig.StrategyName)
	}
}

// processCloseEvents listens for position close events (SL/TP hits) and records them.
func (o *Orchestrator) processCloseEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-o.posMgr.CloseEvents:
			// ═══ FIX: Settle paper balance (credit USD back) ═══
			// Without this, every BUY drains the balance permanently
			// because no SELL ever executes to return the USD.
			o.exec.SettlePosition(event.Position.Side, event.Position.Size, event.ExitPrice)
			netPnL := execution.CalculateNetPnL(
				event.PnL,
				event.Position.EntryPrice,
				event.ExitPrice,
				event.Position.Size,
			)

			// Record in trade journal
			entry := execution.JournalEntry{
				ID:           event.Position.ID,
				StrategyName: event.Position.StrategyName,
				Side:         string(event.Position.Side),
				EntryPrice:   event.Position.EntryPrice,
				ExitPrice:    event.ExitPrice,
				Size:         event.Position.Size,
				GrossPnL:     event.PnL,
				Reason:       string(event.Reason),
				EntryTime:    event.Position.OpenedAt,
				ExitTime:     time.Now(),
			}
			o.journal.RecordTrade(entry)

			// Update strategy tracker
			o.tracker.RecordTradeResult(event.Position.StrategyName, netPnL)

			// Update risk engine daily PnL tracker
			o.risk.RecordPnL(netPnL)

			// Update risk engine (reduce exposure)
			closeSig := strategy.Signal{
				Symbol:     event.Position.Symbol,
				Action:     strategy.ActionSell,
				TargetSize: event.Position.Size,
			}
			if event.Position.Side == strategy.ActionSell {
				closeSig.Action = strategy.ActionBuy
			}
			o.risk.NotifyFill(closeSig)

			log.Printf("[✅ TRADE CLOSED] %s | %s | Entry: $%.2f → Exit: $%.2f | PnL: $%.4f | Reason: %s",
				event.Position.StrategyName, event.Position.Side,
				event.Position.EntryPrice, event.ExitPrice, event.PnL, event.Reason)
		}
	}
}

// strategyCooldownChecker periodically re-enables strategies that have cooled down.
func (o *Orchestrator) strategyCooldownChecker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.tracker.ReEnableExpired()
		}
	}
}

func (o *Orchestrator) RecordBridgeHeartbeat() {
	o.bridgeHeartbeatMu.Lock()
	defer o.bridgeHeartbeatMu.Unlock()
	o.lastBridgeHeartbeat = time.Now()
}

func (o *Orchestrator) RecordBridgeEvent(event, level string) {
	now := time.Now()
	o.bridgeStateMu.Lock()
	defer o.bridgeStateMu.Unlock()
	o.lastBridgeEvent = strings.TrimSpace(event)
	o.lastBridgeEventAt = now
	if strings.EqualFold(level, "error") {
		o.lastBridgeError = strings.TrimSpace(event)
		o.lastBridgeErrorAt = now
	}
}

func (o *Orchestrator) autoFallbackMonitor(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.bridgeHeartbeatMu.RLock()
			bridgeOffline := time.Since(o.lastBridgeHeartbeat) > 15*time.Second
			o.bridgeHeartbeatMu.RUnlock()

			if bridgeOffline {
				o.pendingMu.RLock()
				// Collect IDs to process to avoid holding lock during AI call
				var toProcess []string
				now := time.Now()
				for id, p := range o.pendingSignals {
					if now.Sub(p.CreatedAt) > 45*time.Second {
						toProcess = append(toProcess, id)
					}
				}
				o.pendingMu.RUnlock()

				if len(toProcess) > 0 && !o.hasBackendAIFallback() {
					age := o.bridgeAge().Round(time.Second)
					msg := fmt.Sprintf("Bridge offline for %s and no backend AI fallback is configured; keeping %d parked signal(s) queued", age, len(toProcess))
					log.Printf("[FAILOVER] %s", msg)
					o.RecordBridgeEvent(msg, "warn")
					continue
				}

				for _, id := range toProcess {
					log.Printf("[FAILOVER] 🔄 Bridge Offline (Last seen %s ago). Triggering Auto-Cloud Fallback for %s",
						o.bridgeAge().Round(time.Second), id)
					go func(sigID string) {
						if err := o.ConfirmSignal(ctx, sigID, "AUTOMATIC_CLOUD_FALLBACK"); err != nil {
							log.Printf("[FAILOVER] Auto fallback failed for %s: %v", sigID, err)
						}
					}(id)
				}
			}
		}
	}
}

// GetLastPrice returns the latest BTC price (for API endpoints).
// GetPendingSignals returns the list of signals currently parked in the Command Center.
func (o *Orchestrator) GetPendingSignals() []PendingSignal {
	now := time.Now()

	// Collect stale IDs under read lock first
	o.pendingMu.RLock()
	var stale []string
	res := make([]PendingSignal, 0, len(o.pendingSignals))
	for id, p := range o.pendingSignals {
		if now.Sub(p.CreatedAt) > 5*time.Minute {
			stale = append(stale, id)
		} else {
			res = append(res, p)
		}
	}
	o.pendingMu.RUnlock()

	// Promote to write lock only when there is something to delete
	if len(stale) > 0 {
		o.pendingMu.Lock()
		for _, id := range stale {
			delete(o.pendingSignals, id)
		}
		o.pendingMu.Unlock()
	}

	return res
}

// AddTestSignal - INJECTS a fake signal for local testing of the Robot
func (o *Orchestrator) AddTestSignal() {
	o.pendingMu.Lock()
	defer o.pendingMu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("test_%d", now.UnixNano())
	ctx := ai.MarketContext{
		Symbol:        "BTC-USD",
		Price:         70000,
		Regime:        marketRegimeTrend,
		RSI:           61.5,
		ADX:           28.0,
		VWAP:          69880,
		ATR:           245,
		RecentCandles: []ai.CandleSummary{{High: 70120, Low: 69780, Close: 70040, Volume: 182.4}},
	}
	o.pendingSignals[id] = PendingSignal{
		ID:           id,
		StrategyName: "RAIG_COMBAT_SIMULATOR",
		Signal: strategy.Signal{
			Symbol:        "BTC-USD",
			Action:        strategy.ActionBuy,
			TargetSize:    0.02,
			Confidence:    0.92,
			StopLossPct:   0.45,
			TakeProfitPct: 0.90,
		},
		Context:    ctx,
		CreatedAt:  now,
		AutoPrompt: generateAutoPrompt(ctx, "RAIG_COMBAT_SIMULATOR", string(strategy.ActionBuy)),
	}
	log.Println("[RAIG] TEST SIGNAL INJECTED INTO COMMAND CENTER.")
}

// ConfirmSignal triggers the AI Audit for a parked signal and executes if approved.
func (o *Orchestrator) ConfirmSignal(ctx context.Context, pendingID, userPrompt string) error {
	if !o.hasBackendAIFallback() {
		return fmt.Errorf("backend AI fallback unavailable: configure an AI provider or keep the browser bridge online")
	}

	o.pendingMu.Lock()
	p, ok := o.pendingSignals[pendingID]
	if !ok {
		o.pendingMu.Unlock()
		return fmt.Errorf("signal %s not found or expired", pendingID)
	}
	delete(o.pendingSignals, pendingID)
	o.pendingMu.Unlock()

	// 1. Final AI Audit via Supreme Court (including User Feedback)
	log.Printf("[COMMAND CENTER] 🧠 Submitting Signal %s to ChatGPT for final audit (Note: %s)", pendingID, userPrompt)

	// We pass the userPrompt as the human feedback to the AI
	approved, reason, _, provider := o.aiAgent.AuditSignalWithFallback(ctx, p.Context, p.StrategyName, string(p.Signal.Action), userPrompt)

	if !approved {
		log.Printf("[COMMAND CENTER] ⛔ ChatGPT REJECTED signal: %s", reason)
		return fmt.Errorf("AI Rejection: %s", reason)
	}

	// 2. Execution
	log.Printf("[COMMAND CENTER] ✅ ChatGPT APPROVED signal! Executing...")

	p.Signal.AIDecisionID = provider
	p.Signal.AIReasoning = fmt.Sprintf("[Human Input: %s] %s", userPrompt, reason)

	fill, err := o.exec.ExecuteSignal(p.Signal, execution.OrderModeIOC)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	// 3. Notify sub-systems
	o.risk.NotifyFill(p.Signal)
	o.posMgr.OpenPosition(p.Signal, fill.ExecPrice, p.StrategyName)

	log.Printf("[✅ TRADE EXECUTED] %s %s APPROVED via Command Center!", p.StrategyName, p.Signal.Action)
	return nil
}

// ConfirmSignalFromBridge executes a parked signal based on a browser-automation verdict.
func (o *Orchestrator) ConfirmSignalFromBridge(ctx context.Context, pendingID string, decision BridgeDecision) error {
	if err := validateBridgeDecision(decision); err != nil {
		return fmt.Errorf("invalid bridge decision: %w", err)
	}
	if !o.markBridgeSignalProcessing(pendingID) {
		return fmt.Errorf("duplicate bridge result for %s", pendingID)
	}

	o.pendingMu.Lock()
	p, ok := o.pendingSignals[pendingID]
	if !ok {
		o.pendingMu.Unlock()
		return fmt.Errorf("signal %s not found or expired", pendingID)
	}
	delete(o.pendingSignals, pendingID)
	o.pendingMu.Unlock()

	if !decision.Approved {
		log.Printf("[COMMAND CENTER] Browser bridge rejected %s: %s", pendingID, decision.Reason)
		return fmt.Errorf("bridge rejected signal: %s", decision.Reason)
	}

	action := strings.ToUpper(strings.TrimSpace(decision.Action))
	if action != string(strategy.ActionBuy) && action != string(strategy.ActionSell) {
		action = string(p.Signal.Action)
	}
	if action != string(p.Signal.Action) {
		log.Printf("[COMMAND CENTER] Browser bridge action mismatch for %s: wanted %s got %s",
			pendingID, p.Signal.Action, action)
		return fmt.Errorf("bridge action mismatch: expected %s got %s", p.Signal.Action, action)
	}

	if decision.Confidence > 0 {
		if decision.Confidence < minBridgeApprovalConfidence {
			log.Printf("[COMMAND CENTER] ⛔ Bridge signal %s blocked: ChatGPT confidence %.2f below minimum %.2f",
				pendingID, decision.Confidence, minBridgeApprovalConfidence)
			return fmt.Errorf("bridge confidence %.2f below required %.2f", decision.Confidence, minBridgeApprovalConfidence)
		}
		p.Signal.Confidence = decision.Confidence
	}
	sanitized, reason, allowed := sanitizeSignalForProfit(p.Signal)
	if !allowed {
		log.Printf("[COMMAND CENTER] ⛔ Bridge signal %s blocked by profit filter: %s (conf=%.2f)",
			pendingID, reason, p.Signal.Confidence)
		return fmt.Errorf("bridge signal blocked: %s", reason)
	}
	p.Signal = sanitized

	if err := o.risk.Validate(p.Signal, p.Context.Price); err != nil {
		return fmt.Errorf("risk rejected bridge signal: %w", err)
	}

	p.Signal.AIDecisionID = "browser-bridge"
	p.Signal.AIReasoning = strings.TrimSpace(fmt.Sprintf("[Browser Bridge] %s", decision.Reason))

	fill, err := o.exec.ExecuteSignal(p.Signal, execution.OrderModeIOC)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	o.risk.NotifyFill(p.Signal)
	o.posMgr.OpenPosition(p.Signal, fill.ExecPrice, p.StrategyName)

	log.Printf("[✅ TRADE EXECUTED] %s %s APPROVED via Browser Bridge | conf=%.2f | reason=%s",
		p.StrategyName, p.Signal.Action, p.Signal.Confidence, truncate(decision.Reason, 120))
	return nil
}

func validateBridgeDecision(decision BridgeDecision) error {
	if strings.TrimSpace(decision.Reason) == "" {
		return fmt.Errorf("missing reason")
	}
	if decision.Confidence < 0 || decision.Confidence > 1 {
		return fmt.Errorf("confidence %.4f out of range", decision.Confidence)
	}
	// Only validate action when the bridge is actually approving a trade
	if decision.Approved {
		action := strings.ToUpper(strings.TrimSpace(decision.Action))
		if action != string(strategy.ActionBuy) && action != string(strategy.ActionSell) {
			return fmt.Errorf("unsupported action %q", decision.Action)
		}
	}
	return nil
}

func (o *Orchestrator) markBridgeSignalProcessing(signalID string) bool {
	o.processedBridgeMu.Lock()
	defer o.processedBridgeMu.Unlock()

	now := time.Now()
	for id, ts := range o.processedBridgeSignals {
		if now.Sub(ts) > 10*time.Minute {
			delete(o.processedBridgeSignals, id)
		}
	}
	if _, exists := o.processedBridgeSignals[signalID]; exists {
		return false
	}
	o.processedBridgeSignals[signalID] = now
	return true
}

func (o *Orchestrator) hasBackendAIFallback() bool {
	return o.aiAgent != nil && o.aiAgent.IsAvailable()
}

func (o *Orchestrator) bridgeAge() time.Duration {
	o.bridgeHeartbeatMu.RLock()
	defer o.bridgeHeartbeatMu.RUnlock()
	return time.Since(o.lastBridgeHeartbeat)
}

func (o *Orchestrator) IsBridgeOnline() bool {
	o.bridgeHeartbeatMu.RLock()
	defer o.bridgeHeartbeatMu.RUnlock()
	return time.Since(o.lastBridgeHeartbeat) < 15*time.Second
}

func (o *Orchestrator) GetBridgeStatus() BridgeStatus {
	o.bridgeHeartbeatMu.RLock()
	lastHeartbeat := o.lastBridgeHeartbeat
	secondsSinceBeat := int(time.Since(lastHeartbeat).Seconds())
	online := time.Since(lastHeartbeat) < 15*time.Second
	o.bridgeHeartbeatMu.RUnlock()

	o.pendingMu.RLock()
	pendingCount := len(o.pendingSignals)
	o.pendingMu.RUnlock()

	o.processedBridgeMu.Lock()
	processedCount := len(o.processedBridgeSignals)
	o.processedBridgeMu.Unlock()

	o.bridgeStateMu.RLock()
	lastEvent := o.lastBridgeEvent
	lastEventAt := o.lastBridgeEventAt
	lastError := o.lastBridgeError
	lastErrorAt := o.lastBridgeErrorAt
	o.bridgeStateMu.RUnlock()

	if secondsSinceBeat < 0 {
		secondsSinceBeat = 0
	}

	return BridgeStatus{
		Online:              online,
		LastHeartbeat:       lastHeartbeat,
		SecondsSinceBeat:    secondsSinceBeat,
		PendingSignals:      pendingCount,
		ProcessedSignalKeys: processedCount,
		LastEvent:           lastEvent,
		LastEventAt:         lastEventAt,
		LastError:           lastError,
		LastErrorAt:         lastErrorAt,
	}
}

func (o *Orchestrator) GetLastPrice() float64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.lastPrice
}

func sanitizeSignalForProfit(sig strategy.Signal) (strategy.Signal, string, bool) {
	adjusted := sig

	if adjusted.Confidence == 0 {
		adjusted.Confidence = 1.0
	}
	if adjusted.Confidence < minExecutableConfidence {
		return adjusted, fmt.Sprintf("confidence %.2f below minimum %.2f", adjusted.Confidence, minExecutableConfidence), false
	}

	if adjusted.StopLossPct <= 0 {
		adjusted.StopLossPct = defaultSignalStopLossPct
	}
	if adjusted.StopLossPct > maxSignalStopLossPct {
		adjusted.StopLossPct = maxSignalStopLossPct
	}

	if adjusted.TakeProfitPct <= 0 {
		adjusted.TakeProfitPct = minSignalTakeProfitPct
	}

	minTakeProfitByRR := adjusted.StopLossPct * minRewardToRiskRatio
	if adjusted.TakeProfitPct < minTakeProfitByRR {
		adjusted.TakeProfitPct = minTakeProfitByRR
	}
	if adjusted.TakeProfitPct < minSignalTakeProfitPct {
		adjusted.TakeProfitPct = minSignalTakeProfitPct
	}

	return adjusted, "", true
}

func adjustConfidenceByExecutionWeight(confidence, executionWeight float64) float64 {
	adjusted := confidence
	if adjusted == 0 {
		adjusted = 1.0
	}

	if executionWeight < 1 {
		adjusted *= 0.80 + 0.20*executionWeight
	} else {
		adjusted *= 1.0 + (executionWeight-1.0)*0.25
	}

	if adjusted > 1.5 {
		return 1.5
	}
	if adjusted < 0 {
		return 0
	}
	return adjusted
}

func (o *Orchestrator) classifyMarketRegime() string {
	o.mu.RLock()
	if len(o.priceWindow) < 80 || len(o.volumeWindow) < 80 {
		o.mu.RUnlock()
		return marketRegimeUnknown
	}
	prices := append([]float64(nil), o.priceWindow...)
	volumes := append([]float64(nil), o.volumeWindow...)
	o.mu.RUnlock()

	latestPrice := prices[len(prices)-1]
	fast := strategy.EMA(prices, 21)
	slow := strategy.EMA(prices, 55)
	adx := strategy.ADX(prices, 14)
	vwap := strategy.RollingVWAP(prices, volumes, 55)
	atrFast := strategy.ATR(prices, 14)
	atrSlow := strategy.ATR(prices, 55)
	if atrSlow <= 0 || vwap <= 0 {
		return marketRegimeUnknown
	}

	trendStrength := math.Abs(fast-slow) / (atrSlow * 3.0)
	volRatio := atrFast / atrSlow
	priceVsVWAPPct := math.Abs((latestPrice - vwap) / vwap * 100)
	trendAlignedWithVWAP := (latestPrice >= vwap && fast >= slow) || (latestPrice <= vwap && fast <= slow)

	switch {
	case adx >= 25 && trendStrength >= 0.55 && trendAlignedWithVWAP:
		return marketRegimeTrend
	case volRatio >= 1.45 && adx < 25 && trendStrength < 0.70:
		return marketRegimeVolatile
	case adx <= 20 && trendStrength <= 0.40 && volRatio <= 1.10 && priceVsVWAPPct <= 0.18:
		return marketRegimeRange
	default:
		return marketRegimeMixed
	}
}

func isCategoryAlignedWithRegime(category, regime string) bool {
	switch regime {
	case marketRegimeUnknown:
		// Never trade when market structure is unclassified — too much noise
		return false
	case marketRegimeMixed:
		// Only highest-conviction multi-signal strategies trade in mixed conditions
		switch category {
		case "Multi-Signal", "Trend", "Trend Elite", "Intraday":
			return true
		}
		return false
	case marketRegimeTrend:
		switch category {
		case "Trend", "Trend Elite", "Breakout", "Breakout Elite", "Momentum", "Momentum Elite",
			"Time-of-Day", "Microstructure", "Multi-Signal", "Price Action", "Price Action Elite", "Intraday":
			return true
		}
		return false
	case marketRegimeRange:
		switch category {
		case "Mean Reversion", "Mean Rev Elite", "Statistical", "Adaptive", "Adaptive Elite",
			"Oscillator Elite", "Price Action", "Price Action Elite", "Multi-Signal", "Intraday":
			return true
		}
		return false
	case marketRegimeVolatile:
		switch category {
		case "Volatility", "Volatility Elite", "Breakout", "Breakout Elite", "Microstructure",
			"Time-of-Day", "Multi-Signal", "Momentum Elite", "Intraday":
			return true
		}
		return false
	default:
		return true
	}
}

func generateAutoPrompt(ctx ai.MarketContext, name, action string) string {
	return fmt.Sprintf(`You are reviewing a BTC trading signal for execution safety.

Return ONLY valid JSON with this schema:
{
  "approved": true,
  "action": "%s",
  "confidence": 0.0,
  "reason": "short reason"
}

Rules:
- Keep "action" exactly "%s" if approved.
- Set "approved" false if this trade should be vetoed.
- Confidence must be a number between 0.0 and 1.0.
- Do not include markdown fences.

### BITCOIN TRADING SIGNAL AUDIT ###
STRATEGY: %s
ACTION: %s
PRICE: $%.2f
RSI: %.1f
ADX: %.1f
VWAP: $%.2f
ATR: $%.2f

### RECENT 5M CANDLES ###
(Oldest to Newest)
%s

### INSTRUCTION ###
Analyze the data above and return the JSON decision now.`,
		action, action, name, action, ctx.Price, ctx.RSI, ctx.ADX, ctx.VWAP, ctx.ATR,
		buildCandleHistoryText(ctx))
}

func buildCandleHistoryText(ctx ai.MarketContext) string {
	var sb strings.Builder
	for i, c := range ctx.RecentCandles {
		sb.WriteString(fmt.Sprintf("[%d] H:%.0f L:%.0f C:%.0f V:%.2f\n", i, c.High, c.Low, c.Close, c.Volume))
	}
	return sb.String()
}
