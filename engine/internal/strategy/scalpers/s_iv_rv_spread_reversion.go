package scalpers

import "fmt"

// S10 — IV_RV_Spread_Reversion
//
// Regime:     RANGING, TRENDING
// Timeframes: 1h (RealizedVol + funding bias)
// Rollout:    Phase 1 (see rollout_phase.go — STRATEGY_ROLLOUT_PHASE env var)
//
// Concept (Volatility Risk Premium / IV-RV spread):
// Deribit's BTC DVOL is a 30-day forward-looking implied-volatility index
// (BTC's "VIX"). Empirically and in traditional vol-trading literature,
// implied volatility tends to run persistently above realized volatility —
// the "volatility risk premium" (VRP). When the IV-RV spread is unusually
// wide, options/vol-linked positioning is "rich" and mean-reverts; when it's
// unusually narrow or negative, IV is "cheap" relative to recent realized
// moves. We proxy a directional fade of that extreme using funding rate as
// the crowding/positioning signal: elevated DVOL combined with strongly
// positive funding (longs paying, crowded long) biases SHORT; elevated DVOL
// combined with strongly negative funding (crowded short) biases LONG.
//
// Graceful degradation: if the DVOL feed is unpopulated/unhealthy, this
// strategy falls back to a RealizedVol-only proxy (treats RV expansion alone
// as the "vol elevated" condition) rather than permanently returning
// NoSignal — DVOL being 0 forever must not permanently disable the strategy.

const (
	ivRvSpreadElevatedPct  = 15.0   // DVOL - RV spread (in vol points) considered "elevated"
	ivRvRVOnlyHighPctile   = 1.15   // RV-only fallback: RV > 1.15x its 24-bar avg counts as elevated
	ivRvFundingStrongShort = 0.0004 // raw decimal, +0.04% per 8h — strong crowded-long signal
	ivRvFundingStrongLong  = -0.0004
)

type IVRVSpreadReversion struct{}

func (s *IVRVSpreadReversion) Name() string { return "IV_RV_Spread_Reversion" }

func (s *IVRVSpreadReversion) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}

func (s *IVRVSpreadReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging && ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 25 {
		return NoSignal(name)
	}

	rv := RealizedVol(ctx.Candles1h, 24)
	if rv == 0 {
		return NoSignal(name)
	}

	// Need at least 2 funding readings for a "strong, persistent" crowding read
	// (3-bar confirmation rule: require current + prior reading both extreme).
	if len(ctx.FundingHistory) < 2 {
		return NoSignal(name)
	}
	fCur := ctx.FundingRate
	fPrev := ctx.FundingHistory[len(ctx.FundingHistory)-2]
	fPrev2 := fCur
	if len(ctx.FundingHistory) >= 3 {
		fPrev2 = ctx.FundingHistory[len(ctx.FundingHistory)-3]
	}

	strongPositive := fCur > ivRvFundingStrongShort && fPrev > ivRvFundingStrongShort && fPrev2 > ivRvFundingStrongShort*0.5
	strongNegative := fCur < ivRvFundingStrongLong && fPrev < ivRvFundingStrongLong && fPrev2 < ivRvFundingStrongLong*0.5

	if !strongPositive && !strongNegative {
		return NoSignal(name)
	}

	var volElevated bool
	var volDesc string
	usingDVOL := ctx.DVOLPopulated && ctx.DVOLHealthy && ctx.DVOL > 0
	if usingDVOL {
		spread := ctx.DVOL - rv
		volElevated = spread > ivRvSpreadElevatedPct
		volDesc = fmt.Sprintf("DVOL=%.1f RV=%.1f spread=%.1f", ctx.DVOL, rv, spread)
	} else {
		// Fallback: RV-only proxy when DVOL feed is down/unpopulated.
		rvAvg := rv
		if len(ctx.Candles1h) >= 49 {
			rvAvg = RealizedVol(ctx.Candles1h[:len(ctx.Candles1h)-24], 24)
		}
		if rvAvg == 0 {
			rvAvg = rv
		}
		volElevated = rv > rvAvg*ivRvRVOnlyHighPctile
		volDesc = fmt.Sprintf("DVOL_unavailable, RV-only fallback: RV=%.1f vs avg=%.1f", rv, rvAvg)
	}

	if !volElevated {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	slDist := max64(1.0*atr1h, 0.003*price) // rule 2: max(1.0x ATR, 0.3% price)

	// Confidence reduced when running on the RV-only fallback (less information).
	confidence := 0.66
	if !usingDVOL {
		confidence = 0.52
	}

	if volElevated && strongPositive {
		// Elevated vol + crowded longs paying funding -> fade with SHORT.
		sl := price + slDist
		tp1 := price - 2.0*slDist
		tp2 := price - 3.0*slDist
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  confidence,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"IV-RV fade SHORT: %s, funding persistently positive (%.5f, %.5f, %.5f) — crowded longs",
				volDesc, fPrev2, fPrev, fCur,
			),
		}
	}

	if volElevated && strongNegative {
		// Elevated vol + crowded shorts paying funding -> fade with LONG.
		sl := price - slDist
		tp1 := price + 2.0*slDist
		tp2 := price + 3.0*slDist
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  confidence,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"IV-RV fade LONG: %s, funding persistently negative (%.5f, %.5f, %.5f) — crowded shorts",
				volDesc, fPrev2, fPrev, fCur,
			),
		}
	}

	return NoSignal(name)
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
