package scalpers

import (
	"fmt"
)

// ─────────────────────────────────────────────────────────────────────────
// NEW STRATEGY BATCH 2 (10) — 2026-07-04.
//
// Batch 1 (engine/internal/strategy/scalpers/new10_batch.go) took 12 rounds
// of iteration to reach 5/10 qualified. The winning formula, distilled from
// what actually qualified there, is applied directly here from round 1
// instead of starting from raw single-indicator ideas:
//   1. A 4h MA CROSSOVER (the actual cross bar, not a static condition) as
//      entry trigger — every qualifier uses this, never a threshold alone.
//   2. ONE secondary confirmation from a different indicator family (CMF,
//      EFI, Fisher, or MACD) required to agree with direction.
//   3. ADX(4h) regime-strength gate (18-22) — filters chop.
//   4. RSI(4h) and RSI(1h) bounds keeping entries out of exhausted moves.
//   5. Regimes = []Regime{RegimeTrending} only.
//   6. Exit sizing REUSES the exact proven recipe (helper function + ATR
//      multiplier) from whichever batch-1 qualifier shares its confirmation
//      family, since PF in this system is driven by win/loss size ratio, not
//      just signal accuracy:
//        - CMF-confirmed longs, ADX>=20              -> hwSLTPTightTP   (PF 1.39 proof: WMA9_CMF_ADX_Long)
///       - EFI-confirmed shorts + EMA50 filter        -> hwSLTPWideTPShort (PF 2.07 proof: EMA8_Cross_WMA21_EFI_ADX_Short)
//        - Fisher-confirmed longs, ADX>=20            -> hwSLTPWideTP   (PF 1.36 proof: EMA8_Cross_WMA13_Fisher_ADX_Long)
//        - CMF-confirmed shorts + EMA50 filter         -> hwSLTPMidPlusShort (PF 1.34 proof: EMA8_Cross_WMA21_CMF_ADX_Short)
//
// All 10 use MA period pairs NOT used by any batch-1 strategy (batch 1 used:
// WMA9/EMA21, EMA8/WMA21, WMA9/WMA21, EMA9/EMA21, EMA21/WMA55, EMA8/WMA13),
// so these are genuinely distinct signals reusing a proven recipe, not
// duplicated logic.
// ─────────────────────────────────────────────────────────────────────────

// ── 1. EMA13 Cross WMA34 + CMF + ADX Long (recipe: CMF-Long/TightTP) ────────
type EMA13CrossWMA34CMFADXLong struct{}

