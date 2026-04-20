package options_selling

import (
	"math"
	"time"
)

const (
	maxConcurrentPositions   = 15
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

	optionRegimeMinBars = 30

	optionMaxActiveStrategies      = 40
	optionMaxStrategiesPerCategory = 12
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

	// Use adaptive lookback: prefer 55 bars but fall back to what's available.
	slowPeriod := 55
	if len(prices) < slowPeriod {
		slowPeriod = len(prices)
	}

	fast := ema(prices, 21)
	slow := ema(prices, slowPeriod)

	fairValueLen := 30
	if len(prices) < fairValueLen {
		fairValueLen = len(prices)
	}
	fairValue := avgPrice(prices[len(prices)-fairValueLen:])
	if fairValue <= 0 {
		return optionMarketRegimeUnknown
	}

	trendGap := math.Abs(fast-slow) / latest
	distanceFromFairValue := math.Abs(latest-fairValue) / fairValue
	shortVol := optionRecentAbsMove(prices, 14)
	longVol := optionRecentAbsMove(prices, slowPeriod)
	if longVol <= 0 {
		// When longVol is unavailable, fall back to shortVol-based classification
		longVol = shortVol
	}
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
	// Threshold set to 0.32 — the minimum possible regimeFitScore (Mean Reversion
	// in TREND). The roster system already penalises poorly-aligned strategies via
	// structural score, so this gate should only block categories with a truly zero
	// fit score (i.e. unrecognised regime strings hitting the default 0.50 fallback
	// still passes). Raising the threshold to 0.42 had the unintended side-effect
	// of permanently blocking Momentum in RANGE (0.36) and Mean Reversion in TREND
	// (0.32), two of the most common market states.
	return regimeFitScore(category, regime) >= 0.32
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
	trend := ema(ctx.Prices, 55)
	rsiVal := rsi(ctx.Prices, 14)
	mom3 := momentum(ctx.Prices, 3)
	mom8 := momentum(ctx.Prices, 8)

	bullishSeller := def.Type == Put
	if bullishSeller {
		switch def.Category {
		case "Momentum", "Breakout":
			// trendAligned removed: flat/mixed regimes blocked too many valid puts.
			// mom3 lowered 0.0005→0.0001 (~$8 on $84k BTC); RSI floor 45→38.
			return price >= trend*0.985 && mom3 > 0.0001 && mom8 > 0 && rsiVal >= 38 && rsiVal <= 75
		case "Hybrid":
			// TRIPLE_BULL signal fires at RSI < 35 — no minimum RSI bound here or
			// the signal and confirmation gates would never overlap.
			return price >= trend*0.990 && mom3 > 0.00005 && mom8 > 0 && rsiVal <= 75
		case "Mean Reversion", "Capitulation":
			// RSI is already encoded in the firing signal (RSI_OVERSOLD_EXTREME fires
			// at RSI ≈ 25, OVEREXTENSION_FADE_DOWN fires at RSI < 28). Adding a lower
			// RSI bound ≥ 38 here creates a permanently impossible condition — removed.
			return price >= fast*0.998 && price >= trend*0.980 && mom3 > -0.0015 && rsiVal <= 62
		default:
			return price >= fast && mom3 >= -0.0002
		}
	}

	switch def.Category {
	case "Momentum", "Breakout":
		// trendAligned removed; RSI ceiling 55→62; mom3 threshold -0.0005→-0.0001.
		return price <= trend*1.015 && mom3 < -0.0001 && mom8 < 0 && rsiVal >= 25 && rsiVal <= 62
	case "Hybrid":
		// TRIPLE_BEAR signal fires at RSI > 65 — no maximum RSI bound here.
		return price <= trend*1.010 && mom3 < -0.00005 && mom8 < 0 && rsiVal >= 25
	case "Mean Reversion", "Capitulation":
		// RSI is already encoded in the firing signal (RSI_OVERBOUGHT_EXTREME fires
		// at RSI ≈ 75, OVEREXTENSION_FADE_UP fires at RSI > 72). Adding an upper
		// RSI bound ≤ 62 here creates a permanently impossible condition — removed.
		return price <= fast*1.002 && price <= trend*1.020 && mom3 < 0.0015 && rsiVal >= 38
	default:
		return price <= fast && mom3 <= 0.0002
	}
}

// niftyEntryConfirmed mirrors optionEntryConfirmed with momentum thresholds
// scaled ~0.25× for NIFTY's lower volatility (IV ~18% vs BTC ~80%).
// Typical NIFTY 3-minute move is ~0.025%; BTC's is ~0.10%.
//
// RSI bounds are deliberately wide for Mean Reversion / Capitulation to avoid
// contradicting the signal (e.g. RSI_OVERSOLD_EXTREME fires at RSI≈25, so
// requiring rsiVal >= 38 in confirmation would always reject the trade).
func niftyEntryConfirmed(def StrategyDef, ctx SignalContext, _ string) bool {
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
			// Relax price >= trend to price >= trend*0.995 so intraday dips don't block
			return trendAligned && price >= trend*0.995 && mom3 > 0.0002 && mom8 > 0 && rsiVal >= 45 && rsiVal <= 75
		case "Mean Reversion", "Capitulation":
			// Wide RSI band: signal already sets RSI level; don't double-filter
			return price >= fast*0.999 && mom3 > -0.0003 && rsiVal >= 20 && rsiVal <= 68
		default:
			return price >= fast && mom3 >= -0.00005
		}
	}

	switch def.Category {
	case "Momentum", "Breakout", "Hybrid":
		return trendAligned && price <= trend*1.005 && mom3 < -0.0002 && mom8 < 0 && rsiVal >= 25 && rsiVal <= 55
	case "Mean Reversion", "Capitulation":
		// Wide RSI band for bearish mean reversion (overbought signals fire at RSI~75+)
		return price <= fast*1.001 && mom3 < 0.0003 && rsiVal >= 32 && rsiVal <= 80
	default:
		return price <= fast && mom3 <= 0.00005
	}
}
