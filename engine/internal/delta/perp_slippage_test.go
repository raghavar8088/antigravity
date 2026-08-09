package delta

import (
	"math"
	"testing"
)

// The stop's limit must be marketable without eating the risk budget.
//
// Measured live on COOKIEUSD: entry 0.01251, stop 0.01259 — a 0.64% stop. The
// previous constant put the limit 0.5% of PRICE past the trigger, which is
// 0.78x the entire stop distance, and the order filled at 0.012650 against a
// limit of 0.012653. The stop worked; it cost 1.75x what it was meant to.
func TestStopLimitSlip_CannotSwallowTheRiskBudget(t *testing.T) {
	entry, stop, tick := 0.01251, 0.01259, 0.00001
	risk := stop - entry

	slip := stopLimitSlip(entry, stop, tick)
	if slip > risk*0.25 {
		t.Errorf("slip %.8f is %.2fx the stop distance; a stop-out must not cost materially more than planned",
			slip, slip/risk)
	}

	// The old behaviour, asserted so it cannot come back.
	if old := stop * 0.005; slip >= old {
		t.Errorf("slip %.8f did not improve on the price-proportional %.8f", slip, old)
	}

	// Worst-case realised loss, as a multiple of intended.
	if got := (risk + slip) / risk; got > 1.25 {
		t.Errorf("worst-case stop-out is %.2fx intended; the cap is 1.25x", got)
	}
}

// On a coarse grid a percentage of a tiny risk budget rounds to nothing, which
// puts the limit back ON the trigger — the unfillable stop this mechanism
// exists to prevent.
func TestStopLimitSlip_StaysMarketableOnACoarseGrid(t *testing.T) {
	tick := 0.00001
	// A stop only 3 ticks wide: 20% of it is 0.6 of a tick, which formats away.
	entry, stop := 0.01250, 0.01253
	slip := stopLimitSlip(entry, stop, tick)
	if slip < tick*2 {
		t.Errorf("slip %.8f is under two ticks; it would round onto the trigger and may never fill", slip)
	}
}

// Direction is not this helper's job, but the magnitude must not depend on it —
// a long and a short with the same risk deserve the same allowance.
func TestStopLimitSlip_SymmetricAcrossDirection(t *testing.T) {
	tick := 0.00001
	long := stopLimitSlip(0.01259, 0.01251, tick)  // stop BELOW entry
	short := stopLimitSlip(0.01251, 0.01259, tick) // stop ABOVE entry
	if math.Abs(long-short) > 1e-12 {
		t.Errorf("long %.8f vs short %.8f — allowance must not depend on direction", long, short)
	}
}
