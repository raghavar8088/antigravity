package scalpers

import (
	"fmt"
	"math"
)

// S83 — Know Sure Thing Momentum
//
// Citation:   Martin Pring, "Martin Pring on Market Momentum" (1993).
//             KST is a smoothed composite of four rate-of-change periods,
//             weighted 1/2/3/4, providing a multi-timeframe momentum view.
// Regime:     TRENDING + RANGING
// Timeframes: 1h
// Logic:      KST crosses its 9-period EMA signal line in the direction of
//             the trend (EMA9 vs EMA21 on 1h candles confirms bias).

type KnowSureThingMomentum struct{}

func (s *KnowSureThingMomentum) Name() string { return "Know_Sure_Thing_Momentum" }

func (s *KnowSureThingMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *KnowSureThingMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles1h) < 60 {
		return NoSignal(name)
	}

	candles := ctx.Candles1h
	kst := KST(candles)
	kstPrev := KST(candles[:len(candles)-1])

	if kst.KST == 0 || kstPrev.KST == 0 {
		return NoSignal(name)
	}

	atr1h := ATR(candles, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	// Trend bias from 1h EMA alignment
	ema9 := EMA(candles, 9)
	ema21 := EMA(candles, 21)
	trendUp := ema9 > ema21
	trendDown := ema9 < ema21

	price := ctx.Price

	// Bullish KST signal cross: KST crosses above its signal line
	bullishCross := kstPrev.KST <= kstPrev.Signal && kst.KST > kst.Signal
	// Bearish KST signal cross: KST crosses below its signal line
	bearishCross := kstPrev.KST >= kstPrev.Signal && kst.KST < kst.Signal

	if bullishCross && trendUp {
		minSL := math.Max(1.0*atr1h, 0.003*price)
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
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"KST bullish signal cross (KST=%.3f > sig=%.3f), 1h EMA9=%.0f>EMA21=%.0f",
				kst.KST, kst.Signal, ema9, ema21,
			),
		}
	}

	if bearishCross && trendDown {
		minSL := math.Max(1.0*atr1h, 0.003*price)
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
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"KST bearish signal cross (KST=%.3f < sig=%.3f), 1h EMA9=%.0f<EMA21=%.0f",
				kst.KST, kst.Signal, ema9, ema21,
			),
		}
	}

	return NoSignal(name)
}
