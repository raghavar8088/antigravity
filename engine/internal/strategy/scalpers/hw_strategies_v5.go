package scalpers

import "fmt"

// ─── High-Win-Rate Strategy Family v5 (HW66–HW85) ────────────────────────────
// 20 SHORT strategies using indicator crossovers into bearish territory.
// Template: 4h crossover event + EMA8<EMA21 + MACD<0 + ADX filter + 1h RSI range.
// ─────────────────────────────────────────────────────────────────────────────

// ── HW66: WilliamsR OB Exit Short ────────────────────────────────────────────
type WROBExitShort struct{}

func (s *WROBExitShort) Name() string           { return "WR_OB_Exit_Short" }
func (s *WROBExitShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WROBExitShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	wr := WilliamsR(ctx.Candles4h, 14)
	wrPrev := WilliamsR(ctx.Candles4h[:n4h-1], 14)
	// WR crosses below -20 from overbought (exits OB zone)
	if wrPrev <= -20 || wr >= -20 {
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
		Reason: fmt.Sprintf("4h WR(%.1f) cross↓-20 from OB(%.1f), EMA down. SL=%.2f%%", wr, wrPrev, slDist/ctx.Price*100),
	}
}

// ── HW67: StochRSI K<D From Overbought Short ─────────────────────────────────
type StochRSIKDOBShort struct{}

func (s *StochRSIKDOBShort) Name() string           { return "StochRSI_KD_OB_Short" }
func (s *StochRSIKDOBShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *StochRSIKDOBShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 40 || n1h < 22 {
		return NoSignal(name)
	}
	k, d := StochRSI(ctx.Candles4h, 14, 14, 3, 3)
	kPrev, dPrev := StochRSI(ctx.Candles4h[:n4h-1], 14, 14, 3, 3)
	// K crosses below D from overbought zone (K was >75)
	if kPrev <= dPrev || k >= d || kPrev < 75 {
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
		Reason: fmt.Sprintf("4h StochRSI K(%.1f)<D(%.1f) from OB, EMA down. SL=%.2f%%", k, d, slDist/ctx.Price*100),
	}
}

// ── HW68: CMF Zero Cross Bear Short ──────────────────────────────────────────
type CMFZeroBearShort struct{}

func (s *CMFZeroBearShort) Name() string           { return "CMF_Zero_Bear_Short" }
func (s *CMFZeroBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CMFZeroBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	cmfPrev := ChaikinMoneyFlow(ctx.Candles4h[:n4h-1], 20)
	// CMF crosses below 0 from positive
	if cmfPrev <= 0 || cmf >= 0 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 65 {
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
		Reason: fmt.Sprintf("4h CMF(%.3f) cross↓0 from %.3f, EMA down. SL=%.2f%%", cmf, cmfPrev, slDist/ctx.Price*100),
	}
}

// ── HW69: KST Signal Cross Bear Short ────────────────────────────────────────
type KSTSignalCrossBearShort struct{}

func (s *KSTSignalCrossBearShort) Name() string           { return "KST_Signal_Cross_Bear_Short" }
func (s *KSTSignalCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *KSTSignalCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 65 || n1h < 22 {
		return NoSignal(name)
	}
	kst := KST(ctx.Candles4h)
	kstPrev := KST(ctx.Candles4h[:n4h-1])
	if kst.KST == 0 || kstPrev.KST == 0 {
		return NoSignal(name)
	}
	// KST crosses below Signal
	if kstPrev.KST <= kstPrev.Signal || kst.KST >= kst.Signal {
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
		Reason: fmt.Sprintf("4h KST(%.2f)<Signal(%.2f) cross, EMA down. SL=%.2f%%", kst.KST, kst.Signal, slDist/ctx.Price*100),
	}
}

// ── HW70: Elder Force Index Zero Cross Bear Short ─────────────────────────────
type EFIZeroCrossBearShort struct{}

