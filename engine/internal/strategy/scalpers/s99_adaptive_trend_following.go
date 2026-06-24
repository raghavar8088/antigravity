package scalpers

import (
	"fmt"
	"math"
)

// S99 — Adaptive Trend Following
//
// Citation:   Perry Kaufman, "Smarter Trading" (1995); "A D-P-S Adaptive
//             Moving Average System" (Journal of Technical Analysis, 2001).
//             Unlike S33 (Adaptive_Moving_Average_Trend which uses KAMA slope
//             directionally), this strategy uses the Efficiency Ratio itself
//             as BOTH a regime detector AND a trade signal generator.
// Regime:     ALL regimes (ER determines tradability internally)
// Timeframes: 15m
// Logic:
//   ER > 0.6  → trending efficiently → trade KAMA breakouts (Donchian-like
//               entry but KAMA-filtered for quality)
//   ER 0.3-0.6 → mixed → only take signals confirmed by 3-bar CVD trend
//   ER < 0.3  → random → NoSignal (market is noise)
// Distinction from S33: S33 uses ER only as a gate (>0.6 required) and KAMA
//   slope as the signal. S99 uses ER as a continuous regime classifier with
//   three zones, combining Donchian breakout logic with CVD confirmation.
// Distinction from S38: S38 is a raw Donchian breakout. S99 filters the entry
//   price through KAMA and requires ER-based regime confirmation.

type AdaptiveTrendFollowing struct{}

func (s *AdaptiveTrendFollowing) Name() string { return "Adaptive_Trend_Following" }

func (s *AdaptiveTrendFollowing) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *AdaptiveTrendFollowing) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles15m) < 35 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	n := len(candles)

	kama, er := KAMA(candles, 20)
	if kama == 0 {
		return NoSignal(name)
	}

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	price := ctx.Price

	// ER < 0.3: market is random → do not trade
	if er < 0.3 {
		return NoSignal(name)
	}

	// Donchian 20-bar high/low (excluding current bar) for breakout reference
	donchianHigh := SwingHigh(candles[:n-1], 20)
	donchianLow := SwingLow(candles[:n-1], 20)

	lastClose := candles[n-1].Close

	// CVD 3-bar trend check
	cvdHistory := ctx.CVDHistory
	cvd3Rising := false
	cvd3Falling := false
	if len(cvdHistory) >= 3 {
		cvd3Rising = cvdHistory[len(cvdHistory)-1] > cvdHistory[len(cvdHistory)-2] &&
			cvdHistory[len(cvdHistory)-2] > cvdHistory[len(cvdHistory)-3]
		cvd3Falling = cvdHistory[len(cvdHistory)-1] < cvdHistory[len(cvdHistory)-2] &&
			cvdHistory[len(cvdHistory)-2] < cvdHistory[len(cvdHistory)-3]
	} else {
		// fallback to single-bar CVD
		cvd3Rising = ctx.CVD > ctx.CVDPrev
		cvd3Falling = ctx.CVD < ctx.CVDPrev
	}

	// Price must be above KAMA for longs, below for shorts (KAMA quality filter)
	kamaLongFilter := lastClose > kama
	kamaShortFilter := lastClose < kama

	var bullishBreak, bearishBreak bool

	if er >= 0.6 {
		// Strong trend zone: standard Donchian breakout filtered by KAMA
		bullishBreak = lastClose > donchianHigh && kamaLongFilter
		bearishBreak = lastClose < donchianLow && kamaShortFilter
	} else {
		// Mixed zone (ER 0.3-0.6): require 3-bar CVD confirmation as well
		bullishBreak = lastClose > donchianHigh && kamaLongFilter && cvd3Rising
		bearishBreak = lastClose < donchianLow && kamaShortFilter && cvd3Falling
	}

	// Confidence scales with efficiency ratio quality
	conf := 0.68 + 0.10*(er-0.3)/0.7 // 0.68 at ER=0.3, up to 0.78 at ER=1.0
	conf = math.Min(conf, 0.78)

	if bullishBreak {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := kama - 0.5*atr
		if price-sl < minSL {
			sl = price - minSL
		}
		slDist := price - sl
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		if (tp-price)/slDist < 2.0 {
			return NoSignal(name)
		}
		zone := "mixed (CVD confirmed)"
		if er >= 0.6 {
			zone = "trending"
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"ATF LONG [%s]: ER=%.2f, KAMA=%.0f, Donchian break=%.0f>%.0f",
				zone, er, kama, lastClose, donchianHigh,
			),
		}
	}

	if bearishBreak {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := kama + 0.5*atr
		if sl-price < minSL {
			sl = price + minSL
		}
		slDist := sl - price
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		if (price-tp)/slDist < 2.0 {
			return NoSignal(name)
		}
		zone := "mixed (CVD confirmed)"
		if er >= 0.6 {
			zone = "trending"
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"ATF SHORT [%s]: ER=%.2f, KAMA=%.0f, Donchian break=%.0f<%.0f",
				zone, er, kama, lastClose, donchianLow,
			),
		}
	}

	return NoSignal(name)
}
