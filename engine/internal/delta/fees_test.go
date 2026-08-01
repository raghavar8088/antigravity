package delta

import (
	"math"
	"testing"
	"time"
)

// The desk's real numbers on 2026-07-28: BTC ~$63,870, option premiums quoted
// $24–$127 per BTC. Those are the inputs every assertion below is anchored to,
// so the tests fail if the fee model ever drifts from what the venue charges.
const (
	testSpot       = 63870.0
	testCheapQuote = 53.0   // observed live premium — 0.08% of spot
	testRichQuote  = 1277.4 // 2% of spot
)

func TestOptionFeeUSD_CapBindsOnCheapOptions(t *testing.T) {
	// Cheap option: 10%-of-premium cap is lower than the notional rate, so it wins.
	got := OptionFeeUSD(testCheapQuote, testSpot, 1)
	want := 0.053 * OptionFeeCapOfPremium
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("cheap option: expected the premium cap to bind at $%.6f, got $%.6f", want, got)
	}

	// Rich option: notional rate is lower, so the cap is irrelevant.
	got = OptionFeeUSD(testRichQuote, testSpot, 1)
	want = testSpot * OptionContractSizeBTC * OptionFeeRateOfNotional
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("rich option: expected the notional rate to bind at $%.6f, got $%.6f", want, got)
	}
}

func TestOptionFeeUSD_ScalesWithContracts(t *testing.T) {
	one := OptionFeeUSD(testCheapQuote, testSpot, 1)
	ten := OptionFeeUSD(testCheapQuote, testSpot, 10)
	if math.Abs(ten-10*one) > 1e-9 {
		t.Fatalf("fee must scale linearly with contracts: 1x=%.6f 10x=%.6f", one, ten)
	}
}

func TestOptionFeeUSD_UnknownSpotFallsBackToCap(t *testing.T) {
	// An adopted orphan has no entry spot. The cap alone must apply rather than
	// the fee silently becoming zero, which would flatter the P&L.
	got := OptionFeeUSD(testCheapQuote, 0, 1)
	want := 0.053 * OptionFeeCapOfPremium
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("unknown spot: expected cap $%.6f, got $%.6f", want, got)
	}
}

func TestRoundTripFeePct_CheapOptionCosts28Pct(t *testing.T) {
	// The number that explains the desk: a +80% target on a cap-bound option
	// costs 28% of the premium to round-trip.
	got := RoundTripFeePctOfEntryPremium(testCheapQuote, testSpot, 1, LiveTakeProfitPct)
	if math.Abs(got-28.0) > 0.01 {
		t.Fatalf("expected 28.0%% round-trip on a cap-bound option, got %.2f%%", got)
	}

	// A rich option pays the notional rate instead — nearly a tenth of the cost.
	got = RoundTripFeePctOfEntryPremium(testRichQuote, testSpot, 1, LiveTakeProfitPct)
	if math.Abs(got-3.0) > 0.05 {
		t.Fatalf("expected ~3.0%% round-trip on a 2%%-of-spot option, got %.2f%%", got)
	}
}

func TestRoundTripFeePct_IsScaleInvariant(t *testing.T) {
	// OnOpen prices the guard with a single contract on this assumption.
	one := RoundTripFeePctOfEntryPremium(testCheapQuote, testSpot, 1, LiveTakeProfitPct)
	many := RoundTripFeePctOfEntryPremium(testCheapQuote, testSpot, 37, LiveTakeProfitPct)
	if math.Abs(one-many) > 1e-9 {
		t.Fatalf("fee percentage must not depend on size: 1=%.6f 37=%.6f", one, many)
	}
}

func TestBreakEvenWinRate_ExplainsTheLosses(t *testing.T) {
	// +80%/-50% on a cap-bound option needs a 55.6% win rate purely to break
	// even. The live desk ran at 11%.
	got := BreakEvenWinRatePct(testCheapQuote, testSpot, 1, LiveTakeProfitPct, LiveStopLossPct)
	if math.Abs(got-55.56) > 0.1 {
		t.Fatalf("expected ~55.6%% break-even win rate, got %.2f%%", got)
	}

	// Moving up the premium curve is the single biggest structural improvement.
	got = BreakEvenWinRatePct(testRichQuote, testSpot, 1, LiveTakeProfitPct, LiveStopLossPct)
	if got >= 42 || got <= 39 {
		t.Fatalf("expected ~41%% break-even on a rich option, got %.2f%%", got)
	}
}

func TestEvaluateEntryEconomics_RejectsWhatTheDeskWasBuying(t *testing.T) {
	t.Setenv("DELTA_MAX_ROUNDTRIP_FEE_PCT", "8")

	e := EvaluateEntryEconomics(testCheapQuote, testSpot, 1, LiveTakeProfitPct)
	if e.Acceptable {
		t.Fatalf("a 28%%-fee option must be declined, got acceptable (%s)", e.Reason)
	}
	if e.MinPremiumPerBTC <= testCheapQuote {
		t.Fatalf("rejection must report a higher qualifying premium, got %.2f", e.MinPremiumPerBTC)
	}

	if e := EvaluateEntryEconomics(testRichQuote, testSpot, 1, LiveTakeProfitPct); !e.Acceptable {
		t.Fatalf("a 3%%-fee option must be accepted, got declined (%s)", e.Reason)
	}
}

