package scalpers

import (
	"fmt"
	"math"
)

// S42 — RSI Divergence Multiframe
//
// Research:   Wilder (1978) RSI — multi-timeframe divergence confluence.
// Regime:     RANGING, TRENDING (divergence reversal can fire in either)
// Timeframes: 5m AND 15m simultaneously — both must show RSI-vs-price
//             divergence with 3-bar confirmation each.
// Logic:      Detects classic bearish (price higher-highs, RSI lower-highs)
//             and bullish (price lower-lows, RSI higher-lows) divergence
//             independently on Candles5m and Candles15m using the existing
//             RSI() indicator, each requiring 3 consecutive confirming bars
//             per the mandatory signal-quality rule. Only signals when BOTH
//             timeframes agree — this is intentionally stricter than S6
//             (which is a single-timeframe CVD-based divergence sniper); S42
//             uses RSI only, never CVD.
// Edge:       Multi-timeframe confluence filters out the single-timeframe
//             noise that causes most divergence-strategy false positives.

type RSIDivergenceMultiframe struct{}

func (s *RSIDivergenceMultiframe) Name() string { return "RSI_Divergence_Multiframe" }

func (s *RSIDivergenceMultiframe) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}

// rsiSeries computes RSI at each of the last `bars` positions (newest last).
func rsiSeries(candles []Candle, period, bars int) []float64 {
	if len(candles) < period+1+bars {
		return nil
	}
	out := make([]float64, bars)
	n := len(candles)
	for i := 0; i < bars; i++ {
		end := n - bars + i + 1
		out[i] = RSI(candles[:end], period)
	}
	return out
}

// rsiBearishDiv3Bar checks 3-bar confirmed bearish divergence: price makes
// 3 consecutive higher highs while RSI makes 3 consecutive lower highs.
func rsiBearishDiv3Bar(candles []Candle, period int) bool {
	n := len(candles)
	if n < 4 {
		return false
	}
	rsis := rsiSeries(candles, period, 4)
	if rsis == nil {
		return false
	}
	p0, p1, p2, p3 := candles[n-4].High, candles[n-3].High, candles[n-2].High, candles[n-1].High
	if !(p1 > p0 && p2 > p1 && p3 > p2) {
		return false
	}
	r0, r1, r2, r3 := rsis[0], rsis[1], rsis[2], rsis[3]
	if !(r1 < r0 && r2 < r1 && r3 < r2) {
		return false
	}
	return true
}

// rsiBullishDiv3Bar checks 3-bar confirmed bullish divergence: price makes
// 3 consecutive lower lows while RSI makes 3 consecutive higher lows.
func rsiBullishDiv3Bar(candles []Candle, period int) bool {
	n := len(candles)
	if n < 4 {
		return false
	}
	rsis := rsiSeries(candles, period, 4)
	if rsis == nil {
		return false
	}
	p0, p1, p2, p3 := candles[n-4].Low, candles[n-3].Low, candles[n-2].Low, candles[n-1].Low
	if !(p1 < p0 && p2 < p1 && p3 < p2) {
		return false
	}
	r0, r1, r2, r3 := rsis[0], rsis[1], rsis[2], rsis[3]
	if !(r1 > r0 && r2 > r1 && r3 > r2) {
		return false
	}
	return true
}

func (s *RSIDivergenceMultiframe) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 20 || len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}

	price := ctx.Price

	bear5m := rsiBearishDiv3Bar(ctx.Candles5m, 14)
	bear15m := rsiBearishDiv3Bar(ctx.Candles15m, 14)
	bull5m := rsiBullishDiv3Bar(ctx.Candles5m, 14)
	bull15m := rsiBullishDiv3Bar(ctx.Candles15m, 14)

	if bear5m && bear15m {
		swingHigh5m := SwingHigh(ctx.Candles5m, 4)
		sl := math.Max(swingHigh5m, price) + math.Max(1.0*atr5m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := price - 2.0*risk
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		rsi5m := RSI(ctx.Candles5m, 14)
		rsi15m := RSI(ctx.Candles15m, 14)
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.76,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"%s: 3-bar bearish RSI divergence confirmed on BOTH 5m (RSI=%.1f) and "+
					"15m (RSI=%.1f) — price higher-highs vs RSI lower-highs",
				ctx.Regime, rsi5m, rsi15m,
			),
		}
	}

	if bull5m && bull15m {
		swingLow5m := SwingLow(ctx.Candles5m, 4)
		sl := math.Min(swingLow5m, price) - math.Max(1.0*atr5m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := price + 2.0*risk
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		rsi5m := RSI(ctx.Candles5m, 14)
		rsi15m := RSI(ctx.Candles15m, 14)
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.76,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"%s: 3-bar bullish RSI divergence confirmed on BOTH 5m (RSI=%.1f) and "+
					"15m (RSI=%.1f) — price lower-lows vs RSI higher-lows",
				ctx.Regime, rsi5m, rsi15m,
			),
		}
	}

	return NoSignal(name)
}
