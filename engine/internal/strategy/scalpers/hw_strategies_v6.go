package scalpers

import "fmt"

// ─── High-Win-Rate Strategy Family v6 (HW86–HW105) ───────────────────────────
// 20 SHORT strategies using momentum patterns, candle formations, and volume.
// ─────────────────────────────────────────────────────────────────────────────

// ── HW86: Three Consecutive Bear Candles Short ────────────────────────────────
type ThreeBearCandlesShort struct{}

func (s *ThreeBearCandlesShort) Name() string           { return "Three_Bear_Candles_Short" }
func (s *ThreeBearCandlesShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ThreeBearCandlesShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	c1 := ctx.Candles4h[n4h-3]
	c2 := ctx.Candles4h[n4h-2]
	c3 := ctx.Candles4h[n4h-1]
	// 3 consecutive red candles, each closing lower
	if c1.Close >= c1.Open || c2.Close >= c2.Open || c3.Close >= c3.Open {
		return NoSignal(name)
	}
	if c2.Close >= c1.Close || c3.Close >= c2.Close {
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
		Reason: fmt.Sprintf("3 consecutive bear candles, EMA down, MACD-. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW87: Two Bar Bear Reversal Short ─────────────────────────────────────────
type TwoBarBearReversalShort struct{}

func (s *TwoBarBearReversalShort) Name() string           { return "Two_Bar_Bear_Reversal_Short" }
func (s *TwoBarBearReversalShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *TwoBarBearReversalShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	prev := ctx.Candles4h[n4h-2]
	cur := ctx.Candles4h[n4h-1]
	// Prev was bullish, cur is bearish and body > prev body
	prevBody := prev.Close - prev.Open
	curBody := cur.Open - cur.Close
	if prevBody <= 0 || curBody <= 0 {
		return NoSignal(name)
	}
	if curBody < prevBody*1.1 {
		return NoSignal(name)
	}
	// Current close below previous open
	if cur.Close >= prev.Open {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("2-bar bear reversal: bull(%.0f)→bear(%.0f), EMA down. SL=%.2f%%", prevBody, curBody, slDist/ctx.Price*100),
	}
}

// ── HW88: Lower High Confirm Short ───────────────────────────────────────────
type LowerHighConfirmShort struct{}

func (s *LowerHighConfirmShort) Name() string           { return "Lower_High_Confirm_Short" }
func (s *LowerHighConfirmShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *LowerHighConfirmShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	prev := ctx.Candles4h[n4h-2]
	prev2 := ctx.Candles4h[n4h-3]
	// Lower highs for 3 bars AND current close is bearish
	if cur.High >= prev.High || prev.High >= prev2.High {
		return NoSignal(name)
	}
	if cur.Close >= cur.Open {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("3 lower highs(%.0f<%.0f<%.0f), EMA down. SL=%.2f%%", cur.High, prev.High, prev2.High, slDist/ctx.Price*100),
	}
}

// ── HW89: Big Bear Candle Short ───────────────────────────────────────────────
type BigBearCandleShort struct{}

func (s *BigBearCandleShort) Name() string           { return "Big_Bear_Candle_Short" }
func (s *BigBearCandleShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BigBearCandleShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	atr4h := ATR(ctx.Candles4h, 14)
	if atr4h == 0 {
		return NoSignal(name)
	}
	body := cur.Open - cur.Close
	// Bearish candle with body > 1.5×ATR4h
	if body < atr4h*1.5 {
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
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 60 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Big bear candle body(%.0f) > 1.5×ATR(%.0f), EMA down. SL=%.2f%%", body, atr4h, slDist/ctx.Price*100),
	}
}

// ── HW90: MACD Line Zero Cross Bear Short ────────────────────────────────────
type MACDLineZeroCrossBearShort struct{}

func (s *MACDLineZeroCrossBearShort) Name() string           { return "MACD_Line_Zero_Cross_Short" }
func (s *MACDLineZeroCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MACDLineZeroCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	macd := MACD(ctx.Candles4h)
	macdPrev := MACD(ctx.Candles4h[:n4h-1])
	// MACD line crosses below 0 from above
	if macdPrev.MACD <= 0 || macd.MACD >= 0 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 60 {
		return NoSignal(name)
	}
	// CMF<0 confirms institutional selling pressure on the MACD zero cross
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h MACD line cross↓0(%.4f→%.4f), EMA down. SL=%.2f%%", macdPrev.MACD, macd.MACD, slDist/ctx.Price*100),
	}
}

// ── HW91: BB Upper Band Walk Fail Short ──────────────────────────────────────
type BBUpperWalkFailShort struct{}

func (s *BBUpperWalkFailShort) Name() string           { return "BB_Upper_Walk_Fail_Short" }
func (s *BBUpperWalkFailShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BBUpperWalkFailShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	bb := BB(ctx.Candles4h, 20)
	bbPrev := BB(ctx.Candles4h[:n4h-1], 20)
	cur := ctx.Candles4h[n4h-1]
	prev := ctx.Candles4h[n4h-2]
	// Prev close was above BB upper, current close drops back below
	if prev.Close <= bbPrev.Upper || cur.Close >= bb.Upper {
		return NoSignal(name)
	}
	if cur.Close >= cur.Open {
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
		Reason: fmt.Sprintf("4h close falls back from BB upper walk, EMA down. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW92: ATR Spike Bear Short ────────────────────────────────────────────────
type ATRSpikeBearShort struct{}

func (s *ATRSpikeBearShort) Name() string           { return "ATR_Spike_Bear_Short" }
func (s *ATRSpikeBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ATRSpikeBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	atr4h := ATR(ctx.Candles4h, 14)
	atr4hAvg := ATR(ctx.Candles4h[:n4h-1], 14)
	cur := ctx.Candles4h[n4h-1]
	// ATR spike (current > 1.4× prior ATR) with bearish candle
	if atr4hAvg == 0 || atr4h < atr4hAvg*1.4 {
		return NoSignal(name)
	}
	if cur.Close >= cur.Open {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 62 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h ATR spike(%.0f vs avg %.0f) bearish, EMA down. SL=%.2f%%", atr4h, atr4hAvg, slDist/ctx.Price*100),
	}
}

// ── HW93: Price Below All EMAs Short ─────────────────────────────────────────
type PriceBelowAllEMAsShort struct{}

func (s *PriceBelowAllEMAsShort) Name() string           { return "Price_Below_All_EMAs_Short" }
func (s *PriceBelowAllEMAsShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *PriceBelowAllEMAsShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 55 || n1h < 22 {
		return NoSignal(name)
	}
	ema8 := EMA(ctx.Candles4h, 8)
	ema21 := EMA(ctx.Candles4h, 21)
	ema50 := EMA(ctx.Candles4h, 50)
	cur := ctx.Candles4h[n4h-1]
	// Price below all EMAs, all cascading down
	if cur.Close >= ema8 || ema8 >= ema21 || ema21 >= ema50 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 20 || rsi4h > 55 {
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
		Reason: fmt.Sprintf("4h price(%.0f)<EMA8(%.0f)<EMA21<EMA50, full cascade. SL=%.2f%%", cur.Close, ema8, slDist/ctx.Price*100),
	}
}

// ── HW94: PSAR ADX Strong Short ───────────────────────────────────────────────
type PSARADXStrongShort struct{}

func (s *PSARADXStrongShort) Name() string           { return "PSAR_ADX_Strong_Short" }
func (s *PSARADXStrongShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *PSARADXStrongShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	psar, bullish := PSARValue(ctx.Candles4h, 0.02, 0.2)
	if bullish {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	if cur.Close >= psar {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 25 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 25 || rsi4h > 58 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h PSAR bear+ADX>25(%.1f), EMA down. SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW95: StochRSI 50 Cross Bear Short ───────────────────────────────────────
type StochRSI50CrossBearShort struct{}

func (s *StochRSI50CrossBearShort) Name() string           { return "StochRSI_50_Cross_Bear_Short" }
func (s *StochRSI50CrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *StochRSI50CrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 40 || n1h < 22 {
		return NoSignal(name)
	}
	k, _ := StochRSI(ctx.Candles4h, 14, 14, 3, 3)
	kPrev, _ := StochRSI(ctx.Candles4h[:n4h-1], 14, 14, 3, 3)
	// StochRSI K crosses below 50 from overbought territory (>62) — ensures entering from elevated momentum
	if kPrev <= 62 || k >= 50 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h StochRSI K(%.1f) cross↓50, EMA down. SL=%.2f%%", k, slDist/ctx.Price*100),
	}
}

// ── HW96: WilliamsR Mid Cross Bear Short ─────────────────────────────────────
type WRMidCrossBearShort struct{}

func (s *WRMidCrossBearShort) Name() string           { return "WR_Mid_Cross_Bear_Short" }
func (s *WRMidCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WRMidCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 20 || n1h < 22 {
		return NoSignal(name)
	}
	wr := WilliamsR(ctx.Candles4h, 14)
	wrPrev := WilliamsR(ctx.Candles4h[:n4h-1], 14)
	// WilliamsR crosses below -50 (midline) from above
	if wrPrev <= -50 || wr >= -50 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h WR(%.1f) cross↓-50 midline from %.1f, EMA down. SL=%.2f%%", wr, wrPrev, slDist/ctx.Price*100),
	}
}

// ── HW97: CMF Deep Bear Short ─────────────────────────────────────────────────
type CMFDeepBearShort struct{}

func (s *CMFDeepBearShort) Name() string           { return "CMF_Deep_Bear_Short" }
func (s *CMFDeepBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CMFDeepBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	cmfPrev := ChaikinMoneyFlow(ctx.Candles4h[:n4h-1], 20)
	// CMF crosses below -0.20 (deep institutional selling)
	if cmfPrev <= -0.20 || cmf >= -0.20 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h CMF(%.3f) cross↓-0.20, deep selling, EMA down. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW98: ADX Strong Trend Bear Short ─────────────────────────────────────────
type ADXStrongTrendBearShort struct{}

func (s *ADXStrongTrendBearShort) Name() string           { return "ADX_Strong_Trend_Bear_Short" }
func (s *ADXStrongTrendBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ADXStrongTrendBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	adx := ADX(ctx.Candles4h, 14)
	adxPrev := ADX(ctx.Candles4h[:n4h-1], 14)
	// ADX crosses above 28 (strong trend confirmation)
	if adxPrev >= 28 || adx < 28 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 25 || rsi4h > 58 {
		return NoSignal(name)
	}
	// CMF below -0.05 confirms sustained distribution as ADX surges
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h ADX cross↑28(%.1f→%.1f), EMA down, MACD-, CMF<-0.05. SL=%.2f%%", adxPrev, adx, slDist/ctx.Price*100),
	}
}

// ── HW99: Multi EMA Full Cascade Short ───────────────────────────────────────
type MultiEMAFullCascadeShort struct{}

func (s *MultiEMAFullCascadeShort) Name() string           { return "Multi_EMA_Full_Cascade_Short" }
func (s *MultiEMAFullCascadeShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MultiEMAFullCascadeShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 105 || n1h < 22 {
		return NoSignal(name)
	}
	ema8 := EMA(ctx.Candles4h, 8)
	ema21 := EMA(ctx.Candles4h, 21)
	ema50 := EMA(ctx.Candles4h, 50)
	ema100 := EMA(ctx.Candles4h, 100)
	// Full bearish cascade: EMA8 < EMA21 < EMA50 < EMA100
	if ema8 >= ema21 || ema21 >= ema50 || ema50 >= ema100 {
		return NoSignal(name)
	}
	macdH := MACD(ctx.Candles4h).Histogram
	if macdH >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 20 || rsi4h > 55 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Full EMA cascade: 8<21<50<100, MACD<0, ADX>20. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW100: RSI Bear Slope Short ───────────────────────────────────────────────
type RSIBearSlopeShort struct{}

func (s *RSIBearSlopeShort) Name() string           { return "RSI_Bear_Slope_Short" }
func (s *RSIBearSlopeShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSIBearSlopeShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	r0 := RSI(ctx.Candles4h, 14)
	r1 := RSI(ctx.Candles4h[:n4h-1], 14)
	r2 := RSI(ctx.Candles4h[:n4h-2], 14)
	r3 := RSI(ctx.Candles4h[:n4h-3], 14)
	// RSI declining 3 consecutive bars, currently below 55
	if r0 >= r1 || r1 >= r2 || r2 >= r3 || r0 > 55 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
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
		Reason: fmt.Sprintf("4h RSI declining 3 bars(%.1f<%.1f<%.1f<%.1f), EMA down. SL=%.2f%%", r0, r1, r2, r3, slDist/ctx.Price*100),
	}
}

// ── HW101: KST Below Zero Bear Short ─────────────────────────────────────────
type KSTBelowZeroBearShort struct{}

func (s *KSTBelowZeroBearShort) Name() string           { return "KST_Below_Zero_Bear_Short" }
func (s *KSTBelowZeroBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *KSTBelowZeroBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	kst := KST(ctx.Candles4h)
	kstPrev := KST(ctx.Candles4h[:n4h-1])
	if kst.KST == 0 {
		return NoSignal(name)
	}
	// KST crosses below 0
	if kstPrev.KST <= 0 || kst.KST >= 0 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h KST(%.3f) cross↓0, EMA down, MACD-. SL=%.2f%%", kst.KST, slDist/ctx.Price*100),
	}
}

// ── HW102: Aroon Bear Strong Short ───────────────────────────────────────────
type AroonBearStrongShort struct{}

func (s *AroonBearStrongShort) Name() string           { return "Aroon_Bear_Strong_Short" }
func (s *AroonBearStrongShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *AroonBearStrongShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	ar := Aroon(ctx.Candles4h, 25)
	arPrev := Aroon(ctx.Candles4h[:n4h-1], 25)
	// Aroon Down crosses above 75 (strong bearish trend)
	if arPrev.Down >= 75 || ar.Down < 75 {
		return NoSignal(name)
	}
	if ar.Up > 30 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h Aroon down>75(%.0f), up<30(%.0f), EMA down. SL=%.2f%%", ar.Down, ar.Up, slDist/ctx.Price*100),
	}
}

// ── HW103: EFI Bear Cross Short ───────────────────────────────────────────────
type EFIBearCrossShort struct{}

func (s *EFIBearCrossShort) Name() string           { return "EFI_Bear_Cross_Short" }
func (s *EFIBearCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EFIBearCrossShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	efiPrev := ElderForceIndex(ctx.Candles4h[:n4h-1], 13)
	// EFI crosses below its prior value (declining force) and goes negative
	if efiPrev <= 0 || efi >= efiPrev {
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
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 58 {
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
		Reason: fmt.Sprintf("4h EFI declining from positive(%.0f→%.0f), EMA down. SL=%.2f%%", efiPrev, efi, slDist/ctx.Price*100),
	}
}

// ── HW104: Fisher Deep Bear Short ────────────────────────────────────────────
type FisherDeepBearShort struct{}

func (s *FisherDeepBearShort) Name() string           { return "Fisher_Deep_Bear_Short" }
func (s *FisherDeepBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *FisherDeepBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 20 || n1h < 22 {
		return NoSignal(name)
	}
	fish := FisherTransform(ctx.Candles4h, 14)
	fishPrev := FisherTransform(ctx.Candles4h[:n4h-1], 14)
	// Fisher crosses below -1.0 (deep bearish signal)
	if fishPrev <= -1.0 || fish >= -1.0 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h Fisher cross↓-1.0(%.2f→%.2f), deep bear. SL=%.2f%%", fishPrev, fish, slDist/ctx.Price*100),
	}
}

// ── HW105: Close Low Range Bear Short ────────────────────────────────────────
type CloseLowRangeBearShort struct{}

func (s *CloseLowRangeBearShort) Name() string           { return "Close_Low_Range_Bear_Short" }
func (s *CloseLowRangeBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CloseLowRangeBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	rng := cur.High - cur.Low
	if rng == 0 {
		return NoSignal(name)
	}
	// Close in bottom 20% of 4h candle range (bearish momentum close)
	closePos := (cur.Close - cur.Low) / rng
	if closePos > 0.20 {
		return NoSignal(name)
	}
	atr4h := ATR(ctx.Candles4h, 14)
	// Range must be at least 0.8×ATR (meaningful candle)
	if rng < atr4h*0.8 {
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
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 58 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h close in bottom %.0f%% of range, EMA down, MACD-. SL=%.2f%%", closePos*100, slDist/ctx.Price*100),
	}
}

func BuildHWV6Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &ThreeBearCandlesShort{}, Name: "Three_Bear_Candles_Short", Description: "3 consecutive red candles closing lower+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &TwoBarBearReversalShort{}, Name: "Two_Bar_Bear_Reversal_Short", Description: "Bull bar→larger bear bar closes below bull open+EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &LowerHighConfirmShort{}, Name: "Lower_High_Confirm_Short", Description: "3 consecutive lower highs+bearish close+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BigBearCandleShort{}, Name: "Big_Bear_Candle_Short", Description: "4h bearish body>1.5×ATR+EMA down+MACD-+RSI4h<60", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MACDLineZeroCrossBearShort{}, Name: "MACD_Line_Zero_Cross_Short", Description: "4h MACD line cross↓0+EMA down+ADX>18+RSI4h<60", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBUpperWalkFailShort{}, Name: "BB_Upper_Walk_Fail_Short", Description: "4h close falls below BB upper after walking+EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ATRSpikeBearShort{}, Name: "ATR_Spike_Bear_Short", Description: "4h ATR>1.4×avg+bearish candle+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &PriceBelowAllEMAsShort{}, Name: "Price_Below_All_EMAs_Short", Description: "4h price<EMA8<EMA21<EMA50 full cascade+MACD-+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &PSARADXStrongShort{}, Name: "PSAR_ADX_Strong_Short", Description: "4h PSAR bearish+ADX>25+RSI4h 25-58+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &StochRSI50CrossBearShort{}, Name: "StochRSI_50_Cross_Bear_Short", Description: "4h StochRSI K cross↓50+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WRMidCrossBearShort{}, Name: "WR_Mid_Cross_Bear_Short", Description: "4h WR cross↓-50 midline+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CMFDeepBearShort{}, Name: "CMF_Deep_Bear_Short", Description: "4h CMF cross↓-0.20+EMA down+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ADXStrongTrendBearShort{}, Name: "ADX_Strong_Trend_Bear_Short", Description: "4h ADX cross↑28+EMA down+MACD-+RSI4h 25-58", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MultiEMAFullCascadeShort{}, Name: "Multi_EMA_Full_Cascade_Short", Description: "4h EMA8<EMA21<EMA50<EMA100+MACD-+ADX>20+RSI4h 20-55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &RSIBearSlopeShort{}, Name: "RSI_Bear_Slope_Short", Description: "4h RSI declining 3 bars below 55+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KSTBelowZeroBearShort{}, Name: "KST_Below_Zero_Bear_Short", Description: "4h KST cross↓0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &AroonBearStrongShort{}, Name: "Aroon_Bear_Strong_Short", Description: "4h Aroon down cross>75+up<30+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EFIBearCrossShort{}, Name: "EFI_Bear_Cross_Short", Description: "4h EFI declining from positive+EMA down+MACD-+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FisherDeepBearShort{}, Name: "Fisher_Deep_Bear_Short", Description: "4h Fisher cross↓-1.0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CloseLowRangeBearShort{}, Name: "Close_Low_Range_Bear_Short", Description: "4h close in bottom 20% of range+ADX>22+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
