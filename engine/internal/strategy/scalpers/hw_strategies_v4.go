package scalpers

import (
	"fmt"
	"math"
)

// ─── High-Win-Rate Strategy Family v4 (HW51–HW60) ────────────────────────────
//
// 10 new strategies based on crossover patterns (threshold/level crossovers
// plus trend confirmation). Mirror of the patterns that qualify at PF≥1.30.
// All use hwSLTPShort (2.5×ATR SL / 1.2×ATR TP) for shorts.
// ─────────────────────────────────────────────────────────────────────────────

// ── HW51: MACD Line Crosses Signal Short ──────────────────────────────────────
// Short when 4h MACD *line* crosses below *signal* line (bearish crossover).
// Different from histogram expansion — fires at the crossover moment.

type MACDSignalCrossShort struct{}

func (s *MACDSignalCrossShort) Name() string           { return "MACD_Signal_Cross_Short" }
func (s *MACDSignalCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MACDSignalCrossShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 36 || n1h < 22 {
		return NoSignal(name)
	}
	macd := MACD(ctx.Candles4h)
	macdPrev := MACD(ctx.Candles4h[:n4h-1])
	// MACD line crosses below Signal line
	if macdPrev.MACD <= macdPrev.Signal || macd.MACD >= macd.Signal {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 60 || rsi4h < 25 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 22 || rsi1h > 62 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.81,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h MACD(%.4f) cross↓Signal(%.4f), EMA down, ADX>20. SL=%.2f%%",
			macd.MACD, macd.Signal, slDist/ctx.Price*100),
	}
}

// ── HW52: MACD Line Crosses Signal Long ───────────────────────────────────────
// Long when 4h MACD line crosses above signal line (bullish crossover).

type MACDSignalCrossLong struct{}

func (s *MACDSignalCrossLong) Name() string           { return "MACD_Signal_Cross_Long" }
func (s *MACDSignalCrossLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MACDSignalCrossLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 36 || n1h < 22 {
		return NoSignal(name)
	}
	macd := MACD(ctx.Candles4h)
	macdPrev := MACD(ctx.Candles4h[:n4h-1])
	if macdPrev.MACD >= macdPrev.Signal || macd.MACD <= macd.Signal {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if n4h >= 100 && ctx.Price < EMA(ctx.Candles4h, 100) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 40 || rsi4h > 75 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 38 || rsi1h > 75 {
		return NoSignal(name)
	}
	// Price must be above EMA50 (medium-term bullish structure)
	if n4h >= 55 && ctx.Price < EMA(ctx.Candles4h, 50) {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	// Wider TP (1.0×ATR) to capture MACD momentum before TIME exit kills avg_win
	slDist := math.Max(atr1h*2.5, ctx.Price*0.0180)
	tpDist := math.Max(atr1h*1.0, ctx.Price*0.0070)
	sl := ctx.Price - slDist
	tp := ctx.Price + tpDist
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h MACD(%.4f) cross↑Signal(%.4f), EMA up+50, EMA100 ok. SL=%.2f%%",
			macd.MACD, macd.Signal, slDist/ctx.Price*100),
	}
}

// ── HW53: RSI Mid-Level Cross Short ───────────────────────────────────────────
// Short when 4h RSI crosses below 50 from above (momentum turning bearish).
// Stricter than RSI_4h_Momentum_Short — requires starting from 52-65 range.

type RSIMidCrossShort struct{}

func (s *RSIMidCrossShort) Name() string           { return "RSI_Mid_Cross_Short" }
func (s *RSIMidCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSIMidCrossShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	// Cross below 50 from a moderate overbought range (not deep OB)
	if rsi4hPrev <= 50 || rsi4h >= 50 {
		return NoSignal(name)
	}
	if rsi4hPrev < 52 || rsi4hPrev > 68 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 22 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 22 || rsi1h > 58 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h RSI(%.1f) cross↓50 from %.1f, EMA down, MACD<0. SL=%.2f%%",
			rsi4h, rsi4hPrev, slDist/ctx.Price*100),
	}
}

