package scalpers

import "fmt"

// S6 — CVD Divergence Sniper
//
// Regime:     ALL (fires in TRENDING, RANGING, VOLATILE — but not UNKNOWN)
// Timeframes: 5m (divergence bars) + 15m (swing level reference)
// Logic:      Price makes a new higher high / lower low on the 5m, but the
//             cumulative volume delta does NOT confirm: sellers are absorbing
//             the up move (bearish div) or buyers are absorbing the down move
//             (bullish div). Funding rate leans against the price move.
//             Confidence: 0.71.

type CVDDivergenceSniper struct{}

func (s *CVDDivergenceSniper) Name() string { return "CVD_Divergence_Sniper" }

func (s *CVDDivergenceSniper) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *CVDDivergenceSniper) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 20 || len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	c5 := ctx.Candles5m
	n := len(c5)
	if n < 4 {
		return NoSignal(name)
	}

	cur := c5[n-1]
	prev := c5[n-2]

	atr5m := ATR(c5, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	cvd := ctx.CVD
	cvdPrev := ctx.CVDPrev

	// Bearish divergence: price makes new high, CVD falls
	priceMadeHigher := cur.High > prev.High
	cvdFailed := cvd < cvdPrev
	// Funding leans against — elevated positive funding means longs crowded
	fundingLongBias := ctx.FundingRate > 0.005

	if priceMadeHigher && cvdFailed {
		sl := cur.High + 0.15*atr5m
		tp := price - 2.0*atr5m
		risk := sl - price
		reward := price - tp
		if reward < 1.5*risk || risk <= 0 {
			return NoSignal(name)
		}
		conf := 0.71
		if fundingLongBias {
			conf = 0.74
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"%s: bearish CVD divergence — price new high %.0f>%.0f, "+
					"CVD fell %.0f→%.0f, funding=%.4f%%",
				ctx.Regime, cur.High, prev.High, cvdPrev, cvd, ctx.FundingRate,
			),
		}
	}

	// Bullish divergence: price makes new low, CVD holds/rises
	priceMadeLower := cur.Low < prev.Low
	cvdHeld := cvd >= cvdPrev
	fundingShortBias := ctx.FundingRate < -0.005

	if priceMadeLower && cvdHeld {
		sl := cur.Low - 0.15*atr5m
		tp := price + 2.0*atr5m
		risk := price - sl
		reward := tp - price
		if reward < 1.5*risk || risk <= 0 {
			return NoSignal(name)
		}
		conf := 0.71
		if fundingShortBias {
			conf = 0.74
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"%s: bullish CVD divergence — price new low %.0f<%.0f, "+
					"CVD held %.0f→%.0f, funding=%.4f%%",
				ctx.Regime, cur.Low, prev.Low, cvdPrev, cvd, ctx.FundingRate,
			),
		}
	}

	return NoSignal(name)
}
