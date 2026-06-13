package montecarlo

import (
	"testing"
	"time"
)

func baseInput() SimInput {
	return SimInput{
		EntryPrice:     50000,
		StopLoss:       49000, // 2% SL
		TakeProfit1:    51000, // 2% TP1
		TakeProfit2:    52500, // 5% TP2
		Bias:           "BUY",
		PositionPct:    0.02,
		PortfolioValue: 1_000_000,
		ATR14:          800,
		Regime:         "TRENDING_BULL",
		NSims:          1000,
	}
}

func TestSimulate_BasicBuy(t *testing.T) {
	r := Simulate(baseInput())
	if r.SimCount != 1000 {
		t.Errorf("expected 1000 sims, got %d", r.SimCount)
	}
	if r.ProbSLHit < 0 || r.ProbSLHit > 1 {
		t.Errorf("ProbSLHit out of range: %v", r.ProbSLHit)
	}
	if r.ProbTP1Hit < 0 || r.ProbTP1Hit > 1 {
		t.Errorf("ProbTP1Hit out of range: %v", r.ProbTP1Hit)
	}
	if r.PositionModifier != 1.0 && r.PositionModifier != 0.6 {
		t.Errorf("unexpected PositionModifier: %v", r.PositionModifier)
	}
}

func TestSimulate_TightSLHighVolatility(t *testing.T) {
	inp := baseInput()
	inp.StopLoss = 49900    // very tight 0.2% SL
	inp.Regime = "HIGH_VOLATILITY"
	r := Simulate(inp)
	// High volatility + tight SL should result in high SL hit probability.
	if r.ProbSLHit < 0.40 {
		t.Errorf("expected high SL hit probability with tight SL + high vol, got %v", r.ProbSLHit)
	}
}

func TestSimulate_NegativeEV_Blocked(t *testing.T) {
	inp := baseInput()
	inp.StopLoss = 49500  // 1% SL
	inp.TakeProfit1 = 50200
	inp.TakeProfit2 = 50300 // very small TP
	inp.ATR14 = 2000        // extreme volatility → high SL hit
	r := Simulate(inp)
	// With extreme volatility and small TP, EV may be negative.
	if r.ShouldTrade && r.ExpectedValue < 0 {
		t.Error("negative EV trade should be blocked")
	}
	if !r.ShouldTrade && r.BlockReason == "" {
		t.Error("blocked trade must have a block reason")
	}
}

func TestSimulate_DefaultNSims(t *testing.T) {
	inp := baseInput()
	inp.NSims = 0
	r := Simulate(inp)
	if r.SimCount != 1000 {
		t.Errorf("expected default 1000 sims, got %d", r.SimCount)
	}
}

func BenchmarkSimulate1000(b *testing.B) {
	inp := baseInput()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Simulate(inp)
	}
}

func TestSimulate_PerformanceUnder100ms(t *testing.T) {
	inp := baseInput()
	start := time.Now()
	Simulate(inp)
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("1000 sims took %v, exceeds 100ms target", elapsed)
	}
}

func TestPercentile(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5}
	if p := percentile(sorted, 0); p != 1 {
		t.Errorf("p0 expected 1, got %v", p)
	}
	if p := percentile(sorted, 100); p != 5 {
		t.Errorf("p100 expected 5, got %v", p)
	}
	p50 := percentile(sorted, 50)
	if p50 < 2.9 || p50 > 3.1 {
		t.Errorf("p50 expected ~3, got %v", p50)
	}
}
