package scalpers

import (
	"fmt"
	"math"
)

// S80 — Elder Triple Screen
//
// Citation:   Alexander Elder, "Trading For A Living" (1993) — Three-screen
//             system using trend filter on a higher timeframe, oscillator
//             confirmation on an intermediate timeframe, and intraday entry.
// Regime:     TRENDING only
// Timeframes: 4h (trend direction via EMA slope), 1h (RSI oscillator
//             pullback confirmation), 15m (entry candle with volume).
// Edge:       Unlike S1 (EMA ribbon alignment entry), Elder screens require
//             the oscillator to be AGAINST the trend (oversold in uptrend,
//             overbought in downtrend) — entering on pullbacks, not breakouts.

type ElderTripleScreen struct{}

func (s *ElderTripleScreen) Name() string { return "Elder_Triple_Screen" }

func (s *ElderTripleScreen) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *ElderTripleScreen) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles4h) < 10 || len(ctx.Candles1h) < 20 || len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	// Screen 1: 4h EMA slope determines trend direction
	ema13_4h := EMA(ctx.Candles4h, 13)
	ema13_4hPrev := EMA(ctx.Candles4h[:len(ctx.Candles4h)-1], 13)
	if ema13_4h == 0 || ema13_4hPrev == 0 {
		return NoSignal(name)
	}
	trendUp := ema13_4h > ema13_4hPrev
	trendDown := ema13_4h < ema13_4hPrev

	if !trendUp && !trendDown {
		return NoSignal(name)
	}

	// Screen 2: 1h RSI must be in oscillator pullback zone
	// In uptrend: RSI 30-50 (oversold/neutral pullback = buy opportunity)
	// In downtrend: RSI 50-70 (overbought/neutral rally = sell opportunity)
	rsi1h := RSI(ctx.Candles1h, 14)

	// Screen 3: 15m entry candle + ATR
	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	avgVol15m := AvgVolume(ctx.Candles15m, 20)
	lastVol15m := ctx.Candles15m[len(ctx.Candles15m)-1].Volume

	// Volume must be present for confirmation
	volConfirm := avgVol15m > 0 && lastVol15m > 0.9*avgVol15m

	if trendUp && rsi1h >= 30 && rsi1h <= 52 && volConfirm {
		minSL := math.Max(1.0*atr15m, 0.003*price)
		sl := price - minSL
		slDist := price - sl
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		tpDist := tp - price
		if tpDist/slDist < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Elder 3-screen LONG: 4h EMA slope up (%.0f→%.0f), 1h RSI=%.1f pullback zone, 15m vol=%.0f",
				ema13_4hPrev, ema13_4h, rsi1h, lastVol15m,
			),
		}
	}

	if trendDown && rsi1h >= 48 && rsi1h <= 70 && volConfirm {
		minSL := math.Max(1.0*atr15m, 0.003*price)
		sl := price + minSL
		slDist := sl - price
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		tpDist := price - tp
		if tpDist/slDist < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Elder 3-screen SHORT: 4h EMA slope down (%.0f→%.0f), 1h RSI=%.1f rally zone, 15m vol=%.0f",
				ema13_4hPrev, ema13_4h, rsi1h, lastVol15m,
			),
		}
	}

	return NoSignal(name)
}
