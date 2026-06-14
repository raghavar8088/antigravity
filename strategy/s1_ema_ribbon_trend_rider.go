package strategy

import "fmt"

// S1 — EMA Ribbon Trend Rider
//
// Regime:     TRENDING only
// Timeframes: 1h (bias) + 15m (entry)
// Logic:      1h EMA ribbon aligned (9>21>50). Price pulls back to 15m EMA21,
//             bounces with rising CVD and expanding ATR.
// Edge:       Enters during healthy pullbacks in strong trends — not chasing tops.

type EMARibbonTrendRider struct{}

func (s *EMARibbonTrendRider) Name() string { return "EMA_Ribbon_Trend_Rider" }

func (s *EMARibbonTrendRider) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *EMARibbonTrendRider) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	// ── Guard: regime ────────────────────────────────────────────────────────
	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 55 || len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	// ── 1h ribbon: EMA 9 > EMA 21 > EMA 50 (long bias) ──────────────────────
	ema9_1h := EMA(ctx.Candles1h, 9)
	ema21_1h := EMA(ctx.Candles1h, 21)
	ema50_1h := EMA(ctx.Candles1h, 50)

	longBias := ema9_1h > ema21_1h && ema21_1h > ema50_1h
	shortBias := ema9_1h < ema21_1h && ema21_1h < ema50_1h

	if !longBias && !shortBias {
		return NoSignal(name) // ribbon not aligned — no trade
	}

	// ── 15m: price near EMA 21 (pullback zone) ───────────────────────────────
	ema21_15m := EMA(ctx.Candles15m, 21)
	ema50_15m := EMA(ctx.Candles15m, 50)
	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	nearEMA21 := price >= ema21_15m-0.3*atr15m && price <= ema21_15m+0.3*atr15m

	// ── ATR expanding: current ATR > prev ATR (momentum, not chop) ───────────
	atr15mPrev := ATR(ctx.Candles15m[:len(ctx.Candles15m)-1], 14)
	atrExpanding := atr15m > atr15mPrev

	// ── CVD confirms direction ────────────────────────────────────────────────
	cvdLongConfirm := ctx.CVD > ctx.CVDPrev   // buyers stepping in
	cvdShortConfirm := ctx.CVD < ctx.CVDPrev  // sellers stepping in

	// ── Entry: LONG ──────────────────────────────────────────────────────────
	if longBias && nearEMA21 && atrExpanding && cvdLongConfirm {
		sl := ema50_15m - 0.5*atr15m
		tp1 := price + 1.5*atr15m
		tp2 := price + 3.0*atr15m // trail target
		if price-sl < 0.001*price {
			return NoSignal(name) // SL too tight — likely noise
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.78,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"1h ribbon aligned bullish (EMA9=%.0f>EMA21=%.0f>EMA50=%.0f), "+
					"price %.0f near 15m EMA21=%.0f, CVD rising (%.0f→%.0f), ATR expanding",
				ema9_1h, ema21_1h, ema50_1h, price, ema21_15m, ctx.CVDPrev, ctx.CVD,
			),
		}
	}

	// ── Entry: SHORT ─────────────────────────────────────────────────────────
	if shortBias && nearEMA21 && atrExpanding && cvdShortConfirm {
		sl := ema50_15m + 0.5*atr15m
		tp1 := price - 1.5*atr15m
		tp2 := price - 3.0*atr15m
		if sl-price < 0.001*price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.78,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"1h ribbon aligned bearish (EMA9=%.0f<EMA21=%.0f<EMA50=%.0f), "+
					"price %.0f near 15m EMA21=%.0f, CVD falling (%.0f→%.0f), ATR expanding",
				ema9_1h, ema21_1h, ema50_1h, price, ema21_15m, ctx.CVDPrev, ctx.CVD,
			),
		}
	}

	return NoSignal(name)
}
