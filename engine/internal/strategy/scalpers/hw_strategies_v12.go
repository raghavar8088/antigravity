package scalpers

import "fmt"

// ─── High-Win-Rate Strategy Family v12 (HW206–HW230) ─────────────────────────
// 25 SHORT strategies: New proven combos — Aroon+CMF/EFI, Donchian+ADX/EFI,
// KST+ADX, multi-indicator triples, and 1h-based signals with ADX/EFI.
// ─────────────────────────────────────────────────────────────────────────────

// ── HW206: Aroon CMF Bear Short ───────────────────────────────────────────────
// Aroon Down>70 + CMF<-0.05 (directional + volume confirmation)
type AroonCMFBearVShort struct{}

func (s *AroonCMFBearVShort) Name() string           { return "Aroon_CMF_Bear_V_Short" }
func (s *AroonCMFBearVShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *AroonCMFBearVShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	aroon := Aroon(ctx.Candles4h, 25)
	if aroon.Down < 70 || aroon.Up > 30 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Aroon down>70(%.0f) + CMF<-0.05(%.3f). SL=%.2f%%", aroon.Down, cmf, slDist/ctx.Price*100),
	}
}

// ── HW207: Aroon EFI Bear Short ───────────────────────────────────────────────
type AroonEFIBearShort struct{}

func (s *AroonEFIBearShort) Name() string           { return "Aroon_EFI_Bear_Short" }
func (s *AroonEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *AroonEFIBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	aroon := Aroon(ctx.Candles4h, 25)
	if aroon.Down < 65 || aroon.Up > 35 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Aroon down>65(%.0f) + EFI<0(%.0f). SL=%.2f%%", aroon.Down, efi, slDist/ctx.Price*100),
	}
}

// ── HW208: Donchian EFI Bear Short ────────────────────────────────────────────
// Donchian mid break + EFI<0 + ADX>15
type DonchianEFIBearShort struct{}

