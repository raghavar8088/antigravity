package main

import "testing"

// The scalp desk was exiting 91.2% of trades on the time-stop, averaging -$0.33,
// because the stop sat at 1.00% while the median price move was 0.288%. The
// levels were correctly sized in dollars ($10.91 SL, $23.89 TP) and simply never
// reached, so the desk's P&L was decided by the timeout rather than by its own
// risk levels.
//
// These tests pin BOTH halves of the fix: the dollar risk stays meaningful, and
// the price move stays inside the range the market actually delivers.

// Measured over 500 live trades on the deployed desk.
const (
	measuredMedianMovePct = 0.288
	measuredP90MovePct    = 1.090
)

// Every profile must risk a MEANINGFUL share of the account — and not too much.
//
// This asserted an absolute ">= $10" stop, which was right when the desk ran a
// $3,000 notional against a nominal $1,000-per-strategy account: $10 was ~1% of
// equity. Each strategy now runs a real $100 account, and holding the $10
// literal would demand 10% of equity on a single scalp — the opposite of what
// the test was protecting. The intent was "the level must matter", so it is
// stated against equity, where it survives any re-basing of the notional.
//
// The upper bound is new and is the half the old assertion could not express: a
// stop can fail by being too large just as easily as by being decorative.
const (
	minStopFractionOfEquity = 0.01 // >= 1% of the account, or the level is noise
	maxStopFractionOfEquity = 0.05 // <= 5%, or one bad trade dominates the record
)

func TestProfiles_RiskIsMeaningfulShareOfAccount(t *testing.T) {
	for name, p := range profiles {
		slMin := p.SLMin * defaultNotionalUSD
		slMax := p.SLMax * defaultNotionalUSD
		tpMin := p.TPMin * defaultNotionalUSD

		if got := slMin / liveSimEquityUSD; got < minStopFractionOfEquity {
			t.Errorf("%s: minimum stop risks %.2f%% of the $%.0f account ($%.2f) — below %.0f%%, the level is decorative",
				name, got*100, liveSimEquityUSD, slMin, minStopFractionOfEquity*100)
		}
		if got := slMax / liveSimEquityUSD; got > maxStopFractionOfEquity {
			t.Errorf("%s: maximum stop risks %.2f%% of the $%.0f account ($%.2f) — above %.0f%%, one trade dominates",
				name, got*100, liveSimEquityUSD, slMax, maxStopFractionOfEquity*100)
		}
		// A target below the stop is a negative-expectancy profile whatever the
		// win rate says.
		if tpMin <= slMin {
			t.Errorf("%s: minimum target $%.2f does not exceed minimum stop $%.2f", name, tpMin, slMin)
		}
	}
}

// The round-trip taker fee must not eat the smallest target.
//
// This is the test the desk most needed and never had: at $300 notional a taker
// round trip is $0.354, and the tightest scalp target is $2.10. That ratio -
// not the win rate - is what decides whether a strategy can pay for itself.
func TestProfiles_SmallestTargetClearsTheRoundTripFee(t *testing.T) {
	fee := defaultNotionalUSD * liveSimTakerRoundTrip
	for name, p := range profiles {
		tpMin := p.TPMin * defaultNotionalUSD
		if tpMin <= fee*2 {
			t.Errorf("%s: smallest target $%.2f is under 2x the $%.3f round-trip fee — "+
				"the profile cannot clear its own costs", name, tpMin, fee)
		}
	}
}

// A level the market never reaches is not a risk control — it is a decoration,
// and the time-stop silently becomes the real exit.
func TestProfiles_StopIsReachableWithinObservedMoves(t *testing.T) {
	for name, p := range profiles {
		slPct := p.SLMin * 100
		if slPct > measuredP90MovePct {
			t.Errorf("%s: stop at %.3f%% is beyond the p90 move of %.3f%% — it would almost never fire",
				name, slPct, measuredP90MovePct)
		}
		// It should also not be so tight that ordinary noise stops every trade
		// out before the setup has room to work.
		if slPct < measuredMedianMovePct/2 {
			t.Errorf("%s: stop at %.3f%% is below half the median move %.3f%% — noise would stop every trade",
				name, slPct, measuredMedianMovePct)
		}
	}
}

// The target has to be reachable too, or every winner decays into a timeout.
func TestProfiles_TargetIsReachableWithinObservedMoves(t *testing.T) {
	for name, p := range profiles {
		tpPct := p.TPMin * 100
		// Allow the runner profile more room — that is its whole purpose — but
		// nothing may sit beyond roughly twice the p90 observed move.
		if tpPct > measuredP90MovePct*2 {
			t.Errorf("%s: target at %.3f%% is more than 2x the p90 move %.3f%% — winners will time out instead",
				name, tpPct, measuredP90MovePct)
		}
	}
}

// Reward-to-risk must be at least 1:2 on every profile. The old bands produced
// roughly 1:2 in percentage terms but resolved so rarely that realised R:R was
// set by timeouts, not by the levels.
func TestProfiles_RewardToRiskIsTheHouseRatio(t *testing.T) {
	want := map[string]float64{"scalp": 2.0, "revert": 3.0, "runner": 3.0}
	for name, p := range profiles {
		rr := p.TPMin / p.SLMin
		if rr < 1.99 {
			t.Errorf("%s: reward-to-risk %.2f is below 1:2", name, rr)
		}
		if w, ok := want[name]; ok && rr < w-0.01 {
			t.Errorf("%s: reward-to-risk %.2f, want ~%.1f", name, rr, w)
		}
	}
}

// The max of each band must exceed its min, or clamp() silently inverts.
func TestProfiles_BandsAreWellFormed(t *testing.T) {
	for name, p := range profiles {
		if p.SLMax <= p.SLMin {
			t.Errorf("%s: SLMax %.4f <= SLMin %.4f", name, p.SLMax, p.SLMin)
		}
		if p.TPMax <= p.TPMin {
			t.Errorf("%s: TPMax %.4f <= TPMin %.4f", name, p.TPMax, p.TPMin)
		}
		if p.TTLBars <= 0 {
			t.Errorf("%s: TTLBars %d must be positive", name, p.TTLBars)
		}
	}
}

// The time-stop must give the target a chance to be reached. A target needing a
// 0.70% move with a 15-bar timeout is a timeout dressed as a target.
func TestProfiles_TTLGivesTargetRoomToResolve(t *testing.T) {
	for name, p := range profiles {
		if p.TTLBars < 45 {
			t.Errorf("%s: TTL %d bars is too short for a %.2f%% target to resolve",
				name, p.TTLBars, p.TPMin*100)
		}
	}
}
