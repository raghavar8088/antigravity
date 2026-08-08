package main

import (
	"math"
	"testing"
)

// The $100 live-account simulation, pinned against the real economics.
//
// The failure it prevents: the paper leaderboard showed ANTI_M1_DoubleTop_20bp_Short
// at +$157.19 while the same family's first real fills lost money. The board was
// computed at $1,000 notional with MAKER fees; the live desk runs $100 with
// TAKER fees on both legs. Nothing measured that gap, so paper rank was being
// read as evidence for spending real capital.

// A strategy whose average move is smaller than the round-trip fee must come out
// negative, however good its win rate looks.
func TestLiveSim_FeesInvertASmallEdgeStrategy(t *testing.T) {
	// 100 trades, each +0.05% gross — a plausible scalp edge, and smaller than
	// the 0.118% it costs to take liquidity twice.
	cs := &comboState{N: 100}
	perTrade := 0.0005
	cs.NetSum = float64(cs.N) * (perTrade - 2*makerFee)
	cs.FeeSum = float64(cs.N) * 2 * makerFee

	sim := simulateLiveAccount(cs)

	if sim.GrossUSD <= 0 {
		t.Fatalf("fixture wrong: gross should be positive, got %.2f", sim.GrossUSD)
	}
	if sim.NetUSD >= 0 {
		t.Errorf("a +0.05%%/trade strategy nets %+.2f on taker fees; it must be negative", sim.NetUSD)
	}
	if sim.FeesUSD <= sim.GrossUSD {
		t.Errorf("fees %.2f did not exceed gross %.2f on a sub-fee edge", sim.FeesUSD, sim.GrossUSD)
	}
	if sim.FeeDragPct <= 100 {
		t.Errorf("fee drag %.1f%% should exceed 100%% when fees outrun gross profit", sim.FeeDragPct)
	}
}

// A genuinely larger edge must survive, or the model rejects everything and is
// useless for promotion.
func TestLiveSim_AWiderEdgeSurvives(t *testing.T) {
	cs := &comboState{N: 100}
	perTrade := 0.004 // 0.4% per trade, comfortably above the 0.118% cost
	cs.NetSum = float64(cs.N) * (perTrade - 2*makerFee)
	cs.FeeSum = float64(cs.N) * 2 * makerFee

	sim := simulateLiveAccount(cs)
	if sim.NetUSD <= 0 {
		t.Errorf("a 0.4%%/trade strategy nets %+.2f; it should clear taker fees", sim.NetUSD)
	}
	if sim.ROIPct <= 0 {
		t.Errorf("ROI %.2f%% on a profitable strategy", sim.ROIPct)
	}
	// ROI is against the $100 account, not the notional.
	if math.Abs(sim.ROIPct-sim.NetUSD) > 0.02 {
		t.Errorf("ROI %.2f%% must be net %.2f on a $100 account", sim.ROIPct, sim.NetUSD)
	}
}

// The desk and the live path now charge the SAME fee, and that is the point.
//
// This asserted taker > the desk's maker round trip, because the desk modelled
// a maker entry at 0.02% while the live bridge paid taker on both legs. The
// restatement existed to close that gap. The gap is now closed at the source:
// the paper desk charges Delta's real 0.059% per side, both legs, so a paper
// result and a live result are quoted on the same terms.
//
// simulateLiveAccount is therefore a NO-OP on fees and stays only as a
// consistency check — if the two ever diverge again, this fails.
func TestLiveSim_DeskAndLivePathChargeTheSameFee(t *testing.T) {
	if 2*makerFee != liveSimTakerRoundTrip {
		t.Errorf("desk round trip %.5f != live round trip %.5f — the paper desk is measuring a cost the venue does not charge",
			2*makerFee, liveSimTakerRoundTrip)
	}
	if makerFee != takerFee {
		t.Errorf("maker %.5f and taker %.5f differ; the desk places no maker orders, so a split rate is fiction",
			makerFee, takerFee)
	}
	// And it must be the venue's real number, not a rounder one.
	if makerFee != 0.00059 {
		t.Errorf("fee rate %.5f, want Delta's 0.059%% (0.05%% + 18%% GST)", makerFee)
	}
}

// An untraded stream must report nothing, not a fabricated loss from fees on
// zero trades.
func TestLiveSim_NoTradesReportsNothing(t *testing.T) {
	if got := simulateLiveAccount(&comboState{}); got.NetUSD != 0 || got.FeesUSD != 0 {
		t.Errorf("an untraded stream reported net %.2f fees %.2f", got.NetUSD, got.FeesUSD)
	}
	if got := simulateLiveAccount(nil); got.NetUSD != 0 {
		t.Errorf("nil combo reported %.2f", got.NetUSD)
	}
}

// A losing strategy has no gross profit for fees to come out of. That must read
// as full drag, not as a blank that looks like "no fee problem".
func TestLiveSim_LosingStrategyStillReportsDrag(t *testing.T) {
	cs := &comboState{N: 50}
	cs.NetSum = -0.02
	cs.FeeSum = float64(cs.N) * 2 * makerFee
	sim := simulateLiveAccount(cs)
	if sim.NetUSD >= 0 {
		t.Fatalf("fixture wrong: expected a loss, got %+.2f", sim.NetUSD)
	}
	if sim.FeeDragPct <= 0 {
		t.Error("a losing strategy reported zero fee drag; it should not read as fee-free")
	}
}