// ── HW54: RSI Mid-Level Cross Long ────────────────────────────────────────────
// Long when 4h RSI crosses above 50 from below (momentum turning bullish).
// Starts from 32-48 range (not deeply oversold).

type RSIMidCrossLong struct{}

func (s *RSIMidCrossLong) Name() string           { return "RSI_Mid_Cross_Long" }
func (s *RSIMidCrossLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSIMidCrossLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev >= 50 || rsi4h <= 50 {
		return NoSignal(name)
	}
	if rsi4hPrev > 49 || rsi4hPrev < 28 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if n4h >= 100 && ctx.Price < EMA(ctx.Candles4h, 100) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 35 || rsi1h > 75 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h RSI(%.1f) cross↑50 from %.1f, EMA up, MACD+. SL=%.2f%%",
			rsi4h, rsi4hPrev, slDist/ctx.Price*100),
	}
}

// ── HW55: Three EMA Cascade Short ────────────────────────────────────────────
// Short when EMA8 < EMA21 < EMA50 (all bearishly stacked) AND RSI just crossed
// below 50. Triple EMA alignment is a powerful trend-following signal.

type ThreeEMACascadeShort struct{}

func (s *ThreeEMACascadeShort) Name() string           { return "Three_EMA_Cascade_Short" }
func (s *ThreeEMACascadeShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ThreeEMACascadeShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 55 || n1h < 22 {
		return NoSignal(name)
	}
	ema8 := EMA(ctx.Candles4h, 8)
	ema21 := EMA(ctx.Candles4h, 21)
	ema50 := EMA(ctx.Candles4h, 50)
	// All EMAs cascading bearishly
	if ema8 >= ema21 || ema21 >= ema50 {
		return NoSignal(name)
	}
	// RSI crosses below 50
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev <= 50 || rsi4h >= 50 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 20 || rsi1h > 60 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA8(%.0f)<EMA21(%.0f)<EMA50(%.0f), RSI(%.1f) cross↓50. SL=%.2f%%",
			ema8, ema21, ema50, rsi4h, slDist/ctx.Price*100),
	}
}

// ── HW56: EMA8 Crosses EMA50 Short ───────────────────────────────────────────
// Short when EMA8 crosses below EMA50 on 4h (medium-term death cross).
// Signals a deeper structural shift compared to the faster EMA8/21 cross.

type EMA8CrossEMA50Short struct{}

func (s *EMA8CrossEMA50Short) Name() string           { return "EMA8_Cross_EMA50_Short" }
func (s *EMA8CrossEMA50Short) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA8CrossEMA50Short) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 55 || n1h < 22 {
		return NoSignal(name)
	}
	ema8 := EMA(ctx.Candles4h, 8)
	ema50 := EMA(ctx.Candles4h, 50)
	ema8Prev := EMA(ctx.Candles4h[:n4h-1], 8)
	ema50Prev := EMA(ctx.Candles4h[:n4h-1], 50)
	// EMA8 crosses below EMA50
	if ema8Prev <= ema50Prev || ema8 >= ema50 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 20 || rsi1h > 60 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.81,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h EMA8 crosses below EMA50, MACD<0, ADX>18. SL=%.2f%%",
			slDist/ctx.Price*100),
	}
}

// ── HW57: Bearish Momentum Candle Short ──────────────────────────────────────
// Short when 4h candle range (high-low) > 1.8×ATR AND candle is bearish (close<open)
// AND it closes in the lower 30% of its range — strong bearish momentum candle.

type BearishMomentumCandleShort struct{}

func (s *BearishMomentumCandleShort) Name() string           { return "Bearish_Momentum_Candle_Short" }
func (s *BearishMomentumCandleShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BearishMomentumCandleShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	atr4h := ATR(ctx.Candles4h, 14)
	cur4h := ctx.Candles4h[n4h-1]
	candleRange := cur4h.High - cur4h.Low
	if atr4h == 0 || candleRange < 1.8*atr4h {
		return NoSignal(name)
	}
	// Bearish candle closing in lower 30% of range
	if cur4h.Close >= cur4h.Open {
		return NoSignal(name)
	}
	closePos := (cur4h.Close - cur4h.Low) / candleRange
	if closePos > 0.30 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	// RSI below 50 confirms bearish momentum (filters bear-market bounces)
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 50 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 18 || rsi1h > 58 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h bearish momentum candle: range=%.0f(>1.8×ATR), RSI4h=%.1f<50. SL=%.2f%%",
			candleRange, rsi4h, slDist/ctx.Price*100),
	}
}

