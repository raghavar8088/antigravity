package trading

import (
	"testing"

	"antigravity-engine/internal/strategy"
)

func TestFilterSignalsSelectiveSkipsWeakConsensusBatch(t *testing.T) {
	agg := NewSignalAggregator(15)

	approved := agg.FilterSignalsSelective([]AggregatedSignal{
		{
			StrategyName: "TripleTrend_Confluence_Scalp",
			Category:     "Trend",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionBuy,
				TargetSize: 0.01,
				Confidence: 1.0,
			},
		},
		{
			StrategyName: "RSI_MACD_Divergence_Scalp",
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

func TestFilterSignalsSelectiveCapsApprovalsAtTwo(t *testing.T) {
	agg := NewSignalAggregator(15)

	approved := agg.FilterSignalsSelective([]AggregatedSignal{
		{
			StrategyName: "TripleFilter_Alpha_Scalp",
			Category:     "Multi-Signal",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionBuy,
				TargetSize: 0.01,
				Confidence: 1.0,
			},
		},
		{
			StrategyName: "VolumeWeighted_Trend_Scalp",
			Category:     "Trend",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionBuy,
				TargetSize: 0.01,
				Confidence: 1.0,
			},
		},
		{
			StrategyName: "OpeningRange_Breakout_Scalp",
			Category:     "Time-of-Day",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionBuy,
				TargetSize: 0.01,
				Confidence: 1.0,
			},
		},
	})

	if len(approved) != 2 {
		t.Fatalf("expected exactly two approved signals, got %d", len(approved))
	}
}

func TestFilterSignalsSelectiveDemotesKnownLoser(t *testing.T) {
	agg := NewSignalAggregator(15)

	approved := agg.FilterSignalsSelective([]AggregatedSignal{
		{
			StrategyName: "ATR_Volume_Impulse_Scalp",
			Category:     "Breakout Elite",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionBuy,
				TargetSize: 0.01,
				Confidence: 1.0,
			},
		},
	})

	if len(approved) != 0 {
		t.Fatalf("expected known losing strategy to stay below selective threshold, got %d approvals", len(approved))
	}
}