func (s *DonchianEFIBearShort) Name() string           { return "Donchian_EFI_Bear_Short" }
func (s *DonchianEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DonchianEFIBearShort) Evaluate(ctx MarketContext) Signal {
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
	if prev.Close <= donchPrev.Mid || cur.Close >= donch.Mid {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Donchian mid break + EFI<0(%.0f). SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW209: BB Mid EFI Bear Short ──────────────────────────────────────────────
type BBMidEFIBearShort struct{}

func (s *BBMidEFIBearShort) Name() string           { return "BB_Mid_EFI_Bear_Short" }
func (s *BBMidEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BBMidEFIBearShort) Evaluate(ctx MarketContext) Signal {
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
	if prev.Close <= bbPrev.Middle || cur.Close >= bb.Middle {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h close cross↓BB mid + EFI<0(%.0f). SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW210: Keltner Mid EFI Bear Short ─────────────────────────────────────────
type KeltnerMidEFIBearShort struct{}

func (s *KeltnerMidEFIBearShort) Name() string           { return "Keltner_Mid_EFI_Bear_Short" }
func (s *KeltnerMidEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *KeltnerMidEFIBearShort) Evaluate(ctx MarketContext) Signal {
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
	if prev.Close <= kcPrev.Mid || cur.Close >= kc.Mid {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h close cross↓Keltner mid + EFI<0(%.0f). SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW211: KST ADX Bear Short ─────────────────────────────────────────────────
// KST signal cross below + ADX>20 + CMF<0
type KSTADXBearShort struct{}

func (s *KSTADXBearShort) Name() string           { return "KST_ADX_Bear_Short" }
func (s *KSTADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *KSTADXBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	kst := KST(ctx.Candles4h)
	kstPrev := KST(ctx.Candles4h[:n4h-1])
	if kstPrev.KST <= kstPrev.Signal || kst.KST >= kst.Signal {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 15 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("KST cross↓Signal(%.1f) + ADX>20 + CMF<0. SL=%.2f%%", kst.KST, slDist/ctx.Price*100),
	}
}

// ── HW212: KST EFI Bear Short ─────────────────────────────────────────────────
type KSTEFIBearVShort struct{}

func (s *KSTEFIBearVShort) Name() string           { return "KST_EFI_Bear_V_Short" }
func (s *KSTEFIBearVShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *KSTEFIBearVShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	kst := KST(ctx.Candles4h)
	kstPrev := KST(ctx.Candles4h[:n4h-1])
	if kstPrev.KST <= kstPrev.Signal || kst.KST >= kst.Signal {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("KST cross↓Signal + EFI<0(%.0f). SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW213: Coppock ADX Bear Short ────────────────────────────────────────────
type CoppockADXBearShort struct{}

func (s *CoppockADXBearShort) Name() string           { return "Coppock_ADX_Bear_Short" }
func (s *CoppockADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CoppockADXBearShort) Evaluate(ctx MarketContext) Signal {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Coppock cross↓0(%.2f) + ADX>20. SL=%.2f%%", cop, slDist/ctx.Price*100),
	}
}

// ── HW214: Coppock EFI Bear Short ─────────────────────────────────────────────
type CoppockEFIBearShort struct{}

func (s *CoppockEFIBearShort) Name() string           { return "Coppock_EFI_Bear_Short" }
func (s *CoppockEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CoppockEFIBearShort) Evaluate(ctx MarketContext) Signal {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Coppock cross↓0(%.2f) + EFI<0(%.0f). SL=%.2f%%", cop, efi, slDist/ctx.Price*100),
	}
}

// ── HW215: Lower High CMF Bear Short ──────────────────────────────────────────
// 3 lower highs + CMF<-0.05 (price structure + volume)
type LowerHighCMFBearShort struct{}

func (s *LowerHighCMFBearShort) Name() string           { return "Lower_High_CMF_Bear_Short" }
func (s *LowerHighCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *LowerHighCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 6 || n1h < 22 {
		return NoSignal(name)
	}
	c := ctx.Candles4h
	if !(c[n4h-1].High < c[n4h-2].High && c[n4h-2].High < c[n4h-3].High) {
		return NoSignal(name)
	}
	if c[n4h-1].Close > c[n4h-1].Open {
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
		Reason: fmt.Sprintf("3 lower highs + CMF<-0.05(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW216: Big Bear Candle ADX Short ──────────────────────────────────────────
// Big bearish candle (body>1.5×ATR) + ADX>25 (momentum surge)
type BigBearCandleADXShort struct{}

func (s *BigBearCandleADXShort) Name() string           { return "Big_Bear_Candle_ADX_Short" }
func (s *BigBearCandleADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BigBearCandleADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 20 || n1h < 22 {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	if cur.Close > cur.Open {
		return NoSignal(name)
	}
	atr4h := ATR(ctx.Candles4h, 14)
	body := cur.Open - cur.Close
	if body < 1.5*atr4h {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Big bear body(%.0f)>1.5×ATR + ADX>25(%.1f). SL=%.2f%%", body, ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW217: Three Bear Candles ADX Short ───────────────────────────────────────
type ThreeBearCandlesADXShort struct{}

func (s *ThreeBearCandlesADXShort) Name() string           { return "Three_Bear_Candles_ADX_Short" }
func (s *ThreeBearCandlesADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ThreeBearCandlesADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 6 || n1h < 22 {
		return NoSignal(name)
	}
	c := ctx.Candles4h
	if !(c[n4h-1].Close < c[n4h-1].Open &&
		c[n4h-2].Close < c[n4h-2].Open &&
		c[n4h-3].Close < c[n4h-3].Open &&
		c[n4h-1].Close < c[n4h-2].Close &&
		c[n4h-2].Close < c[n4h-3].Close) {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("3 consecutive bear candles + ADX>25(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW218: 1h MACD Zero Cross EFI Short ──────────────────────────────────────
// 1h MACD line crosses below 0 + 1h EFI<0 + 4h EMA down
type OneHMACDZeroEFIShort struct{}

func (s *OneHMACDZeroEFIShort) Name() string           { return "1h_MACD_Zero_EFI_Short" }
func (s *OneHMACDZeroEFIShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHMACDZeroEFIShort) Evaluate(ctx MarketContext) Signal {
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
	macd1h := MACD(ctx.Candles1h)
	macd1hPrev := MACD(ctx.Candles1h[:n1h-1])
	if macd1hPrev.MACD <= 0 || macd1h.MACD >= 0 {
		return NoSignal(name)
	}
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h MACD line cross↓0 + 1h EFI<0. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW219: 1h RSI 45 Break CMF Short ─────────────────────────────────────────
// 1h RSI crosses below 45 (deeper bearish) + 1h CMF<0 + 4h bear
type OneHRSI45BreakCMFShort struct{}

func (s *OneHRSI45BreakCMFShort) Name() string           { return "1h_RSI45_Break_CMF_Short" }
func (s *OneHRSI45BreakCMFShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHRSI45BreakCMFShort) Evaluate(ctx MarketContext) Signal {
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
	if rsi1hPrev <= 45 || rsi1h >= 45 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= 0 {
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
		Reason: fmt.Sprintf("1h RSI cross↓45(%.1f) + CMF<0(%.3f). SL=%.2f%%", rsi1h, cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW220: 1h Fisher EFI Bear Short ──────────────────────────────────────────
type OneHFisherEFIBearShort struct{}

func (s *OneHFisherEFIBearShort) Name() string           { return "1h_Fisher_EFI_Bear_Short" }
func (s *OneHFisherEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHFisherEFIBearShort) Evaluate(ctx MarketContext) Signal {
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
	efi1h := ElderForceIndex(ctx.Candles1h, 13)
	if efi1h >= 0 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	if cmf1h >= 0 {
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
		Reason: fmt.Sprintf("1h Fisher cross↓0(%.2f) + 1h EFI<0 + CMF<0. SL=%.2f%%", fisher1h, slDist/ctx.Price*100),
	}
}

// ── HW221: 1h WMA13 CMF Bear Short ───────────────────────────────────────────
type OneHWMA13CMFBearShort struct{}

func (s *OneHWMA13CMFBearShort) Name() string           { return "1h_WMA13_CMF_Bear_Short" }
func (s *OneHWMA13CMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHWMA13CMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 50 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	wma13 := WMA(ctx.Candles1h, 13)
	wma34 := WMA(ctx.Candles1h, 34)
	wma13Prev := WMA(ctx.Candles1h[:n1h-1], 13)
	wma34Prev := WMA(ctx.Candles1h[:n1h-1], 34)
	if wma13Prev <= wma34Prev || wma13 >= wma34 {
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
		Reason: fmt.Sprintf("1h WMA13↓WMA34 + CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW222: 1h Donchian CMF Bear Short ────────────────────────────────────────
type OneHDonchianCMFBearShort struct{}

func (s *OneHDonchianCMFBearShort) Name() string           { return "1h_Donchian_CMF_Bear_Short" }
func (s *OneHDonchianCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHDonchianCMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	donch1h := Donchian(ctx.Candles1h, 20)
	donch1hPrev := Donchian(ctx.Candles1h[:n1h-1], 20)
	cur1h := ctx.Candles1h[n1h-1]
	prev1h := ctx.Candles1h[n1h-2]
	if prev1h.Close <= donch1hPrev.Mid || cur1h.Close >= donch1h.Mid {
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
		Reason: fmt.Sprintf("1h close cross↓Donchian mid + CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW223: RSI4h 48 Break CMF Short ──────────────────────────────────────────
// 4h RSI crosses below 48 + CMF<-0.05 (momentum flip with distribution)
type RSI48BreakCMFShort struct{}

func (s *RSI48BreakCMFShort) Name() string           { return "RSI48_Break_CMF_Short" }
func (s *RSI48BreakCMFShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSI48BreakCMFShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev <= 48 || rsi4h >= 48 {
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
	if ADX(ctx.Candles4h, 14) < 12 {
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
		Reason: fmt.Sprintf("4h RSI cross↓48(%.1f) + CMF<-0.05(%.3f). SL=%.2f%%", rsi4h, cmf, slDist/ctx.Price*100),
	}
}

// ── HW224: RSI4h 48 Break EFI Short ──────────────────────────────────────────
type RSI48BreakEFIShort struct{}

func (s *RSI48BreakEFIShort) Name() string           { return "RSI48_Break_EFI_Short" }
func (s *RSI48BreakEFIShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSI48BreakEFIShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev <= 48 || rsi4h >= 48 {
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
	if ADX(ctx.Candles4h, 14) < 12 {
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
		Reason: fmt.Sprintf("4h RSI cross↓48(%.1f) + EFI<0(%.0f). SL=%.2f%%", rsi4h, efi, slDist/ctx.Price*100),
	}
}

// ── HW225: Two Bar Bear Reversal ADX Short ────────────────────────────────────
// Two-bar reversal (bull→big bear) + ADX>22
type TwoBarBearReversalADXShort struct{}

func (s *TwoBarBearReversalADXShort) Name() string           { return "Two_Bar_Bear_Reversal_ADX_Short" }
func (s *TwoBarBearReversalADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *TwoBarBearReversalADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 5 || n1h < 22 {
		return NoSignal(name)
	}
	c := ctx.Candles4h
	bull := c[n4h-2]
	bear := c[n4h-1]
	if bull.Close <= bull.Open {
		return NoSignal(name)
	}
	if bear.Close >= bear.Open {
		return NoSignal(name)
	}
	bullBody := bull.Close - bull.Open
	bearBody := bear.Open - bear.Close
	if bearBody <= bullBody || bear.Close >= bull.Open {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Two-bar bear reversal + ADX>22(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW226: Close Low Range ADX Short ─────────────────────────────────────────
// Close in bottom 20% + ADX>25 (bearish close in strong trend)
type CloseLowRangeADXShort struct{}

func (s *CloseLowRangeADXShort) Name() string           { return "Close_Low_Range_ADX_Short" }
func (s *CloseLowRangeADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CloseLowRangeADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	cur := ctx.Candles4h[n4h-1]
	rng := cur.High - cur.Low
	if rng == 0 {
		return NoSignal(name)
	}
	closePos := (cur.Close - cur.Low) / rng
	if closePos > 0.20 {
		return NoSignal(name)
	}
	if cur.Close >= cur.Open {
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
		Reason: fmt.Sprintf("Close in bottom %.0f%% of range + ADX>25. SL=%.2f%%", closePos*100, slDist/ctx.Price*100),
	}
}

// ── HW227: 1h MACD Hist Flip CMF Short ───────────────────────────────────────
type OneHMACDHistFlipCMFShort struct{}

func (s *OneHMACDHistFlipCMFShort) Name() string           { return "1h_MACD_Hist_Flip_CMF_Short" }
func (s *OneHMACDHistFlipCMFShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHMACDHistFlipCMFShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 30 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	macd1h := MACD(ctx.Candles1h)
	macd1hPrev := MACD(ctx.Candles1h[:n1h-1])
	if macd1hPrev.Histogram <= 0 || macd1h.Histogram >= 0 {
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
		Reason: fmt.Sprintf("1h MACD hist sign flip↓ + 1h CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW228: 1h BB Width Expand CMF Short ──────────────────────────────────────
// 1h BB width expanding (volatility surge) + bearish close + 1h CMF<-0.05
type OneHBBWidthCMFBearShort struct{}

func (s *OneHBBWidthCMFBearShort) Name() string           { return "1h_BB_Width_CMF_Bear_Short" }
func (s *OneHBBWidthCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHBBWidthCMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	bb1h := BB(ctx.Candles1h, 20)
	bb1hPrev := BB(ctx.Candles1h[:n1h-1], 20)
	cur1h := ctx.Candles1h[n1h-1]
	width := bb1h.Upper - bb1h.Lower
	widthPrev := bb1hPrev.Upper - bb1hPrev.Lower
	if width <= widthPrev*1.05 {
		return NoSignal(name)
	}
	if cur1h.Close >= cur1h.Open || cur1h.Close >= bb1h.Middle {
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
		Reason: fmt.Sprintf("1h BB expanding + bear close + CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW229: 1h Aroon CMF Bear Short ───────────────────────────────────────────
type OneHAroonCMFBearShort struct{}

func (s *OneHAroonCMFBearShort) Name() string           { return "1h_Aroon_CMF_Bear_Short" }
func (s *OneHAroonCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHAroonCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 30 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	aroon1h := Aroon(ctx.Candles1h, 25)
	if aroon1h.Down < 65 || aroon1h.Up > 35 {
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
		Reason: fmt.Sprintf("1h Aroon down>65(%.0f) + 1h CMF<-0.05(%.3f). SL=%.2f%%", aroon1h.Down, cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW230: 1h KST EFI Bear Short ─────────────────────────────────────────────
type OneHKSTEFIBearShort struct{}

func (s *OneHKSTEFIBearShort) Name() string           { return "1h_KST_EFI_Bear_Short" }
func (s *OneHKSTEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHKSTEFIBearShort) Evaluate(ctx MarketContext) Signal {
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
	if kst1hPrev.KST <= kst1hPrev.Signal || kst1h.KST >= kst1h.Signal {
		return NoSignal(name)
	}
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
		Reason: fmt.Sprintf("1h KST cross↓Signal + 1h EFI<0(%.0f). SL=%.2f%%", efi1h, slDist/ctx.Price*100),
	}
}

func BuildHWV12Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &AroonCMFBearVShort{}, Name: "Aroon_CMF_Bear_V_Short", Description: "4h Aroon down>70 + CMF<-0.05+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &AroonEFIBearShort{}, Name: "Aroon_EFI_Bear_Short", Description: "4h Aroon down>65 + EFI<0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DonchianEFIBearShort{}, Name: "Donchian_EFI_Bear_Short", Description: "4h Donchian mid break + EFI<0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBMidEFIBearShort{}, Name: "BB_Mid_EFI_Bear_Short", Description: "4h close cross↓BB mid + EFI<0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KeltnerMidEFIBearShort{}, Name: "Keltner_Mid_EFI_Bear_Short", Description: "4h close cross↓Keltner mid + EFI<0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KSTADXBearShort{}, Name: "KST_ADX_Bear_Short", Description: "4h KST cross↓Signal + ADX>20+CMF<0+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KSTEFIBearVShort{}, Name: "KST_EFI_Bear_V_Short", Description: "4h KST cross↓Signal + EFI<0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CoppockADXBearShort{}, Name: "Coppock_ADX_Bear_Short", Description: "4h Coppock cross↓0 + ADX>20+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CoppockEFIBearShort{}, Name: "Coppock_EFI_Bear_Short", Description: "4h Coppock cross↓0 + EFI<0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &LowerHighCMFBearShort{}, Name: "Lower_High_CMF_Bear_Short", Description: "3 lower highs + CMF<-0.05+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BigBearCandleADXShort{}, Name: "Big_Bear_Candle_ADX_Short", Description: "4h bearish body>1.5×ATR + ADX>25+MACD-+RSI4h<60", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ThreeBearCandlesADXShort{}, Name: "Three_Bear_Candles_ADX_Short", Description: "3 consecutive bear candles + ADX>25+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHMACDZeroEFIShort{}, Name: "1h_MACD_Zero_EFI_Short", Description: "1h MACD line cross↓0 + 1h EFI<0+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHRSI45BreakCMFShort{}, Name: "1h_RSI45_Break_CMF_Short", Description: "1h RSI cross↓45 + 1h CMF<0+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHFisherEFIBearShort{}, Name: "1h_Fisher_EFI_Bear_Short", Description: "1h Fisher cross↓0 + 1h EFI<0+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHWMA13CMFBearShort{}, Name: "1h_WMA13_CMF_Bear_Short", Description: "1h WMA13↓WMA34 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHDonchianCMFBearShort{}, Name: "1h_Donchian_CMF_Bear_Short", Description: "1h close cross↓Donchian mid + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &RSI48BreakCMFShort{}, Name: "RSI48_Break_CMF_Short", Description: "4h RSI cross↓48 + CMF<-0.05+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &RSI48BreakEFIShort{}, Name: "RSI48_Break_EFI_Short", Description: "4h RSI cross↓48 + EFI<0+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &TwoBarBearReversalADXShort{}, Name: "Two_Bar_Bear_Reversal_ADX_Short", Description: "Two-bar bear reversal + ADX>22+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CloseLowRangeADXShort{}, Name: "Close_Low_Range_ADX_Short", Description: "4h close in bottom 20% + ADX>25+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHMACDHistFlipCMFShort{}, Name: "1h_MACD_Hist_Flip_CMF_Short", Description: "1h MACD hist flip↓ + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHBBWidthCMFBearShort{}, Name: "1h_BB_Width_CMF_Bear_Short", Description: "1h BB width expanding + bear close + 1h CMF<-0.05", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHAroonCMFBearShort{}, Name: "1h_Aroon_CMF_Bear_Short", Description: "1h Aroon down>65 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHKSTEFIBearShort{}, Name: "1h_KST_EFI_Bear_Short", Description: "1h KST cross↓Signal + 1h EFI<0+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
