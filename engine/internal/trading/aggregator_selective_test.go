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

// TestFilterSignalsSelectiveCapsApprovalsAtTwentyFive verifies that the
// Phase 22A throughput cap of 25 (raised from 8) is enforced correctly.
func TestFilterSignalsSelectiveCapsApprovalsAtTwentyFive(t *testing.T) {
	agg := NewSignalAggregator(15)

	// Build 30 unique buy signals across diverse categories so the category cap
	// (5 per category) does not interfere — 6 categories × 5 signals each.
	var signals []AggregatedSignal
	categories := []string{"Trend", "Breakout Elite", "Momentum Elite", "Mean Rev Elite", "Volatility Elite", "Multi-Signal"}
	for _, cat := range categories {
		for i := 0; i < 5; i++ {
			signals = append(signals, makeBuySignal(
				cat+"_strategy_"+string(rune('A'+i)),
				cat,
			))
		}
	}
	// 30 signals → cap at 25
	approved := agg.FilterSignalsSelective(signals)

	if len(approved) != maxApprovedSignals {
		t.Fatalf("expected exactly %d approved signals, got %d", maxApprovedSignals, len(approved))
	}
}

// TestFilterSignalsSelectiveCapsEachCategoryAtFive verifies that the Phase 22A
// per-category cap of 5 (raised from 2) is enforced correctly.
func TestFilterSignalsSelectiveCapsEachCategoryAtFive(t *testing.T) {
	agg := NewSignalAggregator(15)

	// Submit 7 Trend signals — only 5 should be approved, 2 rejected.
	var signals []AggregatedSignal
	for i := 0; i < 7; i++ {
		signals = append(signals, makeBuySignal(
			"Phase22A_Trend_"+string(rune('A'+i)),
			"Trend",
		))
	}
	approved := agg.FilterSignalsSelective(signals)

	if len(approved) != maxApprovedPerCategory {
		t.Fatalf("expected %d trend signals after category cap, got %d", maxApprovedPerCategory, len(approved))
	}
	snapshot := agg.GetSignalFlowSnapshot()
	const wantRejected = int64(7 - maxApprovedPerCategory)
	if snapshot.RejectedByReason[SignalStageCategoryDeduplication+": category_batch_cap"] != wantRejected {
		t.Fatalf("expected %d category cap rejections, got %+v", wantRejected, snapshot.RejectedByReason)
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

func TestSignalFlowDiagnosticsTracksApprovals(t *testing.T) {
	agg := NewSignalAggregator(15)

	agg.FilterSignalsSelective([]AggregatedSignal{
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
	})

	diag := agg.GetSignalFlowDiagnostics()
	if diag.ApprovedByStrategy["TripleFilter_Alpha_Scalp"] != 1 {
		t.Fatalf("expected 1 approval for TripleFilter_Alpha_Scalp, got %d", diag.ApprovedByStrategy["TripleFilter_Alpha_Scalp"])
	}
	if diag.ApprovedByCategory["Multi-Signal"] != 1 {
		t.Fatalf("expected 1 approval for Multi-Signal category, got %d", diag.ApprovedByCategory["Multi-Signal"])
	}
	if diag.TotalGenerated != 1 {
		t.Fatalf("expected TotalGenerated=1, got %d", diag.TotalGenerated)
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
