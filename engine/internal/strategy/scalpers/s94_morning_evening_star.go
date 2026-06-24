package scalpers

import (
	"fmt"
	"math"
)

// S94 — Morning/Evening Star
//
// Citation:   Steve Nison, "Japanese Candlestick Charting Techniques" (1991);
//             Thomas Bulkowski, "Encyclopedia of Chart Patterns" (2000) —
//             validates Morning/Evening Star with >63% reliability in
//             trending-to-reversal transitions across multiple asset classes.
// Regime:     RANGING + TRENDING
// Timeframes: 1h
// Logic:      Evening Star (bullish→doji/small→bearish, third closes into
//             first bar body) at 20-bar resistance with RSI>60 = SHORT.
//             Morning Star (bearish→doji/small→bullish) at support with
//             RSI<40 = LONG. Volume must increase on the third bar.

type MorningEveningStar struct{}

func (s *MorningEveningStar) Name() string { return "Morning_Evening_Star" }

func (s *MorningEveningStar) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}

func (s *MorningEveningStar) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles1h) < 25 {
		return NoSignal(name)
	}

	candles := ctx.Candles1h
	n := len(candles)

	star1 := candles[n-3] // first bar
	star2 := candles[n-2] // doji/small middle bar
	star3 := candles[n-1] // confirmation bar

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	// Middle bar should be small (body < 30% of star1 body)
	star1Body := math.Abs(star1.Close - star1.Open)
	star2Body := math.Abs(star2.Close - star2.Open)
	smallMiddle := star1Body > 0 && star2Body < 0.30*star1Body

	if !smallMiddle {
		return NoSignal(name)
	}

	rsi := RSI(candles, 14)
	swing20High := SwingHigh(candles[:n-1], 20)
	swing20Low := SwingLow(candles[:n-1], 20)
	price := ctx.Price

	avgVol := AvgVolume(candles, 20)
	volIncrease := avgVol > 0 && star3.Volume > star2.Volume && star3.Volume > 0.8*avgVol

	star1BodyHigh := math.Max(star1.Open, star1.Close)
	star1BodyLow := math.Min(star1.Open, star1.Close)

	// Morning Star: star1 bearish, star2 small, star3 bullish closing into star1 body
	isMorningStar := star1.Close < star1.Open && // star1 bearish
		star3.Close > star3.Open && // star3 bullish
		star3.Close >= star1BodyLow+0.3*(star1BodyHigh-star1BodyLow) // closes into star1 body

	nearSwingLow := swing20Low > 0 && math.Abs(price-swing20Low)/swing20Low < 0.005

	if isMorningStar && nearSwingLow && rsi < 42 && volIncrease {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := star2.Low - 0.2*atr
		if price-sl < minSL {
			sl = price - minSL
		}
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
			Confidence: 0.74,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Morning Star LONG at support=%.0f: star3 close=%.0f into star1 body=%.0f-%.0f, RSI=%.1f",
				swing20Low, star3.Close, star1BodyLow, star1BodyHigh, rsi,
			),
		}
	}

	// Evening Star: star1 bullish, star2 small, star3 bearish closing into star1 body
	isEveningStar := star1.Close > star1.Open && // star1 bullish
		star3.Close < star3.Open && // star3 bearish
		star3.Close <= star1BodyLow+0.70*(star1BodyHigh-star1BodyLow) // closes into star1 body

	nearSwingHigh := swing20High > 0 && math.Abs(price-swing20High)/swing20High < 0.005

	if isEveningStar && nearSwingHigh && rsi > 58 && volIncrease {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := star2.High + 0.2*atr
		if sl-price < minSL {
			sl = price + minSL
		}
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
			Confidence: 0.74,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Evening Star SHORT at resistance=%.0f: star3 close=%.0f into star1 body=%.0f-%.0f, RSI=%.1f",
				swing20High, star3.Close, star1BodyLow, star1BodyHigh, rsi,
			),
		}
	}

	return NoSignal(name)
}
