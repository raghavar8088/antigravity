package options

import (
	"math"
	"time"
)

const (
	// Buyer desk: fewer overlapping longs and smaller base tickets vs the selling desk.
	maxConcurrentPositions = 4

	buyerDailyLossLimitPct = 0.03 // halt new opens after 3% of day-start balance (tighter than generic 5%)

	// Long-option exits: spot moved against thesis while the trade is still losing.
	buyerThesisMovePct = 0.012
	// Theta bleed: by this fraction of contract life, require at least this premium gain or flatten.
	buyerThetaBleedProgress = 0.58
	buyerThetaBleedMinGain  = 0.055

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
	optionProfitLockShare         = 0.60 // alias used in engine.go
	optionLateExitProgress        = 0.56
	optionLateExitMinGain         = 0.05
	optionMomentumFadeProgress    = 0.72
	optionStrikePressureBuffer    = 0.0025

	optionColdStartSizeMultiplier = 0.92
	optionMinSizeMultiplier       = 0.40
	optionMaxSizeMultiplier       = 1.55 // cap leverage stacking on a small-book profile
	optionEarlyMaxMultiplier      = 1.08
	optionLossStreakPenalty       = 0.10
	optionAvgPnLBoost             = 0.22
	optionAvgPnLPenalty           = 0.12

	// ── Upgrade: Dynamic Cooldown Multipliers ──
	// Winners get shorter cooldowns (strike while hot), losers get longer ones.
	cooldownWinMultiplier       = 0.50 // halve cooldown after a win
	cooldownLossMultiplier      = 2.00 // double cooldown after a loss
	cooldownStreakWinMultiplier = 0.33 // 2+ consecutive wins → 1/3 cooldown

	// ── Upgrade: Theta-Decay Acceleration Exit ──
	// Selling strategies benefit from accelerating theta in late life.
	// Tighten TP progressively as position ages to capture the theta cliff.
	thetaAccelPhase1Progress = 0.50 // 50% of life elapsed → tighten TP
	thetaAccelPhase1TPScale  = 0.80 // TP at 80% of normal (e.g. 38%→30%)
	thetaAccelPhase2Progress = 0.70 // 70% of life elapsed → tighten more
	thetaAccelPhase2TPScale  = 0.55 // TP at 55% of normal (e.g. 38%→21%)

	// ── Upgrade: Volatility-Adjusted Strike Selection ──
	// Scale OTM strike distance with realized IV so strikes adapt to conditions.
	strikeIVLowThreshold  = 0.50 // IV below this → tighter strikes
	strikeIVHighThreshold = 0.70 // IV above this → wider strikes
	strikeIVLowScale      = 0.75 // multiply OTM% by this in low-vol
	strikeIVHighScale     = 1.30 // multiply OTM% by this in high-vol
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

// optionTradeAllocationUSD is the nominal per-strategy ticket. Like the selling
// desk it only sets the ratio that drives the size multiplier — real exposure is
// the contract count below.
func optionTradeAllocationUSD() float64 {
	return getInitialOptionsBalanceUSD() * 0.002
}

const (
	// Delta Exchange BTC option contract size. One contract controls 0.001 BTC,
	// so contract counts are converted to BTC exposure before any money math.
	DELTA_CONTRACT_SIZE_BTC = 0.001

	// Delta Exchange retail position limits — mirrors the selling desk so both
	// desks take the same BTC exposure per ticket.
	DELTA_BASE_QUANTITY = 200
	DELTA_MAX_QUANTITY  = 500
	DELTA_MIN_QUANTITY  = 50

	// Delta's option taker fee: 0.03% of UNDERLYING NOTIONAL per side, capped at
	// 10% of the premium per side.
	//
	// Charged from 2026-08-15. Before that this desk charged nothing at all and
	// called the result "netPnl", which made every strategy here look better
	// than it could ever trade. That was not a hypothetical: the Live Engine
	// went to real money on strategies qualified on this desk and bled, and the
	// post-mortem named the fee cap first — on a $0.19 premium the cap binds and
	// the round trip costs 20% of the position before the market moves at all.
	// chain_pricer.go has said in a comment since it was written that fee-free
	// pricing is why paper results here never transferred. Now the desk charges
	// what the venue charges.
	DELTA_OPTION_FEE_PCT_OF_NOTIONAL = 0.0003
	DELTA_OPTION_FEE_CAP_OF_PREMIUM  = 0.10
)

// optionFeeUSD is the taker fee for ONE side, in dollars.
//
// spot is the underlying price at the fill, premium is the per-BTC option
// price, and qty is BTC exposure. The cap is what makes cheap options
// uneconomic and is the entire reason this function is not a single
// multiplication: 0.03% of notional is trivial on a $2 premium and ruinous on a
// $0.19 one, where it lands on the 10% ceiling instead.
func optionFeeUSD(spot, premium, qty float64) float64 {
	if qty <= 0 || spot <= 0 {
		return 0
	}
	notionalFee := DELTA_OPTION_FEE_PCT_OF_NOTIONAL * spot * qty
	premiumCap := DELTA_OPTION_FEE_CAP_OF_PREMIUM * premium * qty
	if premiumCap > 0 && notionalFee > premiumCap {
		return premiumCap
	}
	return notionalFee
}

// feeDragPct is fees as a percentage of GROSS PROFIT — not of premium, and not
// of notional. It answers "how much of what this trade earned did the venue
// take", which is the only form of the question that decides whether a strategy
// is worth trading.
//
// Losers return 0 rather than a negative: drag is undefined when there was no
// profit to give away, and printing "-140%" there would read as a good number
// on a bad trade.
func feeDragPct(grossPnL, fees float64) float64 {
	if grossPnL <= 0 || fees <= 0 {
		return 0
	}
	return fees / grossPnL * 100
}

func newStrategyState(def StrategyDef) *strategyState {
	def.PositionUSD = optionTradeAllocationUSD()
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

	// Long options: calls for bullish signals, puts for bearish (writer mapping was the inverse).
	bullishLong := def.Type == Call
	trendAligned := (price >= fast && fast >= slow) || (price <= fast && fast <= slow)
	if bullishLong {
		switch def.Category {
		case "Momentum", "Breakout", "Hybrid":
			// Slightly stricter than prior short-put gate: long calls need clean uptrend, not late extensions.
			return trendAligned && price >= slow*0.996 && mom3 > 0.0007 && mom8 > -0.0001 && rsiVal >= 46 && rsiVal <= 70
		case "Mean Reversion", "Capitulation":
			return price >= fast*0.997 && mom3 > -0.0012 && rsiVal >= 40 && rsiVal <= 62
		default:
			return price >= fast*0.998 && mom3 >= -0.0002
		}
	}

	switch def.Category {
	case "Momentum", "Breakout", "Hybrid":
		return trendAligned && price <= slow*1.003 && mom3 < -0.0007 && mom8 < 0.0001 && rsiVal >= 28 && rsiVal <= 52
	case "Mean Reversion", "Capitulation":
		return price <= fast*1.001 && mom3 < 0.0012 && rsiVal >= 38 && rsiVal <= 60
	default:
		return price <= fast*1.002 && mom3 <= 0.0002
	}
}