// ── HW58: Bearish Engulfing Short ────────────────────────────────────────────
// Short when 4h bearish engulfing candle forms (current bearish body > prev bullish body)
// confirmed by EMA downtrend and MACD negative.

type BearishEngulfingShort struct{}

func (s *BearishEngulfingShort) Name() string           { return "Bearish_Engulfing_Short" }
func (s *BearishEngulfingShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BearishEngulfingShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	prev := ctx.Candles4h[n4h-2]
	// Current candle is bearish, prev was bullish
	if cur.Close >= cur.Open || prev.Close <= prev.Open {
		return NoSignal(name)
	}
	// Current body engulfs previous body
	if cur.Open <= prev.Close || cur.Close >= prev.Open {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 20 || rsi1h > 62 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.81,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h bearish engulfing (O=%.0f,C=%.0f engulfs prev O=%.0f,C=%.0f). SL=%.2f%%",
			cur.Open, cur.Close, prev.Open, prev.Close, slDist/ctx.Price*100),
	}
}

// ── HW59: BB Upper Rejection Short ───────────────────────────────────────────
// Short when 4h candle touches or exceeds BB upper band (high >= upper×0.998)
// and closes BELOW the upper band midline — rejection at resistance.
// Works in any trend direction (no EMA8/21 requirement) since BB upper is resistance.

type BBUpperRejectionShort struct{}

func (s *BBUpperRejectionShort) Name() string           { return "BB_Upper_Rejection_Short" }
func (s *BBUpperRejectionShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BBUpperRejectionShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 26 || n1h < 22 {
		return NoSignal(name)
	}
	bb := BB(ctx.Candles4h, 20)
	cur4h := ctx.Candles4h[n4h-1]
	// Candle high touches/exceeds upper band
	if cur4h.High < bb.Upper*0.998 {
		return NoSignal(name)
	}
	// Close below the midpoint between middle and upper band (rejection)
	midUpperZone := (bb.Middle + bb.Upper) / 2
	if cur4h.Close >= midUpperZone {
		return NoSignal(name)
	}
	// Bearish candle (close < open)
	if cur4h.Close >= cur4h.Open {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 45 || rsi4h > 80 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 25 || rsi1h > 72 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.81,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h high(%.0f) touches BB upper(%.0f), close=%.0f below mid(%.0f). SL=%.2f%%",
			cur4h.High, bb.Upper, cur4h.Close, midUpperZone, slDist/ctx.Price*100),
	}
}

// ── HW60: Price Below EMA21 Breakdown Short ──────────────────────────────────
// Short when 4h close crosses below EMA21 (was above, now below) with EMA8 already
// below EMA21 — confirms the breakdown in the established downtrend.

type PriceBelowEMA21Short struct{}

func (s *PriceBelowEMA21Short) Name() string           { return "Price_EMA21_Breakdown_Short" }
func (s *PriceBelowEMA21Short) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *PriceBelowEMA21Short) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 26 || n1h < 22 {
		return NoSignal(name)
	}
	ema21 := EMA(ctx.Candles4h, 21)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)
	cur4h := ctx.Candles4h[n4h-1]
	prev4h := ctx.Candles4h[n4h-2]
	// Price crosses below EMA21
	if prev4h.Close <= ema21Prev || cur4h.Close >= ema21 {
		return NoSignal(name)
	}
	// EMA8 must already be below EMA21 (downtrend structure intact)
	if EMA(ctx.Candles4h, 8) >= ema21 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 55 || rsi4h < 20 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 20 || rsi1h > 60 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.81,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h close(%.0f) breaks below EMA21(%.0f), EMA8 below, MACD<0. SL=%.2f%%",
			cur4h.Close, ema21, slDist/ctx.Price*100),
	}
}

