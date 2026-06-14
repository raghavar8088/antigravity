package strategy

import "fmt"

// S7 — Opening Range Breakout (ORB)
//
// Regime:     ALL (Trending, Ranging, Volatile) — skips UNKNOWN only
// Timeframes: 5m (breakout detection) + 1h (trend alignment)
// Logic:      Define range: first 30 min of NY open (13:30–14:00 UTC).
//             Breakout: 5m candle closes above/below ORB high/low.
//             Volume on break > 2× session average.
//             1h EMA bias must align with breakout direction.
//             Active only 14:00–20:00 UTC (NY session, post-open).
// Edge:       The NY open is the highest-volume session for BTC.
//             The first 30 min establishes institutional intent.
//             A volume-confirmed break of that range has strong follow-through.

type OpeningRangeBreakout struct {
	// ORB high/low are computed fresh each evaluation from the candle history
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
	if len(ctx.Candles5m) < 30 || len(ctx.Candles1h) < 15 {
		return NoSignal(name)
	}

	// ── Time filter: NY session 14:00–20:00 UTC (post opening range) ─────────
	hour := ctx.Now.UTC().Hour()
	minute := ctx.Now.UTC().Minute()

	// Opening range window: 13:30–14:00 UTC
	// Entry window: 14:00–20:00 UTC only
	inEntryWindow := (hour == 14 && minute >= 0) || (hour > 14 && hour < 20)
	if !inEntryWindow {
		return NoSignal(name)
	}

	// ── Identify ORB candles: 5m bars from 13:30–14:00 UTC ───────────────────
	// That's 6 five-minute bars. Find them in the candle history.
	var orbCandles []Candle
	for _, c := range ctx.Candles5m {
		h := c.OpenTime.UTC().Hour()
		m := c.OpenTime.UTC().Minute()
		isORBWindow := (h == 13 && m >= 30) || (h == 14 && m == 0)
		if isORBWindow {
			orbCandles = append(orbCandles, c)
		}
	}
	if len(orbCandles) < 3 {
		// Not enough ORB candles found — session may not have opened yet
		// Fallback: use the first 6 candles of the session window
		for _, c := range ctx.Candles5m {
			h := c.OpenTime.UTC().Hour()
			if h == 13 || (h == 14 && c.OpenTime.UTC().Minute() == 0) {
				orbCandles = append(orbCandles, c)
			}
			if len(orbCandles) >= 6 {
				break
			}
		}
	}
	if len(orbCandles) < 3 {
		return NoSignal(name)
	}

	orbHigh := orbCandles[0].High
	orbLow := orbCandles[0].Low
	for _, c := range orbCandles[1:] {
		if c.High > orbHigh {
			orbHigh = c.High
		}
		if c.Low < orbLow {
			orbLow = c.Low
		}
	}
	orbRange := orbHigh - orbLow
	if orbRange <= 0 {
		return NoSignal(name)
	}

	// ── 1h EMA bias ───────────────────────────────────────────────────────────
	ema9_1h := EMA(ctx.Candles1h, 9)
	ema21_1h := EMA(ctx.Candles1h, 21)
	bullBias := ema9_1h > ema21_1h
	bearBias := ema9_1h < ema21_1h

	// ── Last 5m candle: breakout detection ───────────────────────────────────
	lastCandle := ctx.Candles5m[len(ctx.Candles5m)-1]
	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}

	// ── Volume: breakout candle must be > 2× session average ─────────────────
	// Session average: all 5m candles from 13:30 UTC onward
	var sessionCandles []Candle
	for _, c := range ctx.Candles5m {
		h := c.OpenTime.UTC().Hour()
		m := c.OpenTime.UTC().Minute()
		if h > 13 || (h == 13 && m >= 30) {
			sessionCandles = append(sessionCandles, c)
		}
	}
	avgSessionVol := AvgVolume(sessionCandles, len(sessionCandles))
	volumeConfirmed := avgSessionVol > 0 && lastCandle.Volume > 2*avgSessionVol

	price := ctx.Price

	// ── LONG breakout ─────────────────────────────────────────────────────────
	bullBreak := lastCandle.Close > orbHigh && bullBias
	if bullBreak && volumeConfirmed {
		sl := orbHigh - 0.1*atr5m // back inside ORB = failed break
		tp1 := price + orbRange   // 1× range projection
		tp2 := price + 2*orbRange // 2× range projection
		risk := price - sl
		if risk <= 0 || (tp1-price) < 2*risk {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.77,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"ORB LONG: breakout above ORB high=%.0f (ORB range=%.0f), "+
					"vol=%.2f>2×avg=%.2f, 1h bias bullish (EMA9=%.0f>EMA21=%.0f), "+
					"TP1=%.0f (1×range), TP2=%.0f (2×range)",
				orbHigh, orbRange, lastCandle.Volume, 2*avgSessionVol,
				ema9_1h, ema21_1h, tp1, tp2,
			),
		}
	}

	// ── SHORT breakout ────────────────────────────────────────────────────────
	bearBreak := lastCandle.Close < orbLow && bearBias
	if bearBreak && volumeConfirmed {
		sl := orbLow + 0.1*atr5m
		tp1 := price - orbRange
		tp2 := price - 2*orbRange
		risk := sl - price
		if risk <= 0 || (price-tp1) < 2*risk {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.77,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"ORB SHORT: breakdown below ORB low=%.0f (ORB range=%.0f), "+
					"vol=%.2f>2×avg=%.2f, 1h bias bearish (EMA9=%.0f<EMA21=%.0f), "+
					"TP1=%.0f (1×range), TP2=%.0f (2×range)",
				orbLow, orbRange, lastCandle.Volume, 2*avgSessionVol,
				ema9_1h, ema21_1h, tp1, tp2,
			),
		}
	}

	return NoSignal(name)
}
