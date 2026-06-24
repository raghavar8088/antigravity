package scalpers

import (
	"fmt"
	"math"
)

// S31 — Time Series Momentum Rebalance
//
// Citation:   Moskowitz, Ooi & Pedersen, "Time Series Momentum" (2012) —
//             trend signals derived purely from an asset's own past returns,
//             independent of cross-sectional ranking against other assets.
// Regime:     TRENDING only
// Timeframes: 1h (24-bar window proxy for "24h"), 4h (proxy for longer
//             horizon confirmation)
// Logic:      Rolling returns computed over 1h, 4h, and a 24-bar sum on
//             Candles1h (proxy for 24h). If a majority of the three windows
//             are positive, go long; if majority negative, go short.
//             Position sized/confidence-weighted by inverse realized
//             volatility (RealizedVol indicator) — higher vol => lower
//             confidence, consistent with vol-targeting in the TSMOM paper.
// Edge:       Diversified momentum confirmation across horizons reduces
//             whipsaw vs. a single-lookback momentum signal.

type TimeSeriesMomentumRebalance struct{}

func (s *TimeSeriesMomentumRebalance) Name() string { return "Time_Series_Momentum_Rebalance" }

func (s *TimeSeriesMomentumRebalance) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *TimeSeriesMomentumRebalance) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 25 || len(ctx.Candles4h) < 5 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	ret1h := barReturn(ctx.Candles1h, 1)
	ret4h := barReturn(ctx.Candles4h, 1)
	ret24hProxy := barReturn(ctx.Candles1h, 24)

	posCount := 0
	negCount := 0
	for _, r := range []float64{ret1h, ret4h, ret24hProxy} {
		if r > 0 {
			posCount++
		} else if r < 0 {
			negCount++
		}
	}

	longMajority := posCount >= 2
	shortMajority := negCount >= 2
	if !longMajority && !shortMajority {
		return NoSignal(name)
	}

	rvol := RealizedVol(ctx.Candles1h, 24)
	if rvol == 0 {
		return NoSignal(name)
	}
	// Inverse-vol weighting: confidence scales down as realized vol rises.
	// Calibrated so ~50% annualized RV maps to a neutral 0.70 confidence.
	confidence := 0.70 * (50.0 / rvol)
	if confidence > 0.85 {
		confidence = 0.85
	}
	if confidence < 0.55 {
		return NoSignal(name) // vol too high to trust signal weighting
	}

	price := ctx.Price

	if longMajority {
		sl := price - math.Max(1.0*atr1h, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: confidence,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"TSMOM bullish majority (1h=%.4f, 4h=%.4f, 24h-proxy=%.4f), RV=%.1f%%",
				ret1h, ret4h, ret24hProxy, rvol,
			),
		}
	}

	if shortMajority {
		sl := price + math.Max(1.0*atr1h, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: confidence,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"TSMOM bearish majority (1h=%.4f, 4h=%.4f, 24h-proxy=%.4f), RV=%.1f%%",
				ret1h, ret4h, ret24hProxy, rvol,
			),
		}
	}

	return NoSignal(name)
}
