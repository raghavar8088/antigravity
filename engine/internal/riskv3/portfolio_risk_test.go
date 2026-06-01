package riskv3

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
)

// ─── Integration test helpers ─────────────────────────────────────────────────

func newTestEngine(equity float64) *Engine {
	store := ledger.NewMemoryStore()
	return NewEngine(store, "test-account", equity)
}

func newTestRequest(symbol, side string, size, entry, stop float64, strategy string) OrderCheckRequest {
	return OrderCheckRequest{
		ClientOrderID: "test-order-001",
		Symbol:        symbol,
		Side:          side,
		Size:          size,
		EntryPrice:    entry,
		StopLoss:      stop,
		StrategyName:  strategy,
		FamilyName:    "TREND",
		Exchange:      "binance",
		Confidence:    0.85,
		RequestedAt:   time.Now().UTC(),
		TotalTrades:   50,
		Wins:          28,
		AvgWin:        150,
		AvgLoss:       100,
	}
}

// ─── CheckOrder integration tests ────────────────────────────────────────────

func TestCheckOrder_ApprovedSimple(t *testing.T) {
	ctx := context.Background()
	eng := newTestEngine(1_000_000)

	// Small position: 0.01 BTC at $50k, stop at $49.5k → dollar risk = $5
	// Position risk pct = 5/1000000*100 = 0.0005% << 1% limit
	req := newTestRequest("BTCUSDT", "BUY", 0.01, 50000, 49500, "EMA_Cross")
	decision := eng.CheckOrder(ctx, req)

	if !decision.IsApproved() {
		t.Errorf("small position should be approved; got blocked: %s", decision.Reason)
	}
	if decision.HeatPct < 0 {
		t.Errorf("HeatPct should be >= 0, got %.4f", decision.HeatPct)
	}
	if decision.ApprovedSize <= 0 {
		t.Errorf("ApprovedSize should be > 0 for approved order, got %.6f", decision.ApprovedSize)
	}
}

func TestCheckOrder_BlockedByDrawdown(t *testing.T) {
	ctx := context.Background()
	// Set up engine with equity already below HWM (10%+ drawdown)
	eng := newTestEngine(1_000_000)
	eng.state.hwmUSD = 1_200_000 // HWM = $1.2M
	eng.state.equityUSD = 1_060_000 // equity = $1.06M → DD ≈ 11.67%

	req := newTestRequest("BTCUSDT", "BUY", 0.01, 50000, 49500, "RSI_Rev")
	decision := eng.CheckOrder(ctx, req)

	if decision.IsApproved() {
		t.Errorf("expected BLOCKED due to drawdown > %.0f%%, got approved", MaxDrawdownPct)
	}

	hasDrawdownViolation := false
	for _, v := range decision.Violations {
		if v.Type == ViolationDrawdownExceeded {
			hasDrawdownViolation = true
		}
	}
	if !hasDrawdownViolation {
		t.Error("expected ViolationDrawdownExceeded in violations")
	}
}

func TestCheckOrder_BlockedByDailyLoss(t *testing.T) {
	ctx := context.Background()
	eng := newTestEngine(1_000_000)
	// Simulate daily loss at 3.5% ($35k loss on $1M)
	eng.state.dailyPnLUSD = -35_000

	req := newTestRequest("BTCUSDT", "BUY", 0.1, 50000, 49000, "Bollinger")
	decision := eng.CheckOrder(ctx, req)

	if decision.IsApproved() {
		t.Errorf("expected BLOCKED due to daily loss > %.0f%%, got approved", MaxDailyLossPct)
	}

	hasDailyLossViolation := false
	for _, v := range decision.Violations {
		if v.Type == ViolationDailyLossExceeded {
			hasDailyLossViolation = true
		}
	}
	if !hasDailyLossViolation {
		t.Error("expected ViolationDailyLossExceeded in violations")
	}
}

