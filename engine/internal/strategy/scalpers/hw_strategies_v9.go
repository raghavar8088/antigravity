package scalpers

import "fmt"

// ─── High-Win-Rate Strategy Family v9 (HW146–HW165) ──────────────────────────
// 20 SHORT strategies: WMA period variants (5/13, 13/34) and ATR+indicator
// combinations that historically show PF>2.5 in bear regimes.
// ─────────────────────────────────────────────────────────────────────────────

// ── HW146: WMA5/WMA13 Bear Cross Short ───────────────────────────────────────
type WMA5WMA13BearCrossShort struct{}

func (s *WMA5WMA13BearCrossShort) Name() string           { return "WMA5_WMA13_Bear_Cross_Short" }
func (s *WMA5WMA13BearCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA5WMA13BearCrossShort) Evaluate(ctx MarketContext) Signal {
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
	if ElderForceIndex(ctx.Candles4h, 13) >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h WMA5(%.0f) cross↓WMA13(%.0f), EMA down. SL=%.2f%%", wma5, wma13, slDist/ctx.Price*100),
	}
}

// ── HW147: WMA13/WMA34 Bear Cross Short ──────────────────────────────────────
type WMA13WMA34BearCrossShort struct{}

func (s *WMA13WMA34BearCrossShort) Name() string           { return "WMA13_WMA34_Bear_Cross_Short" }
func (s *WMA13WMA34BearCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA13WMA34BearCrossShort) Evaluate(ctx MarketContext) Signal {
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
		Reason: fmt.Sprintf("4h WMA13(%.0f) cross↓WMA34(%.0f), EMA down. SL=%.2f%%", wma13, wma34, slDist/ctx.Price*100),
	}
}

// ── HW148: ATR Expand EFI Bear Short ─────────────────────────────────────────
// ATR expanding (volatility spike) + EFI crosses below 0 (selling force) → panic drop
type ATRExpandEFIBearShort struct{}

func (s *ATRExpandEFIBearShort) Name() string           { return "ATR_Expand_EFI_Bear_Short" }
func (s *ATRExpandEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ATRExpandEFIBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	atr4h := ATR(ctx.Candles4h, 14)
	atr4hPrev := ATR(ctx.Candles4h[:n4h-1], 14)
	atr4hAvg := ATR(ctx.Candles4h[:n4h-5], 14)
	// ATR expanding: current bar ATR > avg by 20% AND rising
	if atr4h <= atr4hPrev || atr4h < atr4hAvg*1.20 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	efiPrev := ElderForceIndex(ctx.Candles4h[:n4h-1], 13)
	// EFI crosses below 0 (selling force activating)
	if efiPrev <= 0 || efi >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("ATR expanding(%.0f>avg%.0f) + EFI cross↓0. SL=%.2f%%", atr4h, atr4hAvg, slDist/ctx.Price*100),
	}
}

// ── HW149: ATR Expand Fisher Bear Short ──────────────────────────────────────
type ATRExpandFisherBearShort struct{}

func (s *ATRExpandFisherBearShort) Name() string           { return "ATR_Expand_Fisher_Bear_Short" }
func (s *ATRExpandFisherBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ATRExpandFisherBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	atr4h := ATR(ctx.Candles4h, 14)
	atr4hAvg := ATR(ctx.Candles4h[:n4h-5], 14)
	// ATR must be expanding beyond average
	if atr4h < atr4hAvg*1.15 {
		return NoSignal(name)
	}
	fisher := FisherTransform(ctx.Candles4h, 10)
	fisherPrev := FisherTransform(ctx.Candles4h[:n4h-1], 10)
	// Fisher crosses below 0 confirming bearish momentum
	if fisherPrev <= 0 || fisher >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("ATR expanding(%.0f) + Fisher cross↓0(%.2f). SL=%.2f%%", atr4h, fisher, slDist/ctx.Price*100),
	}
}

// ── HW150: ATR Expand WilliamsR Bear Short ───────────────────────────────────
type ATRExpandWRBearShort struct{}

func (s *ATRExpandWRBearShort) Name() string           { return "ATR_Expand_WR_Bear_Short" }
func (s *ATRExpandWRBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ATRExpandWRBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	atr4h := ATR(ctx.Candles4h, 14)
	atr4hAvg := ATR(ctx.Candles4h[:n4h-5], 14)
	if atr4h < atr4hAvg*1.15 {
		return NoSignal(name)
	}
	wr := WilliamsR(ctx.Candles4h, 14)
	wrPrev := WilliamsR(ctx.Candles4h[:n4h-1], 14)
	// WilliamsR crosses below -50 (entering bearish territory) during volatility spike
	if wrPrev <= -50 || wr >= -50 {
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
		Reason: fmt.Sprintf("ATR expanding(%.0f) + WR cross↓-50(%.1f). SL=%.2f%%", atr4h, wr, slDist/ctx.Price*100),
	}
}

