package strategy

import "fmt"

// S3 — Liquidity Sweep Reversal
//
// Regime:     VOLATILE only
// Timeframes: 1m (sweep detection) + 5m (structure)
// Logic:      Price spikes through a known swing high/low (grabs liquidity),
//             then closes back inside the range within 2 candles. CVD diverges
//             (price moved, volume didn't follow). Funding rate spike confirms
//             forced liquidations drove the move.
// Edge:       Catches stop hunts and liquidation cascades at their exhaustion
//             point — some of the sharpest reversals in crypto.

type LiquiditySweepReversal struct{}

func (s *LiquiditySweepReversal) Name() string { return "Liquidity_Sweep_Reversal" }

func (s *LiquiditySweepReversal) ValidRegimes() []Regime {
	return []Regime{RegimeVolatile}
}

func (s *LiquiditySweepReversal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeVolatile {
		return NoSignal(name)
	}
	if len(ctx.Candles1m) < 30 || len(ctx.Candles5m) < 25 {
		return NoSignal(name)
	}

	// ── Define the prior range using 5m swing high/low (last 20 bars) ────────
	// Exclude the last 2 candles — those are the potential sweep candles
	lookback5m := ctx.Candles5m[:len(ctx.Candles5m)-2]
	if len(lookback5m) < 20 {
		return NoSignal(name)
	}
	rangeHigh := SwingHigh(lookback5m, 20)
	rangeLow := SwingLow(lookback5m, 20)
	if rangeHigh == 0 || rangeLow == 0 || rangeHigh <= rangeLow {
		return NoSignal(name)
	}

	// ── Last 2 candles on 1m: check for spike beyond range then close inside ─
	c1m := ctx.Candles1m
	n := len(c1m)
	if n < 3 {
		return NoSignal(name)
	}
	prev2 := c1m[n-2]
	prev1 := c1m[n-1]

	atr1m := ATR(ctx.Candles1m, 14)
	atr5m := ATR(ctx.Candles5m, 14)
	if atr1m == 0 || atr5m == 0 {
		return NoSignal(name)
	}

	price := ctx.Price

	// ── Funding rate: significant spike means forced positioning ──────────────
	// A funding rate spike > 0.01% per 8h (annualised ~10.95%) is notable
	fundingSpike := ctx.FundingRate > 0.01 || ctx.FundingRate < -0.01

	// ── Sweep HIGH → expect reversal DOWN ────────────────────────────────────
	// Candle spiked above rangeHigh (wick), but closed back inside
	sweepHigh := prev2.High > rangeHigh && prev2.Close < rangeHigh
	// CVD divergence: price made new high but CVD didn't confirm
	cvdBearishDiv := CVDDivergesBearish(prev2.High, rangeHigh, ctx.CVD, ctx.CVDPrev)

	if sweepHigh && cvdBearishDiv && fundingSpike {
		// Entry just below the close of the sweep candle
		sl := prev2.High + 0.001*prev2.High // just beyond the wick extreme
		tp := rangeLow                       // target opposite liquidity
		risk := sl - price
		reward := price - tp
		if reward < 2*risk || risk <= 0 {
			return NoSignal(name)
		}
		// Volatile regime: confidence capped, position sized at 50% by risk engine
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"VOLATILE: liquidity sweep above %.0f (wick=%.0f, close=%.0f), "+
					"CVD bearish divergence (%.0f→%.0f), funding spike=%.4f%%",
				rangeHigh, prev2.High, prev2.Close, ctx.CVDPrev, ctx.CVD, ctx.FundingRate,
			),
		}
	}

	// ── Sweep LOW → expect reversal UP ───────────────────────────────────────
	sweepLow := prev2.Low < rangeLow && prev2.Close > rangeLow
	cvdBullishDiv := CVDDivergesBullish(prev2.Low, rangeLow, ctx.CVD, ctx.CVDPrev)
	_ = prev1 // prev1 used for future: two-candle confirmation extension

	if sweepLow && cvdBullishDiv && fundingSpike {
		sl := prev2.Low - 0.001*prev2.Low
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
					"CVD bullish divergence (%.0f→%.0f), funding spike=%.4f%%",
				rangeLow, prev2.Low, prev2.Close, ctx.CVDPrev, ctx.CVD, ctx.FundingRate,
			),
		}
	}

	return NoSignal(name)
}
