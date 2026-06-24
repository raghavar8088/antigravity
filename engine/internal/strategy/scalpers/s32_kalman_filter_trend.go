package scalpers

import (
	"fmt"
	"math"
)

// S32 — Kalman Filter Trend
//
// Citation:   Kalman, "A New Approach to Linear Filtering and Prediction
//             Problems" (1960) — optimal recursive estimation under noisy
//             observations, applied here to smooth price into a trend
//             estimate.
// Regime:     TRENDING only
// Timeframes: 15m
// Logic:      Compute the KalmanFilter() estimate on Candles15m. If raw
//             price diverges from the filtered estimate by more than 1
//             standard error (proxied via ATR), treat it as noise and skip.
//             Otherwise, when the filtered trend direction (filtered value
//             vs. filtered value 1 bar earlier) is clear and price is
//             tracking it, trade in that direction.
// Edge:       Kalman smoothing reduces lag vs. a fixed-period MA while
//             still filtering out single-bar noise spikes.

type KalmanFilterTrend struct{}

func (s *KalmanFilterTrend) Name() string { return "Kalman_Filter_Trend" }

func (s *KalmanFilterTrend) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *KalmanFilterTrend) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 22 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	filtered := KalmanFilter(candles)
	filteredPrev := KalmanFilter(candles[:len(candles)-1])

	if filtered == 0 || filteredPrev == 0 {
		return NoSignal(name)
	}

	price := ctx.Price

	// Noise filter: 1 stderr proxied by ATR — if raw price diverges from the
	// filtered estimate by more than 1 ATR, the divergence is likely noise.
	divergence := math.Abs(price - filtered)
	if divergence > atr15m {
		return NoSignal(name)
	}

	slope := filtered - filteredPrev
	trendUp := slope > 0
	trendDown := slope < 0
	priceTracksUp := price >= filtered-0.2*atr15m
	priceTracksDown := price <= filtered+0.2*atr15m

	if trendUp && priceTracksUp {
		sl := price - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Kalman filtered trend up (filtered=%.0f, prev=%.0f, slope=%.2f), "+
					"price %.0f tracking within 1 ATR",
				filtered, filteredPrev, slope, price,
			),
		}
	}

	if trendDown && priceTracksDown {
		sl := price + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Kalman filtered trend down (filtered=%.0f, prev=%.0f, slope=%.2f), "+
					"price %.0f tracking within 1 ATR",
				filtered, filteredPrev, slope, price,
			),
		}
	}

	return NoSignal(name)
}
