package delta

import (
	"context"
	"testing"
	"time"
)

// The live engine's first 20 trades lost money for a structural reason, not a
// signal reason: the paper strategy always closed winners around +15% while the
// custody -50% stop took losses in full. take_profit_80pct never fired once.
// These tests pin the fix — custody owns the upside — and pin the guards that
// the fix makes necessary.

func TestIsStrategyProfitCapExit_ClassifiesExits(t *testing.T) {
	// Profit-capping exits must be declined so the live +80% can be reached.
	for _, reason := range []string{"strategy_TP", "strategy_TRAIL_STOP", "strategy_PROFIT_LOCK"} {
		if !IsStrategyProfitCapExit(reason) {
			t.Errorf("%s caps a winner and must be suppressed on live positions", reason)
		}
	}
	// Loss-cutting and mandatory exits must still close the live position. A
	// strategy stop at ~-12% is strictly better than riding to the -50% custody
	// stop, so suppressing it would widen every loss.
	for _, reason := range []string{
		"strategy_SL", "strategy_STRIKE_PRESSURE", "strategy_LATE_EXIT", "strategy_EXPIRY",
		"take_profit_80pct", "stop_loss_50pct", "near_expiry_30min", "CLOSE_ALL", "",
	} {
		if IsStrategyProfitCapExit(reason) {
			t.Errorf("%s must still close the live position", reason)
		}
	}
}

// Suppressing a paper profit-take must leave the position fully managed — open,
// still mapped, and still closeable by the custody monitor. If suppression
// dropped the mapping it would strand real money with nothing watching it.
func TestOnClose_ProfitCapLeavesPositionUnderCustody(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}, configured: true}
	b.trades = []LiveTrade{{ID: "DLT-0001", PaperTradeID: "paper-1", Status: "OPEN"}}
	b.RegisterOpenMapping("paper-1", "DLT-0001")

	b.OnClose(CloseSignal{PaperTradeID: "paper-1", ExitReason: "strategy_TP"})

	if got := b.trades[0].Status; got != "OPEN" {
		t.Fatalf("position must stay OPEN through a paper profit-take, got %s", got)
	}
	if got := b.trades[0].ExitReason; got != "" {
		t.Errorf("a declined close must not record an exit reason, got %q", got)
	}
	if b.openByPaperID["paper-1"] != "DLT-0001" {
		t.Fatal("mapping dropped — the monitor could no longer close this real position")
	}
	// The custody monitor must still be able to close it afterwards.
	if idx := b.openIndexForPaperID("paper-1"); idx < 0 {
		t.Fatal("position became uncloseable after a declined profit-take")
	}
}

// A stop must never be declined. This is the asymmetry guard: if a future edit
// added SL to the suppression set, losses would run from ~-12% to -50%.
func TestOnClose_StopStillClosesLivePosition(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}, configured: true}
	b.trades = []LiveTrade{{ID: "DLT-0002", PaperTradeID: "paper-2", Status: "OPEN"}}
	b.RegisterOpenMapping("paper-2", "DLT-0002")

	b.OnClose(CloseSignal{PaperTradeID: "paper-2", ExitReason: "strategy_SL"})

	if b.trades[0].ExitReason != "strategy_SL" {
		t.Fatalf("a strategy stop must be honoured on the live leg, got %q", b.trades[0].ExitReason)
	}
	if _, still := b.openByPaperID["paper-2"]; still {
		t.Error("an honoured close must release the paper mapping")
	}
}

// pricedSignal builds an OpenSignal whose fee economics pass, so tests of other
// entry gates are not silently blocked by the entry-economics guard. The premium
// is rich enough that Delta's 10%-of-premium cap does not bind.
func pricedSignal(paperID, strategy string, strategyID int, expiry time.Time) OpenSignal {
	return OpenSignal{
		PaperTradeID:  paperID,
		StrategyID:    strategyID,
		StrategyName:  strategy,
		ExpiryTime:    expiry,
		PremiumPerBTC: testRichQuote,
		BTCPrice:      testSpot,
	}
}

// Entries inside the expiry floor lose by construction: they cannot reach +80%
// before near_expiry_30min force-closes them. Live went 0-for-3 on these.
func TestOnOpen_RejectsEntryTooCloseToExpiry(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}, configured: true, enabled: true, buyingMode: true}

	b.OnOpen(pricedSignal("paper-late", "any", 0, time.Now().Add(45*time.Minute)))
	if len(b.trades) != 0 {
		t.Fatalf("entry 45m from expiry must be declined, got %d live trade(s)", len(b.trades))
	}

	b.OnOpen(pricedSignal("paper-ok", "any", 0, time.Now().Add(6*time.Hour)))
	if len(b.trades) != 1 {
		t.Fatalf("entry 6h from expiry must be accepted, got %d live trade(s)", len(b.trades))
	}
}

// The entry-economics guard is an independent reason to decline: an option too
// cheap to round-trip profitably is refused even when every other gate passes.
func TestOnOpen_RejectsFeeToxicPremium(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}, configured: true, enabled: true, buyingMode: true}

	sig := pricedSignal("paper-cheap", "any", 0, time.Now().Add(6*time.Hour))
	sig.PremiumPerBTC = testCheapQuote // what the desk was actually buying
	b.OnOpen(sig)

	if len(b.trades) != 0 {
		t.Fatalf("a 28%%-round-trip-fee option must be declined, got %d live trade(s)", len(b.trades))
	}
}

// Declining a paper profit-take frees the strategy's paper slot, so it can open
// a fresh paper position while the live leg still runs. Without this guard that
// would stack a second real position on the same strategy.
func TestOnOpen_OneLivePositionPerStrategy(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}, configured: true, enabled: true, buyingMode: true}
	expiry := time.Now().Add(8 * time.Hour)

	// Wire a no-op open handler so the first trade STAYS "OPEN". Without one the
	// submit goroutine marks it FAILED, and the guard then correctly permits a
	// retry — a failed order is not a live position. That is the right production
	// behaviour, so the test has to supply a succeeding handler to exercise the
	// guard at all; previously it only passed because OnOpen held the write lock
	// across the whole function and hid the interleaving.
	b.SetInstitutionalOpenHandler(func(context.Context, OpenSignal, string) error { return nil })

	b.OnOpen(pricedSignal("paper-1", "s7", 7, expiry))
	b.OnOpen(pricedSignal("paper-2", "s7", 7, expiry))

	if len(b.trades) != 1 {
		t.Fatalf("strategy 7 must hold at most one live position, got %d", len(b.trades))
	}
	// A different strategy is unaffected.
	b.OnOpen(pricedSignal("paper-3", "s8", 8, expiry))
	if len(b.trades) != 2 {
		t.Fatalf("a second strategy must still be able to open, got %d", len(b.trades))
	}
	// Once the live leg closes, the strategy may open again.
	b.trades[0].Status = "CLOSED"
	b.OnOpen(pricedSignal("paper-4", "s8", 8, expiry))
	if len(b.trades) != 3 {
		t.Fatalf("strategy 8 must reopen after its live leg closed, got %d", len(b.trades))
	}
}
