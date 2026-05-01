package options_selling

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
	optionRosterRefreshInterval    = 10 * time.Second // reduced from 30s for faster regime response
	optionActiveRetentionBonus     = 6.0
	optionPromotionBuffer          = 2.5

	optionLossStreakDisableThreshold = 5
	optionLossStreakCooldown         = 50 * time.Minute
	optionUnderperformingMinTrades   = 6
	optionUnderperformingMaxWinRate  = 35.0
	optionUnderperformingCooldown    = 90 * time.Minute // reduced from 6h — one bad session no longer kills a strategy all day

	optionProfitLockProgress      = 0.28
	optionProfitLockShareOfTarget = 0.60 // raised from 0.40 — collect more premium before locking in early exits
	optionLateExitProgress        = 0.56
	optionLateExitMinGain         = 0.05
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
	// Lower the gate so Hybrid/Momentum sellers are not permanently locked out.
	// The old 0.75 threshold blocked Hybrid in every regime and sidelined several
	// otherwise strong continuation sellers for most of the session.
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
	// Faster promotion: halved trade-count thresholds so intraday strategies
	// can reach meaningful size within the same session instead of 4+ hours.
	if liveTrades >= 5 && s.stats.WinRate >= 50 {
		multiplier += 0.12
	}
	if liveTrades >= 8 && s.stats.WinRate >= 54 {
		multiplier += 0.12
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
	rsiVal := rsi(ctx.Prices, 14)
	mom3 := momentum(ctx.Prices, 3)
	mom8 := momentum(ctx.Prices, 8)

	bullishSeller := def.Type == Put
	// trendAligned uses 9/21-EMA only — the 55-EMA requirement was blocking
	// all consolidation trades where mean-reversion puts are most profitable.
	trendAligned := (price >= fast && fast >= slow) || (price <= fast && fast <= slow)
	if bullishSeller {
		switch def.Category {
		case "Momentum", "Breakout", "Hybrid":
			// Widened RSI ceiling 72→76; relaxed trend gate to 21-EMA × 0.996
			// instead of requiring price >= 55-EMA (which blocked range/consolidation entries).
			return trendAligned && price >= slow*0.996 && mom3 > 0.0006 && mom8 > -0.0002 && rsiVal >= 44 && rsiVal <= 76
		case "Mean Reversion", "Capitulation":
			// Widened RSI ceiling 60→68; allow entry slightly below 9-EMA.
			return price >= fast*0.998 && mom3 > -0.0015 && rsiVal >= 34 && rsiVal <= 68
		default:
			return price >= fast*0.998 && mom3 >= -0.0003
		}
	}

	switch def.Category {
	case "Momentum", "Breakout", "Hybrid":
		// Widened RSI floor 28→22; relaxed trend gate to 21-EMA × 1.004.
		return trendAligned && price <= slow*1.004 && mom3 < -0.0006 && mom8 < 0.0002 && rsiVal >= 22 && rsiVal <= 56
	case "Mean Reversion", "Capitulation":
		// Widened RSI floor 40→32; allow entry slightly above 9-EMA.
		return price <= fast*1.002 && mom3 < 0.0015 && rsiVal >= 32 && rsiVal <= 66
	default:
		return price <= fast*1.002 && mom3 <= 0.0003
	}
}

func isNSEMarketOpen(utcHour, utcMin int) bool {
	total := utcHour*60 + utcMin
	return total >= 225 && total <= 600
}

func isNSEPreClose(utcHour, utcMin int) bool {
	total := utcHour*60 + utcMin
	return total >= 525 && total <= 570
}

// niftyEntryConfirmed is the NIFTY-calibrated entry gate.
// NIFTY IV ~16% vs BTC ~62%, so momentum thresholds are ~0.25× BTC values.
// NSE session guard prevents phantom fills when the market is closed.
func niftyEntryConfirmed(def StrategyDef, ctx SignalContext, _ string) bool {
	if len(ctx.Prices) < 15 {
		return false
	}
	if !isNSEMarketOpen(ctx.UTCHour, ctx.UTCMin) {
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

	preClose := isNSEPreClose(ctx.UTCHour, ctx.UTCMin)
	trendAligned := (price >= fast && fast >= slow) || (price <= fast && fast <= slow)
	if def.Type == Put {
		switch def.Category {
		case "Momentum", "Breakout", "Hybrid", "MACD", "ATR":
			rsiLo, rsiHi := 44.0, 80.0
			momFloor := 0.0002
			if preClose {
				rsiLo, rsiHi = 48.0, 72.0
				momFloor = 0.0004
			}
			return trendAligned && price >= trend*0.995 && mom3 > momFloor && mom8 > -0.0005 && rsiVal >= rsiLo && rsiVal <= rsiHi
		case "Mean Reversion", "Capitulation":
			rsiLo, rsiHi := 22.0, 75.0
			if preClose {
				rsiLo, rsiHi = 28.0, 68.0
			}
			return price >= fast*0.999 && mom3 > -0.0005 && rsiVal >= rsiLo && rsiVal <= rsiHi
		default:
			return price >= fast && mom3 >= -0.0001
		}
	}
	switch def.Category {
	case "Momentum", "Breakout", "Hybrid", "MACD", "ATR":
		rsiLo, rsiHi := 20.0, 56.0
		momCeil := -0.0002
		if preClose {
			rsiLo, rsiHi = 28.0, 52.0
			momCeil = -0.0004
		}
		return trendAligned && price <= trend*1.005 && mom3 < momCeil && mom8 < 0.0005 && rsiVal >= rsiLo && rsiVal <= rsiHi
	case "Mean Reversion", "Capitulation":
		rsiLo, rsiHi := 25.0, 78.0
		if preClose {
			rsiLo, rsiHi = 32.0, 70.0
		}
		return price <= fast*1.001 && mom3 < 0.0005 && rsiVal >= rsiLo && rsiVal <= rsiHi
	default:
		return price <= fast && mom3 <= 0.0001
	}
}
