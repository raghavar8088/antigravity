package scalpers

import (
	"fmt"
	"math"
)

// S95 — Volume Profile POC Magnet
//
// Citation:   J. Peter Steidlmayer, "Markets and Market Logic" (1986).
//             The Point of Control (highest-volume price level) acts as a
//             gravitational magnet. Widely used by CME institutional traders.
// Regime:     ALL regimes
// Timeframes: 1h (volume profile), 15m (current momentum)
// Logic:      When price is >1% away from the 48-bar POC and momentum is
//             fading (RSI diverging from extreme, volume declining), enter
//             a trade toward the POC. The POC acts as the TP target.

type VolumePOCMagnet struct{}

func (s *VolumePOCMagnet) Name() string { return "Volume_Profile_POC_Magnet" }

func (s *VolumePOCMagnet) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *VolumePOCMagnet) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles1h) < 20 || len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	// Use up to last 48 1h candles for the volume profile
	candles1h := ctx.Candles1h
	if len(candles1h) > 48 {
		candles1h = candles1h[len(candles1h)-48:]
	}

	poc := VolumeProfilePOC(candles1h)
	if poc == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	distFromPOC := math.Abs(price-poc) / price

	if distFromPOC < 0.010 {
		return NoSignal(name) // too close to POC, no magnet signal
	}

	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	rsi := RSI(ctx.Candles15m, 14)
	avgVol := AvgVolume(ctx.Candles15m, 20)
	lastVol := ctx.Candles15m[len(ctx.Candles15m)-1].Volume
	volDecline := avgVol > 0 && lastVol < 0.85*avgVol // fading volume = momentum fading

	// Price above POC and momentum fading → short toward POC
	priceAbovePOC := price > poc
	// Price below POC and momentum fading → long toward POC
	priceBelowPOC := price < poc

	if priceAbovePOC && volDecline && rsi > 60 {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := price + minSL
		slDist := sl - price
		if slDist <= 0 {
			return NoSignal(name)
		}
		// TP: halfway to POC, minimum 2×SL distance
		tpTarget := (price + poc) / 2
		if price-tpTarget < 2.0*slDist {
			tpTarget = price - 2.0*slDist
		}
		tp := tpTarget
		if (price-tp)/slDist < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"POC magnet SHORT: price=%.0f is %.2f%% above POC=%.0f, RSI=%.1f, vol fading",
				price, distFromPOC*100, poc, rsi,
			),
		}
	}

	if priceBelowPOC && volDecline && rsi < 40 {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := price - minSL
		slDist := price - sl
		if slDist <= 0 {
			return NoSignal(name)
		}
		tpTarget := (price + poc) / 2
		if tpTarget-price < 2.0*slDist {
			tpTarget = price + 2.0*slDist
		}
		tp := tpTarget
		if (tp-price)/slDist < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"POC magnet LONG: price=%.0f is %.2f%% below POC=%.0f, RSI=%.1f, vol fading",
				price, distFromPOC*100, poc, rsi,
			),
		}
	}

	return NoSignal(name)
}
