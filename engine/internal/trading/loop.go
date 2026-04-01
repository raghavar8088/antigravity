package trading

import (
	"context"
	"fmt"
	"log"
	"math"
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
	minExecutionSizeBTC  = 0.001
	maxAllocationUsage   = 0.60
	sizeChangeEpsilonBTC = 1e-9

	minExecutableConfidence  = 0.80 // Lowered: allow slightly less certain signals (was 0.85)
	minRewardToRiskRatio     = 1.50 // Raised: require better reward vs risk (was 1.35)
	minSignalTakeProfitPct   = 0.55 // Raised: ensure TP is worth chasing after slippage (was 0.45)
	maxSignalStopLossPct     = 0.80 // Lowered: tighter max SL, keep losses small (was 1.20)
	defaultSignalStopLossPct = 0.50 // RAISED: widened from 0.20 to prevent whipsaws

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

	// Internal state
	lastPrice    float64
	h1Counter    int // Counts 5m candles to simulate 1h (every 12th)
	priceWindow  []float64
	volumeWindow []float64
	mu           sync.RWMutex
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
	log.Printf("[ORCHESTRATOR] Strategy groups: %d tick, %d 1m, %d 5m, %d 1h",
		len(groups.Tick), len(groups.M1), len(groups.M5), len(groups.H1))

	return &Orchestrator{
		client:       c,
		strategies:   strats,
		groups:       groups,
		risk:         r,
		exec:         e,
		aggregator:   agg,
		posMgr:       pm,
		tracker:      tracker,
		journal:      journal,
		candleAgg:    candleAgg,
		priceWindow:  make([]float64, 0, marketHistoryMaxSamples),
		volumeWindow: make([]float64, 0, marketHistoryMaxSamples),
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
		// Also feed to 1h strategies (they use candle data too)
		for _, entry := range o.groups.H1 {
			entry.Strategy.OnTick(tick)
		}
	}

	log.Printf("[WARMUP] Feeding %d historical 5m candles to %d strategies...",
		len(warmup.Candles5m), len(o.groups.M5))

	// Feed 5m candles to 5m strategies
	for _, candle := range warmup.Candles5m {
		tick := candle.ToTick()
		for _, entry := range o.groups.M5 {
			entry.Strategy.OnTick(tick)
		}
	}

	log.Println("[WARMUP] ✅ All strategy buffers pre-filled. Ready for live trading.")
}

// Run is the infinite heartbeat of Antigravity Live Trading.
// It processes ticks and candles through their respective strategy groups.
func (o *Orchestrator) Run(ctx context.Context) {
	log.Printf("[MASTER LOOP] Booting Multi-Strategy Orchestrator with %d strategies...", len(o.strategies))
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

// process5mCandles listens for closed 5-minute candles and runs all 5m + 1h strategies.
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
		Symbol:            "BTC-USD",
		Price:             price,
		Regime:            regime,
		RSI:               rsi,
		ATR:               atr,
		VWAP:              vwap,
		ADX:               adx,
		EMAFast:           emaFast,
		EMASlow:           emaSlow,
		RecentCandles:     candles,
		OpenPositions:     len(openPos),
		LongPositions:     longs,
		ShortPositions:    shorts,
		Balance:           equityUSD,
		DailyPnL:          dailyPnL,
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
	if activeSig.size > 0.05 {
		activeSig.size = 0.05
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
	sanitized, ok := sanitizeSignalForProfit(sig)
	if !ok {
		log.Printf("[AI] Signal sanitization failed — skipping")
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

			for _, sig := range signals {
				if sig.Action == strategy.ActionHold {
					continue
				}
				sigMu.Lock()
				rawSignals = append(rawSignals, AggregatedSignal{
					Signal:       sig,
					StrategyName: e.Strategy.Name(),
					Category:     e.Category,
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
		sanitizedSig, allowed := sanitizeSignalForProfit(sig)
		if !allowed {
			log.Printf("[PROFIT FILTER] %s dropped due to low confidence %.2f",
				aggSig.StrategyName, sig.Confidence)
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
		// AI SIGNAL AUDIT — GPT-4o/Gemini Veto Layer
		// ══════════════════════════════════════════════════════════════════════
		if o.aiAgent != nil && o.aiAgent.IsAvailable() {
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

			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			approved, reason, _ := o.aiAgent.AuditSignal(ctx, market, aggSig.StrategyName, string(sig.Action))
			cancel()

			if !approved {
				log.Printf("[AI AUDIT VETO] %s %s REJECTED: %s", aggSig.StrategyName, sig.Action, reason)
				continue
			}
			log.Printf("[AI AUDIT PASS] %s %s APPROVED: %s", aggSig.StrategyName, sig.Action, reason)
		}

		// Execute
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

// GetLastPrice returns the latest BTC price (for API endpoints).
func (o *Orchestrator) GetLastPrice() float64 {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.lastPrice
}

func sanitizeSignalForProfit(sig strategy.Signal) (strategy.Signal, bool) {
	adjusted := sig

	if adjusted.Confidence == 0 {
		adjusted.Confidence = 1.0
	}
	if adjusted.Confidence < minExecutableConfidence {
		return adjusted, false
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

	return adjusted, true
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
	case marketRegimeUnknown, marketRegimeMixed:
		return true
	case marketRegimeTrend:
		switch category {
		case "Trend", "Trend Elite", "Breakout", "Breakout Elite", "Momentum", "Momentum Elite",
			"Time-of-Day", "Microstructure", "Multi-Signal", "Price Action", "Price Action Elite":
			return true
		}
		return false
	case marketRegimeRange:
		switch category {
		case "Mean Reversion", "Mean Rev Elite", "Statistical", "Adaptive", "Adaptive Elite",
			"Oscillator Elite", "Price Action", "Price Action Elite", "Multi-Signal":
			return true
		}
		return false
	case marketRegimeVolatile:
		switch category {
		case "Volatility", "Volatility Elite", "Breakout", "Breakout Elite", "Microstructure",
			"Time-of-Day", "Multi-Signal", "Momentum Elite":
			return true
		}
		return false
	default:
		return true
	}
}
