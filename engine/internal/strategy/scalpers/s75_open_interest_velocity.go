package scalpers

import (
	"fmt"
	"math"
)

// S75 — Open_Interest_Velocity
//
// Research:   Harris (1986) "Cross-Security Tests of the Mixture of
//
//	Distributions Hypothesis" — open interest change rate
//	(velocity, not just level) as a proxy for the rate of new
//	information/position flow entering a market.
//
// Regime:     TRENDING, RANGING
// Timeframes: 15m (entry trigger), OI velocity computed from ctx.OIHistory
// Logic:      velocity = (latest - OIHistory[len-4]) / OIHistory[len-4],
//
//	i.e. rate of OI change over the last 4 readings. Requires
//	len(OIHistory) >= 4, else NoSignal. High positive velocity
//	(OI building fast) combined with price direction confirmed by
//	EMA9 vs EMA21 on Candles15m → follow the move (fresh
//	directional conviction entering). High negative velocity (OI
//	unwinding fast — covering/liquidation cascade completing) →
//	fade the recent move (trade against it, expecting
//	stabilization once the unwind exhausts).
//
// Edge:       OI velocity (rate of change) captures regime shifts in
//
//	positioning flow earlier than a single-step OI delta, and
//	distinguishes "fresh momentum building" from "capitulation
//	unwind" — two very different trade postures from the same
//	underlying OI feed that other OI-based strategies (S9, S15)
//	don't separate this way.
//
// Data dependency: DONE — ctx.OIHistory is already live, populated by the
//
//	existing UpdateOI feed which appends a new reading every call
//	(rolling window, up to 8 readings, oldest first per types.go).
type OpenInterestVelocity struct{}

func (s *OpenInterestVelocity) Name() string { return "Open_Interest_Velocity" }

func (s *OpenInterestVelocity) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	oiVelocityHighThreshold = 0.03  // +3% OI change over the 4-reading window = "high positive"
	oiVelocityLowThreshold  = -0.03 // -3% = "high negative" (fast unwind)
)

func (s *OpenInterestVelocity) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}
	if len(ctx.OIHistory) < 4 {
		return NoSignal(name)
	}

	oiHist := ctx.OIHistory
	latest := oiHist[len(oiHist)-1]
	base := oiHist[len(oiHist)-4]
	if base == 0 {
		return NoSignal(name)
	}
	velocity := (latest - base) / base

	ema9 := EMA(ctx.Candles15m, 9)
	ema21 := EMA(ctx.Candles15m, 21)
	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 || ema9 == 0 || ema21 == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	slDist := math.Max(1.0*atr15m, 0.003*price)

	priceUptrend := ema9 > ema21
	priceDowntrend := ema9 < ema21

	// ── High positive velocity + confirmed directional trend → FOLLOW ──────
	if velocity >= oiVelocityHighThreshold {
		if priceUptrend {
			sl := price - slDist
			tp1 := price + 2.0*slDist
			tp2 := price + 3.0*slDist
			if sl >= price || tp1 <= price {
				return NoSignal(name)
			}
			return Signal{
				Strategy:    name,
				Direction:   DirectionLong,
				Confidence:  0.64,
				StopLoss:    sl,
				TakeProfit:  tp1,
				TakeProfit2: tp2,
				Reason: fmt.Sprintf(
					"OI velocity FOLLOW LONG: velocity=%.2f%% over 4 readings (latest=%.0f base=%.0f), "+
						"EMA9>EMA21 uptrend confirmed",
					velocity*100, latest, base,
				),
			}
		}
		if priceDowntrend {
			sl := price + slDist
			tp1 := price - 2.0*slDist
			tp2 := price - 3.0*slDist
			if sl <= price || tp1 >= price {
				return NoSignal(name)
			}
			return Signal{
				Strategy:    name,
				Direction:   DirectionShort,
				Confidence:  0.64,
				StopLoss:    sl,
				TakeProfit:  tp1,
				TakeProfit2: tp2,
				Reason: fmt.Sprintf(
					"OI velocity FOLLOW SHORT: velocity=%.2f%% over 4 readings (latest=%.0f base=%.0f), "+
						"EMA9<EMA21 downtrend confirmed",
					velocity*100, latest, base,
				),
			}
		}
		return NoSignal(name)
	}

	// ── High negative velocity (fast unwind) → FADE the recent price move ──
	if velocity <= oiVelocityLowThreshold {
		// Fade: if price has been trending down into the unwind, expect
		// stabilization/bounce (LONG); if trending up, expect pullback (SHORT).
		if priceDowntrend {
			sl := price - slDist
			tp1 := price + 2.0*slDist
			tp2 := price + 3.0*slDist
			if sl >= price || tp1 <= price {
				return NoSignal(name)
			}
			return Signal{
				Strategy:    name,
				Direction:   DirectionLong,
				Confidence:  0.60,
				StopLoss:    sl,
				TakeProfit:  tp1,
				TakeProfit2: tp2,
				Reason: fmt.Sprintf(
					"OI velocity FADE LONG: velocity=%.2f%% over 4 readings (latest=%.0f base=%.0f) — "+
						"fast unwind/covering into downtrend, expect stabilization",
					velocity*100, latest, base,
				),
			}
		}
		if priceUptrend {
			sl := price + slDist
			tp1 := price - 2.0*slDist
			tp2 := price - 3.0*slDist
			if sl <= price || tp1 >= price {
				return NoSignal(name)
			}
			return Signal{
				Strategy:    name,
				Direction:   DirectionShort,
				Confidence:  0.60,
				StopLoss:    sl,
				TakeProfit:  tp1,
				TakeProfit2: tp2,
				Reason: fmt.Sprintf(
					"OI velocity FADE SHORT: velocity=%.2f%% over 4 readings (latest=%.0f base=%.0f) — "+
						"fast unwind/covering into uptrend, expect pullback",
					velocity*100, latest, base,
				),
			}
		}
		return NoSignal(name)
	}

	return NoSignal(name)
}
