package options

import (
	"math"
	"time"
)

const (
	maxConcurrentPositions   = 8
	optionTradeAllocationUSD = initialOptionsBalance * 0.01

	optionStatusReady      = "READY"
	optionStatusInPosition = "IN_POSITION"
	optionStatusCooling    = "COOLING"
	optionStatusWatchlist  = "WATCHLIST"
	optionStatusShadowing  = "SHADOWING"
	optionStatusDisabled   = "DISABLED"

	optionMarketRegimeUnknown  = "UNKNOWN"
	optionMarketRegimeTrend    = "TREND"
	optionMarketRegimeRange    = "RANGE"
	optionMarketRegimeVolatile = "VOLATILE"
	optionMarketRegimeMixed    = "MIXED"

	optionRegimeMinBars = 55

	optionMaxActiveStrategies      = 13
	optionMaxStrategiesPerCategory = 4
	optionRosterRefreshInterval    = 30 * time.Second
	optionActiveRetentionBonus     = 6.0
	optionPromotionBuffer          = 2.5

	optionLossStreakDisableThreshold = 5
	optionLossStreakCooldown         = 50 * time.Minute
	optionUnderperformingMinTrades   = 6
	optionUnderperformingMaxWinRate  = 35.0
	optionUnderperformingCooldown    = 6 * time.Hour

	optionProfitLockProgress      = 0.28
	optionProfitLockShareOfTarget = 0.40
	optionProfitLockShare         = 0.40  // alias used in engine.go
	optionLateExitProgress        = 0.56
	optionLateExitMinGain         = 0.05
	optionMomentumFadeProgress    = 0.72  // late-stage momentum fade exit threshold
	optionStrikePressureBuffer    = 0.0025

	optionColdStartSizeMultiplier = 0.95
	optionMinSizeMultiplier       = 0.45
	optionMaxSizeMultiplier       = 1.90
	optionEarlyMaxMultiplier      = 1.15
	optionLossStreakPenalty       = 0.10
	optionAvgPnLBoost             = 0.22
	optionAvgPnLPenalty           = 0.12
)

func newStrategyStatus(def StrategyDef) StrategyStatus {
	return StrategyStatus{
		StrategyID:        def.ID,
		Name:              def.Name,
		Category:          def.Category,
		OptionType:        string(def.Type),
		RosterState:       StrategyRosterWatchlist,
		Status:            optionStatusWatchlist,
		Regime:            optionMarketRegimeUnknown,
		AllocationUSD:     0,
		SizeMultiplier:    0,
		HasPosition:       false,
		HasShadowPosition: false,
	}
}

func newStrategyState(def StrategyDef) *strategyState {
	def.PositionUSD = optionTradeAllocationUSD
	return &strategyState{
		def:   def,
		stats: newStrategyStatus(def),
	}
}

