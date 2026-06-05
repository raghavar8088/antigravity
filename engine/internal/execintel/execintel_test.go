package execintel

import (
	"fmt"
	"testing"
	"time"
)

// drive runs a full happy-path lifecycle for one signal and returns its id.
func drive(t *Tracker, id, strat, regime string, price, size float64) {
	t.Begin(Meta{
		SignalID: id, Strategy: strat, Category: "Liquidity", AlphaSource: strat,
		Symbol: "BTC-USD", Direction: "BUY", Price: price, Size: size,
		Regime: regime, Timeframe: "1m",
	})
	t.Record(id, StateSignalApproved, "")
	t.Record(id, StateRiskApproved, "")
	t.Record(id, StateOrderSubmitted, "MARKET")
	time.Sleep(time.Millisecond) // create a measurable span
	t.Record(id, StateOrderAcknowledged, "")
	t.Record(id, StateOrderFilled, "")
	t.Record(id, StatePositionOpened, "")
}

func TestLifecycleHappyPathConversionCounters(t *testing.T) {
	tr := New()
	drive(tr, "s1", "LiquiditySweepReversal_Alpha", "VOLATILE", 60000, 0.1)
	// Close as a winner.
	tr.Record("s1", StateTPTriggered, "TAKE_PROFIT")
	tr.Record("s1", StatePositionClosed, "pnl=12.5")
	tr.RecordTradeResult(12.5)

	snap := tr.Snapshot()
	if snap.Conversion.Generated != 1 {
		t.Fatalf("generated = %d, want 1", snap.Conversion.Generated)
	}
	if snap.Conversion.Approved != 1 {
		t.Fatalf("approved = %d, want 1", snap.Conversion.Approved)
	}
	if snap.Conversion.Executed != 1 {
		t.Fatalf("executed (filled) = %d, want 1", snap.Conversion.Executed)
	}
	if snap.Conversion.Profitable != 1 {
		t.Fatalf("profitable = %d, want 1", snap.Conversion.Profitable)
	}
	if snap.Conversion.ApprovalRatePct != 100 || snap.Conversion.ExecutionRatePct != 100 {
		t.Fatalf("rates: approval=%.1f execution=%.1f, want 100/100",
			snap.Conversion.ApprovalRatePct, snap.Conversion.ExecutionRatePct)
	}
	if snap.ActiveSignals != 0 {
		t.Fatalf("active signals = %d, want 0 after close", snap.ActiveSignals)
	}
}

