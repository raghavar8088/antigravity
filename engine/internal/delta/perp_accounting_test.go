package delta

import (
	"math"
	"testing"
)

// These tests are the audit, encoded.
//
// On 2026-08-01 the bridge reported +$0.9424 for a day the venue recorded as
// -$3.5405. The $4.4829 overstatement decomposed exactly into four defects, all
// biasing the same way. Each is pinned below against the real numbers, so any
// one of them coming back fails a test rather than a month of trading.

// DEFECT 1 — closes booked at the trigger MARK, not the fill.
//
// The real case: a 1,099-contract ADAUSD short. The exit decision triggered at a
// mark of 0.17263166; the market/ioc order actually filled at 0.17290. Booking
// the mark recorded +$0.0751. The venue recorded -$0.1395. A sign flip.
func TestPerpAccounting_MarkVersusFillFlipsTheSign(t *testing.T) {
	const (
		entry     = 0.17270
		markAtTP  = 0.17263166 // what triggered the exit, and what was booked
		actualFil = 0.17290    // where the market/ioc order actually filled
		contracts = 1099
		cv        = 1.0 // ADAUSD
	)

	booked := ComputePerpResult(entry, markAtTP, contracts, cv, false)
	real := ComputePerpResult(entry, actualFil, contracts, cv, false)

	// GROSS is where the flip happens: the mark said +$0.0751, the fill said
	// -$0.1395. That alone inverted the trade.
	if booked.Gross <= 0 {
		t.Fatalf("the mark should look like a win, got %.4f", booked.Gross)
	}
	if real.Gross >= 0 {
		t.Fatalf("the fill should be a loss, got %.4f", real.Gross)
	}
	if math.Abs(booked.Gross-0.0751) > 0.01 {
		t.Errorf("mark-based gross = %.4f, the bridge reported +0.0751", booked.Gross)
	}
	// NOT asserted against the venue's per-order figure, and the reason is a
	// finding in its own right: Delta NETS every ADAUSD order into ONE position
	// with ONE average entry, while the bridge tracks a separate position per
	// strategy on the same symbol. Three strategies were short ADAUSD at once
	// here, so the venue's realised for this order (-0.13955) is measured
	// against a blended basis, not against this strategy's own entry (-0.21980).
	//
	// Per-strategy attribution is therefore APPROXIMATE whenever two strategies
	// hold the same symbol; only the account total reconciles exactly. That is a
	// property of the venue, not a bug to fix, and it belongs in the open rather
	// than hidden behind a tolerance.
	if real.Gross >= booked.Gross {
		t.Errorf("the fill-based gross (%.5f) should be worse than the mark-based one (%.5f)",
			real.Gross, booked.Gross)
	}

	// And a second, INDEPENDENT inversion: fees alone would have made even the
	// mark-based figure negative. The reported +$0.0751 required BOTH defects —
	// the wrong price and the missing fees — to survive as a profit.
	if booked.Net >= 0 {
		t.Errorf("with fees the mark-based trade still reports %+.4f; it should already be a loss", booked.Net)
	}
	if real.Net >= booked.Net {
		t.Errorf("the fill-based net (%+.4f) should be worse than the mark-based net (%+.4f)",
			real.Net, booked.Net)
	}
}

// DEFECT 4 — fees were never subtracted. They are only 0.059% of notional, but
// these strategies target a few cents per trade, so a round trip's fees are
// comparable to the whole signal. Reporting gross as net does not shade the
// result, it can invert it.
func TestPerpAccounting_FeesAreSubtractedAndCanInvertATrade(t *testing.T) {
	// A trade that is genuinely positive gross but smaller than its own fees.
	r := ComputePerpResult(0.17290, 0.17291, 1099, 1.0, true)
	if r.Gross <= 0 {
		t.Fatalf("fixture wrong: gross should be positive, got %.6f", r.Gross)
	}
	if r.EntryFee <= 0 || r.ExitFee <= 0 {
		t.Fatal("no fees charged on a real round trip")
	}
	if r.Net >= r.Gross {
		t.Error("net is not below gross; fees were not subtracted")
	}
	if r.Net >= 0 {
		t.Errorf("a +%.6f gross trade whose fees are %.6f still reports net %+.6f",
			r.Gross, r.EntryFee+r.ExitFee, r.Net)
	}
}

// The fee rate is derived from the venue's own order log, not from docs. If
// Delta changes it, this fails rather than quietly mis-stating every trade.
func TestPerpAccounting_FeeRateMatchesTheVenuesOrderLog(t *testing.T) {
	// order value 286.2528 -> fee 0.16888916  (1632 ADAUSD @ 0.17540)
	got := PerpFeeUSD(0.17540, 1632, 1.0)
	if math.Abs(got-0.16888916) > 0.002 {
		t.Errorf("fee on a 286.25 notional order = %.6f, venue charged 0.16888916", got)
	}
	// order value 57.189 -> fee 0.03374151  (330 ADAUSD @ 0.17330)
	got = PerpFeeUSD(0.17330, 330, 1.0)
	if math.Abs(got-0.03374151) > 0.001 {
		t.Errorf("fee on a 57.19 notional order = %.6f, venue charged 0.03374151", got)
	}
}