func optionRecentAbsMove(prices []float64, period int) float64 {
	if len(prices) < period+1 || period <= 0 {
		return 0
	}

	start := len(prices) - period
	total := 0.0
	count := 0
	for i := start; i < len(prices); i++ {
		prev := prices[i-1]
		if prev <= 0 {
			continue
		}
		total += math.Abs((prices[i] - prev) / prev)
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func classifyMarketRegime(prices []float64) string {
	if len(prices) < optionRegimeMinBars {
		return optionMarketRegimeUnknown
	}

	latest := prices[len(prices)-1]
	if latest <= 0 {
		return optionMarketRegimeUnknown
	}

	fast := ema(prices, 21)
	slow := ema(prices, 55)
	fairValue := avgPrice(prices[len(prices)-30:])
	if fairValue <= 0 {
		return optionMarketRegimeUnknown
	}

	trendGap := math.Abs(fast-slow) / latest
	distanceFromFairValue := math.Abs(latest-fairValue) / fairValue
	shortVol := optionRecentAbsMove(prices, 14)
	longVol := optionRecentAbsMove(prices, 55)
	if longVol <= 0 {
		return optionMarketRegimeUnknown
	}

	volRatio := shortVol / longVol
	momentum15 := math.Abs(momentum(prices, 15))
	trendAligned := (latest >= fast && fast >= slow) || (latest <= fast && fast <= slow)

	switch {
	case trendAligned && trendGap >= 0.0020 && momentum15 >= 0.0050:
		return optionMarketRegimeTrend
	case volRatio >= 1.45 && distanceFromFairValue >= 0.0025:
		return optionMarketRegimeVolatile
	case trendGap <= 0.0010 && volRatio <= 1.10 && distanceFromFairValue <= 0.0015:
		return optionMarketRegimeRange
	default:
		return optionMarketRegimeMixed
	}
}

func isCategoryAlignedWithRegime(category, regime string) bool {
	// Lowered from 0.75 → 0.42 so Hybrid strategies (TripleConfluence, MomentumVWAP,
	// BreakoutTrend, Capitulation_Reclaim) are never permanently blocked. The original
	// 0.75 threshold excluded Hybrid in ALL regimes (max score 0.74) and blocked
	// Momentum in RANGE/MIXED — causing most strategies to sit out most of the time.
	return regimeFitScore(category, regime) >= 0.42
}

func clamp(min, value, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}

func liveSizeMultiplierFor(s *strategyState) float64 {
	if !s.disabledUntil.IsZero() && time.Now().Before(s.disabledUntil) {
		return optionMinSizeMultiplier
	}

	multiplier := optionColdStartSizeMultiplier
	liveTrades := s.stats.TotalTrades
	if liveTrades >= 3 {
		multiplier = 1.0
	}
	if liveTrades >= 8 && s.stats.WinRate >= 52 {
		multiplier += 0.10
	}
	if liveTrades >= 12 && s.stats.WinRate >= 56 {
		multiplier += 0.10
	}
	if liveTrades > 0 && s.def.PositionUSD > 0 {
		avgPnLRatio := (s.stats.TotalPnL / float64(liveTrades)) / s.def.PositionUSD
		if avgPnLRatio > 0.08 {
			multiplier += optionAvgPnLBoost
		}
		if avgPnLRatio < -0.06 {
			multiplier -= optionAvgPnLPenalty
		}
	}
	if s.consecutiveLosses > 0 {
		multiplier -= float64(s.consecutiveLosses) * optionLossStreakPenalty
	}
	if liveTrades <= 2 {
		multiplier = math.Min(multiplier, optionEarlyMaxMultiplier)
	}
	return clamp(optionMinSizeMultiplier, multiplier, optionMaxSizeMultiplier)
}

func sizeMultiplierFor(s *strategyState) float64 {
	return liveSizeMultiplierFor(s)
}

func optionEntryConfirmed(def StrategyDef, ctx SignalContext, regime string) bool {
	if len(ctx.Prices) < 25 {
		return false
	}

	price := ctx.BTCPrice
	fast := ema(ctx.Prices, 9)
	slow := ema(ctx.Prices, 21)
	trend := ema(ctx.Prices, 55)
	rsiVal := rsi(ctx.Prices, 14)
	mom3 := momentum(ctx.Prices, 3)
	mom8 := momentum(ctx.Prices, 8)

	bullishSeller := def.Type == Put
	trendAligned := (price >= fast && fast >= slow) || (price <= fast && fast <= slow)
	if bullishSeller {
		switch def.Category {
		case "Momentum", "Breakout", "Hybrid":
			return trendAligned && price >= trend && mom3 > 0.0008 && mom8 > 0 && rsiVal >= 50 && rsiVal <= 72
		case "Mean Reversion", "Capitulation":
			return price >= fast && price >= trend*0.997 && mom3 > -0.0012 && rsiVal >= 38 && rsiVal <= 60
		default:
			return price >= fast && mom3 >= -0.0002
		}
	}

	switch def.Category {
	case "Momentum", "Breakout", "Hybrid":
		return trendAligned && price <= trend && mom3 < -0.0008 && mom8 < 0 && rsiVal >= 28 && rsiVal <= 50
	case "Mean Reversion", "Capitulation":
		return price <= fast && price <= trend*1.003 && mom3 < 0.0012 && rsiVal >= 40 && rsiVal <= 62
	default:
		return price <= fast && mom3 <= 0.0002
	}
}

// niftyEntryConfirmed is the NIFTY-calibrated entry gate.
// NIFTY IV ~18% vs BTC ~80%, so momentum thresholds are ~0.25× BTC values.
// RSI bands are wider because NIFTY intraday RSI rarely exceeds 72 in bull runs.
func niftyEntryConfirmed(def StrategyDef, ctx SignalContext, _ string) bool {
	if len(ctx.Prices) < 15 {
		return false
	}
	price := ctx.BTCPrice
	fast := ema(ctx.Prices, 9)
	slow := ema(ctx.Prices, 21)
	trendPeriod := 55
	if len(ctx.Prices) < 55 {
		trendPeriod = len(ctx.Prices)
	}
	trend := ema(ctx.Prices, trendPeriod)
	rsiVal := rsi(ctx.Prices, 14)
	mom3 := momentum(ctx.Prices, 3)
	mom8 := momentum(ctx.Prices, 8)

	trendAligned := (price >= fast && fast >= slow) || (price <= fast && fast <= slow)
	if def.Type == Put {
		switch def.Category {
		case "Momentum", "Breakout", "Hybrid":
			return trendAligned && price >= trend*0.995 && mom3 > 0.0002 && mom8 > -0.0005 && rsiVal >= 42 && rsiVal <= 82
		case "Mean Reversion", "Capitulation":
			return price >= fast*0.999 && mom3 > -0.0005 && rsiVal >= 20 && rsiVal <= 75
		default:
			return price >= fast && mom3 >= -0.0001
		}
	}
	switch def.Category {
	case "Momentum", "Breakout", "Hybrid":
		return trendAligned && price <= trend*1.005 && mom3 < -0.0002 && mom8 < 0.0005 && rsiVal >= 18 && rsiVal <= 58
	case "Mean Reversion", "Capitulation":
		return price <= fast*1.001 && mom3 < 0.0005 && rsiVal >= 25 && rsiVal <= 80
	default:
		return price <= fast && mom3 <= 0.0001
	}
}
