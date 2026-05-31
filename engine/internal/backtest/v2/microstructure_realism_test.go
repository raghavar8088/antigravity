package v2

import (
	"testing"
	"time"

	"antigravity-engine/internal/alpha/microstructure"
	btExecution "antigravity-engine/internal/backtest/execution"
)

func TestApplyMicrostructureRealismModelsSlippageFundingSpreadAndFillDelay(t *testing.T) {
	ctx := btExecution.MarketContext{
		MidPrice:       100_000,
		VolatilityPct:  0.20,
		LiquidityScore: 0.70,
		ADVNotionalUSD: 100_000_000,
		BookDepthUSD:   5_000_000,
		Latency:        btExecution.LatencyInput{Tier: btExecution.Latency50ms},
	}
	features := microstructure.FeatureSnapshot{
		ATRPct:                1.20,
		Regime:                microstructure.RegimeHighVol,
		FundingRate:           0.001,
		LiquidityConfirmation: false,
	}

	adjusted, attribution := ApplyMicrostructureRealism(ctx, features, 100_000, 16, DefaultMicrostructureBacktestConfig())
	if attribution.SlippageBps < 1 || attribution.SlippageBps > 5 {
		t.Fatalf("expected slippage in 1-5 bps range, got %.2f", attribution.SlippageBps)
	}
	if attribution.FundingCostUSD != 200 {
		t.Fatalf("expected two funding intervals cost of 200, got %.2f", attribution.FundingCostUSD)
	}
	if attribution.SpreadMultiplier <= 1 {
		t.Fatalf("expected spread widening during high volatility")
	}
	if attribution.FillDelay != 250*time.Millisecond {
		t.Fatalf("expected liquidity fill delay, got %s", attribution.FillDelay)
	}
	if adjusted.LiquidityScore >= ctx.LiquidityScore {
		t.Fatalf("expected liquidity score reduction")
	}
	if adjusted.Latency.ExecutionDelay == 0 {
		t.Fatalf("expected execution delay in adjusted market context")
	}
}