func TestConversionRatesMatchSpecExample(t *testing.T) {
	// Spec example: Generated=1000, Approved=350, Executed=280, Profitable=170.
	tr := New()
	for i := range 1000 {
		id := fmt.Sprintf("g%d", i)
		tr.Begin(Meta{SignalID: id, Strategy: "X", Symbol: "BTC-USD", Direction: "BUY", Price: 100, Size: 1, Timeframe: "1m"})
		if i < 350 {
			tr.Record(id, StateSignalApproved, "")
			if i < 280 {
				tr.Record(id, StateOrderFilled, "")
				if i < 170 {
					tr.RecordTradeResult(5)
				} else {
					tr.RecordTradeResult(-5)
				}
				tr.Record(id, StatePositionClosed, "")
			} else {
				tr.RecordRejection(id, "no market price available for execution")
			}
		} else {
			tr.RecordRejection(id, "weak_directional_consensus")
		}
	}
	snap := tr.Snapshot()
	c := snap.Conversion
	if c.Generated != 1000 || c.Approved != 350 || c.Executed != 280 || c.Profitable != 170 {
		t.Fatalf("funnel = gen %d app %d exec %d prof %d; want 1000/350/280/170",
			c.Generated, c.Approved, c.Executed, c.Profitable)
	}
	if r := round1(c.ApprovalRatePct); r != 35.0 {
		t.Errorf("approval rate = %.1f, want 35.0", r)
	}
	if r := round1(c.ExecutionRatePct); r != 28.0 {
		t.Errorf("execution rate = %.1f, want 28.0", r)
	}
	if r := round1(c.ProfitConversionPct); r != 17.0 {
		t.Errorf("profit conversion = %.1f, want 17.0", r)
	}
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

func TestLatencyPercentilesAreComputed(t *testing.T) {
	tr := New()
	for i := range 100 {
		id := fmt.Sprintf("l%d", i)
		tr.Begin(Meta{SignalID: id, Strategy: "L", Regime: "TREND", Symbol: "BTC-USD", Direction: "BUY", Price: 100, Size: 1, Timeframe: "1m"})
		tr.Record(id, StateOrderSubmitted, "")
		time.Sleep(time.Millisecond)
		tr.Record(id, StateOrderFilled, "")
		tr.Record(id, StatePositionClosed, "")
	}
	snap := tr.Snapshot()
	e2e := snap.Latency.ByStage["signal_to_fill_e2e"]
	if e2e.Count == 0 {
		t.Fatal("no e2e latency samples recorded")
	}
	if e2e.P50 <= 0 || e2e.P95 <= 0 || e2e.P99 <= 0 {
		t.Fatalf("percentiles not positive: p50=%.3f p95=%.3f p99=%.3f", e2e.P50, e2e.P95, e2e.P99)
	}
	if e2e.P99 < e2e.P50 {
		t.Fatalf("p99 (%.3f) < p50 (%.3f)", e2e.P99, e2e.P50)
	}
	if _, ok := snap.Latency.ByStrategy["signal_to_fill_e2e|L"]; !ok {
		t.Error("expected per-strategy latency series for L")
	}
	if _, ok := snap.Latency.ByRegime["signal_to_fill_e2e|TREND"]; !ok {
		t.Error("expected per-regime latency series for TREND")
	}
}

func TestPercentileExact(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := percentile(sorted, 0.50); got < 5 || got > 6 {
		t.Errorf("p50 = %.2f, want ~5.5", got)
	}
	if got := percentile(sorted, 0.99); got < 9.5 {
		t.Errorf("p99 = %.2f, want ~9.9", got)
	}
	if got := percentile(sorted, 0.0); got != 1 {
		t.Errorf("p0 = %.2f, want 1", got)
	}
}

func TestSlippageAttribution(t *testing.T) {
	tr := New()
	// BUY filled above reference → adverse positive slippage.
	tr.RecordSlippage(SlippageSample{
		Strategy: "A", AlphaSource: "A", Session: "LONDON", Regime: "TREND",
		Direction: "BUY", SignalPrice: 60000, FilledPrice: 60006, Size: 1, // +1bps
	})
	// SELL filled below reference → adverse positive slippage after sign flip.
	tr.RecordSlippage(SlippageSample{
		Strategy: "A", AlphaSource: "A", Session: "LONDON", Regime: "TREND",
		Direction: "SELL", SignalPrice: 60000, FilledPrice: 59994, Size: 1, // +1bps adverse
	})
	snap := tr.Snapshot()
	o := snap.Slippage.Overall
	if o.Count != 2 {
		t.Fatalf("slippage count = %d, want 2", o.Count)
	}
	if o.AvgBps < 0.9 || o.AvgBps > 1.1 {
		t.Errorf("avg slippage = %.3f bps, want ~1.0", o.AvgBps)
	}
	if _, ok := snap.Slippage.ByStrategy["A"]; !ok {
		t.Error("expected per-strategy slippage for A")
	}
	if _, ok := snap.Slippage.BySession["LONDON"]; !ok {
		t.Error("expected per-session slippage for LONDON")
	}
}

func TestSlippageBpsDirectionSign(t *testing.T) {
	buy := SlippageSample{Direction: "BUY", SignalPrice: 100, FilledPrice: 101}
	if bps := buy.SlippageBps(); bps <= 0 {
		t.Errorf("BUY filled higher should be adverse positive, got %.1f", bps)
	}
	sell := SlippageSample{Direction: "SELL", SignalPrice: 100, FilledPrice: 99}
	if bps := sell.SlippageBps(); bps <= 0 {
		t.Errorf("SELL filled lower should be adverse positive, got %.1f", bps)
	}
}

func TestMissedEntryClassificationAndRanking(t *testing.T) {
	tr := New()
	reasons := []string{
		"stale_signal_expired", "stale_signal_expired", "stale_signal_expired",
		"size_below_minimum",
		"risk: daily drawdown exceeded",
		"weak_directional_consensus", "non_dominant_side",
	}
	for i, r := range reasons {
		id := fmt.Sprintf("m%d", i)
		tr.Begin(Meta{SignalID: id, Strategy: "M", Symbol: "BTC-USD", Direction: "BUY", Price: 100, Size: 2, Timeframe: "1m"})
		tr.RecordRejection(id, r)
	}
	snap := tr.Snapshot()
	if snap.Missed.ByReason[RejectExpired] != 3 {
		t.Errorf("expired count = %d, want 3", snap.Missed.ByReason[RejectExpired])
	}
	if snap.Missed.ByReason[RejectMinimumSize] != 1 {
		t.Errorf("min-size count = %d, want 1", snap.Missed.ByReason[RejectMinimumSize])
	}
	if snap.Missed.ByReason[RejectRisk] != 1 {
		t.Errorf("risk count = %d, want 1", snap.Missed.ByReason[RejectRisk])
	}
	if snap.Missed.ByReason[RejectAggregator] != 2 {
		t.Errorf("aggregator count = %d, want 2", snap.Missed.ByReason[RejectAggregator])
	}
	// Top bottleneck must be the most frequent reason (expired).
	if len(snap.Bottlenecks) == 0 || snap.Bottlenecks[0].Reason != RejectExpired {
		t.Fatalf("top bottleneck = %+v, want RejectExpired first", snap.Bottlenecks)
	}
	// Missed notional for expired = 3 signals × price100 × size2 = 600.
	if snap.Missed.MissedNotional[RejectExpired] != 600 {
		t.Errorf("expired missed notional = %.1f, want 600", snap.Missed.MissedNotional[RejectExpired])
	}
}

func TestClassifyReasons(t *testing.T) {
	cases := map[string]RejectReason{
		"stale_signal_expired":              RejectExpired,
		"size_below_minimum":                RejectMinimumSize,
		"execution_weight_below_floor":      RejectSizing,
		"parked_for_command_center_bridge":  RejectBridge,
		"oms: append event failed":          RejectOMS,
		"no market price available":         RejectBroker,
		"risk: kill switch active":          RejectRisk,
		"category_not_aligned_with_regime":  RejectRegime,
		"low_confidence":                    RejectConfidence,
		"position_limit":                    RejectPositionCap,
		"weak_directional_consensus":        RejectAggregator,
		"something we have never seen":      RejectOther,
	}
	for raw, want := range cases {
		if got := Classify(raw); got != want {
			t.Errorf("Classify(%q) = %s, want %s", raw, got, want)
		}
	}
}

func TestHardExpiryWindows(t *testing.T) {
	cases := map[string]time.Duration{
		"1m":  2 * time.Minute,
		"3m":  6 * time.Minute,
		"5m":  10 * time.Minute,
		"15m": 30 * time.Minute,
	}
	for tf, want := range cases {
		if got := HardExpiry(tf); got != want {
			t.Errorf("HardExpiry(%q) = %v, want %v", tf, got, want)
		}
	}
	// Enforcement: an over-age signal must be flagged expired.
	if !IsExpired("1m", 3*time.Minute) {
		t.Error("1m signal aged 3m must be expired")
	}
	if IsExpired("1m", 1*time.Minute) {
		t.Error("1m signal aged 1m must NOT be expired")
	}
	if !IsExpired("5m", 11*time.Minute) {
		t.Error("5m signal aged 11m must be expired")
	}
}

func TestTPOverrideAuditTighteningHurtsWinner(t *testing.T) {
	tr := New()
	// A winner whose TP was tightened (0.9% → 0.5%) gives up profit.
	tr.RecordTPOverride(TPOverrideSample{Strategy: "T", Source: "tp_floor", OriginalTP: 0.9, AdjustedTP: 0.5})
	tr.RecordTPOutcome("T", 20.0, "TAKE_PROFIT")
	snap := tr.Snapshot()
	if snap.TPOverride.Tightened != 1 {
		t.Fatalf("tightened = %d, want 1", snap.TPOverride.Tightened)
	}
	if snap.TPOverride.ReductionUSD <= 0 {
		t.Errorf("expected positive reductionUSD for tightened winner, got %.3f", snap.TPOverride.ReductionUSD)
	}
	if snap.TPOverride.NetImpactUSD >= 0 {
		t.Errorf("expected negative net impact, got %.3f", snap.TPOverride.NetImpactUSD)
	}
}

func TestExecutionQualityScoreInstitutionalWhenClean(t *testing.T) {
	tr := New()
	// Drive 50 fast, low-slippage, fully-converting signals.
	for i := range 50 {
		id := fmt.Sprintf("q%d", i)
		tr.Begin(Meta{SignalID: id, Strategy: "Q", Regime: "TREND", Symbol: "BTC-USD", Direction: "BUY", Price: 60000, Size: 0.1, Timeframe: "1m"})
		tr.Record(id, StateSignalApproved, "")
		tr.Record(id, StateOrderSubmitted, "")
		tr.Record(id, StateOrderFilled, "") // sub-ms span → fast
		tr.RecordSlippage(SlippageSample{Strategy: "Q", Direction: "BUY", SignalPrice: 60000, FilledPrice: 60001, Size: 0.1}) // ~0.17bps
		tr.Record(id, StatePositionOpened, "")
		tr.Record(id, StatePositionClosed, "")
		tr.RecordTradeResult(3)
	}
	snap := tr.Snapshot()
	q := snap.Quality
	if q.Score < 85 {
		t.Fatalf("quality score = %.1f, want >= 85 for clean execution; components=%+v", q.Score, q.Components)
	}
	if q.Classification != "Institutional" && q.Classification != "Production" {
		t.Errorf("classification = %q, want Institutional/Production", q.Classification)
	}
}

func TestActiveEvictionBound(t *testing.T) {
	tr := NewWithCap(8, 16)
	for i := range 100 {
		tr.Begin(Meta{SignalID: fmt.Sprintf("e%d", i), Strategy: "E", Symbol: "BTC-USD", Direction: "BUY", Price: 1, Size: 1, Timeframe: "1m"})
	}
	if got := tr.ActiveCount(); got > 8 {
		t.Fatalf("active count = %d, want <= 8 (eviction bound)", got)
	}
}

func TestSessionForUTC(t *testing.T) {
	if SessionForUTC(3) != "ASIA" {
		t.Error("03:00 UTC should be ASIA")
	}
	if SessionForUTC(10) != "LONDON" {
		t.Error("10:00 UTC should be LONDON")
	}
	if SessionForUTC(18) != "NEW_YORK" {
		t.Error("18:00 UTC should be NEW_YORK")
	}
}
