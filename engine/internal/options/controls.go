package options

import (
	"math"
	"time"
)

const (
	maxConcurrentPositions = 3

	optionStatusDisabled = "DISABLED"

	optionMarketRegimeUnknown  = "UNKNOWN"
	optionMarketRegimeTrend    = "TREND"
	optionMarketRegimeRange    = "RANGE"
	optionMarketRegimeVolatile = "VOLATILE"
	optionMarketRegimeMixed    = "MIXED"

	optionRegimeMinBars = 55

	optionLossStreakDisableThreshold = 3
	optionLossStreakCooldown         = 90 * time.Minute
	optionUnderperformingMinTrades   = 6
	optionUnderperformingMaxWinRate  = 35.0
	optionUnderperformingCooldown    = 6 * time.Hour

	optionColdStartSizeMultiplier = 0.80
	optionMinSizeMultiplier       = 0.45
	optionMaxSizeMultiplier       = 1.25
	optionEarlyMaxMultiplier      = 1.05
	optionLossStreakPenalty       = 0.12
	optionAvgPnLBoost             = 0.10
	optionAvgPnLPenalty           = 0.10
)

func newStrategyStatus(def StrategyDef) StrategyStatus {
	return StrategyStatus{
		StrategyID:     def.ID,
		Name:           def.Name,
		Category:       def.Category,
		OptionType:     string(def.Type),
		SizeMultiplier: optionColdStartSizeMultiplier,
		Status:         "READY",
		HasPosition:    false,
	}
}

func newStrategyState(def StrategyDef) *strategyState {
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
	switch regime {
	case optionMarketRegimeUnknown:
		return false
	case optionMarketRegimeTrend:
		return category == "Breakout"
	case optionMarketRegimeRange:
		return category == "Mean Reversion" || category == "Capitulation"
	case optionMarketRegimeVolatile:
		return category == "Breakout" || category == "Capitulation"
	case optionMarketRegimeMixed:
		return category == "Breakout" || category == "Capitulation"
	default:
		return false
	}
}

func clamp(min, value, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}

func sizeMultiplierFor(s *strategyState) float64 {
	if !s.disabledUntil.IsZero() && time.Now().Before(s.disabledUntil) {
		return optionMinSizeMultiplier
	}
	if s.stats.TotalTrades == 0 {
		return optionColdStartSizeMultiplier
	}

	winRate := s.stats.WinRate / 100.0
	avgPnL := s.stats.TotalPnL / float64(s.stats.TotalTrades)

	multiplier := 0.90 + (winRate-0.50)*0.80
	if avgPnL > 0 {
		multiplier += optionAvgPnLBoost
	} else if avgPnL < 0 {
		multiplier -= optionAvgPnLPenalty
	}
	multiplier -= float64(s.consecutiveLosses) * optionLossStreakPenalty

	if s.stats.TotalTrades < optionUnderperformingMinTrades && multiplier > optionEarlyMaxMultiplier {
		multiplier = optionEarlyMaxMultiplier
	}

	return clamp(optionMinSizeMultiplier, multiplier, optionMaxSizeMultiplier)
}