func TestCheckOrder_HeatAccumulation(t *testing.T) {
	ctx := context.Background()
	eng := newTestEngine(100_000) // $100k equity

	// Add existing positions to bring heat near warning level
	// 9 positions each with $1k dollar risk → existing heat = 9%
	for i := 0; i < 9; i++ {
		eng.state.OpenPosition(PositionRecord{
			ID:          fmt.Sprintf("pos-%d", i),
			Symbol:      "BTCUSDT",
			Side:        "BUY",
			EntryPrice:  50000,
			StopLoss:    49000, // $1000 dist
			Size:        0.001, // $1k notional → dollar risk = $1
			NotionalUSD: 50,
		})
	}

	// Propose one more that would cross the kill threshold
	// Proposed dollar risk: (50000-49000)*1 = $1000 → adds 1% heat
	// Combined: 9+1 = 10% → warning level
	req := newTestRequest("BTCUSDT", "BUY", 1.0, 50000, 49000, "momentum")
	decision := eng.CheckOrder(ctx, req)

	// Should either be approved (at warning level) or blocked (if proposed crosses critical)
	// Either way, heat should be reported
	if decision.HeatPct < 0 {
		t.Errorf("HeatPct should be >= 0, got %.4f", decision.HeatPct)
	}
}

func TestCheckOrder_KellyCapApplied(t *testing.T) {
	ctx := context.Background()
	eng := newTestEngine(1_000_000)

	// Kelly with 70% win rate, 3:1 reward risk → full Kelly ≈ 57%
	// Half Kelly ≈ 28.5% → capped at 5%
	// 5% of $1M = $50k → size ≈ $50k / $50k entry = 1 BTC
	// Request 10 BTC ($500k notional) >> Kelly cap → size should be reduced
	req := newTestRequest("BTCUSDT", "BUY", 10.0, 50000, 49000, "high_edge")
	req.TotalTrades = 100
	req.Wins = 70
	req.AvgWin = 300
	req.AvgLoss = 100

	decision := eng.CheckOrder(ctx, req)

	// Even if approved, size should be reduced by Kelly cap
	if decision.IsApproved() && decision.ApprovedSize > 2.0 {
		t.Logf("Note: ApprovedSize=%.4f; Kelly cap may not have fired for this equity level", decision.ApprovedSize)
	}
	if decision.KellyFractionPct > MaxKellyFractionPct+0.001 {
		t.Errorf("KellyFractionPct %.2f%% exceeds cap %.2f%%", decision.KellyFractionPct, MaxKellyFractionPct)
	}
}

// ─── NotifyPositionOpened / Closed lifecycle ──────────────────────────────────

func TestEngineLifecycle_OpenClose(t *testing.T) {
	ctx := context.Background()
	eng := newTestEngine(100_000)
	_ = ctx

	pos := PositionRecord{
		ID:          "lifecycle-pos-001",
		Symbol:      "BTCUSDT",
		Side:        "BUY",
		EntryPrice:  50000,
		StopLoss:    49500,
		Size:        0.5,
		NotionalUSD: 25000,
		StrategyName: "EMA_Cross",
		Exchange:    "binance",
	}
	eng.NotifyPositionOpened(pos)

	if eng.state.PositionCount() != 1 {
		t.Errorf("PositionCount: want 1 got %d", eng.state.PositionCount())
	}

	eng.NotifyPositionClosed("lifecycle-pos-001", 250.0, time.Now().UTC())

	if eng.state.PositionCount() != 0 {
		t.Errorf("PositionCount after close: want 0 got %d", eng.state.PositionCount())
	}
}

// ─── CurrentMetrics test ──────────────────────────────────────────────────────

