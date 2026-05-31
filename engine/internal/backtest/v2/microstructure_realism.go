package v2

import (
	"math"
	"time"

	"antigravity-engine/internal/alpha/microstructure"
	btExecution "antigravity-engine/internal/backtest/execution"
)

type MicrostructureBacktestConfig struct {
	MinSlippageBps          float64
	MaxSlippageBps          float64
	FundingInterval         time.Duration
	VolatilitySpreadTrigger float64
	LiquidityFillDelay      time.Duration
}

type MicrostructureExecutionAdjustment struct {
	SlippageBps      float64
	SpreadMultiplier float64
	FundingCostUSD   float64
	FillDelay        time.Duration
	LiquidityScore   float64
}

func DefaultMicrostructureBacktestConfig() MicrostructureBacktestConfig {
	return MicrostructureBacktestConfig{
		MinSlippageBps:          1,
		MaxSlippageBps:          5,
		FundingInterval:         8 * time.Hour,
		VolatilitySpreadTrigger: 0.75,
		LiquidityFillDelay:      250 * time.Millisecond,
	}
}

func ApplyMicrostructureRealism(ctx btExecution.MarketContext, features microstructure.FeatureSnapshot, notionalUSD, holdingHours float64, cfg MicrostructureBacktestConfig) (btExecution.MarketContext, MicrostructureExecutionAdjustment) {
	if cfg.MinSlippageBps <= 0 {
		cfg = DefaultMicrostructureBacktestConfig()
	}
	liqScore := ctx.LiquidityScore
	if !features.LiquidityConfirmation {
		liqScore = math.Max(0.20, liqScore-0.20)
	}
	if features.Regime == microstructure.RegimeHighVol {
		liqScore = math.Max(0.15, liqScore-0.25)
	}
	slippageBps := clamp(cfg.MinSlippageBps+(1-liqScore)*(cfg.MaxSlippageBps-cfg.MinSlippageBps), cfg.MinSlippageBps, cfg.MaxSlippageBps)
	spreadMultiplier := 1.0
	if features.ATRPct >= cfg.VolatilitySpreadTrigger || features.Regime == microstructure.RegimeHighVol {
		spreadMultiplier = 1.0 + clamp(features.ATRPct/cfg.VolatilitySpreadTrigger, 0, 3)*0.35
	}
	fillDelay := time.Duration(0)
	if liqScore < 0.45 {
		fillDelay = cfg.LiquidityFillDelay
	}
	fundingIntervals := 0.0
	if cfg.FundingInterval > 0 && holdingHours > 0 {
		fundingIntervals = holdingHours / cfg.FundingInterval.Hours()
	}
	fundingCost := notionalUSD * features.FundingRate * fundingIntervals

	ctx.LiquidityScore = liqScore
	ctx.VolatilityPct = math.Max(ctx.VolatilityPct, features.ATRPct*spreadMultiplier)
	ctx.BookDepthUSD = math.Max(1, ctx.BookDepthUSD*liqScore)
	ctx.Latency.ExecutionDelay += fillDelay
	ctx.Latency.SignalEdgeBps += slippageBps
	return ctx, MicrostructureExecutionAdjustment{
		SlippageBps:      slippageBps,
		SpreadMultiplier: spreadMultiplier,
		FundingCostUSD:   fundingCost,
		FillDelay:        fillDelay,
		LiquidityScore:   liqScore,
	}
}
