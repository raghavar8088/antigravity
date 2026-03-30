package trading

import (
	"context"
	"log"
	"sync"
	"time"

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

	minExecutableConfidence  = 0.85
	minRewardToRiskRatio     = 1.35
	minSignalTakeProfitPct   = 0.45
	maxSignalStopLossPct     = 1.20
	defaultSignalStopLossPct = 0.30
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

	// Internal state
	lastPrice float64
	h1Counter int // Counts 5m candles to simulate 1h (every 12th)
	mu        sync.RWMutex
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
		client:     c,
		strategies: strats,
		groups:     groups,
		risk:       r,
		exec:       e,
		aggregator: agg,
		posMgr:     pm,
		tracker:    tracker,
		journal:    journal,
		candleAgg:  candleAgg,
	}
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
	o.mu.Unlock()

	// 2. Check SL/TP/trailing on all open positions
	o.posMgr.CheckStopLossAndTakeProfit(t.Price)

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
		}
	}
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

		// Dynamic sizing: reward stable winners, reduce weak performers.
		baseSize := sig.TargetSize
		sizeMultiplier := o.tracker.GetSizingMultiplier(aggSig.StrategyName)
		sig.TargetSize = baseSize * sizeMultiplier

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
			log.Printf("[SIZE ENGINE] %s resized %.4f -> %.4f BTC (x%.2f)",
				aggSig.StrategyName, baseSize, sig.TargetSize, sizeMultiplier)
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

		// Apply slippage (0.01% adverse)
		execPrice := currentPrice
		if sig.Action == strategy.ActionBuy {
			execPrice = currentPrice * 1.0001 // Buy slightly higher
		} else {
			execPrice = currentPrice * 0.9999 // Sell slightly lower
		}

		// Execute
		err = o.exec.PlaceMarketOrder(sig)
		if err != nil {
			log.Printf("[EXECUTION FAILED] %s from %s: %s", sig.Action, aggSig.StrategyName, err.Error())
			continue
		}

		// Notify risk engine
		o.risk.NotifyFill(sig)

		// Open tracked position with SL/TP
		o.posMgr.OpenPosition(sig, execPrice, aggSig.StrategyName)

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