// ── HW151: WMA9 CMF Confirm Bear Short ───────────────────────────────────────
// WMA9/WMA21 cross with deeper CMF filter (already proven PF=3.16 in v8, this variant uses CMF<-0.10)
type WMA9CMFDeepBearShort struct{}

func (s *WMA9CMFDeepBearShort) Name() string           { return "WMA9_CMF_Deep_Bear_Short" }
func (s *WMA9CMFDeepBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA9CMFDeepBearShort) Evaluate(ctx MarketContext) Signal {
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
	// Deeper CMF filter than WMA_CMF_Bear_Short (v8) — must be below -0.10
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= -0.10 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.88,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9 cross↓WMA21 + CMF<-0.10(%.3f). SL=%.2f%%", ChaikinMoneyFlow(ctx.Candles4h, 20), slDist/ctx.Price*100),
	}
}

// ── HW152: WMA5 CMF Bear Short ────────────────────────────────────────────────
type WMA5CMFBearShort struct{}

func (s *WMA5CMFBearShort) Name() string           { return "WMA5_CMF_Bear_Short" }
func (s *WMA5CMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA5CMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.05 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA5 cross↓WMA13 + CMF<-0.05(%.3f)+ADX>18. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW153: WMA13 CMF Bear Short ───────────────────────────────────────────────
type WMA13CMFBearShort struct{}

func (s *WMA13CMFBearShort) Name() string           { return "WMA13_CMF_Bear_Short" }
func (s *WMA13CMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA13CMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	if cmf >= -0.05 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
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
		Reason: fmt.Sprintf("WMA13 cross↓WMA34 + CMF<-0.05(%.3f)+ADX>18. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW154: WMA EFI Bear Short ─────────────────────────────────────────────────
// WMA9/WMA21 cross + EFI negative → selling momentum confirmed by force index
type WMAEFIBearShort struct{}

func (s *WMAEFIBearShort) Name() string           { return "WMA_EFI_Bear_Short" }
func (s *WMAEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMAEFIBearShort) Evaluate(ctx MarketContext) Signal {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9 cross↓WMA21 + EFI<0(%.0f). SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW155: WMA Fisher Bear Short ─────────────────────────────────────────────
type WMAFisherBearShort struct{}

func (s *WMAFisherBearShort) Name() string           { return "WMA_Fisher_Bear_Short" }
func (s *WMAFisherBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMAFisherBearShort) Evaluate(ctx MarketContext) Signal {
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
	fisher := FisherTransform(ctx.Candles4h, 10)
	if fisher >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9 cross↓WMA21 + Fisher<0(%.2f). SL=%.2f%%", fisher, slDist/ctx.Price*100),
	}
}

// ── HW156: WMA RSI Bear Short ────────────────────────────────────────────────
// WMA9/WMA21 bear cross + RSI4h falling below 50
type WMARSIBearShort struct{}

func (s *WMARSIBearShort) Name() string           { return "WMA_RSI_Bear_Short" }
func (s *WMARSIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMARSIBearShort) Evaluate(ctx MarketContext) Signal {
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
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	// RSI4h crosses below 50
	if rsi4hPrev <= 50 || rsi4h >= 50 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9 cross↓WMA21 + RSI4h cross↓50(%.1f). SL=%.2f%%", rsi4h, slDist/ctx.Price*100),
	}
}

// ── HW157: WMA ADX Strong Bear Short ─────────────────────────────────────────
type WMAADXStrongBearShort struct{}

func (s *WMAADXStrongBearShort) Name() string           { return "WMA_ADX_Strong_Bear_Short" }
func (s *WMAADXStrongBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMAADXStrongBearShort) Evaluate(ctx MarketContext) Signal {
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
	adx := ADX(ctx.Candles4h, 14)
	if adx < 25 {
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
		Reason: fmt.Sprintf("WMA9 cross↓WMA21 + ADX>25(%.1f). SL=%.2f%%", adx, slDist/ctx.Price*100),
	}
}

// ── HW158: WMA KST Bear Short ────────────────────────────────────────────────
type WMAKSTBearShort struct{}

func (s *WMAKSTBearShort) Name() string           { return "WMA_KST_Bear_Short" }
func (s *WMAKSTBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMAKSTBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	wma9 := WMA(ctx.Candles4h, 9)
	wma21 := WMA(ctx.Candles4h, 21)
	wma9Prev := WMA(ctx.Candles4h[:n4h-1], 9)
	wma21Prev := WMA(ctx.Candles4h[:n4h-1], 21)
	if wma9Prev <= wma21Prev || wma9 >= wma21 {
		return NoSignal(name)
	}
	kst := KST(ctx.Candles4h)
	if kst.KST >= kst.Signal {
		return NoSignal(name)
	}
	if kst.KST >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9 cross↓WMA21 + KST<Signal(%.1f<%.1f). SL=%.2f%%", kst.KST, kst.Signal, slDist/ctx.Price*100),
	}
}

// ── HW159: ZLEMA CMF Bear Short ──────────────────────────────────────────────
type ZLEMACMFBearShort struct{}

func (s *ZLEMACMFBearShort) Name() string           { return "ZLEMA_CMF_Bear_Short" }
func (s *ZLEMACMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ZLEMACMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("ZLEMA9 cross↓ZLEMA21 + CMF<-0.05(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW160: DEMA CMF Bear Short ───────────────────────────────────────────────
type DEMACMFBearShort struct{}

func (s *DEMACMFBearShort) Name() string           { return "DEMA_CMF_Bear_Short" }
func (s *DEMACMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DEMACMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("DEMA8 cross↓DEMA21 + CMF<-0.05(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW161: Fisher WR Bear Short ──────────────────────────────────────────────
// Fisher crosses below 0 + WR crosses below -50 simultaneously (dual momentum)
type FisherWRBearShort struct{}

func (s *FisherWRBearShort) Name() string           { return "Fisher_WR_Bear_Short" }
func (s *FisherWRBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *FisherWRBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	fisher := FisherTransform(ctx.Candles4h, 10)
	fisherPrev := FisherTransform(ctx.Candles4h[:n4h-1], 10)
	if fisherPrev <= 0 || fisher >= 0 {
		return NoSignal(name)
	}
	wr := WilliamsR(ctx.Candles4h, 14)
	if wr >= -40 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Fisher cross↓0(%.2f) + WR<-40(%.1f). SL=%.2f%%", fisher, wr, slDist/ctx.Price*100),
	}
}

// ── HW162: Coppock CMF Bear Short ────────────────────────────────────────────
type CoppockCMFBearShort struct{}

func (s *CoppockCMFBearShort) Name() string           { return "Coppock_CMF_Bear_Short" }
func (s *CoppockCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CoppockCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	cop := CoppockCurve(ctx.Candles4h, 14, 11, 10)
	copPrev := CoppockCurve(ctx.Candles4h[:n4h-1], 14, 11, 10)
	if copPrev <= 0 || cop >= 0 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Coppock cross↓0(%.2f→%.2f) + CMF<-0.05(%.3f). SL=%.2f%%", copPrev, cop, cmf, slDist/ctx.Price*100),
	}
}

// ── HW163: EMA8 EMA50 CMF Bear Short ─────────────────────────────────────────
type EMA8EMA50CMFBearShort struct{}

func (s *EMA8EMA50CMFBearShort) Name() string           { return "EMA8_EMA50_CMF_Bear_Short" }
func (s *EMA8EMA50CMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA8EMA50CMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	if ema8Prev <= ema50Prev || ema8 >= ema50 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA8 cross↓EMA50 + CMF<-0.05(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW164: 1h WMA Bear CMF Short ─────────────────────────────────────────────
// 1h WMA9/WMA21 bear cross + 1h CMF < -0.05 (intraday confirmation)
type OneHWMABearCMFShort struct{}

func (s *OneHWMABearCMFShort) Name() string           { return "1h_WMA_Bear_CMF_Short" }
func (s *OneHWMABearCMFShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHWMABearCMFShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 35 {
		return NoSignal(name)
	}
	// 4h macro filter
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	// 1h WMA9/WMA21 crossover
	wma9 := WMA(ctx.Candles1h, 9)
	wma21 := WMA(ctx.Candles1h, 21)
	wma9Prev := WMA(ctx.Candles1h[:n1h-1], 9)
	wma21Prev := WMA(ctx.Candles1h[:n1h-1], 21)
	if wma9Prev <= wma21Prev || wma9 >= wma21 {
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
		Reason: fmt.Sprintf("1h WMA9 cross↓WMA21 + 1h CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW165: 1h WMA EFI Bear Short ─────────────────────────────────────────────
type OneHWMAEFIBearShort struct{}

func (s *OneHWMAEFIBearShort) Name() string           { return "1h_WMA_EFI_Bear_Short" }
func (s *OneHWMAEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHWMAEFIBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 35 {
		return NoSignal(name)
	}
	// 4h macro filter
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	// 1h WMA9/WMA21 crossover
	wma9 := WMA(ctx.Candles1h, 9)
	wma21 := WMA(ctx.Candles1h, 21)
	wma9Prev := WMA(ctx.Candles1h[:n1h-1], 9)
	wma21Prev := WMA(ctx.Candles1h[:n1h-1], 21)
	if wma9Prev <= wma21Prev || wma9 >= wma21 {
		return NoSignal(name)
	}
	// 1h EFI must be negative (selling force on 1h)
	efi1h := ElderForceIndex(ctx.Candles1h, 13)
	if efi1h >= 0 {
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
		Reason: fmt.Sprintf("1h WMA9 cross↓WMA21 + 1h EFI<0(%.0f). SL=%.2f%%", efi1h, slDist/ctx.Price*100),
	}
}

func BuildHWV9Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &WMA5WMA13BearCrossShort{}, Name: "WMA5_WMA13_Bear_Cross_Short", Description: "4h WMA5 cross↓WMA13+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA13WMA34BearCrossShort{}, Name: "WMA13_WMA34_Bear_Cross_Short", Description: "4h WMA13 cross↓WMA34+EMA down+MACD-+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ATRExpandEFIBearShort{}, Name: "ATR_Expand_EFI_Bear_Short", Description: "4h ATR expanding>1.20×avg + EFI cross↓0 + EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ATRExpandFisherBearShort{}, Name: "ATR_Expand_Fisher_Bear_Short", Description: "4h ATR expanding>1.15×avg + Fisher cross↓0 + EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ATRExpandWRBearShort{}, Name: "ATR_Expand_WR_Bear_Short", Description: "4h ATR expanding>1.15×avg + WR cross↓-50 + EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA9CMFDeepBearShort{}, Name: "WMA9_CMF_Deep_Bear_Short", Description: "4h WMA9 cross↓WMA21 + CMF<-0.10 (deep selling)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA5CMFBearShort{}, Name: "WMA5_CMF_Bear_Short", Description: "4h WMA5 cross↓WMA13 + CMF<-0.05+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA13CMFBearShort{}, Name: "WMA13_CMF_Bear_Short", Description: "4h WMA13 cross↓WMA34 + CMF<-0.05+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMAEFIBearShort{}, Name: "WMA_EFI_Bear_Short", Description: "4h WMA9 cross↓WMA21 + EFI<0+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMAFisherBearShort{}, Name: "WMA_Fisher_Bear_Short", Description: "4h WMA9 cross↓WMA21 + Fisher<0+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMARSIBearShort{}, Name: "WMA_RSI_Bear_Short", Description: "4h WMA9 cross↓WMA21 + RSI4h cross↓50", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMAADXStrongBearShort{}, Name: "WMA_ADX_Strong_Bear_Short", Description: "4h WMA9 cross↓WMA21 + ADX>25+MACD-+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMAKSTBearShort{}, Name: "WMA_KST_Bear_Short", Description: "4h WMA9 cross↓WMA21 + KST<Signal+KST<0", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ZLEMACMFBearShort{}, Name: "ZLEMA_CMF_Bear_Short", Description: "4h ZLEMA9 cross↓ZLEMA21 + CMF<-0.05+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DEMACMFBearShort{}, Name: "DEMA_CMF_Bear_Short", Description: "4h DEMA8 cross↓DEMA21 + CMF<-0.05+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FisherWRBearShort{}, Name: "Fisher_WR_Bear_Short", Description: "4h Fisher cross↓0 + WR<-40+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CoppockCMFBearShort{}, Name: "Coppock_CMF_Bear_Short", Description: "4h Coppock cross↓0 + CMF<-0.05+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EMA8EMA50CMFBearShort{}, Name: "EMA8_EMA50_CMF_Bear_Short", Description: "4h EMA8 cross↓EMA50 + CMF<-0.05+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHWMABearCMFShort{}, Name: "1h_WMA_Bear_CMF_Short", Description: "1h WMA9 cross↓WMA21 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHWMAEFIBearShort{}, Name: "1h_WMA_EFI_Bear_Short", Description: "1h WMA9 cross↓WMA21 + 1h EFI<0+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
