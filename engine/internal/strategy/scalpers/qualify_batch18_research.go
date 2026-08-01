package scalpers

import "fmt"

// qualify_batch18_research.go — second-round candidate batch, built after
// characterizing what actually survives OOS in the 24 already-qualified
// strategies (see delta_full_qualify_results.json + delta_batch10_qualify_results.json).
//
// RESEARCH FINDINGS (round 1 vs round 2):
//   - 21/22 of the first-batch qualifiers anchor on 4h EMA8-vs-EMA21 direction
//     PLUS 4h MACD histogram sign as the trend-alignment filter, with ONE
//     extra confluence signal (candle pattern, CMF, OBV slope, Fisher, or
//     BB-squeeze). The round-1 follow-up batch (2/10 pass) instead anchored
//     on an ADX-magnitude threshold (ADX>15-18) as the trend filter — 4 of
//     those 8 non-qualifiers fired ZERO trades OOS, i.e. ADX-magnitude alone
//     does not generalize the way EMA-direction+MACD-sign does. This batch
//     reverts to the EMA+MACD anchor pattern that is actually proven OOS.
//   - Roughly half the 22 qualifiers are pure price-action (3 lower highs,
//     3 red candles, close-in-bottom-20%-of-range, bearish engulfing) plus
//     the EMA/MACD filter — no exotic indicator required. Simple confluence
//     generalizes better than deep multi-indicator stacks.
//   - The qualified list skews 21-short/1-long. Not necessarily a short bias
///    in the strategy logic itself — more likely the 2025-10→2026-07 OOS
//     window was a net-declining regime for BTC on Delta (validate-window
//     Sharpe is higher than train-window Sharpe for nearly every qualifier,
//     consistent with a strong down-move rewarding shorts specifically in
//     that window). This batch deliberately adds 5 long-side and 2
//     mixed-regime candidates to test that hypothesis rather than assume it.
//   - All 24 already-qualified strategies use 4h primary / 1h confirm. None
//     use 5m/15m primary. This batch tests 3 candidates on 1h primary / 15m
//     confirm to see whether faster timeframes can raise trade frequency
//     without destroying the edge (BACKTEST.md and prior runs never actually
//     tested this — pure gap in current evidence, not a known dead end).
//
// 18 candidates total: 10 short (4h), 5 long (4h), 3 fast (1h/15m).
// All reuse indicators.go primitives; no new math.

// ── R1: Fisher zero-cross + EMA/MACD anchor Bear Short ──────────────────────
type FisherEMAMACDBearShort struct{}

func (s *FisherEMAMACDBearShort) Name() string           { return "Fisher_EMA_MACD_Bear_Short" }
func (s *FisherEMAMACDBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *FisherEMAMACDBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	fish := FisherTransform(ctx.Candles4h, 10)
	fishPrev := FisherTransform(ctx.Candles4h[:n4h-1], 10)
	if fishPrev < 0 || fish >= 0 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Fisher cross↓0(%.2f)+EMA down+MACD-. SL=%.2f%%", fish, slDist/ctx.Price*100)}
}

// ── R2: StochRSI cross + EMA/MACD anchor Bear Short ─────────────────────────
type StochRSIEMAMACDBearShort struct{}

