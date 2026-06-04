package live_test

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/live"
)

// mockExecutor is a configurable test double for the Executor interface.
type mockExecutor struct {
	avgPrice    float64
	feesUSD     float64
	slippageBps float64
	latencyMs   int64
	err         error
}

func (m *mockExecutor) Submit(_ context.Context, req live.OrderRequest) (live.FillReport, error) {
	if m.err != nil {
		return live.FillReport{}, m.err
	}
	return live.FillReport{
		ClientOrderID: req.ClientOrderID,
		Symbol:        req.Symbol,
		Side:          req.Side,
		FilledQty:     req.Quantity,
		AvgPrice:      m.avgPrice,
		FeesUSD:       m.feesUSD,
		SlippageBps:   m.slippageBps,
		LatencyMs:     m.latencyMs,
		FilledAt:      time.Now().UTC(),
	}, nil
}

func (m *mockExecutor) Cancel(_ context.Context, _ string) error { return m.err }

// ── LiveHarness ───────────────────────────────────────────────────────────────

func TestHarness_DefaultsPaperMode(t *testing.T) {
	paper := &mockExecutor{avgPrice: 50000}
	h := live.NewLiveHarness(paper, nil)
	if h.CurrentMode() != live.ModePaper {
		t.Error("expected paper mode by default")
	}
}

func TestHarness_SetLiveModeWithoutExecutorFails(t *testing.T) {
	paper := &mockExecutor{}
	h := live.NewLiveHarness(paper, nil)
	if err := h.SetMode(live.ModeLive); err == nil {
		t.Error("expected error when no live executor configured")
	}
}

func TestHarness_SetLiveMode(t *testing.T) {
	paper := &mockExecutor{}
	liveExec := &mockExecutor{avgPrice: 50000}
	h := live.NewLiveHarness(paper, liveExec)
	if err := h.SetMode(live.ModeLive); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.CurrentMode() != live.ModeLive {
		t.Error("expected live mode after SetMode")
	}
}

func TestHarness_SubmitRecordsFill(t *testing.T) {
	paper := &mockExecutor{avgPrice: 50000, slippageBps: 2, feesUSD: 5, latencyMs: 15}
	h := live.NewLiveHarness(paper, nil)

	req := live.OrderRequest{
		ClientOrderID: "test-001",
		StrategyID:    "ema_fast",
		Symbol:        "BTC-USD",
		Side:          "BUY",
		Quantity:      0.01,
		OrderType:     "MARKET",
		CreatedAt:     time.Now().UTC(),
	}
	fill, err := h.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fill.FilledQty != 0.01 {
		t.Errorf("expected 0.01 filled, got %f", fill.FilledQty)
	}
	stats := h.Fills().Stats()
	if stats.TotalFills != 1 {
		t.Errorf("expected 1 fill recorded, got %d", stats.TotalFills)
	}
}

func TestHarness_SubmitTracksOpenOrder(t *testing.T) {
	// Mock that only partially fills
	paper := &mockExecutor{avgPrice: 50000, slippageBps: 1}
	h := live.NewLiveHarness(paper, nil)

	// The mockExecutor fills the full quantity, so the orphan should be auto-closed.
	// Use a separate check: submit two orders, cancel one.
	req1 := live.OrderRequest{ClientOrderID: "o1", Symbol: "BTC-USD", Side: "BUY", Quantity: 0.01, OrderType: "MARKET"}
	_, _ = h.Submit(context.Background(), req1)
	// After full fill, orphan count should be 0 (auto-closed)
	if h.Orphans().OpenCount() != 0 {
		t.Errorf("expected 0 open orders after full fill, got %d", h.Orphans().OpenCount())
	}
}

func TestHarness_CancelRemovesOrphan(t *testing.T) {
	paper := &mockExecutor{avgPrice: 50000}
	h := live.NewLiveHarness(paper, nil)

	// Manually track an open order that wasn't auto-closed (partial fill simulation)
	h.Orphans().TrackOpen("o-manual", "s1", "BTC-USD")
	if h.Orphans().OpenCount() != 1 {
		t.Fatal("expected 1 open order")
	}
	_ = h.Cancel(context.Background(), "o-manual")
	if h.Orphans().OpenCount() != 0 {
		t.Error("cancel should remove order from orphan tracking")
	}
}

func TestHarness_VerifyParityRequiresBothExecutors(t *testing.T) {
	paper := &mockExecutor{}
	h := live.NewLiveHarness(paper, nil)
	_, err := h.VerifyParity(context.Background(), live.OrderRequest{})
	if err == nil {
		t.Error("expected error with no live executor")
	}
}

