package scalpers

import "fmt"

// ─── High-Win-Rate Strategy Family v8 (HW126–HW145) ──────────────────────────
// 20 SHORT strategies using multi-indicator confluence and hybrid signals.
// ─────────────────────────────────────────────────────────────────────────────

// ── HW126: RSI MACD KST Triple Bear Short ────────────────────────────────────
type RSIMACDKSTBearShort struct{}

func (s *RSIMACDKSTBearShort) Name() string           { return "RSI_MACD_KST_Bear_Short" }
func (s *RSIMACDKSTBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSIMACDKSTBearShort) Evaluate(ctx MarketContext) Signal {
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
	// KST crosses below Signal + RSI<50 + MACD histogram<0
	if kstPrev.KST <= kstPrev.Signal || kst.KST >= kst.Signal {
		return NoSignal(name)
	}
	if RSI(ctx.Candles4h, 14) >= 50 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
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
		Reason: fmt.Sprintf("Triple bear: KST cross↓signal+RSI<50+MACD-, EMA down. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW127: CMF OBV Confluence Bear Short ─────────────────────────────────────
type CMFOBVConfluenceBearShort struct{}

func (s *CMFOBVConfluenceBearShort) Name() string           { return "CMF_OBV_Confluence_Bear_Short" }
func (s *CMFOBVConfluenceBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CMFOBVConfluenceBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	// CMF crosses below 0 AND OBV declining
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	cmfPrev := ChaikinMoneyFlow(ctx.Candles4h[:n4h-1], 20)
	if cmfPrev <= 0 || cmf >= 0 {
		return NoSignal(name)
	}
	obvSlice := OBVSlice(ctx.Candles4h)
	if len(obvSlice) < 4 {
		return NoSignal(name)
	}
	// OBV declining for 3 bars
	n := len(obvSlice)
	if obvSlice[n-1] >= obvSlice[n-2] || obvSlice[n-2] >= obvSlice[n-3] {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("CMF cross↓0(%.3f)+OBV declining 3 bars+ADX>20, EMA down. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW128: Fisher CMF Confirm Bear Short ─────────────────────────────────────
type FisherCMFConfirmBearShort struct{}

func (s *FisherCMFConfirmBearShort) Name() string           { return "Fisher_CMF_Confirm_Bear_Short" }
func (s *FisherCMFConfirmBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *FisherCMFConfirmBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	fish := FisherTransform(ctx.Candles4h, 14)
	fishPrev := FisherTransform(ctx.Candles4h[:n4h-1], 14)
	// Fisher crosses below 0
	if fishPrev <= 0 || fish >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Fisher cross↓0(%.2f)+CMF(%.3f)<-0.05, EMA down. SL=%.2f%%", fish, cmf, slDist/ctx.Price*100),
	}
}

// ── HW129: StochRSI CMF Bear Short ───────────────────────────────────────────
type StochRSICMFBearShort struct{}

func (s *StochRSICMFBearShort) Name() string           { return "StochRSI_CMF_Bear_Short" }
func (s *StochRSICMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *StochRSICMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 40 || n1h < 22 {
		return NoSignal(name)
	}
	k, d := StochRSI(ctx.Candles4h, 14, 14, 3, 3)
	kPrev, dPrev := StochRSI(ctx.Candles4h[:n4h-1], 14, 14, 3, 3)
	// StochRSI K crosses below D from >60 zone
	if kPrev <= dPrev || k >= d || kPrev < 60 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("StochRSI K(%.1f)<D from>60+CMF(%.3f)<0, EMA down. SL=%.2f%%", k, cmf, slDist/ctx.Price*100),
	}
}

// ── HW130: WR RSI Bear Confirm Short ─────────────────────────────────────────
type WRRSIBearConfirmShort struct{}

func (s *WRRSIBearConfirmShort) Name() string           { return "WR_RSI_Bear_Confirm_Short" }
func (s *WRRSIBearConfirmShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WRRSIBearConfirmShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	wr := WilliamsR(ctx.Candles4h, 14)
	wrPrev := WilliamsR(ctx.Candles4h[:n4h-1], 14)
	// WR crosses below -50 (midline cross)
	if wrPrev <= -50 || wr >= -50 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h >= 50 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WR cross↓-50(%.1f)+RSI(%.1f)<50, EMA down. SL=%.2f%%", wr, rsi4h, slDist/ctx.Price*100),
	}
}

// ── HW131: Aroon CMF Bear Dual Short ─────────────────────────────────────────
type AroonCMFBearDualShort struct{}

func (s *AroonCMFBearDualShort) Name() string           { return "Aroon_CMF_Bear_Dual_Short" }
func (s *AroonCMFBearDualShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *AroonCMFBearDualShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	ar := Aroon(ctx.Candles4h, 25)
	// Aroon: down dominant AND CMF negative
	if ar.Down <= 65 || ar.Up >= 35 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Aroon down(%.0f)>up(%.0f)+CMF(%.3f)<-0.05, EMA down. SL=%.2f%%", ar.Down, ar.Up, cmf, slDist/ctx.Price*100),
	}
}

// ── HW132: BB RSI Double Bear Short ──────────────────────────────────────────
type BBRSIDoubleBearShort struct{}

func (s *BBRSIDoubleBearShort) Name() string           { return "BB_RSI_Double_Bear_Short" }
func (s *BBRSIDoubleBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BBRSIDoubleBearShort) Evaluate(ctx MarketContext) Signal {
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
	// Close crosses below BB mid AND RSI crosses below 50
	if prev.Close <= bbPrev.Middle || cur.Close >= bb.Middle {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev <= 50 || rsi4h >= 50 {
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
		Reason: fmt.Sprintf("BB mid break+RSI cross↓50(%.1f), double bear confirm. SL=%.2f%%", rsi4h, slDist/ctx.Price*100),
	}
}

// ── HW133: KST EFI Bear Short ────────────────────────────────────────────────
type KSTEFIBearShort struct{}

func (s *KSTEFIBearShort) Name() string           { return "KST_EFI_Bear_Short" }
func (s *KSTEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *KSTEFIBearShort) Evaluate(ctx MarketContext) Signal {
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
	// KST crosses below Signal
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("KST cross↓signal+EFI(%.0f)<0, EMA down. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW134: HMA CMF Bear Short ────────────────────────────────────────────────
type HMACMFBearShort struct{}

func (s *HMACMFBearShort) Name() string           { return "HMA_CMF_Bear_Short" }
func (s *HMACMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *HMACMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	// HMA(9) turning down AND CMF < -0.05
	hma := HMA(ctx.Candles4h, 9)
	hmaPrev := HMA(ctx.Candles4h[:n4h-1], 9)
	if hma >= hmaPrev {
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
		Reason: fmt.Sprintf("HMA(9) declining+CMF(%.3f)<-0.05, EMA down. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW135: DEMA RSI Bear Short ────────────────────────────────────────────────
type DEMARSIBearShort struct{}

func (s *DEMARSIBearShort) Name() string           { return "DEMA_RSI_Bear_Short" }
func (s *DEMARSIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DEMARSIBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 50 || n1h < 22 {
		return NoSignal(name)
	}
	dema8 := DEMA(ctx.Candles4h, 8)
	dema21 := DEMA(ctx.Candles4h, 21)
	if dema8 >= dema21 {
		return NoSignal(name)
	}
	// RSI crosses below 50
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev <= 50 || rsi4h >= 50 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("DEMA8<DEMA21+RSI cross↓50(%.1f), EMA down. SL=%.2f%%", rsi4h, slDist/ctx.Price*100),
	}
}

// ── HW136: ZLEMA EFI Bear Short ──────────────────────────────────────────────
type ZLEMAEFIBearShort struct{}

func (s *ZLEMAEFIBearShort) Name() string           { return "ZLEMA_EFI_Bear_Short" }
func (s *ZLEMAEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ZLEMAEFIBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	z9 := ZLEMA(ctx.Candles4h, 9)
	z21 := ZLEMA(ctx.Candles4h, 21)
	z9Prev := ZLEMA(ctx.Candles4h[:n4h-1], 9)
	z21Prev := ZLEMA(ctx.Candles4h[:n4h-1], 21)
	// ZLEMA9 crosses below ZLEMA21
	if z9Prev <= z21Prev || z9 >= z21 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("ZLEMA9 cross↓ZLEMA21+EFI(%.0f)<0+ADX>20+CMF<-0.03, EMA down. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW137: Donchian CMF Bear Short ───────────────────────────────────────────
type DonchianCMFBearShort struct{}

func (s *DonchianCMFBearShort) Name() string           { return "Donchian_CMF_Bear_Short" }
func (s *DonchianCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DonchianCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	don := Donchian(ctx.Candles4h[:n4h-1], 20)
	cur := ctx.Candles4h[n4h-1]
	// Close breaks below Donchian lower
	if cur.Close >= don.Lower {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Donchian break(%.0f)+CMF(%.3f)<-0.05, EMA down. SL=%.2f%%", don.Lower, cmf, slDist/ctx.Price*100),
	}
}

// ── HW138: Keltner CMF Bear Short ────────────────────────────────────────────
type KeltnerCMFBearShort struct{}

func (s *KeltnerCMFBearShort) Name() string           { return "Keltner_CMF_Bear_Short" }
func (s *KeltnerCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *KeltnerCMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	// Close crosses below Keltner lower
	if prev.Close <= kcPrev.Lower || cur.Close >= kc.Lower {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Keltner break+CMF(%.3f)<0, EMA down. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW139: StochRSI ADX Bear Short ───────────────────────────────────────────
type StochRSIADXBearShort struct{}

func (s *StochRSIADXBearShort) Name() string           { return "StochRSI_ADX_Bear_Short" }
func (s *StochRSIADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *StochRSIADXBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 40 || n1h < 22 {
		return NoSignal(name)
	}
	k, d := StochRSI(ctx.Candles4h, 14, 14, 3, 3)
	kPrev, dPrev := StochRSI(ctx.Candles4h[:n4h-1], 14, 14, 3, 3)
	// StochRSI K crosses below D
	if kPrev <= dPrev || k >= d {
		return NoSignal(name)
	}
	adx := ADX(ctx.Candles4h, 14)
	if adx < 25 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 58 {
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
		Reason: fmt.Sprintf("StochRSI K(%.1f)<D+ADX>25(%.1f)+RSI<58, EMA down. SL=%.2f%%", k, adx, slDist/ctx.Price*100),
	}
}

// ── HW140: Five Indicator Bear Confirm Short ──────────────────────────────────
type FiveIndicatorBearShort struct{}

func (s *FiveIndicatorBearShort) Name() string           { return "Five_Indicator_Bear_Short" }
func (s *FiveIndicatorBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *FiveIndicatorBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	// All 5 indicators bearish: RSI<50, MACD<0, CMF<0, ADX>20, EMA8<EMA21
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	// RSI crosses below 50 (fresh trigger)
	if rsi4hPrev <= 50 || rsi4h >= 50 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 20 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("5-indicator bear: RSI cross↓50(%.1f)+MACD-+CMF-+ADX>20+EMA down. SL=%.2f%%", rsi4h, slDist/ctx.Price*100),
	}
}

// ── HW141: Supertrend CMF Bear Short ─────────────────────────────────────────
type SupertrendCMFBearShort struct{}

func (s *SupertrendCMFBearShort) Name() string           { return "Supertrend_CMF_Bear_Short" }
func (s *SupertrendCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *SupertrendCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	st := Supertrend(ctx.Candles4h, 10, 3.0)
	stPrev := Supertrend(ctx.Candles4h[:n4h-1], 10, 3.0)
	// ST flips bearish (was bullish)
	if stPrev.Direction != 1 || st.Direction != -1 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.05 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h < 25 || rsi4h > 60 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h ST flip bear+CMF(%.3f)<-0.05, EMA down. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW142: WMA CMF Bear Short ─────────────────────────────────────────────────
type WMACMFBearShort struct{}

func (s *WMACMFBearShort) Name() string           { return "WMA_CMF_Bear_Short" }
func (s *WMACMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMACMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	w9 := WMA(ctx.Candles4h, 9)
	w21 := WMA(ctx.Candles4h, 21)
	w9Prev := WMA(ctx.Candles4h[:n4h-1], 9)
	w21Prev := WMA(ctx.Candles4h[:n4h-1], 21)
	// WMA9 crosses below WMA21
	if w9Prev <= w21Prev || w9 >= w21 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= 0 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9(%.0f) cross↓WMA21(%.0f)+CMF(%.3f)<0. SL=%.2f%%", w9, w21, cmf, slDist/ctx.Price*100),
	}
}

// ── HW143: ATR CMF Expand Bear Short ─────────────────────────────────────────
type ATRCMFExpandBearShort struct{}

func (s *ATRCMFExpandBearShort) Name() string           { return "ATR_CMF_Expand_Bear_Short" }
func (s *ATRCMFExpandBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ATRCMFExpandBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	atr4h := ATR(ctx.Candles4h, 14)
	atr4hPrev := ATR(ctx.Candles4h[:n4h-1], 14)
	// ATR expanding (increasing volatility)
	if atr4hPrev == 0 || atr4h < atr4hPrev*1.1 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	cmfPrev := ChaikinMoneyFlow(ctx.Candles4h[:n4h-1], 20)
	// CMF crosses below -0.05
	if cmfPrev <= -0.05 || cmf >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("ATR expanding(%.0f→%.0f)+CMF cross↓-0.05, EMA down. SL=%.2f%%", atr4hPrev, atr4h, slDist/ctx.Price*100),
	}
}

// ── HW144: RSI Fisher Bear Short ─────────────────────────────────────────────
type RSIFisherBearShort struct{}

func (s *RSIFisherBearShort) Name() string           { return "RSI_Fisher_Bear_Short" }
func (s *RSIFisherBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSIFisherBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	// RSI crosses below 50 AND Fisher already < 0
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev <= 50 || rsi4h >= 50 {
		return NoSignal(name)
	}
	fish := FisherTransform(ctx.Candles4h, 14)
	if fish >= 0 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
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
		Reason: fmt.Sprintf("RSI cross↓50(%.1f)+Fisher(%.2f)<0, EMA down. SL=%.2f%%", rsi4h, fish, slDist/ctx.Price*100),
	}
}

// ── HW145: Coppock KST Bear Short ────────────────────────────────────────────
type CoppockKSTBearShort struct{}

func (s *CoppockKSTBearShort) Name() string           { return "Coppock_KST_Bear_Short" }
func (s *CoppockKSTBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CoppockKSTBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	copp := CoppockCurve(ctx.Candles4h, 14, 11, 10)
	coppPrev := CoppockCurve(ctx.Candles4h[:n4h-1], 14, 11, 10)
	// Coppock crosses below 0
	if coppPrev <= 0 || copp >= 0 {
		return NoSignal(name)
	}
	kst := KST(ctx.Candles4h)
	if kst.KST == 0 || kst.KST >= 0 {
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
		Reason: fmt.Sprintf("Coppock cross↓0+KST(%.3f)<0, EMA down. SL=%.2f%%", kst.KST, slDist/ctx.Price*100),
	}
}

func BuildHWV8Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &RSIMACDKSTBearShort{}, Name: "RSI_MACD_KST_Bear_Short", Description: "KST cross↓signal+RSI<50+MACD-+EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CMFOBVConfluenceBearShort{}, Name: "CMF_OBV_Confluence_Bear_Short", Description: "CMF cross↓0+OBV declining 3 bars+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FisherCMFConfirmBearShort{}, Name: "Fisher_CMF_Confirm_Bear_Short", Description: "Fisher cross↓0+CMF<-0.05+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &StochRSICMFBearShort{}, Name: "StochRSI_CMF_Bear_Short", Description: "StochRSI K cross↓D from>60+CMF<0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WRRSIBearConfirmShort{}, Name: "WR_RSI_Bear_Confirm_Short", Description: "WR cross↓-50+RSI<50+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &AroonCMFBearDualShort{}, Name: "Aroon_CMF_Bear_Dual_Short", Description: "Aroon down>65+up<35+CMF<-0.05+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBRSIDoubleBearShort{}, Name: "BB_RSI_Double_Bear_Short", Description: "BB mid break+RSI cross↓50+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KSTEFIBearShort{}, Name: "KST_EFI_Bear_Short", Description: "KST cross↓signal+EFI<0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HMACMFBearShort{}, Name: "HMA_CMF_Bear_Short", Description: "HMA9 declining+CMF<-0.05+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DEMARSIBearShort{}, Name: "DEMA_RSI_Bear_Short", Description: "DEMA8<DEMA21+RSI cross↓50+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ZLEMAEFIBearShort{}, Name: "ZLEMA_EFI_Bear_Short", Description: "ZLEMA9 cross↓ZLEMA21+EFI<0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DonchianCMFBearShort{}, Name: "Donchian_CMF_Bear_Short", Description: "Donchian lower break+CMF<-0.05+EMA down+MACD-+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KeltnerCMFBearShort{}, Name: "Keltner_CMF_Bear_Short", Description: "Keltner lower break+CMF<0+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &StochRSIADXBearShort{}, Name: "StochRSI_ADX_Bear_Short", Description: "StochRSI K cross↓D+ADX>25+RSI4h<58+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FiveIndicatorBearShort{}, Name: "Five_Indicator_Bear_Short", Description: "RSI cross↓50+MACD-+CMF-+ADX>20+EMA down (5 confirms)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &SupertrendCMFBearShort{}, Name: "Supertrend_CMF_Bear_Short", Description: "4h ST flip bear+CMF<-0.05+RSI4h 25-60+EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMACMFBearShort{}, Name: "WMA_CMF_Bear_Short", Description: "WMA9 cross↓WMA21+CMF<0+EMA down+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ATRCMFExpandBearShort{}, Name: "ATR_CMF_Expand_Bear_Short", Description: "ATR expanding+CMF cross↓-0.05+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &RSIFisherBearShort{}, Name: "RSI_Fisher_Bear_Short", Description: "RSI cross↓50+Fisher<0+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CoppockKSTBearShort{}, Name: "Coppock_KST_Bear_Short", Description: "Coppock cross↓0+KST<0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
