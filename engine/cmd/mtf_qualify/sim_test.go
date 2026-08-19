package main

import (
	"math"
	"testing"
	"time"

	scalpers "antigravity-engine/internal/strategy/scalpers"
)

func series(n int, start, drift float64) []scalpers.Candle {
	out := make([]scalpers.Candle, n)
	p := start
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range out {
		open := p
		p *= 1 + drift
		out[i] = scalpers.Candle{
			OpenTime: t0.Add(time.Duration(i) * time.Hour),
			Open:     open,
			Close:    p,
			High:     math.Max(open, p) * 1.001,
			Low:      math.Min(open, p) * 0.999,
			Volume:   100,
		}
	}
	return out
}

// spyStrategy records the largest OpenTime it was ever shown.
type spyStrategy struct {
	seenLatest time.Time
	seenCount  int
	emit       func(i int) scalpers.Signal
	calls      int
}

func (s *spyStrategy) Name() string                    { return "SPY" }
func (s *spyStrategy) ValidRegimes() []scalpers.Regime { return nil }
func (s *spyStrategy) Evaluate(ctx scalpers.MarketContext) scalpers.Signal {
	c := ctx.Candles1h
	s.seenCount = len(c)
	if len(c) > 0 && c[len(c)-1].OpenTime.After(s.seenLatest) {
		s.seenLatest = c[len(c)-1].OpenTime
	}
	s.calls++
	if s.emit != nil {
		return s.emit(len(c) - 1)
	}
	return scalpers.Signal{Direction: scalpers.DirectionNone}
}

// THE test. A strategy must never be shown a bar that has not closed.
//
// Every other number this harness produces is worthless if this fails, and it
// would fail silently: a backtest with lookahead does not error, it just
// reports an edge. The check is on the DATA the strategy receives rather than
// on the result it produces, because a strategy cannot be trusted to report
// having cheated.
func TestSimulate_StrategyNeverSeesAnUnclosedBar(t *testing.T) {
	c := series(400, 100, 0.001)
	spy := &spyStrategy{}
	Simulate(spy, "SPY", "TEST", scalpers.TF1h, c)

	if spy.calls == 0 {
		t.Fatal("strategy was never evaluated; the test proves nothing")
	}
	// The loop stops at len(c)-2 so an entry bar always exists, so the newest
	// bar the strategy may ever see is the second to last.
	newestAllowed := c[len(c)-2].OpenTime
	if spy.seenLatest.After(newestAllowed) {
		t.Errorf("strategy saw a bar opening %s; the newest it may see is %s — this is lookahead",
			spy.seenLatest, newestAllowed)
	}
}

// The context must hold exactly the bars up to i, never the whole series.
func TestCtxFor_ExposesOnlyClosedBars(t *testing.T) {
	c := series(50, 100, 0.001)
	for _, i := range []int{0, 7, 33, 49} {
		ctx := ctxFor(scalpers.TF1h, c, i)
		if got := len(ctx.Candles1h); got != i+1 {
			t.Errorf("at i=%d the context held %d candles, want %d", i, got, i+1)
		}
		if ctx.Price != c[i].Close {
			t.Errorf("at i=%d price was %v, want the close of bar i (%v)", i, ctx.Price, c[i].Close)
		}
		// Only the strategy's own timeframe is populated.
		if len(ctx.Candles1m) != 0 || len(ctx.Candles1d) != 0 {
			t.Error("a timeframe the strategy does not trade was populated")
		}
	}
}

// Entry must be the NEXT bar's open, never the signal bar's close.
//
// The difference is a whole bar of hindsight on every trade, and on a flat
// series it is invisible — which is why it is asserted directly.
func TestSimulate_EntersAtTheNextBarOpen(t *testing.T) {
	c := series(300, 100, 0.0)
	fireAt := 200
	spy := &spyStrategy{emit: func(i int) scalpers.Signal {
		if i != fireAt {
			return scalpers.Signal{Direction: scalpers.DirectionNone}
		}
		p := c[i].Close
		return scalpers.Signal{
			Direction:  scalpers.DirectionLong,
			StopLoss:   p * 0.99,
			TakeProfit: p * 1.07, // 7% up against 1% down -> rr 7, clears 1:6
		}
	}}
	res := Simulate(spy, "SPY", "TEST", scalpers.TF1h, c)
	if len(res.Trades) != 1 {
		t.Fatalf("got %d trades, want 1 (signals=%d rejectedRR=%d rejectedBad=%d)",
			len(res.Trades), res.Signals, res.RejectedRR, res.RejectedBad)
	}
	if got, want := res.Trades[0].Entry, c[fireAt+1].Open; got != want {
		t.Errorf("entry %v, want the next bar's open %v", got, want)
	}
}

