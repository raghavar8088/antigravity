package scalpers

import "fmt"

// S22 — Hurst_Regime_Switcher
//
// Regime:     ALL (RegimeTrending, RegimeRanging, RegimeVolatile, RegimeUnknown)
// Timeframes: 4h (Hurst regime classification) + 15m (entry trigger)
// Logic:      Hurst exponent (rescaled-range analysis, see HurstExponent in
//             indicators.go — Mandelbrot/Wallis 1969 standard method) on 4h
//             candles classifies the persistent statistical character of the
//             market itself: H>0.55 = trending/persistent (momentum carries),
//             H<0.45 = mean-reverting/anti-persistent (price oscillates around
//             a mean), 0.45-0.55 = random-walk/ambiguous (no statistical edge
//             either way — NoSignal).
//
// WHY THIS IS THE ONE EXCEPTION TO "NEVER ALL REGIMES":
// Every other strategy in this registry trades a fixed market behavior
// (trend-following or mean-reversion) and therefore must be gated by the
// detected Regime so it only fires in the regime its logic assumes. S22 is
// structurally different: its entire purpose is to FIRST determine, via the
// Hurst exponent, whether the market is currently behaving in a trending or
// mean-reverting statistical fashion, and only THEN pick the matching
// sub-strategy internally. Gating it by the upstream Regime classifier would
// be redundant (or worse, contradictory) since the Hurst computation is
// itself an independent, complementary regime detector running on a longer
// (4h) lookback than the engine's primary regime classifier. This mirrors
// how S17 (Orderbook_Imbalance_Persistence, chunk 2) was documented as the
// one exception to the graceful-degradation rule: a strategy whose core
// logic IS the gate cannot also be gated externally without defeating its
// purpose. ValidRegimes() therefore returns all four regimes, and all the
// actual regime discrimination happens inside Evaluate() via the internal
// Hurst-driven mode switch.
//
// Buffer note: maxCandles4h in engine/internal/trading/scalers_eval.go is 30
// (not the textbook period=100 used in academic Hurst studies). We use the
// maximum usable window — period=28 (the most recent 28 4h candles, ~4.7
// days) — and scale interpretation accordingly: with fewer sub-period splits
// available, the R/S regression has more estimation noise than a period=100
// Hurst, so thresholds are kept conservative (0.45/0.55 band, not tighter)
// and confidence is capped below the academic-grade-data strategies.

type HurstRegimeSwitcher struct{}

func (s *HurstRegimeSwitcher) Name() string { return "Hurst_Regime_Switcher" }

func (s *HurstRegimeSwitcher) ValidRegimes() []Regime {
	// Intentional exception — see file header comment above.
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile, RegimeUnknown}
}

const hurstLookbackPeriod = 28 // max usable given maxCandles4h=30 (see header note)

func (s *HurstRegimeSwitcher) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles4h) < 16 || len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	h := HurstExponent(ctx.Candles4h, hurstLookbackPeriod)
	price := ctx.Price
	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	switch {
	case h > 0.55:
		// Trend-following sub-mode: 4h is statistically persistent. Use
		// 15m EMA9/EMA21 slope + CVD confirmation as the entry trigger.
		ema9 := EMA(ctx.Candles15m, 9)
		ema21 := EMA(ctx.Candles15m, 21)
		if ema9 == 0 || ema21 == 0 {
			return NoSignal(name)
		}
		longTrend := ema9 > ema21 && ctx.CVD > ctx.CVDPrev
		shortTrend := ema9 < ema21 && ctx.CVD < ctx.CVDPrev

		if longTrend {
			sl := price - maxF(1.0*atr15m, 0.003*price)
			risk := price - sl
			tp := price + 2.2*risk
			if risk <= 0 {
				return NoSignal(name)
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionLong,
				Confidence: 0.68,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"Hurst H=%.3f>0.55 (4h persistent/trending), 15m EMA9=%.0f>EMA21=%.0f, CVD rising -> trend-follow LONG",
					h, ema9, ema21,
				),
			}
		}
		if shortTrend {
			sl := price + maxF(1.0*atr15m, 0.003*price)
			risk := sl - price
			tp := price - 2.2*risk
			if risk <= 0 {
				return NoSignal(name)
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionShort,
				Confidence: 0.68,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"Hurst H=%.3f>0.55 (4h persistent/trending), 15m EMA9=%.0f<EMA21=%.0f, CVD falling -> trend-follow SHORT",
					h, ema9, ema21,
				),
			}
		}
		return NoSignal(name)

	case h < 0.45:
		// Mean-reversion sub-mode: 4h is statistically anti-persistent. Use
		// a 15m z-score/Bollinger-style fade as the entry trigger.
		bb := BB(ctx.Candles15m, 20)
		if bb.Middle == 0 {
			return NoSignal(name)
		}
		rsi15m := RSI(ctx.Candles15m, 14)

		atLower := price <= bb.Lower+0.1*atr15m
		atUpper := price >= bb.Upper-0.1*atr15m

		if atLower && rsi15m < 35 {
			sl := bb.Lower - maxF(1.0*atr15m, 0.003*price)
			risk := price - sl
			tp := price + 2.2*risk
			if risk <= 0 {
				return NoSignal(name)
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionLong,
				Confidence: 0.66,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"Hurst H=%.3f<0.45 (4h mean-reverting), price %.0f at lower 15m BB=%.0f, RSI=%.1f -> fade LONG",
					h, price, bb.Lower, rsi15m,
				),
			}
		}
		if atUpper && rsi15m > 65 {
			sl := bb.Upper + maxF(1.0*atr15m, 0.003*price)
			risk := sl - price
			tp := price - 2.2*risk
			if risk <= 0 {
				return NoSignal(name)
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionShort,
				Confidence: 0.66,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"Hurst H=%.3f<0.45 (4h mean-reverting), price %.0f at upper 15m BB=%.0f, RSI=%.1f -> fade SHORT",
					h, price, bb.Upper, rsi15m,
				),
			}
		}
		return NoSignal(name)

	default:
		// 0.45 <= H <= 0.55: random-walk / ambiguous regime, no statistical
		// edge in either direction.
		return NoSignal(name)
	}
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
