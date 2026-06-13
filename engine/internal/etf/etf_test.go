package etf

import (
	"testing"
)

func TestComputeETFScore_VeryBullish(t *testing.T) {
	data := ETFFlowData{TotalFlowUSD: 1_500_000_000}
	if s := ComputeETFScore(data); s != 3.0 {
		t.Errorf("expected 3.0, got %v", s)
	}
}

func TestComputeETFScore_Bullish(t *testing.T) {
	data := ETFFlowData{TotalFlowUSD: 600_000_000}
	if s := ComputeETFScore(data); s != 2.0 {
		t.Errorf("expected 2.0, got %v", s)
	}
}

func TestComputeETFScore_Bearish(t *testing.T) {
	data := ETFFlowData{TotalFlowUSD: -400_000_000}
	if s := ComputeETFScore(data); s != -2.0 {
		t.Errorf("expected -2.0, got %v", s)
	}
}

func TestComputeETFScore_StreakBonus(t *testing.T) {
	data := ETFFlowData{TotalFlowUSD: 600_000_000, ConsecutiveInflow: 5}
	s := ComputeETFScore(data)
	if s != 3.0 { // clamped: 2 + 1 = 3
		t.Errorf("expected 3.0, got %v", s)
	}
}

func TestComputeETFScore_StreakPenalty(t *testing.T) {
	data := ETFFlowData{TotalFlowUSD: -400_000_000, ConsecutiveOutflow: 5}
	s := ComputeETFScore(data)
	if s != -3.0 { // clamped: -2 - 1 = -3
		t.Errorf("expected -3.0, got %v", s)
	}
}

func TestComputeETFScore_Neutral(t *testing.T) {
	data := ETFFlowData{TotalFlowUSD: 50_000_000}
	if s := ComputeETFScore(data); s != 0.0 {
		t.Errorf("expected 0.0, got %v", s)
	}
}

func TestClamp(t *testing.T) {
	if clamp(5.0, -3, 3) != 3.0 {
		t.Error("clamp upper bound failed")
	}
	if clamp(-5.0, -3, 3) != -3.0 {
		t.Error("clamp lower bound failed")
	}
	if clamp(1.5, -3, 3) != 1.5 {
		t.Error("clamp within bounds failed")
	}
}

func TestGetLatest_NilBeforeFetch(t *testing.T) {
	f := NewETFFetcher("python3", "etf_fetcher.py")
	if f.GetLatest() != nil {
		t.Error("expected nil before first fetch")
	}
}
