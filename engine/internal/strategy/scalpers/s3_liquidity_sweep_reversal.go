package scalpers

import (
	"fmt"
	"math"
)

// sweepCVDBearishDiv checks 3-bar rolling CVD divergence for bearish sweep reversals.
// Returns true when CVD has been declining for the last 3 readings.
func sweepCVDBearishDiv(ctx MarketContext) bool {
	if len(ctx.CVDHistory) < 3 {
		return false
	}
	n := len(ctx.CVDHistory)
	return ctx.CVDHistory[n-2] < ctx.CVDHistory[n-3] && ctx.CVDHistory[n-1] < ctx.CVDHistory[n-2]
}

// sweepCVDBullishDiv checks 3-bar rolling CVD divergence for bullish sweep reversals.
// Returns true when CVD has been rising for the last 3 readings.
func sweepCVDBullishDiv(ctx MarketContext) bool {
	if len(ctx.CVDHistory) < 3 {
		return false
	}
	n := len(ctx.CVDHistory)
	return ctx.CVDHistory[n-2] > ctx.CVDHistory[n-3] && ctx.CVDHistory[n-1] > ctx.CVDHistory[n-2]
}

// S3 — Liquidity Sweep Reversal
//
// Regime:     VOLATILE only
// Timeframes: 1m (sweep detection) + 5m (structure)
// Logic:      Price spikes through a known swing high/low (grabs liquidity),
//             then closes back inside the range within 2 candles. CVD diverges.
//             Funding rate spike confirms forced liquidations drove the move.

type LiquiditySweepReversal struct{}

func (s *LiquiditySweepReversal) Name() string { return "Liquidity_Sweep_Reversal" }

func (s *LiquiditySweepReversal) ValidRegimes() []Regime {
	return []Regime{RegimeVolatile}
}

func (s *LiquiditySweepReversal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles1m) < 30 || len(ctx.Candles5m) < 25 {
		return NoSignal(name)
	}

	lookback5m := ctx.Candles5m[:len(ctx.Candles5m)-2]
	if len(lookback5m) < 20 {
		return NoSignal(name)
	}
	rangeHigh := SwingHigh(lookback5m, 20)
	rangeLow := SwingLow(lookback5m, 20)
	if rangeHigh == 0 || rangeLow == 0 || rangeHigh <= rangeLow {
		return NoSignal(name)
	}

	c1m := ctx.Candles1m
	n := len(c1m)
	if n < 3 {
		return NoSignal(name)
	}
	// Use last CLOSED candle (index n-2) as the reference point to avoid look-ahead bias.
	lastClosed := c1m[n-2]

	atr1m := ATR(ctx.Candles1m, 14)
	atr5m := ATR(ctx.Candles5m, 14)
	if atr1m == 0 || atr5m == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	fundingSpike := ctx.FundingRate > 0.0001 || ctx.FundingRate < -0.0001 // raw: 0.0001 = 0.01% per 8h

	sweepHigh := lastClosed.High > rangeHigh && lastClosed.Close < rangeHigh
	cvdBearishDiv := sweepCVDBearishDiv(ctx)

	if sweepHigh && cvdBearishDiv && fundingSpike {
		sl := lastClosed.High + math.Max(0.5*atr5m, 0.001*lastClosed.High)
		tp := rangeLow
		risk := sl - price
		reward := price - tp
		if reward < 2*risk || risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"VOLATILE: liquidity sweep above %.0f (wick=%.0f, close=%.0f), "+
					"3-bar CVD bearish div, funding spike=%.5f raw",
				rangeHigh, lastClosed.High, lastClosed.Close, ctx.FundingRate,
			),
		}
	}

	sweepLow := lastClosed.Low < rangeLow && lastClosed.Close > rangeLow
	cvdBullishDiv := sweepCVDBullishDiv(ctx)

	if sweepLow && cvdBullishDiv && fundingSpike {
		sl := lastClosed.Low - math.Max(0.5*atr5m, 0.001*lastClosed.Low)
		tp := rangeHigh
		risk := price - sl
		reward := tp - price
		if reward < 2*risk || risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"VOLATILE: liquidity sweep below %.0f (wick=%.0f, close=%.0f), "+
					"3-bar CVD bullish div, funding spike=%.5f raw",
				rangeLow, lastClosed.Low, lastClosed.Close, ctx.FundingRate,
			),
		}
	}

	return NoSignal(name)
}
