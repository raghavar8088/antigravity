package delta

import (
	"math"
	"testing"
)

// What a position pays at its target and costs at its stop, NET of fees.
//
// The round trip is charged whichever way the trade goes, so it shrinks the win
// AND deepens the loss — it does not cancel between them. Gross figures would
// overstate the reward and understate the risk simultaneously, on a desk where
// the fee is comparable to the move being targeted.
func TestPerpOutcomeUSD_FeesShrinkTheWinAndDeepenTheLoss(t *testing.T) {
	tr := &PerpLiveTrade{
		Side: SideSell, EntryPrice: 0.12375, Contracts: 239,
		StopPrice: 0.12435, TargetPrice: 0.12195,
	}
	const cv = 10.0
	win, loss := perpOutcomeUSD(tr, cv)

	if win <= 0 {
		t.Errorf("target outcome %+.4f; it must be a gain", win)
	}
	if loss >= 0 {
		t.Errorf("stop outcome %+.4f; it must be a loss", loss)
	}

	qty := 239 * cv
	grossWin := math.Abs(0.12195-0.12375) * qty
	grossLoss := math.Abs(0.12435-0.12375) * qty
	if win >= grossWin {
		t.Errorf("net win %.4f is not below gross %.4f — fees were not subtracted", win, grossWin)
	}
	if -loss <= grossLoss {
		t.Errorf("net loss %.4f is not worse than gross %.4f — fees must deepen it", -loss, grossLoss)
	}

	// The asymmetry itself: the fee moves both numbers the same direction.
	if (grossWin-win) <= 0 || ((-loss)-grossLoss) <= 0 {
		t.Error("fees did not both shrink the win and deepen the loss")
	}
}

// A 1:3 position must still show roughly 1:3 in dollars — noticeably less after
// fees, but not inverted.
func TestPerpOutcomeUSD_RewardStillExceedsRiskAtOneToThree(t *testing.T) {
	tr := &PerpLiveTrade{
		Side: SideBuy, EntryPrice: 100, Contracts: 3,
		StopPrice: 99.3, TargetPrice: 102.1, // 0.7% risk, 2.1% reward
	}
	win, loss := perpOutcomeUSD(tr, 1)
	if win <= 0 || loss >= 0 {
		t.Fatalf("fixture wrong: win %.4f loss %.4f", win, loss)
	}
	if ratio := win / -loss; ratio < 2.0 {
		t.Errorf("net reward:risk is 1:%.2f on a 1:3 position — fees should erode it, not halve it", ratio)
	}
}

// Missing inputs must produce zero, not a number computed from a guess.
func TestPerpOutcomeUSD_RefusesIncompleteInputs(t *testing.T) {
	full := &PerpLiveTrade{Side: SideBuy, EntryPrice: 100, Contracts: 3, StopPrice: 99, TargetPrice: 103}
	for name, tr := range map[string]*PerpLiveTrade{
		"nil":          nil,
		"no entry":     {Side: SideBuy, Contracts: 3, StopPrice: 99, TargetPrice: 103},
		"no contracts": {Side: SideBuy, EntryPrice: 100, StopPrice: 99, TargetPrice: 103},
	} {
		if w, l := perpOutcomeUSD(tr, 1); w != 0 || l != 0 {
			t.Errorf("%s produced (%v, %v); incomplete input must yield zero", name, w, l)
		}
	}
	if w, l := perpOutcomeUSD(full, 0); w != 0 || l != 0 {
		t.Errorf("an unknown contract value produced (%v, %v)", w, l)
	}
	// A position with only one level set reports only that one.
	noTgt := *full
	noTgt.TargetPrice = 0
	if w, l := perpOutcomeUSD(&noTgt, 1); w != 0 || l >= 0 {
		t.Errorf("no target should give (0, negative), got (%v, %v)", w, l)
	}
}