// Fees must scale with the SYMBOL's contract value. BNBUSD is 0.1, so a
// 4-contract order is 0.4 BNB of notional, not 4.
func TestPerpAccounting_FeesUseTheSymbolsContractValue(t *testing.T) {
	// BNBUSD 4 contracts @ 578.64 -> notional 231.456, venue fee 0.13655904.
	got := PerpFeeUSD(578.64, 4, 0.1)
	if math.Abs(got-0.13655904) > 0.002 {
		t.Errorf("BNBUSD fee = %.6f, venue charged 0.13655904 — check the contract value", got)
	}
}

// A short's gross must be signed the other way, or every short is booked as its
// own opposite.
func TestPerpAccounting_ShortGrossIsSigned(t *testing.T) {
	long := ComputePerpResult(100, 101, 10, 1.0, true)
	short := ComputePerpResult(100, 101, 10, 1.0, false)
	if long.Gross <= 0 {
		t.Errorf("long that rose booked %.4f", long.Gross)
	}
	if short.Gross >= 0 {
		t.Errorf("short that rose booked %.4f", short.Gross)
	}
	if math.Abs(long.Gross+short.Gross) > 1e-9 {
		t.Errorf("long and short gross do not negate: %.4f vs %.4f", long.Gross, short.Gross)
	}
}

// DEFECT 2 — an externally-closed position must never book as a flat zero.
// A liquidation at -$0.8632 was recorded as $0.00 because the code booked the
// ENTRY price. Zero is a meaningful result; "unknown" is not zero.
func TestPerpAccounting_UnreconciledIsNotZero(t *testing.T) {
	if ExitReasonUnreconciled == "" {
		t.Fatal("no marker for an unreconciled close; it would book as an ordinary result")
	}
	if ExitReasonUnreconciled == ExitReasonLiquidated {
		t.Error("liquidation and unreconciled must stay distinct — a liquidation means the MARGIN model was wrong, not the strategy")
	}
}

// DEFECT (systemic) — nothing compared the bridge against the venue. Every
// check verified the bridge against itself, so all four defects agreed with each
// other for a full day.
func TestPerpAccounting_ReconcilerCatchesTheDriftItMissed(t *testing.T) {
	// The exact numbers from the audit.
	r := ReconcilePerpPnL(0.9424, -3.5405, 4)
	if r.Matched {
		t.Fatal("a $4.48 drift on a $110 account was reported as matched")
	}
	if math.Abs(r.DriftUSD-4.4829) > 0.001 {
		t.Errorf("drift = %.4f, want 4.4829", r.DriftUSD)
	}

	// Agreement within a cent is agreement — the tolerance must not be so tight
	// that rounding makes the alarm meaningless.
	if !ReconcilePerpPnL(1.0000, 0.9950, 3).Matched {
		t.Error("a half-cent difference was reported as drift")
	}
}

// THE LIQUIDATION. Two positions were force-closed by Delta at EXACTLY 0.500%
// adverse while their own stops sat at 0.93% and 0.98%. ADAUSD ships at
// default_leverage 100 with maintenance_margin 0.5%, so the liquidation price
// sat INSIDE the stop — the venue closed every losing trade before the
// strategy's risk management could act.
func TestPerpAccounting_DefaultLeverageMakesStopsUnreachable(t *testing.T) {
	// The account default that caused it.
	if d := LiquidationDistanceFraction(100, 0.5); math.Abs(d-0.005) > 1e-9 {
		t.Fatalf("at 100x the liquidation distance is %.4f, want the 0.500%% observed", d)
	}
	// A real trade from that day: short 664 ADAUSD @ 0.17290, stop 0.17451.
	if StopIsReachable(0.17290, 0.17451, 100, 0.5) {
		t.Error("a 0.93% stop was judged reachable at 100x, where liquidation is 0.5% away")
	}
	// And the fix.
	if !StopIsReachable(0.17290, 0.17451, PerpLeverage, 0.5) {
		t.Errorf("the same stop is still unreachable at %dx; leverage is not low enough", PerpLeverage)
	}
}

// The configured leverage must leave the widest stop this desk uses a wide
// margin, not a marginal one.
func TestPerpAccounting_ConfiguredLeverageClearsTheWidestStop(t *testing.T) {
	const widestStopFrac = 0.0098 // the runner profile's ~0.98%
	liq := LiquidationDistanceFraction(PerpLeverage, perpMaintenanceMarginPctForTest)
	if liq < widestStopFrac*liquidationSafetyFactor {
		t.Fatalf("liquidation at %.2f%% leaves no room for a %.2f%% stop at %dx safety",
			liq*100, widestStopFrac*100, int(liquidationSafetyFactor))
	}
}

const perpMaintenanceMarginPctForTest = 0.5

// A malformed input must refuse rather than permit.
func TestPerpAccounting_StopReachabilityFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		entry, stop float64
		lev         int
	}{
		{0, 0.17, 10}, {0.17, 0, 10}, {0.17, 0.169, 0},
	} {
		if StopIsReachable(tc.entry, tc.stop, tc.lev, 0.5) {
			t.Errorf("entry=%v stop=%v lev=%v was permitted", tc.entry, tc.stop, tc.lev)
		}
	}
}
