package options

import (
	"math"
	"time"
)

const (
	maxConcurrentPositions   = 3
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

	optionMaxActiveStrategies      = 7
	optionMaxStrategiesPerCategory = 3
	optionRosterRefreshInterval    = 30 * time.Second
	optionActiveRetentionBonus     = 6.0
	optionPromotionBuffer          = 2.5

	optionLossStreakDisableThreshold = 4
	optionLossStreakCooldown         = 70 * time.Minute
	optionUnderperformingMinTrades   = 6
	optionUnderperformingMaxWinRate  = 35.0
	optionUnderperformingCooldown    = 6 * time.Hour

	optionColdStartSizeMultiplier = 0.85
	optionMinSizeMultiplier       = 0.45
	optionMaxSizeMultiplier       = 1.40
	optionEarlyMaxMultiplier      = 1.05
	optionLossStreakPenalty       = 0.12
	optionAvgPnLBoost             = 0.10
	optionAvgPnLPenalty           = 0.10
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
	return regimeFitScore(category, regime) >= 0.75
}

func clamp(min, value, max float64) float64 {
	return math.Max(min, math.Min(max, value))
}

func liveSizeMultiplierFor(s *strategyState) float64 {
	if !s.disabledUntil.IsZero() && time.Now().Before(s.disabledUntil) {
		return optionMinSizeMultiplier
	}
	return 1.0
}

func sizeMultiplierFor(s *strategyState) float64 {
	return liveSizeMultiplierFor(s)
}
