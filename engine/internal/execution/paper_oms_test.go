package execution

import (
	"fmt"
	"testing"
	"time"
)

// deterministicFixture returns a fixed sequence of mark-price ticks and one
// open-position request so every run through the OMS sees identical input.
func deterministicFixture() (OpenPositionRequest, []OMSTick) {
	req := OpenPositionRequest{
		Symbol:       "BTCUSDT",
		Side:         SideLong,
		EntryPrice:   60_000,
		Notional:     1_000,
		Leverage:     10,
		SLPct:        2.0,  // 2% → slPrice = 58,800
		TPPct:        4.0,  // 4% → tpPrice = 62,400
		HoldMinutes:  60,
		StrategyID:   1,
		StrategyName: "test-strat",
	}

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ticks := []OMSTick{
		{Symbol: "BTCUSDT", MarkPrice: 60_100, NowUTC: t0.Add(5 * time.Minute)},
		{Symbol: "BTCUSDT", MarkPrice: 60_500, NowUTC: t0.Add(10 * time.Minute)},
		{Symbol: "BTCUSDT", MarkPrice: 62_500, NowUTC: t0.Add(15 * time.Minute)}, // crosses TP (fill+slippage ≈ 62,431)
	}
	return req, ticks
}

// runOMS feeds the fixture through a fresh OMS and returns the number of closed
// trades and their net PnL values.
func runOMS(t *testing.T, label string) (int, []float64) {
	t.Helper()
	oms := NewPaperOMS(10_000)

	req, ticks := deterministicFixture()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	id, err := oms.OpenPosition(req, t0)
	if err != nil {
		t.Fatalf("[%s] OpenPosition: %v", label, err)
	}
	if id == "" {
		t.Fatalf("[%s] empty position id", label)
	}

	var pnls []float64
	for _, tick := range ticks {
		closed := oms.Tick(tick)
		for _, tr := range closed {
			pnls = append(pnls, tr.NetPnl)
		}
	}
	return len(pnls), pnls
}

// TestPaperOMSDeterminism verifies that two independent OMS instances fed the
// same fixture produce identical trade counts and net PnL values — the core
// determinism guarantee of Epic 1.
func TestPaperOMSDeterminism(t *testing.T) {
	count1, pnls1 := runOMS(t, "run1")
	count2, pnls2 := runOMS(t, "run2")

	if count1 != count2 {
		t.Fatalf("trade count mismatch: run1=%d run2=%d", count1, count2)
	}
	if count1 == 0 {
		t.Fatal("expected at least one closed trade; TP should have fired")
	}
	for i := range pnls1 {
		if fmt.Sprintf("%.6f", pnls1[i]) != fmt.Sprintf("%.6f", pnls2[i]) {
			t.Errorf("netPnl[%d] mismatch: %.6f vs %.6f", i, pnls1[i], pnls2[i])
		}
	}
}

// TestPaperOMSTPFires confirms a TP exit fires when mark price crosses the
// calculated TP price and that net PnL is positive after fees.
func TestPaperOMSTPFires(t *testing.T) {
	count, pnls := runOMS(t, "tp-check")
	if count == 0 {
		t.Fatal("no closed trades: TP did not fire")
	}
	if pnls[0] <= 0 {
		t.Errorf("expected positive net PnL on TP exit, got %.4f", pnls[0])
	}
}

// TestPaperOMSSLFires confirms a SL exit fires and net PnL is negative.
func TestPaperOMSSLFires(t *testing.T) {
	oms := NewPaperOMS(10_000)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	req := OpenPositionRequest{
		Symbol:      "BTCUSDT",
		Side:        SideLong,
		EntryPrice:  60_000,
		Notional:    1_000,
		Leverage:    10,
		SLPct:       2.0, // 2% → SL at 58,800
		TPPct:       10.0,
		HoldMinutes: 120,
		StrategyID:  2,
	}
	if _, err := oms.OpenPosition(req, t0); err != nil {
		t.Fatalf("OpenPosition: %v", err)
	}

	closed := oms.Tick(OMSTick{Symbol: "BTCUSDT", MarkPrice: 58_700, NowUTC: t0.Add(5 * time.Minute)})
	if len(closed) == 0 {
		t.Fatal("SL did not fire when mark price breached stop")
	}
	if closed[0].NetPnl >= 0 {
		t.Errorf("expected negative net PnL on SL exit, got %.4f", closed[0].NetPnl)
	}
	if closed[0].ExitReason != ExitReasonSL {
		t.Errorf("expected exit reason %q, got %q", ExitReasonSL, closed[0].ExitReason)
	}
}

// TestPaperOMSTimeExpiry confirms a position closes with TIME exit when hold
// duration elapses.
func TestPaperOMSTimeExpiry(t *testing.T) {
	oms := NewPaperOMS(10_000)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	req := OpenPositionRequest{
		Symbol:      "BTCUSDT",
		Side:        SideLong,
		EntryPrice:  60_000,
		Notional:    1_000,
		Leverage:    10,
		SLPct:       10.0,
		TPPct:       20.0,
		HoldMinutes: 30,
		StrategyID:  3,
	}
	if _, err := oms.OpenPosition(req, t0); err != nil {
		t.Fatalf("OpenPosition: %v", err)
	}

	// Tick at 31 minutes; price unchanged → no SL/TP → TIME should fire.
	closed := oms.Tick(OMSTick{Symbol: "BTCUSDT", MarkPrice: 60_000, NowUTC: t0.Add(31 * time.Minute)})
	if len(closed) == 0 {
		t.Fatal("TIME exit did not fire after hold duration elapsed")
	}
	if closed[0].ExitReason != ExitReasonTime {
		t.Errorf("expected exit reason %q, got %q", ExitReasonTime, closed[0].ExitReason)
	}
}

// TestPaperOMSReset confirms Reset clears all open positions and restores equity.
func TestPaperOMSReset(t *testing.T) {
	oms := NewPaperOMS(10_000)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	req := OpenPositionRequest{
		Symbol: "BTCUSDT", Side: SideLong, EntryPrice: 60_000,
		Notional: 1_000, Leverage: 10, SLPct: 5.0, TPPct: 10.0, HoldMinutes: 60,
	}
	if _, err := oms.OpenPosition(req, t0); err != nil {
		t.Fatalf("OpenPosition: %v", err)
	}

	oms.Reset()
	snap := oms.State("BTCUSDT")
	if len(snap.OpenPositions) != 0 {
		t.Errorf("expected 0 open positions after Reset, got %d", len(snap.OpenPositions))
	}
	if snap.BalanceUSD != 10_000 {
		t.Errorf("expected balance restored to 10000 after Reset, got %.2f", snap.BalanceUSD)
	}
}
