package scalpers

import (
	"fmt"
	"math"
)

// S82 — Chande Momentum Oscillator
//
// Citation:   Tushar Chande & Stanley Kroll, "The New Technical Trader" (1994).
//             CMO measures raw momentum on both up and down days, range -100..+100.
//             Unlike RSI (smoothed averages), CMO uses the raw sum of gains vs losses.
// Regime:     TRENDING + RANGING
// Timeframes: 15m
// Logic:      CMO(14) crosses above +50 (strong bullish momentum) or below -50
//             (strong bearish) with volume surge >1.5× 20-period average.

type ChandeMomentumOscillator struct{}

func (s *ChandeMomentumOscillator) Name() string { return "Chande_Momentum_Oscillator" }

func (s *ChandeMomentumOscillator) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *ChandeMomentumOscillator) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	cmo := CMO(candles, 14)
	cmoPrev := CMO(candles[:len(candles)-1], 14)

	if cmo == 0 && cmoPrev == 0 {
		return NoSignal(name)
	}

	atr15m := ATR(candles, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	avgVol := AvgVolume(candles, 20)
	lastVol := candles[len(candles)-1].Volume
	volSurge := avgVol > 0 && lastVol >= 1.5*avgVol

	if !volSurge {
		return NoSignal(name)
	}

	price := ctx.Price

	// Bullish: CMO crosses above +50 (was below, now above)
	bullish := cmoPrev < 50 && cmo >= 50
	// Bearish: CMO crosses below -50 (was above, now below)
	bearish := cmoPrev > -50 && cmo <= -50

	if bullish {
		minSL := math.Max(1.0*atr15m, 0.003*price)
		sl := price - minSL
		slDist := price - sl
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		if (tp-price)/slDist < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"CMO bullish surge cross: %.1f→%.1f (>+50), vol=%.0f (%.1fx avg)",
				cmoPrev, cmo, lastVol, lastVol/avgVol,
			),
		}
	}

	if bearish {
		minSL := math.Max(1.0*atr15m, 0.003*price)
		sl := price + minSL
		slDist := sl - price
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		if (price-tp)/slDist < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"CMO bearish surge cross: %.1f→%.1f (<-50), vol=%.0f (%.1fx avg)",
				cmoPrev, cmo, lastVol, lastVol/avgVol,
			),
		}
	}

	return NoSignal(name)
}
