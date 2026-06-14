package strategy

import "fmt"

// S4 — ADX Momentum Breakout
//
// Regime:     TRENDING only
// Timeframes: 4h (ADX trend strength) + 15m (breakout entry)
// Logic:      4h ADX > 25 and rising. 15m price breaks a 20-candle high/low
//             with volume > 1.5× average. CVD confirms direction.
// Edge:       Only enters breakouts that have institutional momentum behind
//             them — filters out the 70% of false breakouts with ADX + volume.

type ADXMomentumBreakout struct{}

func (s *ADXMomentumBreakout) Name() string { return "ADX_Momentum_Breakout" }

func (s *ADXMomentumBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *ADXMomentumBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles4h) < 35 || len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	// ── 4h ADX: must be > 25 and stronger than the previous reading ───────────
	adx4h := ADX(ctx.Candles4h, 14)
	adx4hPrev := ADX(ctx.Candles4h[:len(ctx.Candles4h)-1], 14)

	if adx4h < 25 || adx4h <= adx4hPrev {
		return NoSignal(name) // trend not strong enough or weakening
	}

	// ── 15m: price breaks 20-candle high/low ─────────────────────────────────
	// Use all-but-last-candle for the range (last candle IS the breakout)
	lookback15m := ctx.Candles15m[:len(ctx.Candles15m)-1]
	if len(lookback15m) < 20 {
		return NoSignal(name)
	}
	rangeHigh := SwingHigh(lookback15m, 20)
	rangeLow := SwingLow(lookback15m, 20)
	if rangeHigh == 0 || rangeLow == 0 {
		return NoSignal(name)
	}

	lastCandle := ctx.Candles15m[len(ctx.Candles15m)-1]
	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	// ── Volume confirmation: breakout candle volume > 1.5× 20-bar average ────
	avgVol := AvgVolume(lookback15m, 20)
	volumeConfirmed := avgVol > 0 && lastCandle.Volume > 1.5*avgVol

	price := ctx.Price

	// ── LONG breakout: closed above rangeHigh ────────────────────────────────
	bullBreakout := lastCandle.Close > rangeHigh
	cvdLongConfirm := ctx.CVD > ctx.CVDPrev

	if bullBreakout && volumeConfirmed && cvdLongConfirm {
		sl := rangeHigh - 0.2*atr15m // back inside the range = failed break
		tp1 := price + 2.0*atr15m
		tp2 := price + 4.0*atr15m // trail target
		risk := price - sl
		if risk <= 0 || (tp1-price) < 2*risk {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.80,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"TRENDING: 4h ADX=%.1f>25 and rising (prev=%.1f), "+
					"15m bullish breakout above %.0f (vol=%.2f>1.5×avg=%.2f), "+
					"CVD rising (%.0f→%.0f)",
				adx4h, adx4hPrev, rangeHigh, lastCandle.Volume, 1.5*avgVol,
				ctx.CVDPrev, ctx.CVD,
			),
		}
	}

	// ── SHORT breakout: closed below rangeLow ─────────────────────────────────
	bearBreakout := lastCandle.Close < rangeLow
	cvdShortConfirm := ctx.CVD < ctx.CVDPrev

	if bearBreakout && volumeConfirmed && cvdShortConfirm {
		sl := rangeLow + 0.2*atr15m
		tp1 := price - 2.0*atr15m
		tp2 := price - 4.0*atr15m
		risk := sl - price
		if risk <= 0 || (price-tp1) < 2*risk {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.80,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"TRENDING: 4h ADX=%.1f>25 and rising (prev=%.1f), "+
					"15m bearish breakdown below %.0f (vol=%.2f>1.5×avg=%.2f), "+
					"CVD falling (%.0f→%.0f)",
				adx4h, adx4hPrev, rangeLow, lastCandle.Volume, 1.5*avgVol,
				ctx.CVDPrev, ctx.CVD,
			),
		}
	}

	return NoSignal(name)
}