// ── HW61: Bearish Pin Bar Short ──────────────────────────────────────────────
// Short when 4h shooting star / pin bar forms: upper wick ≥ 2× body size AND
// close in lower 35% of range — momentum rejection at local high.

type BearishPinBarShort struct{}

func (s *BearishPinBarShort) Name() string           { return "Bearish_Pin_Bar_Short" }
func (s *BearishPinBarShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BearishPinBarShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	candleRange := cur.High - cur.Low
	body := math.Abs(cur.Close - cur.Open)
	upperWick := cur.High - math.Max(cur.Open, cur.Close)
	if candleRange == 0 || body == 0 {
		return NoSignal(name)
	}
	// Upper wick must be at least 2× the body (shooting star)
	if upperWick < body*2 {
		return NoSignal(name)
	}
	// Upper wick must be significant (> 0.8× ATR4h)
	atr4h := ATR(ctx.Candles4h, 14)
	if upperWick < atr4h*0.8 {
		return NoSignal(name)
	}
	// Close in lower 35% of range
	closePos := (cur.Close - cur.Low) / candleRange
	if closePos > 0.35 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 20 || rsi1h > 68 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h pin bar: upper wick=%.0f (%.1f×body), close pos=%.0f%%. SL=%.2f%%",
			upperWick, upperWick/body, closePos*100, slDist/ctx.Price*100),
	}
}

// ── HW62: Bullish Hammer Long ─────────────────────────────────────────────────
// Long when 4h hammer forms: lower wick ≥ 2× body AND close in upper 40% of range.

type BullishHammerLong struct{}

func (s *BullishHammerLong) Name() string           { return "Bullish_Hammer_Long" }
func (s *BullishHammerLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BullishHammerLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	candleRange := cur.High - cur.Low
	body := math.Abs(cur.Close - cur.Open)
	lowerWick := math.Min(cur.Open, cur.Close) - cur.Low
	if candleRange == 0 || body == 0 {
		return NoSignal(name)
	}
	if lowerWick < body*2 {
		return NoSignal(name)
	}
	atr4h := ATR(ctx.Candles4h, 14)
	if lowerWick < atr4h*0.8 {
		return NoSignal(name)
	}
	closePos := (cur.Close - cur.Low) / candleRange
	if closePos < 0.60 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if n4h >= 100 && ctx.Price < EMA(ctx.Candles4h, 100) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 35 || rsi1h > 75 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.81,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h hammer: lower wick=%.0f (%.1f×body), close pos=%.0f%%. SL=%.2f%%",
			lowerWick, lowerWick/body, closePos*100, slDist/ctx.Price*100),
	}
}

// ── HW63: Fisher Transform Zero Cross Short ──────────────────────────────────
// Short when 4h Fisher Transform crosses below 0 (was positive, now negative).
// Zero crossing = momentum turning bearish. Confirmed by EMA trend.

type MultiTFRSIOverboughtShort struct{}

func (s *MultiTFRSIOverboughtShort) Name() string           { return "Fisher_Zero_Cross_Short" }
func (s *MultiTFRSIOverboughtShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MultiTFRSIOverboughtShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	fish := FisherTransform(ctx.Candles4h, 14)
	fishPrev := FisherTransform(ctx.Candles4h[:n4h-1], 14)
	// Fisher crosses below 0
	if fishPrev <= 0 || fish >= 0 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 20 || rsi1h > 65 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.81,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h Fisher(%.2f) cross↓0 from %.2f, EMA down, MACD<0. SL=%.2f%%",
			fish, fishPrev, slDist/ctx.Price*100),
	}
}

// ── HW64: MACD Hist Sign Flip Short ──────────────────────────────────────────
// Short when 4h MACD histogram flips from positive to negative (histogram sign crossover).
// Captures the exact moment momentum turns bearish. EMA8<EMA21 + ADX>20.

type EMA21BounceShort struct{}

