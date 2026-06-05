package execintel

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestEvidenceSnapshotDump drives a realistic funnel and prints the full
// execution-intelligence snapshot. It exists to produce hard, reproducible
// runtime numbers for the Phase 22D report (run with -v to see the dump).
func TestEvidenceSnapshotDump(t *testing.T) {
	tr := New()

	// 200 generated signals with a realistic rejection mix derived from the
	// orchestrator's actual rejection sites.
	rejected := map[string]int{
		"weak_directional_consensus":       40, // aggregator
		"stale_signal_expired":             12, // expiry
		"category_not_aligned_with_regime": 18, // regime
		"execution_weight_below_floor":     10, // sizing
		"size_below_minimum":               6,  // min size
		"risk: exposure cap reached":       8,  // risk
		"parked_for_command_center_bridge": 6,  // bridge
	}
	i := 0
	for reason, n := range rejected {
		for range n {
			id := fmt.Sprintf("r%d", i)
			i++
			tr.Begin(Meta{SignalID: id, Strategy: "Mix", Category: "Liquidity",
				Symbol: "BTC-USD", Direction: "BUY", Price: 60000, Size: 0.0167, Timeframe: "1m"})
			tr.Record(id, StateSignalApproved, "")
			tr.RecordRejection(id, reason)
		}
	}

	// 100 fully-executed signals with the paper desk's deterministic MARKET
	// slippage (+1bps on BUY) and ~1ms pipeline spans.
	winners := 0
	for j := range 100 {
		id := fmt.Sprintf("x%d", j)
		tr.Begin(Meta{SignalID: id, Strategy: "Mix", Category: "Liquidity", Regime: "VOLATILE",
			Symbol: "BTC-USD", Direction: "BUY", Price: 60000, Size: 0.0167, Timeframe: "1m"})
		tr.Record(id, StateSignalApproved, "")
		tr.Record(id, StateRiskApproved, "")
		tr.Record(id, StateOrderSubmitted, "MARKET")
		time.Sleep(time.Millisecond)
		tr.Record(id, StateOrderAcknowledged, "")
		tr.Record(id, StateOrderFilled, "")
		tr.RecordSlippage(SlippageSample{Strategy: "Mix", AlphaSource: "Mix", Session: "NEW_YORK",
			Regime: "VOLATILE", Direction: "BUY", SignalPrice: 60000, FilledPrice: 60006, Size: 0.0167})
		tr.Record(id, StatePositionOpened, "")
		// 60% winners.
		pnl := -4.0
		if j%5 < 3 {
			pnl = 7.0
			winners++
		}
		tr.Record(id, StatePositionClosed, "")
		tr.RecordTradeResult(pnl)
		tr.RecordTPOutcome("Mix", pnl, "TAKE_PROFIT")
	}

	snap := tr.Snapshot()
	out, _ := json.MarshalIndent(snap, "", "  ")
	t.Logf("PHASE 22D EVIDENCE SNAPSHOT:\n%s", string(out))

	// Sanity assertions so the evidence is self-validating.
	if snap.Conversion.Generated != 200 {
		t.Fatalf("generated = %d, want 200", snap.Conversion.Generated)
	}
	if snap.Conversion.Executed != 100 {
		t.Fatalf("executed = %d, want 100", snap.Conversion.Executed)
	}
	if snap.Quality.Score <= 0 {
		t.Fatal("quality score must be computed")
	}
	t.Logf("Conversion: approval=%.1f%% execution=%.1f%% profit=%.1f%% win=%.1f%%",
		snap.Conversion.ApprovalRatePct, snap.Conversion.ExecutionRatePct,
		snap.Conversion.ProfitConversionPct, snap.Conversion.WinRatePct)
	t.Logf("Missed entry rate: %.1f%% | top bottleneck: %s (%d)",
		snap.Missed.MissedEntryRate, snap.Bottlenecks[0].Reason, snap.Bottlenecks[0].LostTrades)
	t.Logf("E2E latency p50=%.3fms p95=%.3fms p99=%.3fms",
		snap.Latency.ByStage["signal_to_fill_e2e"].P50,
		snap.Latency.ByStage["signal_to_fill_e2e"].P95,
		snap.Latency.ByStage["signal_to_fill_e2e"].P99)
	t.Logf("Slippage avg=%.3fbps median=%.3fbps worst=%.3fbps",
		snap.Slippage.Overall.AvgBps, snap.Slippage.Overall.MedianBps, snap.Slippage.Overall.WorstBps)
	t.Logf("Execution quality: %.1f (%s)", snap.Quality.Score, snap.Quality.Classification)
}
