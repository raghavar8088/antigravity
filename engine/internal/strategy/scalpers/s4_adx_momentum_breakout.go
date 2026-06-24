package scalpers

import "fmt"

// S4 — ADX Momentum Breakout
//
// Regime:     TRENDING only
// Timeframes: 4h (macro trend strength) + 15m (entry trigger)
// Logic:      4h ADX > 25 confirms strong trend. 15m price breaks above/below
//             the prior 20-bar swing high/low with rising volume. ATR expanding
//             on 15m. MACD histogram positive/negative and widening.

type ADXMomentumBreakout struct{}

func (s *ADXMomentumBreakout) Name() string { return "ADX_Momentum_Breakout" }

func (s *ADXMomentumBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *ADXMomentumBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles4h) < 30 || len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	adx4h := ADX(ctx.Candles4h, 14)
	if adx4h < 25 {
		return NoSignal(name)
	}

	c15 := ctx.Candles15m
	n := len(c15)

	lookback := c15[:n-1]
	if len(lookback) < 20 {
		return NoSignal(name)
	}
	structureHigh := SwingHigh(lookback, 20)
	structureLow := SwingLow(lookback, 20)

	atr15m := ATR(c15, 14)
	atr15mPrev := ATR(c15[:n-1], 14)
	if atr15m == 0 || atr15m <= atr15mPrev {
		return NoSignal(name)
	}

	macd15m := MACD(c15)
	macdPrev := MACD(c15[:n-1])

	price := ctx.Price
	lastCandle := c15[n-1]
	prevCandle := c15[n-2]

	avgVol15m := AvgVolume(c15[:n-1], 20)
	volumeSurge := lastCandle.Volume > avgVol15m*1.4

	bullBreakout := lastCandle.Close > structureHigh && prevCandle.Close <= structureHigh
	bullMACD := macd15m.Histogram > 0 && macd15m.Histogram > macdPrev.Histogram

	if bullBreakout && bullMACD && volumeSurge {
		sl := structureHigh - 0.5*atr15m
		tp1 := price + 2.0*atr15m
		tp2 := price + 4.0*atr15m
		risk := price - sl
		if risk <= 0 || (tp1-price) < 2*risk {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.76,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"TRENDING: 4h ADX=%.1f (strong), 15m breakout above %.0f, "+
					"vol surge %.0f vs avg %.0f, MACD hist=%.2f expanding",
				adx4h, structureHigh, lastCandle.Volume, avgVol15m, macd15m.Histogram,
			),
		}
	}

	bearBreakout := lastCandle.Close < structureLow && prevCandle.Close >= structureLow
	bearMACD := macd15m.Histogram < 0 && macd15m.Histogram < macdPrev.Histogram

	if bearBreakout && bearMACD && volumeSurge {
		sl := structureLow + 0.5*atr15m
		tp1 := price - 2.0*atr15m
		tp2 := price - 4.0*atr15m
		risk := sl - price
		if risk <= 0 || (price-tp1) < 2*risk {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.76,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"TRENDING: 4h ADX=%.1f (strong), 15m breakdown below %.0f, "+
					"vol surge %.0f vs avg %.0f, MACD hist=%.2f expanding",
				adx4h, structureLow, lastCandle.Volume, avgVol15m, macd15m.Histogram,
			),
		}
	}

	return NoSignal(name)
}