func (s *EMA21BounceShort) Name() string           { return "MACD_Hist_Sign_Flip_Short" }
func (s *EMA21BounceShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA21BounceShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	histCurr := MACD(ctx.Candles4h).Histogram
	histPrev := MACD(ctx.Candles4h[:n4h-1]).Histogram
	// Histogram flips from positive to negative this bar
	if histPrev <= 0 || histCurr >= 0 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 30 || rsi4h > 68 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 22 || rsi1h > 65 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.81,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h MACD hist flip +%.4f→%.4f, EMA down, ADX>20. SL=%.2f%%",
			histPrev, histCurr, slDist/ctx.Price*100),
	}
}

// ── HW65: CMF Extreme Bear Short ─────────────────────────────────────────────
// Short when CMF crosses below -0.15 (deep negative — institutional distribution).
// Stricter than CMF_Cross_Bearish_Short (-0.05) for cleaner signals.

type CMFExtremeBearShort struct{}

func (s *CMFExtremeBearShort) Name() string           { return "CMF_Extreme_Bear_Short" }
func (s *CMFExtremeBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CMFExtremeBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 26 || n1h < 22 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	cmfPrev := ChaikinMoneyFlow(ctx.Candles4h[:n4h-1], 20)
	// CMF crosses below -0.15 (deep distribution)
	if cmfPrev <= -0.15 || cmf >= -0.15 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 55 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 20 || rsi1h > 62 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h CMF(%.3f) cross↓-0.15, EMA down, RSI4h=%.1f. SL=%.2f%%",
			cmf, rsi4h, slDist/ctx.Price*100),
	}
}

// BuildHWV4Strategies returns the HW v4 strategy registry entries.
func BuildHWV4Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &MACDSignalCrossShort{}, Name: "MACD_Signal_Cross_Short", Description: "4h MACD line cross↓signal+EMA down+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MACDSignalCrossLong{}, Name: "MACD_Signal_Cross_Long", Description: "4h MACD line cross↑signal+EMA up+EMA100", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &RSIMidCrossShort{}, Name: "RSI_Mid_Cross_Short", Description: "4h RSI cross↓50 from 52-68+EMA down+MACD-+ADX>22", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &RSIMidCrossLong{}, Name: "RSI_Mid_Cross_Long", Description: "4h RSI cross↑50 from 32-48+EMA up+MACD++ADX>22", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ThreeEMACascadeShort{}, Name: "Three_EMA_Cascade_Short", Description: "EMA8<EMA21<EMA50+RSI cross↓50+MACD-+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EMA8CrossEMA50Short{}, Name: "EMA8_Cross_EMA50_Short", Description: "4h EMA8 crosses below EMA50+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BearishMomentumCandleShort{}, Name: "Bearish_Momentum_Candle_Short", Description: "4h high-range bearish candle(>1.8×ATR)+close low 30%", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BearishEngulfingShort{}, Name: "Bearish_Engulfing_Short", Description: "4h bearish engulfing+EMA down+MACD-+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBUpperRejectionShort{}, Name: "BB_Upper_Rejection_Short", Description: "4h wick above BB upper but close below+EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &PriceBelowEMA21Short{}, Name: "Price_EMA21_Breakdown_Short", Description: "4h close crosses below EMA21+EMA8 already below+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BearishPinBarShort{}, Name: "Bearish_Pin_Bar_Short", Description: "4h shooting star: upper wick≥2×body+close low 35%+EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BullishHammerLong{}, Name: "Bullish_Hammer_Long", Description: "4h hammer: lower wick≥2×body+close top 40%+EMA up+EMA100", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MultiTFRSIOverboughtShort{}, Name: "Fisher_Zero_Cross_Short", Description: "4h Fisher crosses below 0+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EMA21BounceShort{}, Name: "MACD_Hist_Sign_Flip_Short", Description: "4h MACD hist flips +→−+EMA8<EMA21+ADX>20+RSI4h 30-68", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CMFExtremeBearShort{}, Name: "CMF_Extreme_Bear_Short", Description: "4h CMF cross↓-0.15+EMA down+RSI4h<55 deep distribution", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}

// _ suppresses unused import warning for math if not used above
var _ = math.MaxFloat64