func (s *EFIZeroCrossBearShort) Name() string           { return "EFI_Zero_Cross_Bear_Short" }
func (s *EFIZeroCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EFIZeroCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	efiPrev := ElderForceIndex(ctx.Candles4h[:n4h-1], 13)
	// EFI crosses below 0 from positive
	if efiPrev <= 0 || efi >= 0 {
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
		Reason: fmt.Sprintf("4h EFI(%.0f) cross↓0 from %.0f, EMA down. SL=%.2f%%", efi, efiPrev, slDist/ctx.Price*100),
	}
}

// ── HW71: BB Midline Cross Bear Short ────────────────────────────────────────
type BBMidBreakShort struct{}

func (s *BBMidBreakShort) Name() string           { return "BB_Mid_Break_Short" }
func (s *BBMidBreakShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BBMidBreakShort) Evaluate(ctx MarketContext) Signal {
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
	// Close crosses below BB midline
	if prev.Close <= bbPrev.Middle || cur.Close >= bb.Middle {
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
		Reason: fmt.Sprintf("4h close(%.0f) cross↓BB mid(%.0f), EMA down. SL=%.2f%%", cur.Close, bb.Middle, slDist/ctx.Price*100),
	}
}

// ── HW72: EMA21 Death Cross EMA50 Short ──────────────────────────────────────
type EMA21EMA50DeathCrossShort struct{}

func (s *EMA21EMA50DeathCrossShort) Name() string           { return "EMA21_EMA50_Death_Cross_Short" }
func (s *EMA21EMA50DeathCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA21EMA50DeathCrossShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 55 || n1h < 22 {
		return NoSignal(name)
	}
	ema21 := EMA(ctx.Candles4h, 21)
	ema50 := EMA(ctx.Candles4h, 50)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)
	ema50Prev := EMA(ctx.Candles4h[:n4h-1], 50)
	// EMA21 crosses below EMA50
	if ema21Prev <= ema50Prev || ema21 >= ema50 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h EMA21(%.0f) cross↓EMA50(%.0f) death cross, MACD-. SL=%.2f%%", ema21, ema50, slDist/ctx.Price*100),
	}
}

// ── HW73: Donchian Midline Cross Bear Short ───────────────────────────────────
type DonchianMidBreakShort struct{}

func (s *DonchianMidBreakShort) Name() string           { return "Donchian_Mid_Break_Short" }
func (s *DonchianMidBreakShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DonchianMidBreakShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	don := Donchian(ctx.Candles4h[:n4h-1], 20)
	donPrev := Donchian(ctx.Candles4h[:n4h-2], 20)
	cur := ctx.Candles4h[n4h-1]
	prev := ctx.Candles4h[n4h-2]
	// Close crosses below Donchian midline
	if prev.Close <= donPrev.Mid || cur.Close >= don.Mid {
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
		Reason: fmt.Sprintf("4h close cross↓Donchian mid(%.0f), EMA down. SL=%.2f%%", don.Mid, slDist/ctx.Price*100),
	}
}

// ── HW74: Fisher High Exit Short ─────────────────────────────────────────────
type FisherHighExitShort struct{}

func (s *FisherHighExitShort) Name() string           { return "Fisher_High_Exit_Short" }
func (s *FisherHighExitShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *FisherHighExitShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 20 || n1h < 22 {
		return NoSignal(name)
	}
	fish := FisherTransform(ctx.Candles4h, 14)
	fishPrev := FisherTransform(ctx.Candles4h[:n4h-1], 14)
	// Fisher was above 1.0 (overbought) and crosses below 0.8
	if fishPrev <= 1.0 || fish >= 0.8 {
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
		Reason: fmt.Sprintf("4h Fisher OB exit %.2f→%.2f, EMA down. SL=%.2f%%", fishPrev, fish, slDist/ctx.Price*100),
	}
}

// ── HW75: RSI 60 Break Short ──────────────────────────────────────────────────
type RSI60BreakShort struct{}