func TestEvaluateEntryEconomics_ThresholdIsExact(t *testing.T) {
	t.Setenv("DELTA_MAX_ROUNDTRIP_FEE_PCT", "8")

	// The reported minimum premium must itself pass — otherwise the guard tells
	// the operator to do something that still fails.
	min := MinPremiumPerBTCForFeeLimit(testSpot, LiveTakeProfitPct, 8)
	if e := EvaluateEntryEconomics(min, testSpot, 1, LiveTakeProfitPct); !e.Acceptable {
		t.Fatalf("the advertised minimum premium %.2f must pass, got: %s", min, e.Reason)
	}
	if e := EvaluateEntryEconomics(min*0.9, testSpot, 1, LiveTakeProfitPct); e.Acceptable {
		t.Fatal("10% below the advertised minimum must fail")
	}
}

func TestEvaluateEntryEconomics_UnknownPremiumFailsClosed(t *testing.T) {
	// A signal that cannot be priced must not reach the broker. Failing open here
	// is the exact class of bug that let unpriced risk through before.
	if e := EvaluateEntryEconomics(0, testSpot, 1, LiveTakeProfitPct); e.Acceptable {
		t.Fatal("an unpriced entry must be declined")
	}
}

func TestEvaluateEntryEconomics_GuardCanBeDisabled(t *testing.T) {
	t.Setenv("DELTA_MAX_ROUNDTRIP_FEE_PCT", "0")
	if e := EvaluateEntryEconomics(testCheapQuote, testSpot, 1, LiveTakeProfitPct); !e.Acceptable {
		t.Fatalf("limit 0 must disable the guard, got: %s", e.Reason)
	}
}

func TestLiveStrategyRecord_IsNetOfFees(t *testing.T) {
	open := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	close1 := open.Add(3 * time.Hour)
	close2 := open.Add(27 * time.Hour)

	b := &Bridge{openByPaperID: map[string]string{}}
	b.trades = []LiveTrade{
		{
			StrategyName: "S", Status: "CLOSED", Contracts: 1,
			FillPrice: 100, OpenedAt: open, ClosedAt: &close1,
			GrossPnl: 0.080, EntryFeeUSD: 0.010, ExitFeeUSD: 0.018,
			RealizedPnl: 0.080 - 0.010 - 0.018, // +0.052
		},
		{
			StrategyName: "S", Status: "CLOSED", Contracts: 1,
			FillPrice: 100, OpenedAt: open, ClosedAt: &close2,
			GrossPnl: -0.050, EntryFeeUSD: 0.010, ExitFeeUSD: 0.005,
			RealizedPnl: -0.050 - 0.010 - 0.005, // -0.065
		},
		// A different strategy and an open trade must both be excluded.
		{StrategyName: "OTHER", Status: "CLOSED", RealizedPnl: 99, OpenedAt: open, ClosedAt: &close1},
		{StrategyName: "S", Status: "OPEN", RealizedPnl: 99, OpenedAt: open},
	}

	rec := b.LiveStrategyRecord("S")
	if rec.Fills != 2 {
		t.Fatalf("expected 2 closed fills, got %d", rec.Fills)
	}
	if rec.Wins != 1 || rec.Losses != 1 {
		t.Fatalf("expected 1W/1L, got %dW/%dL", rec.Wins, rec.Losses)
	}
	if want := 0.052 - 0.065; math.Abs(rec.NetPnl-want) > 1e-9 {
		t.Fatalf("net pnl: want %.6f got %.6f", want, rec.NetPnl)
	}
	if want := 0.043; math.Abs(rec.Fees-want) > 1e-9 {
		t.Fatalf("fees: want %.6f got %.6f", want, rec.Fees)
	}
	// Gross says this book is roughly flat; net says it loses. That difference is
	// the entire reason the gate must read net.
	if rec.GrossPnl <= rec.NetPnl {
		t.Fatal("gross must exceed net whenever fees were paid")
	}
	if rec.Expectancy >= 0 {
		t.Fatalf("expectancy must be negative net of fees, got %.6f", rec.Expectancy)
	}
	if pf := 0.052 / 0.065; math.Abs(rec.ProfitFactor-pf) > 1e-9 {
		t.Fatalf("profit factor: want %.6f got %.6f", pf, rec.ProfitFactor)
	}
	if rec.Days != 2 {
		t.Fatalf("expected the record to span 2 days, got %d", rec.Days)
	}
}

func TestLiveStrategyRecord_EmptyIsZeroNotLive(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}}
	rec := b.LiveStrategyRecord("nothing")
	if rec.Fills != 0 || rec.Days != 0 || rec.Expectancy != 0 {
		t.Fatalf("a strategy with no live history must report an empty record, got %+v", rec)
	}
}