// A bar containing BOTH levels must resolve as a STOP.
//
// Nothing in OHLC says which extreme came first. Taking the target would turn
// every wide bar into a 6R win, and at a 1:6 target the wide bars are exactly
// the ambiguous ones — so this single choice moves the headline result more
// than any other assumption in the harness.
func TestWalk_AmbiguousBarResolvesAsAStop(t *testing.T) {
	base := series(10, 100, 0)
	// One enormous bar that spans both levels.
	base[1].High = 200
	base[1].Low = 50
	tr, _ := walk(base, 1, 100, 90, 160, true)
	if tr.Reason != "SL" {
		t.Errorf("a bar spanning both levels resolved as %s; it must resolve as SL", tr.Reason)
	}
}

// Costs must be charged on every trade, with no exemption.
func TestFinish_ChargesTheRoundTrip(t *testing.T) {
	tr := finish(Trade{Entry: 100, Exit: 106, Stop: 99}, true)
	if math.Abs(tr.GrossPct-6.0) > 1e-9 {
		t.Errorf("gross %.4f, want 6.0", tr.GrossPct)
	}
	if want := 6.0 - roundTripCostPct; math.Abs(tr.NetPct-want) > 1e-9 {
		t.Errorf("net %.4f, want %.4f — the round trip was not charged", tr.NetPct, want)
	}
	// R is measured on the NET return, not the gross one.
	if wantR := (6.0 - roundTripCostPct) / 1.0; math.Abs(tr.RMultiple-wantR) > 1e-9 {
		t.Errorf("R %.4f, want %.4f (net/risk)", tr.RMultiple, wantR)
	}
}

// A short must be priced in the opposite direction.
func TestFinish_ShortDirection(t *testing.T) {
	tr := finish(Trade{Entry: 100, Exit: 94, Stop: 101}, false)
	if math.Abs(tr.GrossPct-6.0) > 1e-9 {
		t.Errorf("short gross %.4f, want +6.0 — price fell and this is a short", tr.GrossPct)
	}
}

// The 1:6 bar must REFUSE, and the refusal must be counted rather than dropped.
//
// The count is the measurement. Without it a strategy that fires 900 times and
// takes 3 trades is indistinguishable from one that fires 3 times, and the
// whole question of whether 1:6 filters or redefines cannot be answered.
func TestSimulate_SubSixRRIsRefusedAndCounted(t *testing.T) {
	c := series(300, 100, 0.0)
	spy := &spyStrategy{emit: func(i int) scalpers.Signal {
		if i != 200 {
			return scalpers.Signal{Direction: scalpers.DirectionNone}
		}
		p := c[i].Close
		return scalpers.Signal{
			Direction:  scalpers.DirectionLong,
			StopLoss:   p * 0.99,
			TakeProfit: p * 1.03, // rr 3 — below the bar
		}
	}}
	res := Simulate(spy, "SPY", "TEST", scalpers.TF1h, c)
	if len(res.Trades) != 0 {
		t.Errorf("a 1:3 signal was traded under a 1:6 eligibility bar")
	}
	if res.RejectedRR != 1 {
		t.Errorf("RejectedRR = %d, want 1 — the refusal must be counted, not silently dropped", res.RejectedRR)
	}
	if res.Signals != 1 {
		t.Errorf("Signals = %d, want 1", res.Signals)
	}
}

// Only one position at a time per stream.
//
// Without this a strategy signalling on every bar of a trend books the same
// move repeatedly and reports it as independent wins — the single easiest way
// to manufacture a spectacular backtest.
func TestSimulate_NoOverlappingPositions(t *testing.T) {
	c := series(400, 100, 0.0)
	spy := &spyStrategy{emit: func(i int) scalpers.Signal {
		p := c[i].Close
		return scalpers.Signal{ // fires on EVERY bar
			Direction:  scalpers.DirectionLong,
			StopLoss:   p * 0.99,
			TakeProfit: p * 1.07,
		}
	}}
	res := Simulate(spy, "SPY", "TEST", scalpers.TF1h, c)
	for i := 1; i < len(res.Trades); i++ {
		if !res.Trades[i].OpenedAt.After(res.Trades[i-1].ClosedAt) {
			t.Fatalf("trade %d opened at %s before trade %d closed at %s — positions overlap",
				i, res.Trades[i].OpenedAt, i-1, res.Trades[i-1].ClosedAt)
		}
	}
}
