package dominance

import (
	"testing"
)

func TestComputeDominanceScore_GenuineStrength(t *testing.T) {
	data := DominanceData{Trend: "RISING", Delta24h: 0.5}
	s := ComputeDominanceScore(data, "UP")
	if s != 2.0 {
		t.Errorf("expected 2.0, got %v", s)
	}
}

func TestComputeDominanceScore_BroadSelloff(t *testing.T) {
	data := DominanceData{Trend: "FALLING", Delta24h: 0.5}
	s := ComputeDominanceScore(data, "DOWN")
	if s != -2.0 {
		t.Errorf("expected -2.0, got %v", s)
	}
}

func TestComputeDominanceScore_StrongTrendBonus(t *testing.T) {
	data := DominanceData{Trend: "RISING", Delta24h: 2.0}
	s := ComputeDominanceScore(data, "UP")
	// 2.0 × 1.3 = 2.6, clamped to 2.6
	if s < 2.5 || s > 3.0 {
		t.Errorf("expected ~2.6, got %v", s)
	}
}

func TestComputeDominanceScore_Flat(t *testing.T) {
	data := DominanceData{Trend: "FLAT"}
	s := ComputeDominanceScore(data, "UP")
	if s != 0.0 {
		t.Errorf("expected 0.0, got %v", s)
	}
}

func TestEMA(t *testing.T) {
	vals := []float64{50, 51, 52, 53, 54}
	e := ema(vals, 3)
	if e < 52 || e > 55 {
		t.Errorf("EMA out of expected range: %v", e)
	}
}
