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

func TestFilterSignalsSelectiveRanksApprovedSignals(t *testing.T) {
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
			StrategyName: "Generic_Momentum_Scalp",
			Category:     "Momentum Elite",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionSell,
				TargetSize: 0.01,
				Confidence: 1.1,
			},
		},
	})

	if len(approved) != 2 {
		t.Fatalf("expected two approved signals, got %d", len(approved))
	}
	if approved[0].StrategyName != "TripleFilter_Alpha_Scalp" {
		t.Fatalf("expected top-ranked strategy to win, got %s", approved[0].StrategyName)
	}
}

func TestFilterSignalsSelectiveCapsApprovalsAtEight(t *testing.T) {
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
			StrategyName: "OpeningRange_Breakout_Scalp_A",
			Category:     "Time-of-Day",
			Signal: strategy.Signal{
				Symbol:     "BTC-USD",
				Action:     strategy.ActionBuy,
				TargetSize: 0.01,
				Confidence: 1.0,
			},
		},
		makeBuySignal("Phase22A_Breakout_A", "Breakout Elite"),
		makeBuySignal("Phase22A_Breakout_B", "Breakout Elite"),
		makeBuySignal("Phase22A_Momentum_A", "Momentum Elite"),
		makeBuySignal("Phase22A_Momentum_B", "Momentum Elite"),
		makeBuySignal("Phase22A_MeanRev_A", "Mean Rev Elite"),
		makeBuySignal("Phase22A_MeanRev_B", "Mean Rev Elite"),
		makeBuySignal("Phase22A_Volatility_A", "Volatility Elite"),
	})

	if len(approved) != 8 {
		t.Fatalf("expected exactly eight approved signals, got %d", len(approved))
	}
}

func TestFilterSignalsSelectiveCapsEachCategoryAtTwo(t *testing.T) {
	agg := NewSignalAggregator(15)

	approved := agg.FilterSignalsSelective([]AggregatedSignal{
		makeBuySignal("Phase22A_Trend_A", "Trend"),
		makeBuySignal("Phase22A_Trend_B", "Trend"),
		makeBuySignal("Phase22A_Trend_C", "Trend"),
	})

	if len(approved) != 2 {
		t.Fatalf("expected two trend signals after category cap, got %d", len(approved))
	}
	snapshot := agg.GetSignalFlowSnapshot()
	if snapshot.RejectedByReason[SignalStageCategoryDeduplication+": category_batch_cap"] != 1 {
		t.Fatalf("expected one category cap rejection, got %+v", snapshot.RejectedByReason)
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

func makeBuySignal(name, category string) AggregatedSignal {
	return AggregatedSignal{
		StrategyName: name,
		Category:     category,
		Signal: strategy.Signal{
			Symbol:     "BTC-USD",
			Action:     strategy.ActionBuy,
			TargetSize: 0.01,
			Confidence: 1.0,
		},
	}
}