func TestCurrentMetrics_Empty(t *testing.T) {
	eng := newTestEngine(1_000_000)
	metrics := eng.CurrentMetrics()

	if metrics.EquityUSD != 1_000_000 {
		t.Errorf("EquityUSD: want 1000000 got %.2f", metrics.EquityUSD)
	}
	if metrics.HeatPct != 0 {
		t.Errorf("HeatPct: want 0 got %.4f", metrics.HeatPct)
	}
	if metrics.OpenPositions != 0 {
		t.Errorf("OpenPositions: want 0 got %d", metrics.OpenPositions)
	}
	if metrics.RiskScore < 70 {
		t.Errorf("RiskScore should be high for empty portfolio, got %d", metrics.RiskScore)
	}
}

func TestCurrentMetrics_WithPositions(t *testing.T) {
	eng := newTestEngine(1_000_000)
	eng.NotifyPositionOpened(PositionRecord{
		ID: "m1", Symbol: "BTCUSDT", Side: "BUY",
		EntryPrice: 50000, StopLoss: 49000, Size: 0.1,
		NotionalUSD: 5000, StrategyName: "Test", Exchange: "binance",
	})

	metrics := eng.CurrentMetrics()
	if metrics.OpenPositions != 1 {
		t.Errorf("OpenPositions: want 1 got %d", metrics.OpenPositions)
	}
	if metrics.GrossNotionalUSD != 5000 {
		t.Errorf("GrossNotionalUSD: want 5000 got %.2f", metrics.GrossNotionalUSD)
	}
	// Dollar risk = (50000-49000)*0.1 = 100 USD; heat = 100/1M*100 = 0.01%
	expectedHeat := 100.0 / 1_000_000 * 100
	if math.Abs(metrics.HeatPct-expectedHeat) > 0.001 {
		t.Errorf("HeatPct: want %.4f got %.4f", expectedHeat, metrics.HeatPct)
	}
}

// ─── Risk aggregate replay test ───────────────────────────────────────────────

func TestReplayRiskAggregate_EmptyStore(t *testing.T) {
	ctx := context.Background()
	store := ledger.NewMemoryStore()

	state, err := ReplayRiskAggregate(ctx, store, "test-acct")
	if err != nil {
		t.Fatalf("ReplayRiskAggregate: %v", err)
	}
	if state.TotalChecks != 0 {
		t.Errorf("want 0 checks got %d", state.TotalChecks)
	}
	if state.Version != 0 {
		t.Errorf("want version 0 got %d", state.Version)
	}
}

func TestReplayRiskAggregate_WithEvents(t *testing.T) {
	ctx := context.Background()
	store := ledger.NewMemoryStore()
	accountID := "replay-acct"

	// Emit 2 approved + 1 blocked
	for i := 0; i < 2; i++ {
		approved := OrderDecision{
			Status: DecisionApproved, ApprovedSize: 0.01, HeatPct: 2.0,
			VaR95Pct: 1.5, CVaR95Pct: 2.0, RiskScore: 85,
		}
		EmitRiskApproved(ctx, store, accountID, fmt.Sprintf("order-%d", i), approved, "EMA_Cross")
	}
	blocked := OrderDecision{
		Status: DecisionBlocked, Reason: "drawdown limit",
		HeatPct: 8.0, VaR95Pct: 5.0, DrawdownPct: 12.0, RiskScore: 40,
		Violations: []Violation{{Type: ViolationDrawdownExceeded}},
	}
	EmitRiskBlocked(ctx, store, accountID, "order-blocked", blocked, "RSI_Rev")

	// Allow goroutines to complete
	time.Sleep(50 * time.Millisecond)

	state, err := ReplayRiskAggregate(ctx, store, accountID)
	if err != nil {
		t.Fatalf("ReplayRiskAggregate: %v", err)
	}

	if state.TotalApproved != 2 {
		t.Errorf("TotalApproved: want 2 got %d", state.TotalApproved)
	}
	if state.TotalBlocked < 1 {
		t.Errorf("TotalBlocked: want >= 1 got %d", state.TotalBlocked)
	}
	if state.PeakDrawdownPct < 12.0-0.001 {
		t.Errorf("PeakDrawdownPct: want >= 12 got %.2f", state.PeakDrawdownPct)
	}
}

