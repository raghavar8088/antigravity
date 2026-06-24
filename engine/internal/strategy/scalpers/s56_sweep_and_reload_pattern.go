package scalpers

import (
	"fmt"
	"math"
)

// S56 — Sweep And Reload Pattern
//
// Regime:     VOLATILE only
// Timeframes: 5m (wick/reload detection)
// Research:   Documented in institutional trading-desk training materials on
//             stop-hunt / liquidity-sweep patterns: a single candle prints a
//             long wick beyond recent structure (sweeping resting stops),
//             then the very next candle "reloads" by closing back inside the
//             prior established range — and crucially the realized order
//             flow (CVD) during the wick candle opposed the direction the
//             wick implied, confirming the move was a liquidity grab rather
//             than genuine directional conviction.
// Logic:      DISTINCT FROM S3 (Liquidity_Sweep_Reversal): S3 operates on
//             swing-level sweeps measured against a 20-bar SwingHigh/SwingLow
//             range using 1m wick + 5m structure + funding-rate confirmation,
//             with reversal confirmed via 3-bar CVD divergence across
//             CVDHistory. S56 instead is a SINGLE-CANDLE-WICK pattern on the
//             5m timeframe alone: it requires (1) one candle whose wick
//             (high-low minus body) is > 2x its own body, (2) the very NEXT
//             candle closes back inside the range of the prior 3 candles
//             (the "reload"), and (3) ctx.CVD direction at evaluation time
//             opposes what price did during the wick (absorption
//             confirmation) — no swing-high/low range lookup, no funding
//             gate, no multi-bar CVD divergence history requirement.
// Edge:       Catches stop-hunt wicks that get immediately reclaimed,
//             entering in the direction opposite the wick once the reload
//             candle confirms the sweep failed to hold.

type SweepAndReloadPattern struct{}

func (s *SweepAndReloadPattern) Name() string { return "Sweep_And_Reload_Pattern" }

func (s *SweepAndReloadPattern) ValidRegimes() []Regime {
	return []Regime{RegimeVolatile}
}

func (s *SweepAndReloadPattern) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	c5 := ctx.Candles5m
	n := len(c5)
	if n < 6 {
		return NoSignal(name)
	}

	atr5m := ATR(c5, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}

	// Wick candle = second-to-last CLOSED candle (n-3); reload candle = last
	// CLOSED candle (n-2); n-1 is the still-forming candle.
	wickCandle := c5[n-3]
	reloadCandle := c5[n-2]
	priorRange := c5[n-6 : n-3] // 3 candles prior to the wick candle

	body := math.Abs(wickCandle.Close - wickCandle.Open)
	upperWick := wickCandle.High - math.Max(wickCandle.Close, wickCandle.Open)
	lowerWick := math.Min(wickCandle.Close, wickCandle.Open) - wickCandle.Low

	if body == 0 {
		return NoSignal(name)
	}

	priorHigh := priorRange[0].High
	priorLow := priorRange[0].Low
	for _, c := range priorRange {
		if c.High > priorHigh {
			priorHigh = c.High
		}
		if c.Low < priorLow {
			priorLow = c.Low
		}
	}
	if priorHigh <= priorLow {
		return NoSignal(name)
	}

	slDistMin := math.Max(1.0*atr5m, 0.003*price)

	// ── Bearish wick (swept highs) -> expect reload down -> LONG is wrong;
	//    sweeping UP and reloading back DOWN means price rejected highs -> SHORT bias is wrong too.
	//    Standard interpretation: wick UP through highs that gets rejected and
	//    reclaimed back inside range = SHORT (fade the failed breakout up).
	upperWickSweep := upperWick > 2.0*body && wickCandle.High > priorHigh
	reloadedInsideAfterUpperSweep := reloadCandle.Close <= priorHigh && reloadCandle.Close >= priorLow
	// CVD should oppose the upward wick, i.e. CVD currently net falling
	// (sell pressure), confirming the upside sweep was not real demand.
	cvdOpposesUpperSweep := ctx.CVD < ctx.CVDPrev

	if upperWickSweep && reloadedInsideAfterUpperSweep && cvdOpposesUpperSweep {
		sl := math.Max(wickCandle.High, price) + slDistMin
		slDist := sl - price
		if slDist < slDistMin {
			sl = price + slDistMin
			slDist = slDistMin
		}
		tp := price - 2.0*slDist
		risk := sl - price
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.69,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Sweep and reload SHORT: wick high=%.0f swept prior range high=%.0f (wick=%.0f > 2x body=%.0f), "+
					"reload candle closed back inside [%.0f,%.0f], CVD opposes (%.0f->%.0f)",
				wickCandle.High, priorHigh, upperWick, body, priorLow, priorHigh, ctx.CVDPrev, ctx.CVD,
			),
		}
	}

	// ── Lower wick sweep (swept lows) that reloads back inside range = LONG
	//    (fade the failed breakdown down).
	lowerWickSweep := lowerWick > 2.0*body && wickCandle.Low < priorLow
	reloadedInsideAfterLowerSweep := reloadCandle.Close >= priorLow && reloadCandle.Close <= priorHigh
	cvdOpposesLowerSweep := ctx.CVD > ctx.CVDPrev

	if lowerWickSweep && reloadedInsideAfterLowerSweep && cvdOpposesLowerSweep {
		sl := math.Min(wickCandle.Low, price) - slDistMin
		slDist := price - sl
		if slDist < slDistMin {
			sl = price - slDistMin
			slDist = slDistMin
		}
		tp := price + 2.0*slDist
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.69,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Sweep and reload LONG: wick low=%.0f swept prior range low=%.0f (wick=%.0f > 2x body=%.0f), "+
					"reload candle closed back inside [%.0f,%.0f], CVD opposes (%.0f->%.0f)",
				wickCandle.Low, priorLow, lowerWick, body, priorLow, priorHigh, ctx.CVDPrev, ctx.CVD,
			),
		}
	}

	return NoSignal(name)
}
