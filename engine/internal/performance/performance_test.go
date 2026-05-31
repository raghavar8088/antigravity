package performance

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

type holdStrategy struct{ name string }

func (h holdStrategy) Name() string { return h.name }
func (h holdStrategy) OnTick(t marketdata.Tick) []strategy.Signal {
	return []strategy.Signal{{Symbol: t.Symbol, Action: strategy.ActionHold}}
}
func (h holdStrategy) OnCandle(t marketdata.Tick) []strategy.Signal { return h.OnTick(t) }

func TestStrategySchedulerEvaluatesHundredsOfStrategies(t *testing.T) {
	scheduler := NewStrategyScheduler(8)
	items := make([]ScheduledStrategy, 500)
	for i := range items {
		items[i] = ScheduledStrategy{ID: string(rune('a' + i%26)), Mode: EvaluateOnTick, Strategy: holdStrategy{name: "hold"}, Priority: i}
	}
	start := time.Now()
	results := scheduler.Evaluate(context.Background(), marketdata.Tick{Symbol: "BTCUSD", Price: 100000, TimeMs: time.Now().UnixMilli()}, EvaluateOnTick, items)
	if len(results) != 500 {
		t.Fatalf("expected 500 strategy results, got %d", len(results))
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Fatalf("expected scheduler to stay below 50ms for trivial strategies, got %s", time.Since(start))
	}
}

func TestMarketDataBusDeduplicatesTicks(t *testing.T) {
	bus := NewMarketDataBus(4)
	tick := marketdata.Tick{Symbol: "BTCUSD", TradeID: 1, TimeMs: 1000, Price: 100000}
	if !bus.Publish(MarketEvent{Type: MarketEventTick, Exchange: "coinbase", Tick: tick}) {
		t.Fatal("expected first tick publish")
	}
	if bus.Publish(MarketEvent{Type: MarketEventTick, Exchange: "coinbase", Tick: tick}) {
		t.Fatal("expected duplicate tick rejection")
	}
	stats := bus.Stats()
	if stats.Duplicates != 1 {
		t.Fatalf("expected one duplicate, got %+v", stats)
	}
}

func TestRunningStatsUsesIncrementalUpdates(t *testing.T) {
	var stats RunningStats
	stats.Add(1)
	stats.Add(2)
	stats.Add(3)
	if stats.Count != 3 || stats.Mean != 2 {
		t.Fatalf("unexpected stats %+v", stats)
	}
	if stats.Variance() == 0 {
		t.Fatal("expected non-zero variance")
	}
}
