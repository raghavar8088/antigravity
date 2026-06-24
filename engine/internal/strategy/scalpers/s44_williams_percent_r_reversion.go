package scalpers

import (
	"fmt"
	"math"
)

// S44 — Williams %R Reversion
//
// Research:   Larry Williams (1979) Williams %R momentum oscillator.
// Regime:     RANGING only
// Timeframes: 5m (Williams %R, period 14, implemented inline)
// Logic:      Williams %R = ((highN - close) / (highN - lowN)) * -100 over
//             Candles5m period 14. Requires the oscillator held below -80
//             (oversold) or above -20 (overbought) for 2+ consecutive bars,
//             then reverting back through the -50 midline.
// Edge:       The "held for 2+ bars then crossing -50" pattern filters out
//             single-bar spikes that revert immediately without real
//             momentum exhaustion.

type WilliamsPercentRReversion struct{}

func (s *WilliamsPercentRReversion) Name() string { return "Williams_Percent_R_Reversion" }

func (s *WilliamsPercentRReversion) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

// williamsPercentR computes Williams %R for the candle ending at the last
// index of `candles`, over `period` bars. Returns 0 if insufficient data
// (treated as neutral midline).
func williamsPercentR(candles []Candle, period int) float64 {
	if len(candles) < period {
		return -50
	}
	tail := candles[len(candles)-period:]
	hi, lo := tail[0].High, tail[0].Low
	for _, c := range tail {
		if c.High > hi {
			hi = c.High
		}
		if c.Low < lo {
			lo = c.Low
		}
	}
	if hi == lo {
		return -50
	}
	close := tail[len(tail)-1].Close
	return ((hi - close) / (hi - lo)) * -100
}

func (s *WilliamsPercentRReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 17 {
		return NoSignal(name)
	}

	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}

	c5 := ctx.Candles5m
	n := len(c5)

	wr0 := williamsPercentR(c5[:n-2], 14) // 2 bars ago
	wr1 := williamsPercentR(c5[:n-1], 14) // 1 bar ago
	wr2 := williamsPercentR(c5, 14)       // current

	price := ctx.Price

	// Bullish: held below -80 for 2+ bars, now reverting up through -50.
	heldOversold := wr0 <= -80 && wr1 <= -80
	crossedUpThroughMid := wr1 < -50 && wr2 >= -50

	if heldOversold && crossedUpThroughMid {
		sl := price - math.Max(1.0*atr5m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := price + 2.0*risk
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.68,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: Williams %%R held oversold (<-80) for 2 bars (%.1f, %.1f), "+
					"now reverted through -50 to %.1f, price=%.0f",
				wr0, wr1, wr2, price,
			),
		}
	}

	// Bearish: held above -20 for 2+ bars, now reverting down through -50.
	heldOverbought := wr0 >= -20 && wr1 >= -20
	crossedDownThroughMid := wr1 > -50 && wr2 <= -50

	if heldOverbought && crossedDownThroughMid {
		sl := price + math.Max(1.0*atr5m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := price - 2.0*risk
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.68,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: Williams %%R held overbought (>-20) for 2 bars (%.1f, %.1f), "+
					"now reverted through -50 to %.1f, price=%.0f",
				wr0, wr1, wr2, price,
			),
		}
	}

	return NoSignal(name)
}
