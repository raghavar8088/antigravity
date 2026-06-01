package v3

import (
	"testing"
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
)

func TestSlippageEngine_IncreasesWithOrderSize(t *testing.T) {
	eng := btExecution.NewSlippageEngine()
	small := eng.Estimate(btExecution.SlippageInput{
		OrderNotionalUSD: 1_000,
		ADVNotionalUSD:   50_000_000,
		VolatilityPct:    20,
		LiquidityScore:   0.8,
	})
	large := eng.Estimate(btExecution.SlippageInput{
		OrderNotionalUSD: 10_000_000,
		ADVNotionalUSD:   50_000_000,
		VolatilityPct:    20,
		LiquidityScore:   0.8,
	})
	if large.ExpectedBps <= small.ExpectedBps {
		t.Fatalf("large order slippage (%f bps) should exceed small (%f bps)", large.ExpectedBps, small.ExpectedBps)
	}
}

func TestSlippageEngine_IncreasesWithVolatility(t *testing.T) {
	eng := btExecution.NewSlippageEngine()
	calm := eng.Estimate(btExecution.SlippageInput{
		OrderNotionalUSD: 10_000,
		ADVNotionalUSD:   50_000_000,
		VolatilityPct:    5,
		LiquidityScore:   0.7,
	})
	volatile := eng.Estimate(btExecution.SlippageInput{
		OrderNotionalUSD: 10_000,
		ADVNotionalUSD:   50_000_000,
		VolatilityPct:    150,
		LiquidityScore:   0.7,
	})
	if volatile.ExpectedBps <= calm.ExpectedBps {
		t.Fatalf("high-vol slippage (%f) should exceed calm (%f)", volatile.ExpectedBps, calm.ExpectedBps)
	}
}

func TestSlippageEngine_IncreasesWithLowLiquidity(t *testing.T) {
	eng := btExecution.NewSlippageEngine()
	liquid := eng.Estimate(btExecution.SlippageInput{
		OrderNotionalUSD: 10_000,
		ADVNotionalUSD:   50_000_000,
		VolatilityPct:    20,
		LiquidityScore:   0.9,
	})
	illiquid := eng.Estimate(btExecution.SlippageInput{
		OrderNotionalUSD: 10_000,
		ADVNotionalUSD:   50_000_000,
		VolatilityPct:    20,
		LiquidityScore:   0.1,
	})
	if illiquid.ExpectedBps <= liquid.ExpectedBps {
		t.Fatalf("illiquid slippage (%f) should exceed liquid (%f)", illiquid.ExpectedBps, liquid.ExpectedBps)
	}
}

func TestLiquidityScenario_AddsSlippage(t *testing.T) {
	normal := NewLiquidityEngine(ScenarioNormal)
	march := NewLiquidityEngine(ScenarioMarch2020)
	if march.SlippageAdder() <= normal.SlippageAdder() {
		t.Fatal("March 2020 should add more slippage than normal")
	}
}

func TestFillModel_SlippageCostPositive(t *testing.T) {
	fill := btExecution.NewFillModel()
	order := btExecution.Order{
		ID:          "T1",
		Symbol:      "BTCUSD",
		Side:        btExecution.SideBuy,
		Type:        btExecution.OrderMarket,
		Quantity:    0.5,
		SignalPrice: 50000,
		GeneratedAt: time.Now(),
	}
	ctx := btExecution.MarketContext{
		MidPrice:       50000,
		VolatilityPct:  20,
		LiquidityScore: 0.65,
		ADVNotionalUSD: 250_000_000,
		BookDepthUSD:   12_500_000,
		MomentumPct:    0.02,
		Timestamp:      time.Now(),
		Latency:        btExecution.LatencyInput{Tier: btExecution.Latency50ms},
	}
	result := fill.Execute(order, ctx)
	if result.SlippageCostUSD < 0 {
		t.Fatal("slippage cost must be >= 0")
	}
	if result.FillPrice <= 0 {
		t.Fatal("fill price must be positive")
	}
}
