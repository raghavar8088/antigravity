package riskv3

import (
	"math"
	"testing"
)

func TestComputeKelly_ProfitableStrategy(t *testing.T) {
	// 55% win rate, 1.5:1 reward/risk — classic trend-following profile
	result := ComputeKelly(KellyInput{
		WinRate: 0.55,
		AvgWin:  1.5,
		AvgLoss: 1.0,
		Capital: 1_000_000,
	})

	// Full Kelly = p - q/b = 0.55 - 0.45/1.5 ≈ 0.25
	expectedFull := 0.55 - (0.45 / 1.5)
	if math.Abs(result.FullKellyFraction-expectedFull) > 0.001 {
		t.Errorf("FullKelly: want %.4f got %.4f", expectedFull, result.FullKellyFraction)
	}

	// Half Kelly ≈ 0.125
	expectedHalf := expectedFull * 0.5
	if math.Abs(result.HalfKellyFraction-expectedHalf) > 0.001 {
		t.Errorf("HalfKelly: want %.4f got %.4f", expectedHalf, result.HalfKellyFraction)
	}

	// Recommended should be half-Kelly * 100 capped at MaxKellyFractionPct (5%)
	if result.RecommendedFractionPct > MaxKellyFractionPct {
		t.Errorf("RecommendedFractionPct %.2f%% exceeds cap %.2f%%",
			result.RecommendedFractionPct, MaxKellyFractionPct)
	}

	// RecommendedUSD should be Capital * fraction/100
	expectedUSD := 1_000_000 * result.RecommendedFractionPct / 100
	if math.Abs(result.RecommendedUSD-expectedUSD) > 1 {
		t.Errorf("RecommendedUSD: want %.2f got %.2f", expectedUSD, result.RecommendedUSD)
	}

	// KellyEdge should be positive for a profitable strategy
	if result.KellyEdge <= 0 {
		t.Errorf("KellyEdge should be positive for WR=55%%, b=1.5, got %.4f", result.KellyEdge)
	}

	// ProfitFactor > 1 for profitable strategy
	if result.ProfitFactor <= 1 {
		t.Errorf("ProfitFactor should be > 1 for profitable strategy, got %.4f", result.ProfitFactor)
	}
}

func TestComputeKelly_NegativeEdge(t *testing.T) {
	// 40% win rate, 1:1 reward/risk — negative edge
	result := ComputeKelly(KellyInput{
		WinRate: 0.40,
		AvgWin:  1.0,
		AvgLoss: 1.0,
		Capital: 1_000_000,
	})

	if result.FullKellyFraction != 0 {
		t.Errorf("FullKelly should be 0 for negative edge, got %.4f", result.FullKellyFraction)
	}
	if result.KellyEdge > 0 {
		t.Errorf("KellyEdge should be <= 0 for negative edge, got %.4f", result.KellyEdge)
	}
	if result.RecommendedUSD != 0 {
		t.Errorf("RecommendedUSD should be 0 for negative edge, got %.2f", result.RecommendedUSD)
	}
}

func TestComputeKelly_CapEnforced(t *testing.T) {
	// 80% win rate, 3:1 — would produce very high Kelly; must be capped
	result := ComputeKelly(KellyInput{
		WinRate: 0.80,
		AvgWin:  3.0,
		AvgLoss: 1.0,
		Capital: 1_000_000,
	})

	if result.RecommendedFractionPct > MaxKellyFractionPct {
		t.Errorf("cap not enforced: %.2f%% > %.2f%%", result.RecommendedFractionPct, MaxKellyFractionPct)
	}
	if !result.CapApplied {
		t.Error("CapApplied should be true for 80%% win rate / 3:1 reward")
	}
}

func TestComputeKelly_Symmetry(t *testing.T) {
	// QuarterKelly should be exactly half of HalfKelly
	result := ComputeKelly(KellyInput{WinRate: 0.55, AvgWin: 2.0, AvgLoss: 1.0, Capital: 100_000})
	if math.Abs(result.QuarterKellyFraction-result.HalfKellyFraction*0.5) > 1e-9 {
		t.Errorf("QuarterKelly (%.6f) ≠ HalfKelly*0.5 (%.6f)",
			result.QuarterKellyFraction, result.HalfKellyFraction*0.5)
	}
}

func TestKellyFromStats_Basic(t *testing.T) {
	// 100 trades, 55 wins, avg win $100, avg loss $80
	result := KellyFromStats(100, 55, 100, 80, 1_000_000)

	if result.WinRate != 0.55 {
		t.Errorf("WinRate: want 0.55 got %.4f", result.WinRate)
	}
	if result.RewardRiskRatio <= 0 {
		t.Error("RewardRiskRatio should be positive")
	}
}

func TestFractionalKellyUSD_Modes(t *testing.T) {
	result := ComputeKelly(KellyInput{WinRate: 0.55, AvgWin: 1.5, AvgLoss: 1.0, Capital: 1_000_000})
	capital := 1_000_000.0

	fullUSD := FractionalKellyUSD(result, "full", capital)
	halfUSD := FractionalKellyUSD(result, "half", capital)
	quarterUSD := FractionalKellyUSD(result, "quarter", capital)

	// Half should be less than full
	if halfUSD > fullUSD && !result.CapApplied {
		t.Errorf("halfUSD (%.2f) should be <= fullUSD (%.2f)", halfUSD, fullUSD)
	}
	// Quarter should be <= half
	if quarterUSD > halfUSD {
		t.Errorf("quarterUSD (%.2f) should be <= halfUSD (%.2f)", quarterUSD, halfUSD)
	}
	// All should be <= MaxKellyFractionPct% of capital
	maxUSD := capital * MaxKellyFractionPct / 100
	for _, usd := range []float64{fullUSD, halfUSD, quarterUSD} {
		if usd > maxUSD+1 { // +1 for float64 rounding
			t.Errorf("USD %.2f exceeds cap %.2f", usd, maxUSD)
		}
	}
}

func TestRiskOfRuin_HighEdge(t *testing.T) {
	// High edge → low risk of ruin
	result := ComputeKelly(KellyInput{WinRate: 0.65, AvgWin: 2.0, AvgLoss: 1.0, Capital: 1_000_000})
	if result.RiskOfRuinPct > 50 {
		t.Errorf("high-edge strategy should have low ruin risk, got %.2f%%", result.RiskOfRuinPct)
	}
}

func TestRiskOfRuin_NegativeEdge(t *testing.T) {
	// Negative edge → 100% risk of ruin
	result := ComputeKelly(KellyInput{WinRate: 0.35, AvgWin: 1.0, AvgLoss: 1.0, Capital: 1_000_000})
	if result.RiskOfRuinPct != 100 {
		t.Errorf("negative edge should give 100%% ruin risk, got %.2f%%", result.RiskOfRuinPct)
	}
}
