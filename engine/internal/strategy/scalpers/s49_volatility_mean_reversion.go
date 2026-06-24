package scalpers

import (
	"fmt"
	"math"
)

// S49 — Volatility Mean Reversion
//
// Research:   Schwert (1989) "Why Does Stock Market Volatility Change Over
//             Time?" — volatility clustering and mean reversion.
// Regime:     RANGING only
// Timeframes: 1h (vol history) + 15m (range-bound entry trigger)
// Logic:      Uses ctx.DVOL/DVOLHistory when DVOLPopulated && DVOLHealthy.
//             When current vol deviates more than 2 std devs above its own
//             rolling mean (computed from DVOLHistory), expect volatility
//             mean reversion — trade a tight-range fade-the-extension entry
//             (similar style to S46's Keltner reversion) on Candles15m.
//
// FALLBACK NOTE: if DVOL is unavailable (DVOLPopulated==false or
// DVOLHealthy==false), this strategy gracefully falls back to RealizedVol()
// computed on Candles1h as the volatility proxy instead of permanently
// returning NoSignal — this keeps the strategy live even when the Deribit
// DVOL feed is down, per spec.
// Edge:       Vol spikes are the trigger, not price levels alone — entries
//             only fire when the underlying environment itself is
//             statistically overextended, not just price.

type VolatilityMeanReversion struct{}

func (s *VolatilityMeanReversion) Name() string { return "Volatility_Mean_Reversion" }

func (s *VolatilityMeanReversion) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func stdevFloats(vals []float64, mean float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var variance float64
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	return math.Sqrt(variance / float64(len(vals)))
}

func meanFloats(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func (s *VolatilityMeanReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 22 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	var currentVol float64
	var volMean float64
	var volStdev float64
	var source string

	if ctx.DVOLPopulated && ctx.DVOLHealthy && len(ctx.DVOLHistory) >= 5 {
		currentVol = ctx.DVOL
		volMean = meanFloats(ctx.DVOLHistory)
		volStdev = stdevFloats(ctx.DVOLHistory, volMean)
		source = "DVOL"
	} else {
		// Fallback: RealizedVol on Candles1h, build a small rolling history
		// from successively trimmed windows since we don't have a persisted
		// realized-vol history field.
		if len(ctx.Candles1h) < 30 {
			return NoSignal(name)
		}
		const histLen = 8
		const rvPeriod = 24
		if len(ctx.Candles1h) < rvPeriod+1+histLen {
			return NoSignal(name)
		}
		hist := make([]float64, 0, histLen)
		n := len(ctx.Candles1h)
		for i := histLen; i >= 1; i-- {
			end := n - i
			if end < rvPeriod+1 {
				continue
			}
			hist = append(hist, RealizedVol(ctx.Candles1h[:end], rvPeriod))
		}
		currentVol = RealizedVol(ctx.Candles1h, rvPeriod)
		if currentVol == 0 || len(hist) < 5 {
			return NoSignal(name)
		}
		volMean = meanFloats(hist)
		volStdev = stdevFloats(hist, volMean)
		source = "RealizedVol(1h, fallback)"
	}

	if volStdev == 0 {
		return NoSignal(name)
	}

	zScore := (currentVol - volMean) / volStdev
	const zThreshold = 2.0

	if zScore < zThreshold {
		// Vol not extended — no mean-reversion edge to trade.
		return NoSignal(name)
	}

	// Vol is overextended — trade a tight-range fade similar to S46's
	// Keltner-style reversion: fade price back toward EMA20 on 15m.
	c15 := ctx.Candles15m
	ema20 := EMA(c15, 20)
	if ema20 == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	upperBand := ema20 + 2*atr15m
	lowerBand := ema20 - 2*atr15m

	if price >= upperBand {
		sl := price + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := ema20
		reward := price - tp
		if reward < 2.0*risk {
			tp = price - 2.0*risk
			reward = price - tp
		}
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
				"RANGING: %s vol z-score=%.2f (>2 std dev above mean=%.2f), price=%.0f "+
					"extended above EMA20+2ATR=%.0f, fading toward EMA20",
				source, zScore, volMean, price, upperBand,
			),
		}
	}

	if price <= lowerBand {
		sl := price - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := ema20
		reward := tp - price
		if reward < 2.0*risk {
			tp = price + 2.0*risk
			reward = tp - price
		}
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
				"RANGING: %s vol z-score=%.2f (>2 std dev above mean=%.2f), price=%.0f "+
					"extended below EMA20-2ATR=%.0f, fading toward EMA20",
				source, zScore, volMean, price, lowerBand,
			),
		}
	}

	return NoSignal(name)
}
