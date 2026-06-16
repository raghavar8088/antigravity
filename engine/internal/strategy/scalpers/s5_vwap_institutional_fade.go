package scalpers

import (
	"fmt"
	"math"
)

// S5 — VWAP Institutional Fade
//
// Regime:     RANGING only
// Timeframes: 5m (entry) + 15m (VWAP / structure)
// Logic:      Price extends aggressively away from session VWAP (>1.5×ATR).
//             Order book shows large wall on the extended side (institutional
//             supply/demand). RSI divergence confirms reversal setup.
//             Volume gate: current 1m-projected volume must be >= 50% of 24h avg.
//             Session boost: 07-12 UTC or 13-20 UTC → confidence 0.78 vs 0.73 base.
//             Fade back toward VWAP.

type VWAPInstitutionalFade struct{}

func (s *VWAPInstitutionalFade) Name() string { return "VWAP_Institutional_Fade" }

func (s *VWAPInstitutionalFade) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *VWAPInstitutionalFade) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 25 || len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	// Volume gate: replace session gate with 24h volume activity check.
	// Use 1m candles (last 12 bars ≈ current 1h) vs avg vol of all 5m candles × 12
	// to project onto same timescale. Require >= 50% of average activity.
	// ctx.Candles5m last bar volume × 12 approximates current-hour rate.
	last5m := ctx.Candles5m[len(ctx.Candles5m)-1]
	avgVol24h := AvgVolume(ctx.Candles5m, len(ctx.Candles5m)) // avg per 5m bar
	currentVolProjected := last5m.Volume * 12                  // project to 1h equivalent
	avgVol24hHourly := avgVol24h * 12
	if avgVol24hHourly > 0 && currentVolProjected < avgVol24hHourly*0.5 {
		return NoSignal(name)
	}

	vwap := VWAP(ctx.Candles15m)
	if vwap == 0 {
		return NoSignal(name)
	}

	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}

	// RSI divergence: compare current RSI vs RSI of 4 bars ago
	c5 := ctx.Candles5m
	n5 := len(c5)
	rsiCurrent := RSI(c5, 14)
	var rsiPrior float64
	if n5 >= 4 {
		rsiPrior = RSI(c5[:n5-4], 14)
	} else {
		rsiPrior = rsiCurrent
	}

	price := ctx.Price
	ob := ctx.OrderBook
	deviation := price - vwap
	absDeviation := math.Abs(deviation)
	extended := absDeviation > 1.5*atr5m

	if !extended {
		return NoSignal(name)
	}

	// Active session boost: 07-12 UTC or 13-20 UTC
	h := ctx.Now.Hour()
	activeSession := (h >= 7 && h < 12) || (h >= 13 && h < 20)
	baseConf := 0.73
	if activeSession {
		baseConf = 0.78
	}

	if deviation > 0 {
		// Price above VWAP — look for bearish fade
		// RSI bearish divergence: new high in price, but RSI lower than prior
		currentHigh := c5[n5-1].High
		priorHigh := float64(0)
		if n5 >= 4 {
			priorHigh = c5[n5-5].High
		}
		bearishRSIDiv := currentHigh > priorHigh && rsiCurrent < rsiPrior

		if !bearishRSIDiv && rsiCurrent <= 70 {
			return NoSignal(name)
		}

		askWallDominant := ob.AskWallSize > ob.BidWallSize*1.5
		if !ob.IsPopulated() {
			// fallback when no OB data: require RSI divergence
			if !bearishRSIDiv {
				return NoSignal(name)
			}
		} else if !askWallDominant {
			return NoSignal(name)
		}

		sl := price + 0.5*atr5m
		tp := vwap
		risk := sl - price
		reward := price - tp
		if reward < 2*risk || risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: baseConf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: price %.0f extended %.0f above VWAP=%.0f (>1.5×ATR=%.0f), "+
					"RSI=%.1f (prior=%.1f), bearishDiv=%v, ask wall %.2f vs bid %.2f, active=%v",
				price, deviation, vwap, atr5m, rsiCurrent, rsiPrior, bearishRSIDiv,
				ob.AskWallSize, ob.BidWallSize, activeSession,
			),
		}
	}

	if deviation < 0 {
		// Price below VWAP — look for bullish fade
		// RSI bullish divergence: new low in price, but RSI higher than prior
		currentLow := c5[n5-1].Low
		priorLow := float64(0)
		if n5 >= 4 {
			priorLow = c5[n5-5].Low
		}
		bullishRSIDiv := currentLow < priorLow && rsiCurrent > rsiPrior

		if !bullishRSIDiv && rsiCurrent >= 30 {
			return NoSignal(name)
		}

		bidWallDominant := ob.BidWallSize > ob.AskWallSize*1.5
		if !ob.IsPopulated() {
			if !bullishRSIDiv {
				return NoSignal(name)
			}
		} else if !bidWallDominant {
			return NoSignal(name)
		}

		sl := price - 0.5*atr5m
		tp := vwap
		risk := price - sl
		reward := tp - price
		if reward < 2*risk || risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: baseConf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: price %.0f extended %.0f below VWAP=%.0f (>1.5×ATR=%.0f), "+
					"RSI=%.1f (prior=%.1f), bullishDiv=%v, bid wall %.2f vs ask %.2f, active=%v",
				price, -deviation, vwap, atr5m, rsiCurrent, rsiPrior, bullishRSIDiv,
				ob.BidWallSize, ob.AskWallSize, activeSession,
			),
		}
	}

	return NoSignal(name)
}
