package scalpers

import (
	"fmt"
	"time"
)

// S7 — Opening Range Breakout (NY session)
//
// Regime:     ALL (fires in TRENDING, RANGING, VOLATILE — not UNKNOWN)
// Timeframes: 5m (range formation) + 1h (macro direction)
// Logic:      NY session opens at 14:00 UTC. First 30 min (14:00–14:30) defines
//             the Opening Range (OR) high and low. After 14:30, wait for a
//             decisive breakout with volume surge above the OR. Macro 1h bias
//             must align (bullish = long break, bearish = short break).
//             Confidence: 0.74. Session gate: only 14:00–20:00 UTC.

const (
	nySessionOpen  = 14 // UTC hour
	nySessionClose = 20 // UTC hour
	nyOREndMinute  = 30 // opening range ends at :30 of the open hour
)

type OpeningRangeBreakout struct {
	orHigh  float64
	orLow   float64
	orBuilt bool
	orDate  time.Time // UTC date the OR was built for
}

func (s *OpeningRangeBreakout) Name() string { return "Opening_Range_Breakout" }

func (s *OpeningRangeBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *OpeningRangeBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 20 || len(ctx.Candles1h) < 10 {
		return NoSignal(name)
	}

	now := ctx.Now.UTC()
	hour := now.Hour()
	minute := now.Minute()

	// Only active during NY session 14:00–20:00 UTC
	if hour < nySessionOpen || hour >= nySessionClose {
		return NoSignal(name)
	}

	todayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Rebuild OR on a new day or if not yet built
	if !s.orBuilt || !s.orDate.Equal(todayUTC) {
		s.orHigh = 0
		s.orLow = 0
		s.orBuilt = false
		s.orDate = todayUTC
	}

	// Build/extend OR: first 30 min of NY session (14:00–14:29 UTC)
	inORWindow := hour == nySessionOpen && minute < nyOREndMinute
	if inORWindow {
		s.buildOR(ctx.Candles5m, now)
		return NoSignal(name) // don't trade during OR formation
	}

	// After OR window: finalise if we just crossed 14:30
	if !s.orBuilt {
		s.buildOR(ctx.Candles5m, now)
		s.orBuilt = true
	}

	// Use ScalerBundle-persisted OR values from context if struct state not populated.
	if s.orHigh == 0 && ctx.ORHigh > 0 {
		s.orHigh = ctx.ORHigh
	}
	if s.orLow == 0 && ctx.ORLow > 0 {
		s.orLow = ctx.ORLow
	}

	if s.orHigh == 0 || s.orLow == 0 || s.orHigh <= s.orLow {
		return NoSignal(name)
	}

	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}

	c5 := ctx.Candles5m
	n := len(c5)
	lastCandle := c5[n-1]
	avgVol := AvgVolume(c5[:n-1], 20)
	volumeSurge := lastCandle.Volume > avgVol*1.5

	// 1h macro bias: EMA9 vs EMA21
	ema9_1h := EMA(ctx.Candles1h, 9)
	ema21_1h := EMA(ctx.Candles1h, 21)
	bullMacro := ema9_1h > ema21_1h
	bearMacro := ema9_1h < ema21_1h

	price := ctx.Price

	// Bullish breakout
	if price > s.orHigh+0.1*atr5m && bullMacro && volumeSurge {
		sl := s.orHigh - 0.3*atr5m
		tp := price + 2.5*(price-sl)
		risk := price - sl
		if risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.74,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"NY ORB: price %.0f breaks above OR high %.0f, "+
					"vol surge %.0f vs avg %.0f, 1h EMA9=%.0f>EMA21=%.0f (bullish)",
				price, s.orHigh, lastCandle.Volume, avgVol, ema9_1h, ema21_1h,
			),
		}
	}

	// Bearish breakout
	if price < s.orLow-0.1*atr5m && bearMacro && volumeSurge {
		sl := s.orLow + 0.3*atr5m
		tp := price - 2.5*(sl-price)
		risk := sl - price
		if risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.74,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"NY ORB: price %.0f breaks below OR low %.0f, "+
					"vol surge %.0f vs avg %.0f, 1h EMA9=%.0f<EMA21=%.0f (bearish)",
				price, s.orLow, lastCandle.Volume, avgVol, ema9_1h, ema21_1h,
			),
		}
	}

	return NoSignal(name)
}

// buildOR scans 5m candles during the NY opening range window to set OR high/low.
func (s *OpeningRangeBreakout) buildOR(candles []Candle, now time.Time) {
	orStart := time.Date(now.Year(), now.Month(), now.Day(), nySessionOpen, 0, 0, 0, time.UTC)
	orEnd := orStart.Add(time.Duration(nyOREndMinute) * time.Minute)
	for _, c := range candles {
		if c.OpenTime.Before(orStart) || c.OpenTime.After(orEnd) {
			continue
		}
		if s.orHigh == 0 || c.High > s.orHigh {
			s.orHigh = c.High
		}
		if s.orLow == 0 || c.Low < s.orLow {
			s.orLow = c.Low
		}
	}
}
