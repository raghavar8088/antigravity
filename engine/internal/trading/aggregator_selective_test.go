package trading

import (
	"testing"

	"antigravity-engine/internal/strategy"
)

func TestFilterSignalsSelectiveSkipsWeakConsensusBatch(t *testing.T) {
	agg := NewSignalAggregator(15)

	approved := agg.FilterSignalsSelective([]AggregatedSignal{
		{
			StrategyName: "ADX_Trend_Scalp",
			Category:     "Trend",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionBuy,
				TargetSize: 0.01,
				Confidence: 1.0,
			},
		},
		{
			StrategyName: "EMA_Cross_Scalp",
			Category:     "Trend",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionSell,
				TargetSize: 0.01,
				Confidence: 1.0,
			},
		},
	})

	if len(approved) != 0 {
		t.Fatalf("expected weak consensus batch to be skipped, got %d approvals", len(approved))
	}
}

func TestFilterSignalsSelectiveApprovesOnlyTopSignal(t *testing.T) {
	agg := NewSignalAggregator(15)

	approved := agg.FilterSignalsSelective([]AggregatedSignal{
		{
			StrategyName: "TripleFilter_Alpha_Scalp",
			Category:     "Multi-Signal",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionSell,
				TargetSize: 0.01,
				Confidence: 1.0,
			},
		},
		{
			StrategyName: "ATR_Breakout_Scalp",
			Category:     "Breakout Elite",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionSell,
				TargetSize: 0.01,
				Confidence: 1.0,
			},
		},
	})

	if len(approved) != 1 {
		t.Fatalf("expected exactly one approved signal, got %d", len(approved))
	}
	if approved[0].StrategyName != "TripleFilter_Alpha_Scalp" {
		t.Fatalf("expected top-ranked strategy to win, got %s", approved[0].StrategyName)
	}
}
