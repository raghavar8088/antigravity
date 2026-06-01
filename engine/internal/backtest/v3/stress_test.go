package v3

import (
	"testing"
	"time"
)

func TestStressEngine_FlashCrash_Active(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	engine := NewStressEngine([]StressScenarioType{StressFlashCrash}, start)
	// Midpoint during flash crash.
	mid := start.Add(2 * time.Minute)
	active := engine.ActiveAt(mid)
	if len(active) == 0 {
		t.Fatal("flash crash should be active 2 minutes after start")
	}
}

func TestStressEngine_FlashCrash_Inactive_After(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	engine := NewStressEngine([]StressScenarioType{StressFlashCrash}, start)
	after := start.Add(10 * time.Minute) // flash crash is 5 min
	active := engine.ActiveAt(after)
	if len(active) != 0 {
		t.Fatalf("flash crash should be inactive 10 minutes after start, got %v", active)
	}
}

func TestStressEngine_ExchangeOutage_BlocksOrders(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	engine := NewStressEngine([]StressScenarioType{StressExchangeOutage}, start)
	mid := start.Add(10 * time.Minute)
	if !engine.IsOrderBlocked(mid) {
		t.Fatal("orders should be blocked during exchange outage")
	}
}

func TestStressEngine_Multipliers_Increase_Spread_Decrease_Depth(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	engine := NewStressEngine([]StressScenarioType{StressLiquidityCollapse}, start)
	mid := start.Add(5 * time.Minute)
	spreadMult, depthMult := engine.CompositeMultipliers(mid)
	if spreadMult <= 1 {
		t.Fatalf("spread multiplier should be > 1 during liquidity collapse, got %f", spreadMult)
	}
	if depthMult >= 1 {
		t.Fatalf("depth multiplier should be < 1 during liquidity collapse, got %f", depthMult)
	}
}

func TestStressEngine_AddAt_CustomTime(t *testing.T) {
	engine := &StressEngine{}
	ts := time.Date(2024, 6, 1, 9, 0, 0, 0, time.UTC)
	err := engine.AddAt(StressFundingShock, ts)
	if err != nil {
		t.Fatalf("AddAt returned error: %v", err)
	}
	active := engine.ActiveAt(ts.Add(1 * time.Hour))
	if len(active) == 0 {
		t.Fatal("funding shock should be active 1h after start")
	}
}

func TestStressEngine_PriceAdjustment_Negative(t *testing.T) {
	start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	engine := NewStressEngine([]StressScenarioType{StressFlashCrash}, start)
	adj := engine.PriceAdjustment(start.Add(1 * time.Minute))
	if adj >= 0 {
		t.Fatalf("flash crash price adjustment should be negative, got %f", adj)
	}
}

func TestLiquidityEngine_March2020_SpreadsWiden(t *testing.T) {
	normal := NewLiquidityEngine(ScenarioNormal)
	crisis := NewLiquidityEngine(ScenarioMarch2020)

	book := NewOrderBook(50000, 0.20, 0.65, time.Now())
	normalSpread := book.SpreadPriceAbs()

	// Apply normal (no change).
	normalBook := NewOrderBook(50000, 0.20, 0.65, time.Now())
	normal.AdjustBook(normalBook)
	crisisBook := NewOrderBook(50000, 0.20, 0.65, time.Now())
	crisis.AdjustBook(crisisBook)

	crisisSpread := crisisBook.SpreadPriceAbs()
	normalSpread2 := normalBook.SpreadPriceAbs()

	if crisisSpread <= normalSpread {
		t.Fatalf("March 2020 spread (%f) should exceed normal (%f)", crisisSpread, normalSpread)
	}
	_ = normalSpread2
}

func TestLiquidityEngine_SlippageAdder_PositiveInCrisis(t *testing.T) {
	normal := NewLiquidityEngine(ScenarioNormal)
	crisis := NewLiquidityEngine(ScenarioFTX)
	if crisis.SlippageAdder() <= normal.SlippageAdder() {
		t.Fatal("FTX collapse should add more slippage than normal conditions")
	}
}

func TestLiquidityTimeline_EngineAt(t *testing.T) {
	tl := &LiquidityTimeline{}
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	tl.Add(base.Add(1*time.Hour), ScenarioMarch2020)
	tl.Add(base.Add(5*time.Hour), ScenarioNormal)

	// Before any event → normal.
	e1 := tl.EngineAt(base)
	if e1.scenario != ScenarioNormal {
		t.Errorf("expected Normal before events, got %s", e1.scenario)
	}
	// Between first and second → March 2020.
	e2 := tl.EngineAt(base.Add(2 * time.Hour))
	if e2.scenario != ScenarioMarch2020 {
		t.Errorf("expected March2020 between events, got %s", e2.scenario)
	}
	// After second event → Normal.
	e3 := tl.EngineAt(base.Add(6 * time.Hour))
	if e3.scenario != ScenarioNormal {
		t.Errorf("expected Normal after recovery, got %s", e3.scenario)
	}
}

func TestBuildStressReport_EmptyTrades(t *testing.T) {
	report := BuildStressReport(nil, nil)
	if report.PortfolioSurvival != 100 {
		t.Fatalf("empty backtest should have 100 survival, got %f", report.PortfolioSurvival)
	}
}