func (s *RSI60BreakShort) Name() string           { return "RSI_60_Break_Short" }
func (s *RSI60BreakShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSI60BreakShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 20 || n1h < 22 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	// RSI crosses below 60 from 62-80 (momentum weakening)
	if rsi4hPrev < 62 || rsi4hPrev > 80 || rsi4h >= 60 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h RSI(%.1f) cross↓60 from %.1f, EMA down. SL=%.2f%%", rsi4h, rsi4hPrev, slDist/ctx.Price*100),
	}
}

// ── HW76: HMA Slope Turn Short ────────────────────────────────────────────────
type HMASlopeTurnShort struct{}

func (s *HMASlopeTurnShort) Name() string           { return "HMA_Slope_Turn_Short" }
func (s *HMASlopeTurnShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *HMASlopeTurnShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	hma := HMA(ctx.Candles4h, 9)
	hmaPrev := HMA(ctx.Candles4h[:n4h-1], 9)
	hmaPrev2 := HMA(ctx.Candles4h[:n4h-2], 9)
	// HMA turns down: prev2<prev (was rising) and now curr<prev (turned down)
	if hmaPrev2 >= hmaPrev || hma >= hmaPrev {
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
	// RSI4h must be below 50 — confirms HMA reversal is in a bearish momentum context
	if RSI(ctx.Candles4h, 14) >= 50 {
		return NoSignal(name)
	}
	// CMF<-0.02 confirms distribution during HMA turn-down
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= -0.02 {
		return NoSignal(name)
	}
	// EFI<0 confirms selling force at the HMA turn
	if ElderForceIndex(ctx.Candles4h, 13) >= 0 {
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
		Reason: fmt.Sprintf("4h HMA(9) turns down %.0f→%.0f, EMA down. SL=%.2f%%", hmaPrev, hma, slDist/ctx.Price*100),
	}
}

// ── HW77: Coppock Zero Bear Cross Short ──────────────────────────────────────
type CoppockZeroBearShort struct{}

func (s *CoppockZeroBearShort) Name() string           { return "Coppock_Zero_Bear_Short" }
func (s *CoppockZeroBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *CoppockZeroBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 60 || n1h < 22 {
		return NoSignal(name)
	}
	copp := CoppockCurve(ctx.Candles4h, 14, 11, 10)
	coppPrev := CoppockCurve(ctx.Candles4h[:n4h-1], 14, 11, 10)
	// Coppock crosses below 0
	if coppPrev <= 0 || copp >= 0 {
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
		Reason: fmt.Sprintf("4h Coppock(%.4f) cross↓0 from %.4f, EMA down. SL=%.2f%%", copp, coppPrev, slDist/ctx.Price*100),
	}
}

// ── HW78: DEMA Death Cross Short ─────────────────────────────────────────────
type DEMADeathCrossShort struct{}

func (s *DEMADeathCrossShort) Name() string           { return "DEMA_Death_Cross_Short" }
func (s *DEMADeathCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DEMADeathCrossShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 50 || n1h < 22 {
		return NoSignal(name)
	}
	dema8 := DEMA(ctx.Candles4h, 8)
	dema21 := DEMA(ctx.Candles4h, 21)
	dema8Prev := DEMA(ctx.Candles4h[:n4h-1], 8)
	dema21Prev := DEMA(ctx.Candles4h[:n4h-1], 21)
	// DEMA8 crosses below DEMA21
	if dema8Prev <= dema21Prev || dema8 >= dema21 {
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
		Reason: fmt.Sprintf("4h DEMA8(%.0f) cross↓DEMA21(%.0f), MACD-. SL=%.2f%%", dema8, dema21, slDist/ctx.Price*100),
	}
}

// ── HW79: Keltner Lower Break Short ──────────────────────────────────────────
type KeltnerBreakShort struct{}

func (s *KeltnerBreakShort) Name() string           { return "Keltner_Lower_Break_Short" }
func (s *KeltnerBreakShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *KeltnerBreakShort) Evaluate(ctx MarketContext) Signal {
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
	// Close crosses below Keltner lower band
	if prev.Close <= kcPrev.Lower || cur.Close >= kc.Lower {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h close cross↓Keltner lower(%.0f), EMA down. SL=%.2f%%", kc.Lower, slDist/ctx.Price*100),
	}
}

// ── HW80: OBV Slope Bear Short ────────────────────────────────────────────────
type OBVSlopeBearShort struct{}

func (s *OBVSlopeBearShort) Name() string           { return "OBV_Slope_Bear_Short" }
func (s *OBVSlopeBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OBVSlopeBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	obvSlice := OBVSlice(ctx.Candles4h)
	if len(obvSlice) < 6 {
		return NoSignal(name)
	}
	// OBV declining for 5 consecutive bars
	declining := true
	for i := len(obvSlice) - 5; i < len(obvSlice); i++ {
		if obvSlice[i] >= obvSlice[i-1] {
			declining = false
			break
		}
	}
	if !declining {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.80,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h OBV declining 5 bars, EMA down, MACD-. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW81: ZLEMA Bear Cross Short ─────────────────────────────────────────────
type ZLEMABearCrossShort struct{}

func (s *ZLEMABearCrossShort) Name() string           { return "ZLEMA_Bear_Cross_Short" }
func (s *ZLEMABearCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ZLEMABearCrossShort) Evaluate(ctx MarketContext) Signal {
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
		Reason: fmt.Sprintf("4h ZLEMA9(%.0f) cross↓ZLEMA21(%.0f), MACD-. SL=%.2f%%", z9, z21, slDist/ctx.Price*100),
	}
}

// ── HW82: WMA Bear Cross Short ────────────────────────────────────────────────
type WMABearCrossShort struct{}

func (s *WMABearCrossShort) Name() string           { return "WMA_Bear_Cross_Short" }
func (s *WMABearCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMABearCrossShort) Evaluate(ctx MarketContext) Signal {
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
		Reason: fmt.Sprintf("4h WMA9(%.0f) cross↓WMA21(%.0f), EMA down. SL=%.2f%%", w9, w21, slDist/ctx.Price*100),
	}
}

// ── HW83: RSI 65 Break Short ──────────────────────────────────────────────────
type RSI65BreakShort struct{}

func (s *RSI65BreakShort) Name() string           { return "RSI_65_Break_Short" }
func (s *RSI65BreakShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSI65BreakShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 20 || n1h < 22 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	// RSI crosses below 65 from 67-85
	if rsi4hPrev < 67 || rsi4hPrev > 85 || rsi4h >= 65 {
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
	if rsi1h < 20 || rsi1h > 70 {
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
		Reason: fmt.Sprintf("4h RSI(%.1f) cross↓65 from %.1f, EMA down. SL=%.2f%%", rsi4h, rsi4hPrev, slDist/ctx.Price*100),
	}
}

// ── HW84: Chandelier Bear Short ───────────────────────────────────────────────
type ChandelierBearBreakShort struct{}

func (s *ChandelierBearBreakShort) Name() string           { return "Chandelier_Bear_Break_Short" }
func (s *ChandelierBearBreakShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *ChandelierBearBreakShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	longStop, _ := ChandelierExit(ctx.Candles4h, 22, 3.0)
	longStopPrev, _ := ChandelierExit(ctx.Candles4h[:n4h-1], 22, 3.0)
	cur := ctx.Candles4h[n4h-1]
	prev := ctx.Candles4h[n4h-2]
	// Price drops below Chandelier long stop (bearish break)
	if prev.Close <= longStopPrev || cur.Close >= longStop {
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
	// CMF<-0.05 confirms institutional selling as Chandelier breaks
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h close cross↓Chandelier stop(%.0f), CMF<-0.05, EMA down. SL=%.2f%%", longStop, slDist/ctx.Price*100),
	}
}

// ── HW85: BB Width Expand Bear Short ─────────────────────────────────────────
type BBWidthExpandBearShort struct{}

func (s *BBWidthExpandBearShort) Name() string           { return "BB_Width_Expand_Bear_Short" }
func (s *BBWidthExpandBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BBWidthExpandBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	bb := BB(ctx.Candles4h, 20)
	bbPrev := BB(ctx.Candles4h[:n4h-1], 20)
	curWidth := bb.Upper - bb.Lower
	prevWidth := bbPrev.Upper - bbPrev.Lower
	cur := ctx.Candles4h[n4h-1]
	// BB width expanding (volatility increasing) AND bearish close AND close below middle
	if prevWidth == 0 || curWidth < prevWidth*1.05 {
		return NoSignal(name)
	}
	if cur.Close >= cur.Open || cur.Close >= bb.Middle {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.81,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h BB expanding(%.0f>%.0f) bearish close, EMA down. SL=%.2f%%", curWidth, prevWidth, slDist/ctx.Price*100),
	}
}

func BuildHWV5Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &WROBExitShort{}, Name: "WR_OB_Exit_Short", Description: "4h WR cross↓-20 from OB+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &StochRSIKDOBShort{}, Name: "StochRSI_KD_OB_Short", Description: "4h StochRSI K cross↓D from>75+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CMFZeroBearShort{}, Name: "CMF_Zero_Bear_Short", Description: "4h CMF cross↓0 from positive+EMA down+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KSTSignalCrossBearShort{}, Name: "KST_Signal_Cross_Bear_Short", Description: "4h KST cross↓Signal+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EFIZeroCrossBearShort{}, Name: "EFI_Zero_Cross_Bear_Short", Description: "4h EFI cross↓0 from positive+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBMidBreakShort{}, Name: "BB_Mid_Break_Short", Description: "4h close cross↓BB20 midline+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EMA21EMA50DeathCrossShort{}, Name: "EMA21_EMA50_Death_Cross_Short", Description: "4h EMA21 cross↓EMA50+MACD-+RSI4h<60", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DonchianMidBreakShort{}, Name: "Donchian_Mid_Break_Short", Description: "4h close cross↓Donchian20 mid+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FisherHighExitShort{}, Name: "Fisher_High_Exit_Short", Description: "4h Fisher OB(>1.0) exit cross↓0.8+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &RSI60BreakShort{}, Name: "RSI_60_Break_Short", Description: "4h RSI cross↓60 from 62-80+EMA down+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HMASlopeTurnShort{}, Name: "HMA_Slope_Turn_Short", Description: "4h HMA9 turns down (peak reversal)+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &CoppockZeroBearShort{}, Name: "Coppock_Zero_Bear_Short", Description: "4h Coppock cross↓0+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DEMADeathCrossShort{}, Name: "DEMA_Death_Cross_Short", Description: "4h DEMA8 cross↓DEMA21+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KeltnerBreakShort{}, Name: "Keltner_Lower_Break_Short", Description: "4h close cross↓Keltner lower(1.5×ATR)+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OBVSlopeBearShort{}, Name: "OBV_Slope_Bear_Short", Description: "4h OBV declining 5 bars+EMA down+MACD-+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ZLEMABearCrossShort{}, Name: "ZLEMA_Bear_Cross_Short", Description: "4h ZLEMA9 cross↓ZLEMA21+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMABearCrossShort{}, Name: "WMA_Bear_Cross_Short", Description: "4h WMA9 cross↓WMA21+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &RSI65BreakShort{}, Name: "RSI_65_Break_Short", Description: "4h RSI cross↓65 from 67-85+EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &ChandelierBearBreakShort{}, Name: "Chandelier_Bear_Break_Short", Description: "4h close cross↓Chandelier long stop+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBWidthExpandBearShort{}, Name: "BB_Width_Expand_Bear_Short", Description: "4h BB width expanding+bearish close below mid+EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
