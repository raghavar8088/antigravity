package scalpers

import "fmt"

// ─── High-Win-Rate Strategy Family v13 (HW231–HW245) ─────────────────────────
// 15 SHORT strategies: proven WMA/EMA crossover + EFI+ADX template applied to
// new base signals. All designed around the WMA9_EFI_ADX PF=3.98 blueprint.
// ─────────────────────────────────────────────────────────────────────────────

// ── HW231: MACD Signal Cross EFI ADX Short ────────────────────────────────────
// MACD line crosses below signal + EFI<0 + ADX>25 (MACD_Signal = PF=1.65 base)
type MACDSignalEFIADXShort struct{}

func (s *MACDSignalEFIADXShort) Name() string           { return "MACD_Signal_EFI_ADX_Short" }
func (s *MACDSignalEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MACDSignalEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	macd := MACD(ctx.Candles4h)
	macdPrev := MACD(ctx.Candles4h[:n4h-1])
	if macdPrev.MACD <= macdPrev.Signal || macd.MACD >= macd.Signal {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 25 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.88,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("MACD cross↓Signal + EFI<0 + ADX>25(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW232: Stoch Bear Cross EFI ADX Short ─────────────────────────────────────
// Stoch %K/%D bearish cross + EFI<0 + ADX>20 (Stoch_Bear = PF=1.51, 51 trades)
type StochBearCrossEFIADXShort struct{}

func (s *StochBearCrossEFIADXShort) Name() string           { return "Stoch_Bear_EFI_ADX_Short" }
func (s *StochBearCrossEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *StochBearCrossEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	k, d := StochRSI(ctx.Candles4h, 14, 14, 3, 3)
	kPrev, dPrev := StochRSI(ctx.Candles4h[:n4h-1], 14, 14, 3, 3)
	if kPrev <= dPrev || k >= d {
		return NoSignal(name)
	}
	if k >= 60 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Stoch K(%.1f)↓D(%.1f) + EFI<0 + ADX>20. SL=%.2f%%", k, d, slDist/ctx.Price*100),
	}
}

// ── HW233: OBV Slope EFI ADX Short ───────────────────────────────────────────
// OBV declining 3+ bars + EFI<0 + ADX>20 (OBV_Slope = PF=1.62, 144 trades)
type OBVSlopeEFIADXShort struct{}

func (s *OBVSlopeEFIADXShort) Name() string           { return "OBV_Slope_EFI_ADX_Short" }
func (s *OBVSlopeEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OBVSlopeEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	obvSlice := OBVSlice(ctx.Candles4h)
	nO := len(obvSlice)
	if nO < 4 {
		return NoSignal(name)
	}
	if !(obvSlice[nO-1] < obvSlice[nO-2] && obvSlice[nO-2] < obvSlice[nO-3]) {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("OBV declining 3 bars + EFI<0(%.0f) + ADX>20. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW234: BB Squeeze Break EFI ADX Short ────────────────────────────────────
// BB squeeze fires down + EFI<0 + ADX>20 (BB_Squeeze = PF=1.58, 158 trades)
type BBSqueezeBreakEFIADXShort struct{}

func (s *BBSqueezeBreakEFIADXShort) Name() string           { return "BB_Squeeze_EFI_ADX_Short" }
func (s *BBSqueezeBreakEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BBSqueezeBreakEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	bb := BB(ctx.Candles4h, 20)
	bbPrev := BB(ctx.Candles4h[:n4h-1], 20)
	width := bb.Upper - bb.Lower
	widthPrev := bbPrev.Upper - bbPrev.Lower
	// squeeze: BB was narrow, now expanding downward
	if widthPrev > width*0.9 {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	if cur.Close >= cur.Open || cur.Close >= bb.Middle {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("BB squeeze expand↓ + EFI<0(%.0f) + ADX>20. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW235: EMA8 Cross EMA21 EFI ADX Short ────────────────────────────────────
// EMA8 crosses below EMA21 + EFI<0 + ADX>22 (EMA8_Cross = PF=1.30, 171 trades)
type EMA8CrossEFIADXShort struct{}

func (s *EMA8CrossEFIADXShort) Name() string           { return "EMA8_Cross_EFI_ADX_Short" }
func (s *EMA8CrossEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA8CrossEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	ema8 := EMA(ctx.Candles4h, 8)
	ema21 := EMA(ctx.Candles4h, 21)
	ema8Prev := EMA(ctx.Candles4h[:n4h-1], 8)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)
	if ema8Prev <= ema21Prev || ema8 >= ema21 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 22 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA8↓EMA21 + EFI<0(%.0f) + ADX>22. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW236: EMA8 Cross EMA21 CMF ADX Short ────────────────────────────────────
type EMA8CrossCMFADXShort struct{}

func (s *EMA8CrossCMFADXShort) Name() string           { return "EMA8_Cross_CMF_ADX_Short" }
func (s *EMA8CrossCMFADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA8CrossCMFADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	ema8 := EMA(ctx.Candles4h, 8)
	ema21 := EMA(ctx.Candles4h, 21)
	ema8Prev := EMA(ctx.Candles4h[:n4h-1], 8)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)
	if ema8Prev <= ema21Prev || ema8 >= ema21 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.05 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 22 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA8↓EMA21 + CMF<-0.05(%.3f) + ADX>22. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW237: Fisher Zero Cross EFI ADX Short ────────────────────────────────────
// 4h Fisher crosses below 0 + EFI<0 + ADX>20 (Fisher_Zero = PF=1.83, 147 trades)
type FisherZeroEFIADXShort struct{}

func (s *FisherZeroEFIADXShort) Name() string           { return "Fisher_Zero_EFI_ADX_Short" }
func (s *FisherZeroEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *FisherZeroEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 20 || n1h < 22 {
		return NoSignal(name)
	}
	fisher := FisherTransform(ctx.Candles4h, 10)
	fisherPrev := FisherTransform(ctx.Candles4h[:n4h-1], 10)
	if fisherPrev <= 0 || fisher >= 0 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Fisher cross↓0(%.2f) + EFI<0 + ADX>20. SL=%.2f%%", fisher, slDist/ctx.Price*100),
	}
}

// ── HW238: Donchian Lower Break EFI ADX Short ─────────────────────────────────
// Donchian channel breakdown + EFI<0 + ADX>20
type DonchianLowerBreakEFIADXShort struct{}

func (s *DonchianLowerBreakEFIADXShort) Name() string           { return "Donchian_Lower_EFI_ADX_Short" }
func (s *DonchianLowerBreakEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DonchianLowerBreakEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	donch := Donchian(ctx.Candles4h, 20)
	donchPrev := Donchian(ctx.Candles4h[:n4h-1], 20)
	cur := ctx.Candles4h[n4h-1]
	prev := ctx.Candles4h[n4h-2]
	if prev.Close <= donchPrev.Lower || cur.Close >= donch.Lower {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Donchian lower break + EFI<0(%.0f) + ADX>20. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW239: WMA9 CMF Deep EFI Short ───────────────────────────────────────────
// WMA9/21 cross + CMF<-0.08 (deep) + EFI<0 (triple confirmation)
type WMA9CMFDeepEFIShort struct{}

func (s *WMA9CMFDeepEFIShort) Name() string           { return "WMA9_CMF_Deep_EFI_Short" }
func (s *WMA9CMFDeepEFIShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA9CMFDeepEFIShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	wma9 := WMA(ctx.Candles4h, 9)
	wma21 := WMA(ctx.Candles4h, 21)
	wma9Prev := WMA(ctx.Candles4h[:n4h-1], 9)
	wma21Prev := WMA(ctx.Candles4h[:n4h-1], 21)
	if wma9Prev <= wma21Prev || wma9 >= wma21 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.08 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.88,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9↓WMA21 + CMF<-0.08(%.3f) + EFI<0. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW240: WR Mid Cross EFI ADX Short ────────────────────────────────────────
// WR crosses below -50 (mid) + EFI<0 + ADX>20 (WR_Mid_Cross = PF=1.83, 147 trades)
type WRMidCrossEFIADXShort struct{}

func (s *WRMidCrossEFIADXShort) Name() string           { return "WR_Mid_EFI_ADX_Short" }
func (s *WRMidCrossEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WRMidCrossEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 20 || n1h < 22 {
		return NoSignal(name)
	}
	wr := WilliamsR(ctx.Candles4h, 14)
	wrPrev := WilliamsR(ctx.Candles4h[:n4h-1], 14)
	if wrPrev <= -50 || wr >= -50 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WR cross↓-50(%.1f) + EFI<0 + ADX>20. SL=%.2f%%", wr, slDist/ctx.Price*100),
	}
}

// ── HW241: CMF Cross EFI ADX Short ───────────────────────────────────────────
// 4h CMF crosses below 0 + EFI<0 + ADX>20 (CMF_Cross_Bearish = PF=1.33, 327 trades)
type CMFCrossEFIADXShort struct{}

func (s *CMFCrossEFIADXShort) Name() string           { return "CMF_Cross_EFI_ADX_Short" }
func (s *CMFCrossEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CMFCrossEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	cmfPrev := ChaikinMoneyFlow(ctx.Candles4h[:n4h-1], 20)
	if cmfPrev <= 0 || cmf >= 0 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("CMF cross↓0(%.3f) + EFI<0 + ADX>20. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW242: Keltner Lower Break EFI ADX Short ─────────────────────────────────
// Keltner lower break + EFI<0 + ADX>20 (Keltner_Lower = PF=1.31, 523 trades)
type KeltnerLowerBreakEFIADXShort struct{}

func (s *KeltnerLowerBreakEFIADXShort) Name() string           { return "Keltner_Lower_EFI_ADX_Short" }
func (s *KeltnerLowerBreakEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *KeltnerLowerBreakEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	kc := KeltnerChannel(ctx.Candles4h, 20, 14, 1.5)
	kcPrev := KeltnerChannel(ctx.Candles4h[:n4h-1], 20, 14, 1.5)
	cur := ctx.Candles4h[n4h-1]
	prev := ctx.Candles4h[n4h-2]
	if prev.Close <= kcPrev.Lower || cur.Close >= kc.Lower {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Keltner lower break + EFI<0(%.0f) + ADX>20. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW243: Bearish Engulf EFI ADX Short ──────────────────────────────────────
// Bearish engulfing + EFI<0 + ADX>22 (Bearish_Engulf = PF=1.58, 181 trades)
type BearishEngulfEFIADXShort struct{}

func (s *BearishEngulfEFIADXShort) Name() string           { return "Bearish_Engulf_EFI_ADX_Short" }
func (s *BearishEngulfEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BearishEngulfEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 5 || n1h < 22 {
		return NoSignal(name)
	}
	c := ctx.Candles4h
	bull := c[n4h-2]
	bear := c[n4h-1]
	if bull.Close <= bull.Open || bear.Close >= bear.Open {
		return NoSignal(name)
	}
	if bear.Open <= bull.Close || bear.Close >= bull.Open {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 22 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Bearish engulf + EFI<0(%.0f) + ADX>22. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW244: Bearish Momentum Candle EFI ADX Short ─────────────────────────────
// Bearish momentum candle + EFI<0 + ADX>22 (Bearish_Momentum = PF=1.33, 294 trades)
type BearishMomentumCandleEFIADXShort struct{}

func (s *BearishMomentumCandleEFIADXShort) Name() string {
	return "Bearish_Momentum_EFI_ADX_Short"
}
func (s *BearishMomentumCandleEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BearishMomentumCandleEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 20 || n1h < 22 {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	if cur.Close >= cur.Open {
		return NoSignal(name)
	}
	atr4h := ATR(ctx.Candles4h, 14)
	body := cur.Open - cur.Close
	if body < atr4h {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 22 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Bear momentum candle(body>ATR) + EFI<0(%.0f) + ADX>22. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW245: HMA9 Slope Turn EFI ADX Short ─────────────────────────────────────
// HMA9 peak reversal + ADX>25 + EFI<0 (stronger than HMA_Slope_Turn, PF=1.29)
type HMA9SlopeTurnEFIADXShort struct{}

func (s *HMA9SlopeTurnEFIADXShort) Name() string           { return "HMA9_Slope_EFI_ADX_Short" }
func (s *HMA9SlopeTurnEFIADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *HMA9SlopeTurnEFIADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	hma := HMA(ctx.Candles4h, 9)
	hmaPrev := HMA(ctx.Candles4h[:n4h-1], 9)
	hmaPrev2 := HMA(ctx.Candles4h[:n4h-2], 9)
	if hmaPrev2 >= hmaPrev || hma >= hmaPrev {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
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
	if rsi4h > 55 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("HMA9 peak turn↓ + EFI<0(%.0f) + ADX>25. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW246: EFI Cross Zero ADX CMF Short ──────────────────────────────────────
// EFI crosses below 0 + ADX>22 + CMF<-0.05 + EMA down (EFI zero cross with trend)
type EFICrossZeroADXCMFShort struct{}

func (s *EFICrossZeroADXCMFShort) Name() string           { return "EFI_Cross_Zero_ADX_CMF_Short" }
func (s *EFICrossZeroADXCMFShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EFICrossZeroADXCMFShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	efiPrev := ElderForceIndex(ctx.Candles4h[:n4h-1], 13)
	if efiPrev <= 0 || efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 22 {
		return NoSignal(name)
	}
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EFI cross↓0(%.0f) + ADX>22 + CMF<-0.05. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW247: WMA9 Fisher ADX Short ──────────────────────────────────────────────
// WMA9/21 cross + Fisher<0 + ADX>22 (variation on proven WMA9 crossover template)
type WMA9FisherADXShort struct{}

func (s *WMA9FisherADXShort) Name() string           { return "WMA9_Fisher_ADX_Short" }
func (s *WMA9FisherADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA9FisherADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	wma9 := WMA(ctx.Candles4h, 9)
	wma21 := WMA(ctx.Candles4h, 21)
	wma9Prev := WMA(ctx.Candles4h[:n4h-1], 9)
	wma21Prev := WMA(ctx.Candles4h[:n4h-1], 21)
	if wma9Prev <= wma21Prev || wma9 >= wma21 {
		return NoSignal(name)
	}
	if FisherTransform(ctx.Candles4h, 10) >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 22 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9↓WMA21 + Fisher<0 + ADX>22(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

func BuildHWV13Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &MACDSignalEFIADXShort{}, Name: "MACD_Signal_EFI_ADX_Short", Description: "MACD cross↓Signal + EFI<0+ADX>25+EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &StochBearCrossEFIADXShort{}, Name: "Stoch_Bear_EFI_ADX_Short", Description: "Stoch KD bearish cross + EFI<0+ADX>20+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OBVSlopeEFIADXShort{}, Name: "OBV_Slope_EFI_ADX_Short", Description: "OBV declining 3 bars + EFI<0+ADX>20+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBSqueezeBreakEFIADXShort{}, Name: "BB_Squeeze_EFI_ADX_Short", Description: "BB squeeze expand↓ + EFI<0+ADX>20+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EMA8CrossEFIADXShort{}, Name: "EMA8_Cross_EFI_ADX_Short", Description: "EMA8↓EMA21 crossover + EFI<0+ADX>22+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EMA8CrossCMFADXShort{}, Name: "EMA8_Cross_CMF_ADX_Short", Description: "EMA8↓EMA21 crossover + CMF<-0.05+ADX>22+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FisherZeroEFIADXShort{}, Name: "Fisher_Zero_EFI_ADX_Short", Description: "4h Fisher cross↓0 + EFI<0+ADX>20+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DonchianLowerBreakEFIADXShort{}, Name: "Donchian_Lower_EFI_ADX_Short", Description: "Donchian lower break + EFI<0+ADX>20+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA9CMFDeepEFIShort{}, Name: "WMA9_CMF_Deep_EFI_Short", Description: "WMA9↓WMA21 + CMF<-0.08+EFI<0+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WRMidCrossEFIADXShort{}, Name: "WR_Mid_EFI_ADX_Short", Description: "WR cross↓-50 + EFI<0+ADX>20+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CMFCrossEFIADXShort{}, Name: "CMF_Cross_EFI_ADX_Short", Description: "4h CMF cross↓0 + EFI<0+ADX>20+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KeltnerLowerBreakEFIADXShort{}, Name: "Keltner_Lower_EFI_ADX_Short", Description: "Keltner lower break + EFI<0+ADX>20+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BearishEngulfEFIADXShort{}, Name: "Bearish_Engulf_EFI_ADX_Short", Description: "Bearish engulfing + EFI<0+ADX>22+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BearishMomentumCandleEFIADXShort{}, Name: "Bearish_Momentum_EFI_ADX_Short", Description: "Bear momentum candle>ATR + EFI<0+ADX>22+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HMA9SlopeTurnEFIADXShort{}, Name: "HMA9_Slope_EFI_ADX_Short", Description: "HMA9 peak turn↓ + EFI<0+ADX>25+RSI4h<55+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EFICrossZeroADXCMFShort{}, Name: "EFI_Cross_Zero_ADX_CMF_Short", Description: "EFI cross↓0 + ADX>22+CMF<-0.05+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA9FisherADXShort{}, Name: "WMA9_Fisher_ADX_Short", Description: "WMA9↓WMA21 + Fisher<0+ADX>22+RSI4h<58+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