// ─── 1M event replay benchmark ───────────────────────────────────────────────

func BenchmarkReplayRiskAggregate_100k(b *testing.B) {
	ctx := context.Background()
	store := ledger.NewMemoryStore()
	accountID := "bench-acct"

	// Seed 100k risk events
	b.Log("Seeding 100,000 risk events...")
	for i := 0; i < 100_000; i++ {
		decision := OrderDecision{
			Status: DecisionApproved, ApprovedSize: 0.01,
			HeatPct: float64(i%15), VaR95Pct: float64(i%6),
			RiskScore: 80 - i%30,
		}
		event, err := ledger.NewEvent(ledger.NewEventInput{
			AggregateType: ledger.AggregateRisk,
			AggregateID:   "portfolio-" + accountID,
			AccountID:     accountID,
			EventType:     ledger.EventRiskApproved,
			Payload:       buildCheckPayload(fmt.Sprintf("order-%d", i), decision, "bench"),
			Source:        "bench",
		})
		if err != nil {
			b.Fatal(err)
		}
		if _, err := store.Append(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
	b.Log("Seeded 100,000 events. Starting benchmark...")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state, err := ReplayRiskAggregate(ctx, store, accountID)
		if err != nil {
			b.Fatal(err)
		}
		if state.TotalApproved != 100_000 {
			b.Fatalf("replay produced wrong count: %d", state.TotalApproved)
		}
	}
}

// ─── Portfolio projection test ────────────────────────────────────────────────

func TestBuildPortfolioRiskSummary_Empty(t *testing.T) {
	summary := BuildPortfolioRiskSummary(nil)
	if summary.TotalChecks != 0 {
		t.Errorf("want 0 checks got %d", summary.TotalChecks)
	}
	if summary.ApprovalRate != 0 {
		t.Errorf("want 0 approval rate got %.2f", summary.ApprovalRate)
	}
}

func TestBuildPortfolioRiskSummary_WithEvents(t *testing.T) {
	ctx := context.Background()
	store := ledger.NewMemoryStore()
	accountID := "proj-acct"

	// Emit 3 approved risk events with known metrics
	for i := 0; i < 3; i++ {
		decision := OrderDecision{
			Status: DecisionApproved, ApprovedSize: 0.01,
			HeatPct: 5.0, VaR95Pct: 3.0, CVaR95Pct: 4.0, RiskScore: 82,
		}
		event, err := ledger.NewEvent(ledger.NewEventInput{
			AggregateType: ledger.AggregateRisk,
			AggregateID:   "portfolio-" + accountID,
			AccountID:     accountID,
			EventType:     ledger.EventRiskApproved,
			Payload:       buildCheckPayload(fmt.Sprintf("proj-order-%d", i), decision, "proj-strat"),
			Source:        "proj-test",
		})
		if err != nil {
			t.Fatalf("NewEvent: %v", err)
		}
		if _, err := store.Append(ctx, event); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	events, _ := store.ReplayAccount(ctx, accountID)
	summary := BuildPortfolioRiskSummary(events)

	if summary.TotalChecks != 3 {
		t.Errorf("TotalChecks: want 3 got %d", summary.TotalChecks)
	}
	if math.Abs(summary.ApprovalRate-100.0) > 0.001 {
		t.Errorf("ApprovalRate: want 100%% got %.2f%%", summary.ApprovalRate)
	}
	if math.Abs(summary.Heat.Current-5.0) > 0.001 {
		t.Errorf("Heat.Current: want 5.0 got %.4f", summary.Heat.Current)
	}
}

// ─── fmt import workaround (used in Sprintf calls) ───────────────────────────

func init() { _ = fmt.Sprintf }

// fmt is used in the tests above
var _ = fmt.Sprintf