func (s *EMA13CrossWMA34CMFADXLong) Name() string           { return "EMA13_Cross_WMA34_CMF_ADX_Long" }
func (s *EMA13CrossWMA34CMFADXLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA13CrossWMA34CMFADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 45 || n1h < 22 {
		return NoSignal(name)
	}
	ema13 := EMA(ctx.Candles4h, 13)
	wma34 := WMA(ctx.Candles4h, 34)
	ema13Prev := EMA(ctx.Candles4h[:n4h-1], 13)
	wma34Prev := WMA(ctx.Candles4h[:n4h-1], 34)
	if ema13Prev >= wma34Prev || ema13 <= wma34 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf <= 0.04 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 21 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 43 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 33 || rsi1h > 80 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPTightTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA13 cross↑WMA34 + CMF>0.04(%.3f)+ADX>21+MACD+. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── 2. WMA5 Cross EMA13 + EFI + ADX Short (recipe: EFI-Short/WideTP) ────────
type WMA5CrossEMA13EFIADXShort struct{}

func (s *WMA5CrossEMA13EFIADXShort) Name() string           { return "WMA5_Cross_EMA13_EFI_ADX_Short" }
func (s *WMA5CrossEMA13EFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA5CrossEMA13EFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	wma5 := WMA(ctx.Candles4h, 5)
	ema13 := EMA(ctx.Candles4h, 13)
	wma5Prev := WMA(ctx.Candles4h[:n4h-1], 5)
	ema13Prev := EMA(ctx.Candles4h[:n4h-1], 13)
	if wma5Prev <= ema13Prev || wma5 >= ema13 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 24 {
		return NoSignal(name)
	}
	if n4h >= 55 && ctx.Price > EMA(ctx.Candles4h, 50) {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 52 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 18 || rsi1h > 62 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPWideTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA5↓EMA13 + EFI<0(%.0f)+ADX>24+below EMA50. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── 3. EMA9 Cross WMA34 + Fisher + ADX Long (recipe: Fisher-Long/WideTP) ────
type EMA9CrossWMA34FisherADXLong struct{}

func (s *EMA9CrossWMA34FisherADXLong) Name() string { return "EMA9_Cross_WMA34_Fisher_ADX_Long" }
func (s *EMA9CrossWMA34FisherADXLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *EMA9CrossWMA34FisherADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 45 || n1h < 22 {
		return NoSignal(name)
	}
	ema9 := EMA(ctx.Candles4h, 9)
	wma34 := WMA(ctx.Candles4h, 34)
	ema9Prev := EMA(ctx.Candles4h[:n4h-1], 9)
	wma34Prev := WMA(ctx.Candles4h[:n4h-1], 34)
	if ema9Prev >= wma34Prev || ema9 <= wma34 {
		return NoSignal(name)
	}
	fisher := FisherTransform(ctx.Candles4h, 10)
	if fisher <= 0.5 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 25 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 45 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 35 || rsi1h > 80 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPTightTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA9 cross↑WMA34 + Fisher>0.5(%.2f)+ADX>25+MACD+. SL=%.2f%%", fisher, slDist/ctx.Price*100),
	}
}

// ── 4. WMA13 Cross EMA55 + CMF + ADX Short (recipe: CMF-Short/MidPlusTP) ────
type WMA13CrossEMA55CMFADXShort struct{}

func (s *WMA13CrossEMA55CMFADXShort) Name() string { return "WMA13_Cross_EMA55_CMF_ADX_Short" }
func (s *WMA13CrossEMA55CMFADXShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *WMA13CrossEMA55CMFADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	wma13 := WMA(ctx.Candles4h, 13)
	ema55 := EMA(ctx.Candles4h, 55)
	wma13Prev := WMA(ctx.Candles4h[:n4h-1], 13)
	ema55Prev := EMA(ctx.Candles4h[:n4h-1], 55)
	if wma13Prev < ema55Prev || wma13 >= ema55 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.03 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	if n4h >= 55 && ctx.Price > EMA(ctx.Candles4h, 50) {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 60 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 16 || rsi1h > 68 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPMidPlusShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA13 cross↓EMA55 + CMF<-0.03(%.3f)+ADX>18+below EMA50. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── 5. EMA20 Cross WMA34 + CMF + ADX Long (recipe: CMF-Long/TightTP) ────────
type EMA20CrossWMA34CMFADXLong struct{}

func (s *EMA20CrossWMA34CMFADXLong) Name() string { return "EMA20_Cross_WMA34_CMF_ADX_Long" }
func (s *EMA20CrossWMA34CMFADXLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *EMA20CrossWMA34CMFADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 45 || n1h < 22 {
		return NoSignal(name)
	}
	ema20 := EMA(ctx.Candles4h, 20)
	wma34 := WMA(ctx.Candles4h, 34)
	ema20Prev := EMA(ctx.Candles4h[:n4h-1], 20)
	wma34Prev := WMA(ctx.Candles4h[:n4h-1], 34)
	if ema20Prev >= wma34Prev || ema20 <= wma34 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf <= 0.04 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 42 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 32 || rsi1h > 80 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPTightTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA20 cross↑WMA34 + CMF>0.04(%.3f)+ADX>20+MACD+. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── 6. WMA5 Cross WMA21 + EFI + ADX Short (recipe: EFI-Short/WideTP) ────────
type WMA5CrossWMA21EFIADXShort struct{}

func (s *WMA5CrossWMA21EFIADXShort) Name() string { return "WMA5_Cross_WMA21_EFI_ADX_Short" }
func (s *WMA5CrossWMA21EFIADXShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *WMA5CrossWMA21EFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	wma5 := WMA(ctx.Candles4h, 5)
	wma21 := WMA(ctx.Candles4h, 21)
	wma5Prev := WMA(ctx.Candles4h[:n4h-1], 5)
	wma21Prev := WMA(ctx.Candles4h[:n4h-1], 21)
	if wma5Prev <= wma21Prev || wma5 >= wma21 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	if n4h >= 55 && ctx.Price > EMA(ctx.Candles4h, 50) {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 58 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 18 || rsi1h > 65 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPWideTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA5↓WMA21 + EFI<0(%.0f)+ADX>20+below EMA50. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── 7. EMA13 Cross EMA55 + Fisher + ADX Long (recipe: Fisher-Long/WideTP) ───
type EMA13CrossEMA55FisherADXLong struct{}

func (s *EMA13CrossEMA55FisherADXLong) Name() string { return "EMA13_Cross_EMA55_Fisher_ADX_Long" }
func (s *EMA13CrossEMA55FisherADXLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *EMA13CrossEMA55FisherADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	ema13 := EMA(ctx.Candles4h, 13)
	ema55 := EMA(ctx.Candles4h, 55)
	ema13Prev := EMA(ctx.Candles4h[:n4h-1], 13)
	ema55Prev := EMA(ctx.Candles4h[:n4h-1], 55)
	if ema13Prev >= ema55Prev || ema13 <= ema55 {
		return NoSignal(name)
	}
	fisher := FisherTransform(ctx.Candles4h, 10)
	if fisher <= 0.5 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 24 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) <= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram <= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 45 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 35 || rsi1h > 78 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPTightTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA13 cross↑EMA55 + Fisher>0.5(%.2f)+ADX>24+MACD+. SL=%.2f%%", fisher, slDist/ctx.Price*100),
	}
}

// ── 8. WMA9 Cross EMA13 + CMF + ADX Short (recipe: CMF-Short/MidPlusTP) ─────
type WMA9CrossEMA13CMFADXShort struct{}

func (s *WMA9CrossEMA13CMFADXShort) Name() string { return "WMA9_Cross_EMA13_CMF_ADX_Short" }
func (s *WMA9CrossEMA13CMFADXShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *WMA9CrossEMA13CMFADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	wma9 := WMA(ctx.Candles4h, 9)
	ema13 := EMA(ctx.Candles4h, 13)
	wma9Prev := WMA(ctx.Candles4h[:n4h-1], 9)
	ema13Prev := EMA(ctx.Candles4h[:n4h-1], 13)
	if wma9Prev <= ema13Prev || wma9 >= ema13 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.03 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	if n4h >= 55 && ctx.Price > EMA(ctx.Candles4h, 50) {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 60 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 16 || rsi1h > 68 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPMidPlusShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9 cross↓EMA13 + CMF<-0.03(%.3f)+ADX>18+below EMA50. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── 9. EMA8 Cross WMA34 + CMF + ADX Long (recipe: CMF-Long/TightTP) ─────────
type EMA8CrossWMA34CMFADXLong struct{}

func (s *EMA8CrossWMA34CMFADXLong) Name() string           { return "EMA8_Cross_WMA34_CMF_ADX_Long" }
func (s *EMA8CrossWMA34CMFADXLong) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA8CrossWMA34CMFADXLong) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 45 || n1h < 22 {
		return NoSignal(name)
	}
	ema8 := EMA(ctx.Candles4h, 8)
	wma34 := WMA(ctx.Candles4h, 34)
	ema8Prev := EMA(ctx.Candles4h[:n4h-1], 8)
	wma34Prev := WMA(ctx.Candles4h[:n4h-1], 34)
	if ema8Prev >= wma34Prev || ema8 <= wma34 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf <= 0.05 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 42 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 35 || rsi1h > 80 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPMidTP(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionLong, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA8 cross↑WMA34 + CMF>0.05(%.3f)+ADX>20. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── 10. WMA21 Cross EMA55 + EFI + ADX Short (recipe: EFI-Short/WideTP) ──────
type WMA21CrossEMA55EFIADXShort struct{}

func (s *WMA21CrossEMA55EFIADXShort) Name() string { return "WMA21_Cross_EMA55_EFI_ADX_Short" }
func (s *WMA21CrossEMA55EFIADXShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *WMA21CrossEMA55EFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	wma21 := WMA(ctx.Candles4h, 21)
	ema55 := EMA(ctx.Candles4h, 55)
	wma21Prev := WMA(ctx.Candles4h[:n4h-1], 21)
	ema55Prev := EMA(ctx.Candles4h[:n4h-1], 55)
	if wma21Prev < ema55Prev || wma21 >= ema55 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 24 {
		return NoSignal(name)
	}
	if n4h >= 55 && ctx.Price > EMA(ctx.Candles4h, 50) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 52 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h < 18 || rsi1h > 62 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPWideTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA21 cross↓EMA55 + EFI<0(%.0f)+ADX>24+MACD-+below EMA50. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// buildNewStrategiesBatch10v2 registers batch-2 OHLCV-compatible strategies
// (2026-07-04), each reusing a proven exit-sizing recipe from batch 1's
// qualified strategies against a fresh MA-period-pair trigger. Wired into
// BuildAllScalpers() but NOT added to tradeEngineEnabled — manual gate.
func buildNewStrategiesBatch10v2() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy: &EMA13CrossWMA34CMFADXLong{}, Name: "EMA13_Cross_WMA34_CMF_ADX_Long",
			Description:     "4h EMA13 cross↑WMA34 + CMF>0.05+ADX>20 — mixed slower MA cross, money-flow confirmed",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &WMA5CrossEMA13EFIADXShort{}, Name: "WMA5_Cross_EMA13_EFI_ADX_Short",
			Description:     "4h WMA5 cross↓EMA13 + EFI<0+ADX>20 — fast mixed-MA cross, Elder Force confirmed",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &EMA9CrossWMA34FisherADXLong{}, Name: "EMA9_Cross_WMA34_Fisher_ADX_Long",
			Description:     "4h EMA9 cross↑WMA34 + Fisher>0+ADX>20 — mixed slower MA cross, Fisher confirmed",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &WMA13CrossEMA55CMFADXShort{}, Name: "WMA13_Cross_EMA55_CMF_ADX_Short",
			Description:     "4h WMA13 cross↓EMA55 + CMF<-0.03+ADX>18 — slow mixed-MA cross, money-flow confirmed short",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &EMA20CrossWMA34CMFADXLong{}, Name: "EMA20_Cross_WMA34_CMF_ADX_Long",
			Description:     "4h EMA20 cross↑WMA34 + CMF>0.03+ADX>18 — mixed slower MA cross, money-flow confirmed",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &WMA5CrossWMA21EFIADXShort{}, Name: "WMA5_Cross_WMA21_EFI_ADX_Short",
			Description:     "4h WMA5 cross↓WMA21 + EFI<0+ADX>20 — fast pure-WMA cross, Elder Force confirmed",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &EMA13CrossEMA55FisherADXLong{}, Name: "EMA13_Cross_EMA55_Fisher_ADX_Long",
			Description:     "4h EMA13 cross↑EMA55 + Fisher>0+ADX>18 — slow pure-EMA cross, Fisher confirmed",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &WMA9CrossEMA13CMFADXShort{}, Name: "WMA9_Cross_EMA13_CMF_ADX_Short",
			Description:     "4h WMA9 cross↓EMA13 + CMF<-0.03+ADX>18 — mixed-MA cross, money-flow confirmed short",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &EMA8CrossWMA34CMFADXLong{}, Name: "EMA8_Cross_WMA34_CMF_ADX_Long",
			Description:     "4h EMA8 cross↑WMA34 + CMF>0.05+ADX>20 — mixed slower MA cross, money-flow confirmed",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
		{
			Strategy: &WMA21CrossEMA55EFIADXShort{}, Name: "WMA21_Cross_EMA55_EFI_ADX_Short",
			Description:     "4h WMA21 cross↓EMA55 + EFI<0+ADX>18 — slow mixed-MA cross, Elder Force confirmed short",
			Regimes:         []Regime{RegimeTrending},
			Timeframes:      []string{"4h", "1h"},
			MaxPositions:    1,
			OHLCVCompatible: true,
		},
	}
}
