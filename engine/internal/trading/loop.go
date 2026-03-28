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

// Orchestrator is the multi-strategy parallel trading engine.
// It fans out every tick to all 40 strategies, collects signals,
// filters through aggregation, validates via risk, and executes.
type Orchestrator struct {
	client     marketdata.MarketDataClient
	strategies []strategy.RegistryEntry
	risk       *risk.RiskEngine
	exec       *execution.PaperClient
	aggregator *SignalAggregator
	posMgr     *positions.Manager
	tracker    *risk.StrategyTracker
	journal    *execution.TradeJournal

	// Internal state
	lastPrice float64
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
) *Orchestrator {
	return &Orchestrator{
		client:     c,
		strategies: strats,
		risk:       r,
		exec:       e,
		aggregator: agg,
		posMgr:     pm,
		tracker:    tracker,
		journal:    journal,
	}
}

// Run is the infinite heartbeat of Antigravity Live Trading.
// It processes every incoming tick through all 40 strategies in parallel.
func (o *Orchestrator) Run(ctx context.Context) {
	log.Printf("[MASTER LOOP] Booting Multi-Strategy Orchestrator with %d strategies...", len(o.strategies))
	ticks := o.client.GetTickChannel()

	// Background: process position close events (SL/TP/trailing)
	go o.processCloseEvents(ctx)

	// Background: re-enable cooled-down strategies every minute
	go o.strategyCooldownChecker(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[MASTER LOOP] Gracefully halting execution loop...")
			return
		case t := <-ticks:
			o.processTick(ctx, t)
		}
	}
}

func (o *Orchestrator) processTick(ctx context.Context, t marketdata.Tick) {
	// 1. Update market state
	o.exec.UpdateMarketState(t.Price)
	o.mu.Lock()
	o.lastPrice = t.Price
	o.mu.Unlock()

	// 2. Check SL/TP/trailing on all open positions
	o.posMgr.CheckStopLossAndTakeProfit(t.Price)

	// 3. Fan out tick to ALL strategies concurrently
	var wg sync.WaitGroup
	var sigMu sync.Mutex
	var rawSignals []AggregatedSignal

	for _, entry := range o.strategies {
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

	// 4. Aggregate and filter signals
	if len(rawSignals) == 0 {
		return
	}
	approved := o.aggregator.FilterSignals(rawSignals)

	// 5. Execute approved signals
	for _, aggSig := range approved {
		sig := aggSig.Signal

		// Record signal in tracker
		o.tracker.RecordSignal(aggSig.StrategyName)

		// Risk validation
		err := o.risk.Validate(sig, t.Price)
		if err != nil {
			log.Printf("[RISK DROPPED] %s from %s: %s", sig.Action, aggSig.StrategyName, err.Error())
			continue
		}

		// Apply slippage (0.01% adverse)
		execPrice := t.Price
		if sig.Action == strategy.ActionBuy {
			execPrice = t.Price * 1.0001 // Buy slightly higher
		} else {
			execPrice = t.Price * 0.9999 // Sell slightly lower
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
			o.tracker.RecordTradeResult(event.Position.StrategyName, event.PnL)

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
