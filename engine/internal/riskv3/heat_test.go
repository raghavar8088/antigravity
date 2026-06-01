package riskv3

import (
	"math"
	"testing"
	"time"
)

func makeSnapshot(positions []PositionRecord, equityUSD float64) PortfolioSnapshot {
	return PortfolioSnapshot{
		Positions:  positions,
		EquityUSD:  equityUSD,
		HWMUSD:     equityUSD,
		SnapshotAt: time.Now().UTC(),
	}
}

func makePosition(id, side string, entry, stop, size float64) PositionRecord {
	notional := entry * size
	if side == "SELL" {
		notional = -notional
	}
	return PositionRecord{
		ID:           id,
		Symbol:       "BTCUSDT",
		Side:         side,
		EntryPrice:   entry,
		StopLoss:     stop,
		Size:         size,
		NotionalUSD:  math.Abs(notional),
		StrategyName: "test-strategy",
		Exchange:     "binance",
	}
}

func TestComputeHeat_Empty(t *testing.T) {
	snap := makeSnapshot(nil, 1_000_000)
	result := ComputeHeat(snap)
	if result.HeatPct != 0 {
		t.Errorf("empty portfolio: want heat=0 got %.4f", result.HeatPct)
	}
	if result.HeatLevel != HeatNormal {
		t.Errorf("empty portfolio: want NORMAL got %s", result.HeatLevel)
	}
}

func TestComputeHeat_SingleLong(t *testing.T) {
	// entry=50000, stop=49500, size=0.4 BTC
	// dollar risk = (50000-49500)*0.4 = 500*0.4 = 200 USD
	// heat% = 200/100000 * 100 = 0.2%
	pos := makePosition("p1", "BUY", 50000, 49500, 0.4)
	snap := makeSnapshot([]PositionRecord{pos}, 100_000)
	result := ComputeHeat(snap)

	expectedDollarRisk := (50000 - 49500) * 0.4
	if math.Abs(result.TotalDollarRiskUSD-expectedDollarRisk) > 0.01 {
		t.Errorf("TotalDollarRisk: want %.2f got %.2f", expectedDollarRisk, result.TotalDollarRiskUSD)
	}

	expectedHeat := expectedDollarRisk / 100_000 * 100
	if math.Abs(result.HeatPct-expectedHeat) > 0.001 {
		t.Errorf("HeatPct: want %.4f got %.4f", expectedHeat, result.HeatPct)
	}
}

func TestComputeHeat_SingleShort(t *testing.T) {
	// short: stop-entry = 50500-50000 = 500, size=0.2 → dollar risk = 100 USD
	pos := makePosition("p1", "SELL", 50000, 50500, 0.2)
	snap := makeSnapshot([]PositionRecord{pos}, 100_000)
	result := ComputeHeat(snap)

	expectedDollarRisk := (50500 - 50000) * 0.2 // 100 USD
	if math.Abs(result.TotalDollarRiskUSD-expectedDollarRisk) > 0.01 {
		t.Errorf("Short TotalDollarRisk: want %.2f got %.2f", expectedDollarRisk, result.TotalDollarRiskUSD)
	}
}

func TestComputeHeat_WarningLevel(t *testing.T) {
	// Construct heat at warning level: 10% of 100k = $10k dollar risk
	// entry=50000, stop=45000 (10% below), size=2 → dr = 5000*2 = $10k
	pos := makePosition("p1", "BUY", 50000, 45000, 2.0)
	snap := makeSnapshot([]PositionRecord{pos}, 100_000)
	result := ComputeHeat(snap)

	if result.HeatLevel != HeatWarning && result.HeatLevel != HeatCritical {
		t.Errorf("Expected WARNING or CRITICAL heat level, got %s (heat=%.2f%%)",
			result.HeatLevel, result.HeatPct)
	}
	if !result.WarningBreached {
		t.Errorf("WarningBreached should be true at heat=%.2f%%", result.HeatPct)
	}
}

