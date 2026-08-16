package main

import (
	"math"
	"testing"
)

// The profile table is written at 1:3 — the TP band is exactly 3x the SL band.
// Everything about scaling the ratio depends on that being true, so it is
// checked rather than assumed: if someone retunes the table without updating
// baselineRewardRisk, SCALP_REWARD_RISK would silently produce a ratio nobody
// asked for.
func TestProfileTableIsWrittenAtBaseline(t *testing.T) {
	for name, cfg := range profiles {
		if got := cfg.TPMin / cfg.SLMin; math.Abs(got-baselineRewardRisk) > 1e-9 {
			t.Errorf("%s: TPMin/SLMin = %.4f, want baselineRewardRisk %.1f", name, got, baselineRewardRisk)
		}
		if got := cfg.TPMax / cfg.SLMax; math.Abs(got-baselineRewardRisk) > 1e-9 {
			t.Errorf("%s: TPMax/SLMax = %.4f, want baselineRewardRisk %.1f", name, got, baselineRewardRisk)
		}
	}
}

// Scaling to 1:5 and 1:6 must move the TARGET only. The stop bands were
// measured against where price actually goes; moving them would change how
// often a trade resolves at all, which is a different decision from changing
// the payoff.
func TestScaleProfileTargets(t *testing.T) {
	for _, ratio := range []float64{5.0, 6.0} {
		base := map[string]profileCfg{
			"scalp":  {2.5, 3.5, 0.0035, 0.0060, 0.0105, 0.0180, 60},
			"revert": {3.0, 2.0, 0.0035, 0.0060, 0.0105, 0.0180, 45},
			"runner": {2.5, 6.0, 0.0035, 0.0060, 0.0105, 0.0180, 120},
		}
		if !scaleProfileTargets(base, ratio) {
			t.Fatalf("ratio %.1f: reported no change", ratio)
		}
		for name, cfg := range base {
			if got := cfg.TPMin / cfg.SLMin; math.Abs(got-ratio) > 1e-9 {
				t.Errorf("%s at 1:%.0f: TPMin/SLMin = %.4f, want %.1f", name, ratio, got, ratio)
			}
			if got := cfg.TPMax / cfg.SLMax; math.Abs(got-ratio) > 1e-9 {
				t.Errorf("%s at 1:%.0f: TPMax/SLMax = %.4f, want %.1f", name, ratio, got, ratio)
			}
			// Stops untouched.
			if cfg.SLMin != 0.0035 || cfg.SLMax != 0.0060 {
				t.Errorf("%s at 1:%.0f: stop band moved to [%.4f,%.4f], must stay [0.0035,0.0060]",
					name, ratio, cfg.SLMin, cfg.SLMax)
			}
			// TTL untouched — the holding period is a separate decision.
			if name == "scalp" && cfg.TTLBars != 60 {
				t.Errorf("scalp TTLBars changed to %d, want 60", cfg.TTLBars)
			}
		}
		// Relative profile character survives: runner still reaches furthest.
		if base["runner"].TPATR <= base["scalp"].TPATR {
			t.Errorf("at 1:%.0f runner TPATR %.3f no longer exceeds scalp %.3f",
				ratio, base["runner"].TPATR, base["scalp"].TPATR)
		}
	}
}

// A ratio equal to the baseline, zero, or negative must leave the table alone
// rather than scaling it by a nonsense factor.
func TestScaleProfileTargetsNoOp(t *testing.T) {
	for _, ratio := range []float64{baselineRewardRisk, 0, -2} {
		base := map[string]profileCfg{"scalp": {2.5, 3.5, 0.0035, 0.0060, 0.0105, 0.0180, 60}}
		if scaleProfileTargets(base, ratio) {
			t.Errorf("ratio %.1f: reported a change, want no-op", ratio)
		}
		if got := base["scalp"].TPMin; got != 0.0105 {
			t.Errorf("ratio %.1f: TPMin moved to %.4f, want 0.0105", ratio, got)
		}
	}
}
