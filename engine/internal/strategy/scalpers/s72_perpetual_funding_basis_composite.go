package scalpers

import (
	"fmt"
	"math"
)

// S72 — Perpetual_Funding_Basis_Composite
//
// Research:   Documented "Funding Rate Premium" alpha-source literature on
//
//	perpetual swap funding as a persistent, harvestable signal of
//	crowded positioning (funding mean-reverts because it is the
//	mechanism that re-balances long/short open interest).
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h
// Logic:      Differentiation from S8 (Funding_Rate_Fade — single-reading
//
//	threshold fade) and S15 (OI_Funding_Crowding_Composite —
//	combines OI extremity with funding extremity): S72 combines
//	THREE funding-only signals that neither S8 nor S15 combine
//	with each other: (1) current FundingRate level/magnitude,
//	(2) the TREND across ctx.FundingHistory (is funding rising or
//	falling — compares the last 2 readings to detect acceleration,
//	not just level), and (3) a cumulative-funding-paid proxy
//	(sum of the available FundingHistory readings as a rough
//	proxy for "how much funding has recently been paid" — NOTE:
//	this is a proxy, not a true 24h-cumulative figure, since
//	FundingHistory only retains the last 3 readings and a true
//	cumulative would need a longer retained window). All three
//	legs must agree (high level + rising/accelerating trend in the
//	crowded direction + high cumulative proxy) for max-crowding
//	classification → fade.
//
// Edge:       Requiring trend + cumulative agreement on top of level (not
//
//	just level alone, which is all S8 checks) targets the subset
//	of funding extremes that are actively WORSENING rather than
//	already peaking/reverting — historically a higher-conviction
//	fade entry than a level-only threshold.
//
// Data dependency: DONE — ctx.FundingRate and ctx.FundingHistory are already
//
//	live/wired in production (same feed S8/S15 use).
type PerpetualFundingBasisComposite struct{}

func (s *PerpetualFundingBasisComposite) Name() string {
	return "Perpetual_Funding_Basis_Composite"
}

func (s *PerpetualFundingBasisComposite) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	pfbcLevelThreshold = 0.0003 // ±0.03% per 8h raw decimal — "high magnitude" leg
	pfbcCumThreshold   = 0.0007 // sum of FundingHistory readings considered "high cumulative" proxy
)

func (s *PerpetualFundingBasisComposite) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	// 3-bar confirmation: require full funding history for the trend leg.
	if len(ctx.FundingHistory) < 3 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}

	fh := ctx.FundingHistory
	current := ctx.FundingRate
	last := fh[len(fh)-1]
	prev := fh[len(fh)-2]

	// Leg 1: level magnitude.
	levelHighPositive := current >= pfbcLevelThreshold
	levelHighNegative := current <= -pfbcLevelThreshold

	// Leg 2: trend across history — rising (accelerating positive) or
	// falling (accelerating negative), comparing last 2 readings.
	trendRising := last > prev
	trendFalling := last < prev

	// Leg 3: cumulative-funding-paid proxy — sum of available readings.
	// Documented as a PROXY: true 24h-cumulative funding needs a longer
	// retained history than FundingHistory's 3 readings; this is a rough
	// "recent funding pressure" stand-in only.
	var cumSum float64
	for _, f := range fh {
		cumSum += f
	}
	cumHighPositive := cumSum >= pfbcCumThreshold
	cumHighNegative := cumSum <= -pfbcCumThreshold

	slDist := math.Max(1.0*atr1h, 0.003*price)

	// ── Max crowding LONG-side (positive funding, rising, high cumulative) → fade SHORT ──
	if levelHighPositive && trendRising && cumHighPositive {
		sl := price + slDist
		tp1 := price - 2.0*slDist
		tp2 := price - 3.0*slDist
		if sl <= price || tp1 >= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.70,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"funding composite fade SHORT: level=%.5f (>=%.5f), rising (%.5f->%.5f), "+
					"cumulative proxy=%.5f (>=%.5f) — max crowding",
				current, pfbcLevelThreshold, prev, last, cumSum, pfbcCumThreshold,
			),
		}
	}

	// ── Max crowding SHORT-side (negative funding, falling, high-negative cumulative) → fade LONG ──
	if levelHighNegative && trendFalling && cumHighNegative {
		sl := price - slDist
		tp1 := price + 2.0*slDist
		tp2 := price + 3.0*slDist
		if sl >= price || tp1 <= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.70,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"funding composite fade LONG: level=%.5f (<=-%.5f), falling (%.5f->%.5f), "+
					"cumulative proxy=%.5f (<=-%.5f) — max crowding",
				current, pfbcLevelThreshold, prev, last, cumSum, pfbcCumThreshold,
			),
		}
	}

	return NoSignal(name)
}