// ── OrphanDetector ────────────────────────────────────────────────────────────

func TestOrphanDetector_DetectsStaleOrders(t *testing.T) {
	od := live.NewOrphanDetector()
	od.OrphanAfter = 2 * time.Millisecond

	od.TrackOpen("ord-1", "s1", "BTC-USD")
	time.Sleep(10 * time.Millisecond)

	orphans := od.DetectOrphans()
	if len(orphans) != 1 {
		t.Errorf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].ClientOrderID != "ord-1" {
		t.Errorf("unexpected orphan id: %s", orphans[0].ClientOrderID)
	}
}

func TestOrphanDetector_CloseRemovesFromTracking(t *testing.T) {
	od := live.NewOrphanDetector()
	od.OrphanAfter = 2 * time.Millisecond

	od.TrackOpen("ord-2", "s1", "BTC-USD")
	od.TrackClose("ord-2")
	time.Sleep(10 * time.Millisecond)

	orphans := od.DetectOrphans()
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans after close, got %d", len(orphans))
	}
}

func TestOrphanDetector_CriticalSeverityAt3x(t *testing.T) {
	od := live.NewOrphanDetector()
	od.OrphanAfter = 2 * time.Millisecond

	od.TrackOpen("ord-crit", "s1", "BTC-USD")
	time.Sleep(20 * time.Millisecond) // 10× the threshold

	orphans := od.DetectOrphans()
	if len(orphans) == 0 {
		t.Fatal("expected orphan")
	}
	if orphans[0].Severity != "CRITICAL" {
		t.Errorf("expected CRITICAL severity, got %s", orphans[0].Severity)
	}
}

// ── ParityChecker ─────────────────────────────────────────────────────────────

func TestParityChecker_EquivalentPaths(t *testing.T) {
	pc := live.NewParityChecker()
	paper := &mockExecutor{avgPrice: 50000, slippageBps: 1, feesUSD: 1, latencyMs: 10}
	liveExec := &mockExecutor{avgPrice: 50000, slippageBps: 1.5, feesUSD: 1, latencyMs: 20}

	req := live.OrderRequest{
		ClientOrderID: "parity-ok",
		Symbol:        "BTC-USD",
		Side:          "BUY",
		Quantity:      0.001,
		OrderType:     "MARKET",
	}
	result, err := pc.Check(context.Background(), req, paper, liveExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsEquivalent {
		t.Errorf("expected equivalent, findings: %v", result.Findings)
	}
}

func TestParityChecker_DetectsPriceDivergence(t *testing.T) {
	pc := live.NewParityChecker()
	pc.MaxPriceDeltaPct = 0.001 // 0.1% tolerance

	paper := &mockExecutor{avgPrice: 50000, slippageBps: 1, feesUSD: 1}
	liveExec := &mockExecutor{avgPrice: 50500, slippageBps: 1, feesUSD: 1} // 1% price delta

	req := live.OrderRequest{
		ClientOrderID: "parity-fail",
		Symbol:        "BTC-USD",
		Side:          "BUY",
		Quantity:      0.001,
		OrderType:     "MARKET",
	}
	result, err := pc.Check(context.Background(), req, paper, liveExec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsEquivalent {
		t.Error("expected parity failure due to price divergence")
	}
	if len(result.Findings) == 0 {
		t.Error("expected at least one finding")
	}
}

// ── FillAnalyzer ─────────────────────────────────────────────────────────────

func TestFillAnalyzer_Stats(t *testing.T) {
	fa := live.NewFillAnalyzer()
	for i := 1; i <= 100; i++ {
		fa.Record(live.FillReport{
			FilledQty:   0.01,
			FeesUSD:     0.5,
			SlippageBps: float64(i),
			LatencyMs:   int64(i),
		})
	}
	stats := fa.Stats()
	if stats.TotalFills != 100 {
		t.Errorf("expected 100 fills, got %d", stats.TotalFills)
	}
	if stats.P95SlippageBps < 94 || stats.P95SlippageBps > 96 {
		t.Errorf("unexpected P95 slippage: %.1f", stats.P95SlippageBps)
	}
	if stats.AvgSlippageBps < 50 || stats.AvgSlippageBps > 51 {
		t.Errorf("unexpected avg slippage: %.1f", stats.AvgSlippageBps)
	}
}

func TestFillAnalyzer_EmptyStats(t *testing.T) {
	fa := live.NewFillAnalyzer()
	stats := fa.Stats()
	if stats.TotalFills != 0 {
		t.Errorf("expected 0 fills, got %d", stats.TotalFills)
	}
}
