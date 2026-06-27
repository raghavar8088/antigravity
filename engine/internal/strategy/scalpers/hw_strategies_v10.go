package scalpers

import "fmt"

// ─── High-Win-Rate Strategy Family v10 (HW166–HW185) ─────────────────────────
// 20 SHORT strategies: WMA period variants with ADX/EFI/CMF filters that proved
// PF=3.19-3.98 in v9. Extends to shorter (5/13) and longer (13/34) periods and
// adds 1h-timeframe variants.
// ─────────────────────────────────────────────────────────────────────────────

// ── HW166: WMA5/WMA13 ADX Bear Short ─────────────────────────────────────────
type WMA5WMA13ADXBearShort struct{}

func (s *WMA5WMA13ADXBearShort) Name() string           { return "WMA5_WMA13_ADX_Bear_Short" }
func (s *WMA5WMA13ADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA5WMA13ADXBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	wma5 := WMA(ctx.Candles4h, 5)
	wma13 := WMA(ctx.Candles4h, 13)
	wma5Prev := WMA(ctx.Candles4h[:n4h-1], 5)
	wma13Prev := WMA(ctx.Candles4h[:n4h-1], 13)
	if wma5Prev <= wma13Prev || wma5 >= wma13 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA5 cross↓WMA13 + ADX>25(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW167: WMA5/WMA13 EFI Bear Short ─────────────────────────────────────────
type WMA5WMA13EFIBearShort struct{}

func (s *WMA5WMA13EFIBearShort) Name() string           { return "WMA5_WMA13_EFI_Bear_Short" }
func (s *WMA5WMA13EFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA5WMA13EFIBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	wma5 := WMA(ctx.Candles4h, 5)
	wma13 := WMA(ctx.Candles4h, 13)
	wma5Prev := WMA(ctx.Candles4h[:n4h-1], 5)
	wma13Prev := WMA(ctx.Candles4h[:n4h-1], 13)
	if wma5Prev <= wma13Prev || wma5 >= wma13 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA5 cross↓WMA13 + EFI<0(%.0f). SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW168: WMA13/WMA34 ADX Bear Short ────────────────────────────────────────
type WMA13WMA34ADXBearShort struct{}

func (s *WMA13WMA34ADXBearShort) Name() string           { return "WMA13_WMA34_ADX_Bear_Short" }
func (s *WMA13WMA34ADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA13WMA34ADXBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 50 || n1h < 22 {
		return NoSignal(name)
	}
	wma13 := WMA(ctx.Candles4h, 13)
	wma34 := WMA(ctx.Candles4h, 34)
	wma13Prev := WMA(ctx.Candles4h[:n4h-1], 13)
	wma34Prev := WMA(ctx.Candles4h[:n4h-1], 34)
	if wma13Prev <= wma34Prev || wma13 >= wma34 {
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
		Reason: fmt.Sprintf("WMA13 cross↓WMA34 + ADX>25(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW169: WMA13/WMA34 EFI Bear Short ────────────────────────────────────────
type WMA13WMA34EFIBearShort struct{}

func (s *WMA13WMA34EFIBearShort) Name() string           { return "WMA13_WMA34_EFI_Bear_Short" }
func (s *WMA13WMA34EFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA13WMA34EFIBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 50 || n1h < 22 {
		return NoSignal(name)
	}
	wma13 := WMA(ctx.Candles4h, 13)
	wma34 := WMA(ctx.Candles4h, 34)
	wma13Prev := WMA(ctx.Candles4h[:n4h-1], 13)
	wma34Prev := WMA(ctx.Candles4h[:n4h-1], 34)
	if wma13Prev <= wma34Prev || wma13 >= wma34 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	// ADX>20 ensures we're in a trending market when WMA13/34 crosses
	if ADX(ctx.Candles4h, 14) < 20 {
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
		Reason: fmt.Sprintf("WMA13 cross↓WMA34 + EFI<0(%.0f) + ADX>20. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW170: WMA13/WMA34 CMF Deep Bear Short ───────────────────────────────────
type WMA13WMA34CMFDeepBearShort struct{}

func (s *WMA13WMA34CMFDeepBearShort) Name() string           { return "WMA13_WMA34_CMF_Deep_Bear_Short" }
func (s *WMA13WMA34CMFDeepBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA13WMA34CMFDeepBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 50 || n1h < 22 {
		return NoSignal(name)
	}
	wma13 := WMA(ctx.Candles4h, 13)
	wma34 := WMA(ctx.Candles4h, 34)
	wma13Prev := WMA(ctx.Candles4h[:n4h-1], 13)
	wma34Prev := WMA(ctx.Candles4h[:n4h-1], 34)
	if wma13Prev <= wma34Prev || wma13 >= wma34 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.10 {
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
		Reason: fmt.Sprintf("WMA13 cross↓WMA34 + CMF<-0.10(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW171: ZLEMA ADX Bear Short ───────────────────────────────────────────────
type ZLEMAADXBearShort struct{}

func (s *ZLEMAADXBearShort) Name() string           { return "ZLEMA_ADX_Bear_Short" }
func (s *ZLEMAADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ZLEMAADXBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	zlema9 := ZLEMA(ctx.Candles4h, 9)
	zlema21 := ZLEMA(ctx.Candles4h, 21)
	zlema9Prev := ZLEMA(ctx.Candles4h[:n4h-1], 9)
	zlema21Prev := ZLEMA(ctx.Candles4h[:n4h-1], 21)
	if zlema9Prev <= zlema21Prev || zlema9 >= zlema21 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 25 {
		return NoSignal(name)
	}
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("ZLEMA9 cross↓ZLEMA21 + ADX>25(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW172: ZLEMA Fisher Bear Short ────────────────────────────────────────────
type ZLEMAFisherBearShort struct{}

func (s *ZLEMAFisherBearShort) Name() string           { return "ZLEMA_Fisher_Bear_Short" }
func (s *ZLEMAFisherBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ZLEMAFisherBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	zlema9 := ZLEMA(ctx.Candles4h, 9)
	zlema21 := ZLEMA(ctx.Candles4h, 21)
	zlema9Prev := ZLEMA(ctx.Candles4h[:n4h-1], 9)
	zlema21Prev := ZLEMA(ctx.Candles4h[:n4h-1], 21)
	if zlema9Prev <= zlema21Prev || zlema9 >= zlema21 {
		return NoSignal(name)
	}
	fisher := FisherTransform(ctx.Candles4h, 10)
	if fisher >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
		return NoSignal(name)
	}
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= 0 {
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
		Reason: fmt.Sprintf("ZLEMA9 cross↓ZLEMA21 + Fisher<0(%.2f)+ADX>20. SL=%.2f%%", fisher, slDist/ctx.Price*100),
	}
}

// ── HW173: DEMA ADX Strong Bear Short ────────────────────────────────────────
type DEMAADXStrongBearShort struct{}

func (s *DEMAADXStrongBearShort) Name() string           { return "DEMA_ADX_Strong_Bear_Short" }
func (s *DEMAADXStrongBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DEMAADXStrongBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	dema8 := DEMA(ctx.Candles4h, 8)
	dema21 := DEMA(ctx.Candles4h, 21)
	dema8Prev := DEMA(ctx.Candles4h[:n4h-1], 8)
	dema21Prev := DEMA(ctx.Candles4h[:n4h-1], 21)
	if dema8Prev <= dema21Prev || dema8 >= dema21 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("DEMA8 cross↓DEMA21 + ADX>25(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW174: DEMA EFI Bear Short ────────────────────────────────────────────────
type DEMAEFIBearShort struct{}

func (s *DEMAEFIBearShort) Name() string           { return "DEMA_EFI_Bear_Short" }
func (s *DEMAEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DEMAEFIBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 35 || n1h < 22 {
		return NoSignal(name)
	}
	dema8 := DEMA(ctx.Candles4h, 8)
	dema21 := DEMA(ctx.Candles4h, 21)
	dema8Prev := DEMA(ctx.Candles4h[:n4h-1], 8)
	dema21Prev := DEMA(ctx.Candles4h[:n4h-1], 21)
	if dema8Prev <= dema21Prev || dema8 >= dema21 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("DEMA8 cross↓DEMA21 + EFI<0(%.0f). SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW175: 1h WMA9/WMA21 ADX Bear Short ──────────────────────────────────────
type OneHWMA9WMA21ADXBearShort struct{}

func (s *OneHWMA9WMA21ADXBearShort) Name() string           { return "1h_WMA9_WMA21_ADX_Bear_Short" }
func (s *OneHWMA9WMA21ADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHWMA9WMA21ADXBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 35 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	wma9 := WMA(ctx.Candles1h, 9)
	wma21 := WMA(ctx.Candles1h, 21)
	wma9Prev := WMA(ctx.Candles1h[:n1h-1], 9)
	wma21Prev := WMA(ctx.Candles1h[:n1h-1], 21)
	if wma9Prev <= wma21Prev || wma9 >= wma21 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles1h, 14) < 20 {
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
		Reason: fmt.Sprintf("1h WMA9 cross↓WMA21 + 1h ADX>20(%.1f). SL=%.2f%%", ADX(ctx.Candles1h, 14), slDist/ctx.Price*100),
	}
}

// ── HW176: 1h ZLEMA Bear CMF Short ───────────────────────────────────────────
type OneHZLEMACMFBearShort struct{}

func (s *OneHZLEMACMFBearShort) Name() string           { return "1h_ZLEMA_CMF_Bear_Short" }
func (s *OneHZLEMACMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHZLEMACMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 35 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	zlema9 := ZLEMA(ctx.Candles1h, 9)
	zlema21 := ZLEMA(ctx.Candles1h, 21)
	zlema9Prev := ZLEMA(ctx.Candles1h[:n1h-1], 9)
	zlema21Prev := ZLEMA(ctx.Candles1h[:n1h-1], 21)
	if zlema9Prev <= zlema21Prev || zlema9 >= zlema21 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h ZLEMA9 cross↓ZLEMA21 + 1h CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW177: 1h Fisher CMF Confirm Short ───────────────────────────────────────
type OneHFisherCMFConfirmShort struct{}

func (s *OneHFisherCMFConfirmShort) Name() string           { return "1h_Fisher_CMF_Confirm_Short" }
func (s *OneHFisherCMFConfirmShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHFisherCMFConfirmShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 25 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	fisher1h := FisherTransform(ctx.Candles1h, 10)
	fisher1hPrev := FisherTransform(ctx.Candles1h[:n1h-1], 10)
	if fisher1hPrev <= 0 || fisher1h >= 0 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= -0.05 {
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
		Reason: fmt.Sprintf("1h Fisher cross↓0 + 1h CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW178: 1h WMA5 WMA13 Bear Short ──────────────────────────────────────────
type OneHWMA5WMA13BearShort struct{}

func (s *OneHWMA5WMA13BearShort) Name() string           { return "1h_WMA5_WMA13_Bear_Short" }
func (s *OneHWMA5WMA13BearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHWMA5WMA13BearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 25 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	wma5 := WMA(ctx.Candles1h, 5)
	wma13 := WMA(ctx.Candles1h, 13)
	wma5Prev := WMA(ctx.Candles1h[:n1h-1], 5)
	wma13Prev := WMA(ctx.Candles1h[:n1h-1], 13)
	if wma5Prev <= wma13Prev || wma5 >= wma13 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h WMA5 cross↓WMA13 + CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW179: 1h WR CMF Bear Short ───────────────────────────────────────────────
type OneHWRCMFBearShort struct{}

func (s *OneHWRCMFBearShort) Name() string           { return "1h_WR_CMF_Bear_Short" }
func (s *OneHWRCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHWRCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	wr1h := WilliamsR(ctx.Candles1h, 14)
	wr1hPrev := WilliamsR(ctx.Candles1h[:n1h-1], 14)
	if wr1hPrev <= -50 || wr1h >= -50 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= -0.05 {
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
		Reason: fmt.Sprintf("1h WR cross↓-50(%.1f) + CMF<-0.05(%.3f). SL=%.2f%%", wr1h, cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW180: 1h RSI CMF Bear Short ──────────────────────────────────────────────
type OneHRSICMFBearShort struct{}

func (s *OneHRSICMFBearShort) Name() string           { return "1h_RSI_CMF_Bear_Short" }
func (s *OneHRSICMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHRSICMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	rsi1hPrev := RSI(ctx.Candles1h[:n1h-1], 14)
	// 1h RSI crosses below 50 from above
	if rsi1hPrev <= 50 || rsi1h >= 50 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= -0.05 {
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
		Reason: fmt.Sprintf("1h RSI cross↓50(%.1f) + CMF<-0.05(%.3f). SL=%.2f%%", rsi1h, cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW181: 1h EFI CMF Bear Short ──────────────────────────────────────────────
type OneHEFICMFBearShort struct{}

func (s *OneHEFICMFBearShort) Name() string           { return "1h_EFI_CMF_Bear_Short" }
func (s *OneHEFICMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHEFICMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	efi1h := ElderForceIndex(ctx.Candles1h, 13)
	efi1hPrev := ElderForceIndex(ctx.Candles1h[:n1h-1], 13)
	// 1h EFI crosses below 0
	if efi1hPrev <= 0 || efi1h >= 0 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h EFI cross↓0(%.0f) + CMF<-0.05(%.3f). SL=%.2f%%", efi1h, cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW182: 1h DEMA Bear CMF Short ────────────────────────────────────────────
type OneHDEMACMFBearShort struct{}

func (s *OneHDEMACMFBearShort) Name() string           { return "1h_DEMA_CMF_Bear_Short" }
func (s *OneHDEMACMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHDEMACMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 35 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	dema8 := DEMA(ctx.Candles1h, 8)
	dema21 := DEMA(ctx.Candles1h, 21)
	dema8Prev := DEMA(ctx.Candles1h[:n1h-1], 8)
	dema21Prev := DEMA(ctx.Candles1h[:n1h-1], 21)
	if dema8Prev <= dema21Prev || dema8 >= dema21 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h DEMA8 cross↓DEMA21 + CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW183: 1h BB Mid CMF Bear Short ──────────────────────────────────────────
type OneHBBMidCMFBearShort struct{}

func (s *OneHBBMidCMFBearShort) Name() string           { return "1h_BB_Mid_CMF_Bear_Short" }
func (s *OneHBBMidCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHBBMidCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 25 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	bb := BB(ctx.Candles1h, 20)
	bbPrev := BB(ctx.Candles1h[:n1h-1], 20)
	cur1h := ctx.Candles1h[n1h-1]
	prev1h := ctx.Candles1h[n1h-2]
	// Price crosses below 1h BB mid
	if prev1h.Close <= bbPrev.Middle || cur1h.Close >= bb.Middle {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h close cross↓BB20 mid + CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW184: 1h KST Bear CMF Short ─────────────────────────────────────────────
type OneHKSTCMFBearShort struct{}

func (s *OneHKSTCMFBearShort) Name() string           { return "1h_KST_CMF_Bear_Short" }
func (s *OneHKSTCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHKSTCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 65 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	kst1h := KST(ctx.Candles1h)
	kst1hPrev := KST(ctx.Candles1h[:n1h-1])
	// 1h KST crosses below Signal
	if kst1hPrev.KST <= kst1hPrev.Signal || kst1h.KST >= kst1h.Signal {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= -0.05 {
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
		Reason: fmt.Sprintf("1h KST cross↓Signal + CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW185: 1h StochRSI CMF Bear Short ────────────────────────────────────────
type OneHStochRSICMFBearShort struct{}

func (s *OneHStochRSICMFBearShort) Name() string           { return "1h_StochRSI_CMF_Bear_Short" }
func (s *OneHStochRSICMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHStochRSICMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 40 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	k1h, _ := StochRSI(ctx.Candles1h, 14, 14, 3, 3)
	k1hPrev, _ := StochRSI(ctx.Candles1h[:n1h-1], 14, 14, 3, 3)
	// 1h StochRSI K crosses below 50 from overbought (>62)
	if k1hPrev <= 62 || k1h >= 50 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= -0.05 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h StochRSI K cross↓50(%.1f) + CMF<-0.05(%.3f). SL=%.2f%%", k1h, cmf1h, slDist/ctx.Price*100),
	}
}

func BuildHWV10Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &WMA5WMA13ADXBearShort{}, Name: "WMA5_WMA13_ADX_Bear_Short", Description: "4h WMA5 cross↓WMA13 + ADX>25+MACD-+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA5WMA13EFIBearShort{}, Name: "WMA5_WMA13_EFI_Bear_Short", Description: "4h WMA5 cross↓WMA13 + EFI<0+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA13WMA34ADXBearShort{}, Name: "WMA13_WMA34_ADX_Bear_Short", Description: "4h WMA13 cross↓WMA34 + ADX>25+MACD-+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA13WMA34EFIBearShort{}, Name: "WMA13_WMA34_EFI_Bear_Short", Description: "4h WMA13 cross↓WMA34 + EFI<0+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA13WMA34CMFDeepBearShort{}, Name: "WMA13_WMA34_CMF_Deep_Bear_Short", Description: "4h WMA13 cross↓WMA34 + CMF<-0.10+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ZLEMAADXBearShort{}, Name: "ZLEMA_ADX_Bear_Short", Description: "4h ZLEMA9 cross↓ZLEMA21 + ADX>25+MACD-+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ZLEMAFisherBearShort{}, Name: "ZLEMA_Fisher_Bear_Short", Description: "4h ZLEMA9 cross↓ZLEMA21 + Fisher<0+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DEMAADXStrongBearShort{}, Name: "DEMA_ADX_Strong_Bear_Short", Description: "4h DEMA8 cross↓DEMA21 + ADX>25+MACD-+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DEMAEFIBearShort{}, Name: "DEMA_EFI_Bear_Short", Description: "4h DEMA8 cross↓DEMA21 + EFI<0+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHWMA9WMA21ADXBearShort{}, Name: "1h_WMA9_WMA21_ADX_Bear_Short", Description: "1h WMA9 cross↓WMA21 + 1h ADX>20+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHZLEMACMFBearShort{}, Name: "1h_ZLEMA_CMF_Bear_Short", Description: "1h ZLEMA9 cross↓ZLEMA21 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHFisherCMFConfirmShort{}, Name: "1h_Fisher_CMF_Confirm_Short", Description: "1h Fisher cross↓0 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHWMA5WMA13BearShort{}, Name: "1h_WMA5_WMA13_Bear_Short", Description: "1h WMA5 cross↓WMA13 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHWRCMFBearShort{}, Name: "1h_WR_CMF_Bear_Short", Description: "1h WR cross↓-50 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHRSICMFBearShort{}, Name: "1h_RSI_CMF_Bear_Short", Description: "1h RSI cross↓50 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHEFICMFBearShort{}, Name: "1h_EFI_CMF_Bear_Short", Description: "1h EFI cross↓0 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHDEMACMFBearShort{}, Name: "1h_DEMA_CMF_Bear_Short", Description: "1h DEMA8 cross↓DEMA21 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHBBMidCMFBearShort{}, Name: "1h_BB_Mid_CMF_Bear_Short", Description: "1h close cross↓BB20 mid + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHKSTCMFBearShort{}, Name: "1h_KST_CMF_Bear_Short", Description: "1h KST cross↓Signal + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHStochRSICMFBearShort{}, Name: "1h_StochRSI_CMF_Bear_Short", Description: "1h StochRSI K cross↓50 from >62 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
