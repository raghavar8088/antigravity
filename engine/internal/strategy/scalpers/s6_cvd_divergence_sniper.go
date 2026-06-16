package scalpers

import (
	"fmt"
	"math"
)

// S6 — CVD Divergence Sniper
//
// Regime:     ALL (fires in TRENDING, RANGING, VOLATILE — but not UNKNOWN)
// Timeframes: 5m (divergence bars) + 15m (swing level reference)
// Logic:      Price makes 3 consecutive higher highs / lower lows on the 5m,
//             but the cumulative volume delta diverges over those same 3 bars.
//             MACD histogram confirms momentum direction.
//             Funding rate leans against the price move.
//             Confidence: 0.71.

type CVDDivergenceSniper struct{}

func (s *CVDDivergenceSniper) Name() string { return "CVD_Divergence_Sniper" }

func (s *CVDDivergenceSniper) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

// rollingCVDBearishDiv checks 3-bar rolling divergence for shorts:
// price made 3 consecutive higher highs, CVD made 3 consecutive lower readings,
// and volume is declining.
func rollingCVDBearishDiv(c5 []Candle, cvdHistory []float64) bool {
	n := len(c5)
	if n < 4 || len(cvdHistory) < 3 {
		return false
	}
	// price: 3 consecutive higher highs
	bar0 := c5[n-4]
	bar1 := c5[n-3]
	bar2 := c5[n-2]
	bar3 := c5[n-1]
	if !(bar1.High > bar0.High && bar2.High > bar1.High && bar3.High > bar2.High) {
		return false
	}
	// CVD: 3 consecutive lower readings (newest = cvdHistory[len-1])
	nh := len(cvdHistory)
	cvd0 := cvdHistory[nh-3]
	cvd1 := cvdHistory[nh-2]
	cvd2 := cvdHistory[nh-1]
	if !(cvd1 < cvd0 && cvd2 < cvd1) {
		return false
	}
	// volume declining
	if !(bar2.Volume < bar1.Volume && bar3.Volume < bar2.Volume) {
		return false
	}
	return true
}

// rollingCVDBullishDiv checks 3-bar rolling divergence for longs:
// price made 3 consecutive lower lows, CVD made 3 consecutive higher readings,
// and volume is declining.
func rollingCVDBullishDiv(c5 []Candle, cvdHistory []float64) bool {
	n := len(c5)
	if n < 4 || len(cvdHistory) < 3 {
		return false
	}
	bar0 := c5[n-4]
	bar1 := c5[n-3]
	bar2 := c5[n-2]
	bar3 := c5[n-1]
	if !(bar1.Low < bar0.Low && bar2.Low < bar1.Low && bar3.Low < bar2.Low) {
		return false
	}
	nh := len(cvdHistory)
	cvd0 := cvdHistory[nh-3]
	cvd1 := cvdHistory[nh-2]
	cvd2 := cvdHistory[nh-1]
	if !(cvd1 > cvd0 && cvd2 > cvd1) {
		return false
	}
	if !(bar2.Volume < bar1.Volume && bar3.Volume < bar2.Volume) {
		return false
	}
	return true
}

func (s *CVDDivergenceSniper) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	// Increased candle guard to 30
	if len(ctx.Candles5m) < 20 || len(ctx.Candles15m) < 30 {
		return NoSignal(name)
	}

	c5 := ctx.Candles5m
	n := len(c5)
	if n < 4 {
		return NoSignal(name)
	}

	last15m := ctx.Candles15m[len(ctx.Candles15m)-1]

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	price := ctx.Price

	// MACD confirmation on 5m
	macd5m := MACD(c5)
	macd5mPrev := MACD(c5[:n-1])

	bearMACDConfirm := macd5m.Histogram < 0 && macd5m.Histogram < macd5mPrev.Histogram
	bullMACDConfirm := macd5m.Histogram > 0 && macd5m.Histogram > macd5mPrev.Histogram

	// Use CVDHistory from context (ring buffer); fall back to CVD/CVDPrev if empty
	cvdHist := ctx.CVDHistory
	if len(cvdHist) < 3 {
		// build minimal 2-entry history from CVD + CVDPrev so single-bar logic still works
		cvdHist = []float64{ctx.CVDPrev, ctx.CVD}
	}

	ob := ctx.OrderBook
	obPopulated := ob.IsPopulated()

	// ── Bearish divergence ────────────────────────────────────────────────────
	bearishDiv := rollingCVDBearishDiv(c5, cvdHist)
	if bearishDiv && bearMACDConfirm {
		// SL: last 15m high + max(1.0×ATR15m, 0.3% of price)
		sl := last15m.High + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist < 0.0025*price {
			return NoSignal(name)
		}
		// TP capped at 2×SL distance
		tp := price - 2.0*slDist
		// Validate R:R
		reward := price - tp
		risk := sl - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}

		fundingLongBias := ctx.FundingRate > 0.00005 // raw: 0.00005 = 0.005% per 8h
		conf := 0.71
		if fundingLongBias {
			conf = 0.74
		}

		// Order book fallback (FIX 4)
		if obPopulated {
			if ob.AskWallSize <= ob.BidWallSize {
				return NoSignal(name)
			}
		} else {
			// fallback: require both CVD divergence AND MACD (already confirmed above)
			conf *= 0.90
		}

		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"%s: 3-bar bearish CVD divergence — 15m last high=%.0f, "+
					"MACD hist=%.4f declining, funding=%.4f%%",
				ctx.Regime, last15m.High, macd5m.Histogram, ctx.FundingRate,
			),
		}
	}

	// ── Bullish divergence ────────────────────────────────────────────────────
	bullishDiv := rollingCVDBullishDiv(c5, cvdHist)
	if bullishDiv && bullMACDConfirm {
		sl := last15m.Low - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist < 0.0025*price {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		reward := tp - price
		risk := price - sl
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}

		fundingShortBias := ctx.FundingRate < -0.00005 // raw: -0.00005 = -0.005% per 8h
		conf := 0.71
		if fundingShortBias {
			conf = 0.74
		}

		// Order book fallback (FIX 4)
		if obPopulated {
			if ob.BidWallSize <= ob.AskWallSize {
				return NoSignal(name)
			}
		} else {
			conf *= 0.90
		}

		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"%s: 3-bar bullish CVD divergence — 15m last low=%.0f, "+
					"MACD hist=%.4f rising, funding=%.4f%%",
				ctx.Regime, last15m.Low, macd5m.Histogram, ctx.FundingRate,
			),
		}
	}

	return NoSignal(name)
}
