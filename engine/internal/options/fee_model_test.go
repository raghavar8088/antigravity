package options

import (
	"math"
	"testing"
)

// The desk charged NOTHING until 2026-08-15 and reported the result as
// "netPnl". Every strategy qualified here was qualified on a number it could
// never trade, and the Live Engine went to real money on that basis and bled.
//
// These pin the fee model itself, because the cap is the part that is easy to
// get wrong and the part that decides the desk.

const feeEpsilon = 1e-9

// On a rich premium the percentage-of-notional term binds and the cap is
// irrelevant. This is the ordinary case and it should stay cheap.
func TestOptionFee_NotionalTermBindsOnRichPremium(t *testing.T) {
	// $2.00 premium per BTC, 1 BTC, spot $60,000.
	// notional fee = 0.0003 x 60000 x 1 = $18.00
	// cap          = 0.10 x 2.00 x 1    = $0.20  -> cap binds, actually
	got := optionFeeUSD(60_000, 2.00, 1)
	if math.Abs(got-0.20) > feeEpsilon {
		t.Fatalf("fee = %v; want the 10%%-of-premium cap at 0.20", got)
	}
}

// The cap is not an edge case on this desk — it binds on essentially every
// contract it trades, because BTC option premiums are small relative to a
// $60,000+ underlying. A model that only charged 0.03% of notional would
// OVERCHARGE here, and one that only charged 10% of premium would undercharge
// on deep-ITM contracts. Both terms have to exist.
func TestOptionFee_CapBindsOnCheapPremium(t *testing.T) {
	// $0.19 premium — the figure from the Live Engine post-mortem.
	// notional fee = 0.0003 x 60000 x 1 = $18.00
	// cap          = 0.10 x 0.19 x 1    = $0.019
	got := optionFeeUSD(60_000, 0.19, 1)
	want := 0.019
	if math.Abs(got-want) > feeEpsilon {
		t.Fatalf("fee = %v; want %v (10%% of premium)", got, want)
	}
}

// The number that killed the real-money desk: a round trip on a cheap option
// costs 20% of the premium before the market has moved at all. If this ever
// stops being true, the desk has stopped modelling the venue.
func TestOptionFee_RoundTripOnCheapOptionCostsFifthOfPremium(t *testing.T) {
	premium, qty := 0.19, 1.0
	entry := optionFeeUSD(60_000, premium, qty)
	exit := optionFeeUSD(60_000, premium, qty)
	roundTrip := entry + exit

	premiumPaid := premium * qty
	share := roundTrip / premiumPaid * 100
	if math.Abs(share-20) > 0.001 {
		t.Fatalf("round trip is %.2f%% of premium; want 20%% — the cap is 10%% per side", share)
	}
}

// A deep-ITM contract is where the notional term is the smaller of the two and
// therefore the one that should be charged.
func TestOptionFee_NotionalTermBindsWhenPremiumIsLarge(t *testing.T) {
	// $10,000 premium per BTC: cap = $1,000, notional fee = $18.
	got := optionFeeUSD(60_000, 10_000, 1)
	if math.Abs(got-18) > feeEpsilon {
		t.Fatalf("fee = %v; want 18.00 (0.03%% of notional)", got)
	}
}

// Degenerate inputs must not invent a charge. A zero-quantity or unpriced
// position is not a trade.
func TestOptionFee_ZeroInputsChargeNothing(t *testing.T) {
	for _, tc := range []struct{ spot, premium, qty float64 }{
		{0, 1, 1},
		{60_000, 1, 0},
		{60_000, 1, -1},
	} {
		if got := optionFeeUSD(tc.spot, tc.premium, tc.qty); got != 0 {
			t.Errorf("optionFeeUSD(%v, %v, %v) = %v; want 0", tc.spot, tc.premium, tc.qty, got)
		}
	}
}

// Drag is a share of PROFIT, not of premium or notional. A fee that looks
// trivial against notional can still be most of the edge.
func TestFeeDragPct_IsShareOfGrossProfit(t *testing.T) {
	if got := feeDragPct(10, 4); math.Abs(got-40) > feeEpsilon {
		t.Errorf("feeDragPct(10, 4) = %v; want 40", got)
	}
	// Above 100% means the trade earned less than it cost to make.
	if got := feeDragPct(2, 3); math.Abs(got-150) > feeEpsilon {
		t.Errorf("feeDragPct(2, 3) = %v; want 150", got)
	}
}

// Losers report 0 drag rather than a negative percentage. "-140% drag" on a
// losing trade reads as a good number at a glance, which is the opposite of
// what this column exists to communicate.
func TestFeeDragPct_LosersReportZeroNotNegative(t *testing.T) {
	if got := feeDragPct(-5, 2); got != 0 {
		t.Errorf("feeDragPct(-5, 2) = %v; want 0 — drag is undefined without profit", got)
	}
	if got := feeDragPct(0, 2); got != 0 {
		t.Errorf("feeDragPct(0, 2) = %v; want 0", got)
	}
}