func (s *StochRSIEMAMACDBearShort) Name() string           { return "StochRSI_EMA_MACD_Bear_Short" }
func (s *StochRSIEMAMACDBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *StochRSIEMAMACDBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	k, d := StochRSI(ctx.Candles4h, 14, 14, 3, 3)
	kPrev, dPrev := StochRSI(ctx.Candles4h[:n4h-1], 14, 14, 3, 3)
	if kPrev <= dPrev || k >= d || kPrev < 55 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("StochRSI K cross↓D from>55+EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── R3: HMA slope turn + EMA/MACD anchor Bear Short ─────────────────────────
type HMASlopeEMAMACDBearShort struct{}

func (s *HMASlopeEMAMACDBearShort) Name() string           { return "HMA_Slope_EMA_MACD_Bear_Short" }
func (s *HMASlopeEMAMACDBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *HMASlopeEMAMACDBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	h := HMA(ctx.Candles4h, 9)
	hPrev := HMA(ctx.Candles4h[:n4h-1], 9)
	hPrev2 := HMA(ctx.Candles4h[:n4h-2], 9)
	if !(hPrev2 <= hPrev && hPrev > h) {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("HMA9 slope turn down+EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── R4: Donchian mid-break + EMA/MACD anchor Bear Short ─────────────────────
type DonchianEMAMACDBearShort struct{}

func (s *DonchianEMAMACDBearShort) Name() string           { return "Donchian_EMA_MACD_Bear_Short" }
func (s *DonchianEMAMACDBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *DonchianEMAMACDBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	don := Donchian(ctx.Candles4h, 20)
	closePrev := ctx.Candles4h[n4h-2].Close
	closeNow := ctx.Candles4h[n4h-1].Close
	if !(closePrev >= don.Mid && closeNow < don.Mid) {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Donchian mid-break down+EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── R5: Keltner mid-break + EMA/MACD anchor Bear Short ──────────────────────
type KeltnerEMAMACDBearShort struct{}

func (s *KeltnerEMAMACDBearShort) Name() string           { return "Keltner_EMA_MACD_Bear_Short" }
func (s *KeltnerEMAMACDBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *KeltnerEMAMACDBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	kc := KeltnerChannel(ctx.Candles4h, 20, 10, 1.5)
	closePrev := ctx.Candles4h[n4h-2].Close
	closeNow := ctx.Candles4h[n4h-1].Close
	if !(closePrev >= kc.Mid && closeNow < kc.Mid) {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Keltner mid-break down+EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── R6: OBV 4-bar decline + EMA/MACD anchor Bear Short ──────────────────────
type OBVDeclineEMAMACDBearShort struct{}

func (s *OBVDeclineEMAMACDBearShort) Name() string           { return "OBV_Decline_EMA_MACD_Bear_Short" }
func (s *OBVDeclineEMAMACDBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *OBVDeclineEMAMACDBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	obvNow := OBV(ctx.Candles4h)
	obvPrev := OBV(ctx.Candles4h[:n4h-4])
	if obvNow >= obvPrev {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("OBV declining 4 bars+EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── R7: WMA5/WMA13 cross + EMA/MACD anchor Bear Short ───────────────────────
type WMACrossEMAMACDBearShort struct{}

func (s *WMACrossEMAMACDBearShort) Name() string           { return "WMA_Cross_EMA_MACD_Bear_Short" }
func (s *WMACrossEMAMACDBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *WMACrossEMAMACDBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	w5, w13 := WMA(ctx.Candles4h, 5), WMA(ctx.Candles4h, 13)
	w5p, w13p := WMA(ctx.Candles4h[:n4h-1], 5), WMA(ctx.Candles4h[:n4h-1], 13)
	if !(w5p >= w13p && w5 < w13) {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA5 cross↓WMA13+EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── R8: RSI 55-break + EMA/MACD anchor Bear Short ───────────────────────────
type RSI55EMAMACDBearShort struct{}

func (s *RSI55EMAMACDBearShort) Name() string           { return "RSI55_EMA_MACD_Bear_Short" }
func (s *RSI55EMAMACDBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *RSI55EMAMACDBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	rsi := RSI(ctx.Candles4h, 14)
	rsiPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsiPrev < 55 || rsi >= 55 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("RSI cross↓55(%.1f)+EMA down+MACD-. SL=%.2f%%", rsi, slDist/ctx.Price*100)}
}

// ── R9: EFI zero-cross + EMA/MACD anchor Bear Short ─────────────────────────
type EFIZeroEMAMACDBearShort struct{}

func (s *EFIZeroEMAMACDBearShort) Name() string           { return "EFI_Zero_EMA_MACD_Bear_Short" }
func (s *EFIZeroEMAMACDBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *EFIZeroEMAMACDBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	efiPrev := ElderForceIndex(ctx.Candles4h[:n4h-1], 13)
	if efiPrev < 0 || efi >= 0 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EFI cross↓0+EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── R10: 2 lower highs (lighter than the 3-high pattern) + EMA/MACD Bear Short
type TwoLowerHighsEMAMACDBearShort struct{}

func (s *TwoLowerHighsEMAMACDBearShort) Name() string           { return "Two_Lower_Highs_EMA_MACD_Bear_Short" }
func (s *TwoLowerHighsEMAMACDBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *TwoLowerHighsEMAMACDBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	h0 := ctx.Candles4h[n4h-1].High
	h1 := ctx.Candles4h[n4h-2].High
	h2 := ctx.Candles4h[n4h-3].High
	if !(h0 < h1 && h1 < h2) {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("2 lower highs+EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── L1: EMA8 cross↑EMA21 + MACD+ + CMF>0.05 Bull Long ───────────────────────
type EMAMACDCMFBullLong struct{}

func (s *EMAMACDCMFBullLong) Name() string           { return "EMA_MACD_CMF_Bull_Long" }
func (s *EMAMACDCMFBullLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *EMAMACDCMFBullLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	e8, e21 := EMA(ctx.Candles4h, 8), EMA(ctx.Candles4h, 21)
	e8p, e21p := EMA(ctx.Candles4h[:n4h-1], 8), EMA(ctx.Candles4h[:n4h-1], 21)
	if !(e8p <= e21p && e8 > e21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf <= 0.05 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionLong, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA8 cross↑EMA21+MACD+ +CMF(%.3f)>0.05. SL=%.2f%%", cmf, slDist/ctx.Price*100)}
}

// ── L2: HMA slope turns up + EMA/MACD anchor Bull Long ──────────────────────
type HMASlopeEMAMACDBullLong struct{}

func (s *HMASlopeEMAMACDBullLong) Name() string           { return "HMA_Slope_EMA_MACD_Bull_Long" }
func (s *HMASlopeEMAMACDBullLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *HMASlopeEMAMACDBullLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	h := HMA(ctx.Candles4h, 9)
	hPrev := HMA(ctx.Candles4h[:n4h-1], 9)
	hPrev2 := HMA(ctx.Candles4h[:n4h-2], 9)
	if !(hPrev2 >= hPrev && hPrev < h) {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionLong, Confidence: 0.82, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("HMA9 slope turn up+EMA up+MACD+. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── L3: 3 consecutive green candles + EMA/MACD anchor Bull Long ────────────
type ThreeBullCandlesLong struct{}

func (s *ThreeBullCandlesLong) Name() string           { return "Three_Bull_Candles_Long" }
func (s *ThreeBullCandlesLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *ThreeBullCandlesLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	c0, c1, c2 := ctx.Candles4h[n4h-1], ctx.Candles4h[n4h-2], ctx.Candles4h[n4h-3]
	if !(c0.Close > c0.Open && c1.Close > c1.Open && c2.Close > c2.Open && c0.Close > c1.Close && c1.Close > c2.Close) {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionLong, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("3 green candles closing higher+EMA up+MACD+. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── L4: 3 higher lows + EMA/MACD anchor Bull Long (mirror of Lower_High_Confirm_Short)
type HigherLowConfirmLong struct{}

func (s *HigherLowConfirmLong) Name() string           { return "Higher_Low_Confirm_Long" }
func (s *HigherLowConfirmLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *HigherLowConfirmLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	l0 := ctx.Candles4h[n4h-1].Low
	l1 := ctx.Candles4h[n4h-2].Low
	l2 := ctx.Candles4h[n4h-3].Low
	if !(l0 > l1 && l1 > l2) {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionLong, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("3 higher lows+EMA up+MACD+. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── L5: close in top 20% of range + ADX>25 + MACD+ Bull Long (mirror of Close_Low_Range_ADX_Short)
type CloseHighRangeADXLong struct{}

func (s *CloseHighRangeADXLong) Name() string           { return "Close_High_Range_ADX_Long" }
func (s *CloseHighRangeADXLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *CloseHighRangeADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	c := ctx.Candles4h[n4h-1]
	rng := c.High - c.Low
	if rng <= 0 {
		return NoSignal(name)
	}
	posInRange := (c.Close - c.Low) / rng
	if posInRange < 0.80 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 25 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTP(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionLong, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("close top-20%% of range+ADX>25+MACD+. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── F1: 1h primary — 3 red 1h candles + 1h EMA/MACD anchor Bear Short ───────
type OneHThreeBearCandlesShort struct{}

func (s *OneHThreeBearCandlesShort) Name() string           { return "1h_Three_Bear_Candles_Short" }
func (s *OneHThreeBearCandlesShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *OneHThreeBearCandlesShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n1h := len(ctx.Candles1h)
	if n1h < 40 || len(ctx.Candles15m) < 30 {
		return NoSignal(name)
	}
	c0, c1, c2 := ctx.Candles1h[n1h-1], ctx.Candles1h[n1h-2], ctx.Candles1h[n1h-3]
	if !(c0.Close < c0.Open && c1.Close < c1.Open && c2.Close < c2.Open && c0.Close < c1.Close && c1.Close < c2.Close) {
		return NoSignal(name)
	}
	if EMA(ctx.Candles1h, 8) >= EMA(ctx.Candles1h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles1h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.78, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h 3 red candles closing lower+EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── F2: 1h primary — CMF cross down -0.05 + 1h EMA/MACD anchor Bear Short ───
type OneHCMFCrossBearShortR2 struct{}

func (s *OneHCMFCrossBearShortR2) Name() string           { return "1h_CMF_Cross_EMA_MACD_Bear_Short" }
func (s *OneHCMFCrossBearShortR2) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *OneHCMFCrossBearShortR2) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n1h := len(ctx.Candles1h)
	if n1h < 40 || len(ctx.Candles15m) < 30 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles1h, 20)
	cmfPrev := ChaikinMoneyFlow(ctx.Candles1h[:n1h-1], 20)
	if cmfPrev >= -0.05 || cmf < -0.05 {
		return NoSignal(name) // want the cross itself, not already-deep territory
	}
	if EMA(ctx.Candles1h, 8) >= EMA(ctx.Candles1h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles1h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.78, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h CMF cross↓-0.05(%.3f)+EMA down+MACD-. SL=%.2f%%", cmf, slDist/ctx.Price*100)}
}

// ── F3: 1h primary — close bottom 20% of range + ADX + MACD Bear Short ──────
type OneHCloseLowRangeShort struct{}

func (s *OneHCloseLowRangeShort) Name() string           { return "1h_Close_Low_Range_Short" }
func (s *OneHCloseLowRangeShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *OneHCloseLowRangeShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n1h := len(ctx.Candles1h)
	if n1h < 40 || len(ctx.Candles15m) < 30 {
		return NoSignal(name)
	}
	c := ctx.Candles1h[n1h-1]
	rng := c.High - c.Low
	if rng <= 0 {
		return NoSignal(name)
	}
	posInRange := (c.Close - c.Low) / rng
	if posInRange > 0.20 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles1h, 14) < 25 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles1h).Histogram >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.78, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h close bottom-20%% of range+ADX>25+MACD-. SL=%.2f%%", slDist/ctx.Price*100)}
}

// buildQualifyBatch18Research registers all 18 round-2 candidates.
func buildQualifyBatch18Research() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &FisherEMAMACDBearShort{}, Name: "Fisher_EMA_MACD_Bear_Short", Description: "4h Fisher cross↓0 + EMA down + MACD- (EMA/MACD anchor, round 2)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &StochRSIEMAMACDBearShort{}, Name: "StochRSI_EMA_MACD_Bear_Short", Description: "4h StochRSI K cross↓D from>55 + EMA down + MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HMASlopeEMAMACDBearShort{}, Name: "HMA_Slope_EMA_MACD_Bear_Short", Description: "4h HMA9 slope turns down + EMA down + MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DonchianEMAMACDBearShort{}, Name: "Donchian_EMA_MACD_Bear_Short", Description: "4h close breaks below Donchian(20) mid + EMA down + MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KeltnerEMAMACDBearShort{}, Name: "Keltner_EMA_MACD_Bear_Short", Description: "4h close breaks below Keltner mid + EMA down + MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OBVDeclineEMAMACDBearShort{}, Name: "OBV_Decline_EMA_MACD_Bear_Short", Description: "4h OBV declining 4 bars + EMA down + MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMACrossEMAMACDBearShort{}, Name: "WMA_Cross_EMA_MACD_Bear_Short", Description: "4h WMA5 cross↓WMA13 + EMA down + MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &RSI55EMAMACDBearShort{}, Name: "RSI55_EMA_MACD_Bear_Short", Description: "4h RSI cross↓55 + EMA down + MACD- (retry of round-1 RSI55 with correct anchor)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EFIZeroEMAMACDBearShort{}, Name: "EFI_Zero_EMA_MACD_Bear_Short", Description: "4h EFI(13) cross↓0 + EMA down + MACD- (retry of round-1 EFI with correct anchor)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &TwoLowerHighsEMAMACDBearShort{}, Name: "Two_Lower_Highs_EMA_MACD_Bear_Short", Description: "4h 2 lower highs (lighter than 3-high pattern) + EMA down + MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},

		{Strategy: &EMAMACDCMFBullLong{}, Name: "EMA_MACD_CMF_Bull_Long", Description: "4h EMA8 cross↑EMA21 + MACD+ + CMF>0.05 institutional accumulation", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HMASlopeEMAMACDBullLong{}, Name: "HMA_Slope_EMA_MACD_Bull_Long", Description: "4h HMA9 slope turns up + EMA up + MACD+", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ThreeBullCandlesLong{}, Name: "Three_Bull_Candles_Long", Description: "4h 3 green candles closing higher + EMA up + MACD+", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HigherLowConfirmLong{}, Name: "Higher_Low_Confirm_Long", Description: "4h 3 higher lows + EMA up + MACD+ (mirror of Lower_High_Confirm_Short)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CloseHighRangeADXLong{}, Name: "Close_High_Range_ADX_Long", Description: "4h close top-20% of range + ADX>25 + MACD+ (mirror of Close_Low_Range_ADX_Short)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},

		{Strategy: &OneHThreeBearCandlesShort{}, Name: "1h_Three_Bear_Candles_Short", Description: "1h 3 red candles closing lower + 1h EMA down + 1h MACD- (fast-timeframe test)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"1h", "15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHCMFCrossBearShortR2{}, Name: "1h_CMF_Cross_EMA_MACD_Bear_Short", Description: "1h CMF cross↓-0.05 + 1h EMA down + 1h MACD- (fast-timeframe test)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"1h", "15m"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHCloseLowRangeShort{}, Name: "1h_Close_Low_Range_Short", Description: "1h close bottom-20% of range + 1h ADX>25 + 1h MACD- (fast-timeframe test)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"1h", "15m"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