func TestComputeHeat_KillLevel(t *testing.T) {
	// Heat >= 20%: entry=50000, stop=40000 (20% below), size=1 → dr=$10k on $50k equity
	// heat = 10000/50000*100 = 20%
	pos := makePosition("p1", "BUY", 50000, 40000, 1.0)
	snap := makeSnapshot([]PositionRecord{pos}, 50_000)
	result := ComputeHeat(snap)

	if result.HeatLevel != HeatKill {
		t.Errorf("Expected KILL heat level at %.2f%%, got %s", result.HeatPct, result.HeatLevel)
	}
	if !result.KillBreached {
		t.Error("KillBreached should be true")
	}
}

func TestComputeHeatWithProposal(t *testing.T) {
	// Existing: heat=0%, proposed adds $5k risk on $100k equity → projected heat=5%
	snap := makeSnapshot(nil, 100_000)
	result := ComputeHeatWithProposal(snap, 5_000)

	if math.Abs(result.ProposedHeatPct-5.0) > 0.001 {
		t.Errorf("ProposedHeatPct: want 5.0%% got %.4f%%", result.ProposedHeatPct)
	}
	if result.ProposedHeatLevel != HeatNormal {
		t.Errorf("ProposedHeatLevel: want NORMAL got %s", result.ProposedHeatLevel)
	}
}

func TestPositionDollarRisk_Long(t *testing.T) {
	// entry=50000, stop=49000, size=0.5 → dr = 1000*0.5 = 500
	dr := PositionDollarRisk(50000, 49000, 0.5, "BUY")
	if math.Abs(dr-500) > 0.001 {
		t.Errorf("Long dollar risk: want 500 got %.2f", dr)
	}
}

func TestPositionDollarRisk_Short(t *testing.T) {
	// short: stop=51000, entry=50000 → dist=1000, size=0.3 → dr=300
	dr := PositionDollarRisk(50000, 51000, 0.3, "SELL")
	if math.Abs(dr-300) > 0.001 {
		t.Errorf("Short dollar risk: want 300 got %.2f", dr)
	}
}

func TestPositionDollarRisk_NegativeDistance(t *testing.T) {
	// stop placed on wrong side of entry (protective but logically invalid)
	dr := PositionDollarRisk(50000, 51000, 0.5, "BUY") // stop above entry for long
	if dr != 0 {
		t.Errorf("Negative distance should produce 0 dollar risk, got %.2f", dr)
	}
}

func TestStrategyHeatPct(t *testing.T) {
	positions := []PositionRecord{
		{ID: "p1", StrategyName: "EMA_Cross", EntryPrice: 50000, StopLoss: 49000, Size: 0.2, Side: "BUY"},
		{ID: "p2", StrategyName: "EMA_Cross", EntryPrice: 50000, StopLoss: 49500, Size: 0.2, Side: "BUY"},
		{ID: "p3", StrategyName: "RSI_Rev",   EntryPrice: 50000, StopLoss: 49000, Size: 0.1, Side: "BUY"},
	}
	snap := makeSnapshot(positions, 100_000)

	emaCrossHeat := StrategyHeatPct(snap, "EMA_Cross")
	// p1: (50000-49000)*0.2=200, p2: (50000-49500)*0.2=100 → total=300 → 0.3%
	expectedEMA := 300.0 / 100_000 * 100
	if math.Abs(emaCrossHeat-expectedEMA) > 0.001 {
		t.Errorf("EMA_Cross heat: want %.4f%% got %.4f%%", expectedEMA, emaCrossHeat)
	}

	rsiHeat := StrategyHeatPct(snap, "RSI_Rev")
	// p3: (50000-49000)*0.1=100 → 0.1%
	if math.Abs(rsiHeat-0.1) > 0.001 {
		t.Errorf("RSI_Rev heat: want 0.1%% got %.4f%%", rsiHeat)
	}
}

func TestClassifyHeat(t *testing.T) {
	cases := []struct {
		heat     float64
		expected HeatLevel
	}{
		{0, HeatNormal},
		{9.9, HeatNormal},
		{10.0, HeatWarning},
		{14.9, HeatWarning},
		{15.0, HeatCritical},
		{19.9, HeatCritical},
		{20.0, HeatKill},
		{50.0, HeatKill},
	}
	for _, tc := range cases {
		got := ClassifyHeat(tc.heat)
		if got != tc.expected {
			t.Errorf("ClassifyHeat(%.1f): want %s got %s", tc.heat, tc.expected, got)
		}
	}
}
