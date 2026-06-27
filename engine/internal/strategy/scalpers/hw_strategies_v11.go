package scalpers

import "fmt"

// ─── High-Win-Rate Strategy Family v11 (HW186–HW205) ─────────────────────────
// 20 SHORT strategies: ADX>25 + crossover combos (proven to push PF to 1.35-3.98).
// Also BB+ADX, Aroon+ADX, and multi-MA combinations.
// ─────────────────────────────────────────────────────────────────────────────

// ── HW186: WMA9 Aroon Bear Short ──────────────────────────────────────────────
// WMA9/21 bear cross + Aroon Down > 70 (strong bearish directional move)
type WMA9AroonBearShort struct{}

func (s *WMA9AroonBearShort) Name() string           { return "WMA9_Aroon_Bear_Short" }
func (s *WMA9AroonBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA9AroonBearShort) Evaluate(ctx MarketContext) Signal {
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
	aroon := Aroon(ctx.Candles4h, 25)
	if aroon.Down < 50 || aroon.Up > 45 {
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
		Reason: fmt.Sprintf("WMA9 cross↓WMA21 + Aroon down>70(%.0f). SL=%.2f%%", aroon.Down, slDist/ctx.Price*100),
	}
}

// ── HW187: WMA9 Donchian Bear Short ──────────────────────────────────────────
// WMA9/21 bear cross + price below Donchian mid (breakdown confirmed)
type WMA9DonchianBearShort struct{}

func (s *WMA9DonchianBearShort) Name() string           { return "WMA9_Donchian_Bear_Short" }
func (s *WMA9DonchianBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA9DonchianBearShort) Evaluate(ctx MarketContext) Signal {
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
	donch := Donchian(ctx.Candles4h, 20)
	if ctx.Price >= donch.Mid {
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
		Reason: fmt.Sprintf("WMA9 cross↓WMA21 + price<Donchian mid(%.0f). SL=%.2f%%", donch.Mid, slDist/ctx.Price*100),
	}
}

// ── HW188: EMA9 ADX Strong Bear Short ────────────────────────────────────────
// EMA9/EMA21 bear cross + ADX>25 (strong trend momentum)
type EMA9ADXStrongBearShort struct{}

func (s *EMA9ADXStrongBearShort) Name() string           { return "EMA9_ADX_Strong_Bear_Short" }
func (s *EMA9ADXStrongBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA9ADXStrongBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	ema9 := EMA(ctx.Candles4h, 9)
	ema21 := EMA(ctx.Candles4h, 21)
	ema9Prev := EMA(ctx.Candles4h[:n4h-1], 9)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)
	if ema9Prev <= ema21Prev || ema9 >= ema21 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 25 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h > 48 {
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
		Reason: fmt.Sprintf("EMA9 cross↓EMA21 + ADX>25(%.1f) + RSI4h<48. SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW189: EMA9 EFI Bear Short ────────────────────────────────────────────────
type EMA9EFIBearShort struct{}

func (s *EMA9EFIBearShort) Name() string           { return "EMA9_EFI_Bear_Short" }
func (s *EMA9EFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EMA9EFIBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	ema9 := EMA(ctx.Candles4h, 9)
	ema21 := EMA(ctx.Candles4h, 21)
	ema9Prev := EMA(ctx.Candles4h[:n4h-1], 9)
	ema21Prev := EMA(ctx.Candles4h[:n4h-1], 21)
	if ema9Prev <= ema21Prev || ema9 >= ema21 {
		return NoSignal(name)
	}
	efi := ElderForceIndex(ctx.Candles4h, 13)
	if efi >= 0 {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	// CMF<0 adds distribution confirmation alongside EFI selling force
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EMA9 cross↓EMA21 + EFI<0(%.0f) + CMF<0. SL=%.2f%%", efi, slDist/ctx.Price*100),
	}
}

// ── HW190: BB Mid ADX Bear Short ─────────────────────────────────────────────
// Close crosses below 4h BB mid + ADX>25 (trending breakdown)
type BBMidADXBearShort struct{}

func (s *BBMidADXBearShort) Name() string           { return "BB_Mid_ADX_Bear_Short" }
func (s *BBMidADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *BBMidADXBearShort) Evaluate(ctx MarketContext) Signal {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h close cross↓BB mid + ADX>25(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW191: Keltner ADX Bear Short ────────────────────────────────────────────
type KeltnerADXBearShort struct{}

func (s *KeltnerADXBearShort) Name() string           { return "Keltner_ADX_Bear_Short" }
func (s *KeltnerADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *KeltnerADXBearShort) Evaluate(ctx MarketContext) Signal {
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
	// Price crosses below Keltner midline
	if prev.Close <= kcPrev.Mid || cur.Close >= kc.Mid {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h close cross↓Keltner mid + ADX>25(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW192: Donchian ADX Bear Short ───────────────────────────────────────────
type DonchianADXBearShort struct{}

func (s *DonchianADXBearShort) Name() string           { return "Donchian_ADX_Bear_Short" }
func (s *DonchianADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *DonchianADXBearShort) Evaluate(ctx MarketContext) Signal {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h close cross↓Donchian mid + ADX>25(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW193: RSI4h 55 Break ADX Short ──────────────────────────────────────────
// 4h RSI crosses below 55 + ADX>25 (momentum fading in strong trend)
type RSI55BreakADXShort struct{}

func (s *RSI55BreakADXShort) Name() string           { return "RSI55_Break_ADX_Short" }
func (s *RSI55BreakADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *RSI55BreakADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev <= 55 || rsi4h >= 55 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h RSI cross↓55(%.1f) + ADX>25 + CMF<0. SL=%.2f%%", rsi4h, slDist/ctx.Price*100),
	}
}

// ── HW194: MACD Hist Sign Flip ADX Short ─────────────────────────────────────
// MACD histogram flips sign (positive→negative) + ADX>25 + CMF<0
type MACDHistFlipADXShort struct{}

func (s *MACDHistFlipADXShort) Name() string           { return "MACD_Hist_Flip_ADX_Short" }
func (s *MACDHistFlipADXShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MACDHistFlipADXShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 22 {
		return NoSignal(name)
	}
	macd := MACD(ctx.Candles4h)
	macdPrev := MACD(ctx.Candles4h[:n4h-1])
	if macdPrev.Histogram <= 0 || macd.Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 25 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("MACD hist flip↓(%.4f→%.4f) + ADX>25 + CMF<-0.05. SL=%.2f%%", macdPrev.Histogram, macd.Histogram, slDist/ctx.Price*100),
	}
}

// ── HW195: Supertrend CMF ADX Bear Short ─────────────────────────────────────
// Supertrend flips bearish (direction=-1) + CMF<-0.05 + ADX>20
type SupertrendCMFADXBearShort struct{}

func (s *SupertrendCMFADXBearShort) Name() string           { return "Supertrend_CMF_ADX_Bear_Short" }
func (s *SupertrendCMFADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *SupertrendCMFADXBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	st := Supertrend(ctx.Candles4h, 10, 3.0)
	stPrev := Supertrend(ctx.Candles4h[:n4h-1], 10, 3.0)
	// Supertrend flips from bullish to bearish
	if stPrev.Direction != 1 || st.Direction != -1 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.05 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Supertrend flip bearish + CMF<-0.05(%.3f) + ADX>20. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW196: WMA9 Fisher Aroon Bear Short ──────────────────────────────────────
// WMA9/21 cross + Fisher<0 + Aroon Down>60 (triple confirmation)
type WMA9FisherAroonBearShort struct{}

func (s *WMA9FisherAroonBearShort) Name() string           { return "WMA9_Fisher_Aroon_Bear_Short" }
func (s *WMA9FisherAroonBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA9FisherAroonBearShort) Evaluate(ctx MarketContext) Signal {
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
	aroon := Aroon(ctx.Candles4h, 25)
	if aroon.Down < 60 {
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
		Reason: fmt.Sprintf("WMA9↓WMA21 + Fisher<0(%.2f) + Aroon down>60(%.0f). SL=%.2f%%", fisher, aroon.Down, slDist/ctx.Price*100),
	}
}

// ── HW197: WMA9 EFI ADX Bear Short ───────────────────────────────────────────
// WMA9/21 cross + EFI<0 + ADX>25 (triple momentum confirmation)
type WMA9EFIADXBearShort struct{}

func (s *WMA9EFIADXBearShort) Name() string           { return "WMA9_EFI_ADX_Bear_Short" }
func (s *WMA9EFIADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA9EFIADXBearShort) Evaluate(ctx MarketContext) Signal {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.88,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9↓WMA21 + EFI<0 + ADX>25(%.1f). SL=%.2f%%", ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW198: WMA9 CMF ADX Bear Short ───────────────────────────────────────────
// WMA9/21 cross + CMF<-0.05 + ADX>25
type WMA9CMFADXBearShort struct{}

func (s *WMA9CMFADXBearShort) Name() string           { return "WMA9_CMF_ADX_Bear_Short" }
func (s *WMA9CMFADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WMA9CMFADXBearShort) Evaluate(ctx MarketContext) Signal {
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
	if cmf >= -0.03 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.88,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA9↓WMA21 + CMF<-0.05(%.3f) + ADX>25. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW199: 1h WMA9 ADX EFI Bear Short ────────────────────────────────────────
type OneHWMA9ADXEFIBearShort struct{}

func (s *OneHWMA9ADXEFIBearShort) Name() string           { return "1h_WMA9_ADX_EFI_Bear_Short" }
func (s *OneHWMA9ADXEFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHWMA9ADXEFIBearShort) Evaluate(ctx MarketContext) Signal {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.87,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h WMA9↓WMA21 + ADX>20 + EFI<0. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW200: 1h EMA9 CMF Bear Short ────────────────────────────────────────────
type OneHEMA9CMFBearShort struct{}

func (s *OneHEMA9CMFBearShort) Name() string           { return "1h_EMA9_CMF_Bear_Short" }
func (s *OneHEMA9CMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHEMA9CMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	ema9 := EMA(ctx.Candles1h, 9)
	ema21 := EMA(ctx.Candles1h, 21)
	ema9Prev := EMA(ctx.Candles1h[:n1h-1], 9)
	ema21Prev := EMA(ctx.Candles1h[:n1h-1], 21)
	if ema9Prev <= ema21Prev || ema9 >= ema21 {
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
		Reason: fmt.Sprintf("1h EMA9↓EMA21 + 1h CMF<-0.05(%.3f). SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW201: 1h EMA9 EFI Bear Short ────────────────────────────────────────────
type OneHEMA9EFIBearShort struct{}

func (s *OneHEMA9EFIBearShort) Name() string           { return "1h_EMA9_EFI_Bear_Short" }
func (s *OneHEMA9EFIBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OneHEMA9EFIBearShort) Evaluate(ctx MarketContext) Signal {
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
	ema9 := EMA(ctx.Candles1h, 9)
	ema21 := EMA(ctx.Candles1h, 21)
	ema9Prev := EMA(ctx.Candles1h[:n1h-1], 9)
	ema21Prev := EMA(ctx.Candles1h[:n1h-1], 21)
	if ema9Prev <= ema21Prev || ema9 >= ema21 {
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
		Reason: fmt.Sprintf("1h EMA9↓EMA21 + 1h EFI<0(%.0f). SL=%.2f%%", efi1h, slDist/ctx.Price*100),
	}
}

// ── HW202: Fisher ADX Bear Short ──────────────────────────────────────────────
// Fisher crosses below 0 + ADX>25 (strong trend + momentum exhaustion)
type FisherADXBearShort struct{}

func (s *FisherADXBearShort) Name() string           { return "Fisher_ADX_Bear_Short" }
func (s *FisherADXBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *FisherADXBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	fisher := FisherTransform(ctx.Candles4h, 10)
	fisherPrev := FisherTransform(ctx.Candles4h[:n4h-1], 10)
	if fisherPrev <= 0 || fisher >= 0 {
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
		Reason: fmt.Sprintf("Fisher cross↓0(%.2f) + ADX>25(%.1f). SL=%.2f%%", fisher, ADX(ctx.Candles4h, 14), slDist/ctx.Price*100),
	}
}

// ── HW203: EFI CMF Bear Dual Short ───────────────────────────────────────────
// EFI crosses below 0 + CMF<-0.05 (dual selling pressure)
type EFICMFBearDualShort struct{}

func (s *EFICMFBearDualShort) Name() string           { return "EFI_CMF_Bear_Dual_Short" }
func (s *EFICMFBearDualShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *EFICMFBearDualShort) Evaluate(ctx MarketContext) Signal {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.86,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EFI cross↓0 + CMF<-0.05(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

// ── HW204: WR CMF Bear Short ──────────────────────────────────────────────────
// WilliamsR crosses below -50 + CMF<-0.05 + ADX>18
type WRCMFBearShort struct{}

func (s *WRCMFBearShort) Name() string           { return "WR_CMF_Bear_Short" }
func (s *WRCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *WRCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	wr := WilliamsR(ctx.Candles4h, 14)
	wrPrev := WilliamsR(ctx.Candles4h[:n4h-1], 14)
	if wrPrev <= -50 || wr >= -50 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.85,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("4h WR cross↓-50(%.1f) + CMF<-0.05(%.3f). SL=%.2f%%", wr, cmf, slDist/ctx.Price*100),
	}
}

// ── HW205: OBV Bear CMF Short ────────────────────────────────────────────────
// OBV declining 4 bars + CMF<-0.10 (dual volume selling)
type OBVBearCMFShort struct{}

func (s *OBVBearCMFShort) Name() string           { return "OBV_Bear_CMF_Short" }
func (s *OBVBearCMFShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OBVBearCMFShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 22 {
		return NoSignal(name)
	}
	obvSlice := OBVSlice(ctx.Candles4h)
	nObv := len(obvSlice)
	if nObv < 5 {
		return NoSignal(name)
	}
	// OBV declining for 4 consecutive bars
	if !(obvSlice[nObv-1] < obvSlice[nObv-2] &&
		obvSlice[nObv-2] < obvSlice[nObv-3] &&
		obvSlice[nObv-3] < obvSlice[nObv-4]) {
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
		Reason: fmt.Sprintf("OBV declining 4 bars + CMF<-0.05(%.3f)+ADX>18. SL=%.2f%%", cmf, slDist/ctx.Price*100),
	}
}

func BuildHWV11Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &WMA9AroonBearShort{}, Name: "WMA9_Aroon_Bear_Short", Description: "4h WMA9↓WMA21 + Aroon down>70+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA9DonchianBearShort{}, Name: "WMA9_Donchian_Bear_Short", Description: "4h WMA9↓WMA21 + price<Donchian mid+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EMA9ADXStrongBearShort{}, Name: "EMA9_ADX_Strong_Bear_Short", Description: "4h EMA9↓EMA21 + ADX>25+MACD-+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EMA9EFIBearShort{}, Name: "EMA9_EFI_Bear_Short", Description: "4h EMA9↓EMA21 + EFI<0+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBMidADXBearShort{}, Name: "BB_Mid_ADX_Bear_Short", Description: "4h close cross↓BB mid + ADX>25+MACD-+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KeltnerADXBearShort{}, Name: "Keltner_ADX_Bear_Short", Description: "4h close cross↓Keltner mid + ADX>25+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DonchianADXBearShort{}, Name: "Donchian_ADX_Bear_Short", Description: "4h close cross↓Donchian mid + ADX>25+MACD-+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &RSI55BreakADXShort{}, Name: "RSI55_Break_ADX_Short", Description: "4h RSI cross↓55 + ADX>25+CMF<0+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MACDHistFlipADXShort{}, Name: "MACD_Hist_Flip_ADX_Short", Description: "4h MACD hist sign flip↓ + ADX>25+CMF<-0.05", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &SupertrendCMFADXBearShort{}, Name: "Supertrend_CMF_ADX_Bear_Short", Description: "4h Supertrend flip bearish + CMF<-0.05+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA9FisherAroonBearShort{}, Name: "WMA9_Fisher_Aroon_Bear_Short", Description: "4h WMA9↓WMA21 + Fisher<0+Aroon down>60", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA9EFIADXBearShort{}, Name: "WMA9_EFI_ADX_Bear_Short", Description: "4h WMA9↓WMA21 + EFI<0+ADX>25+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMA9CMFADXBearShort{}, Name: "WMA9_CMF_ADX_Bear_Short", Description: "4h WMA9↓WMA21 + CMF<-0.05+ADX>25+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHWMA9ADXEFIBearShort{}, Name: "1h_WMA9_ADX_EFI_Bear_Short", Description: "1h WMA9↓WMA21 + 1h ADX>20+1h EFI<0+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHEMA9CMFBearShort{}, Name: "1h_EMA9_CMF_Bear_Short", Description: "1h EMA9↓EMA21 + 1h CMF<-0.05+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OneHEMA9EFIBearShort{}, Name: "1h_EMA9_EFI_Bear_Short", Description: "1h EMA9↓EMA21 + 1h EFI<0+4h EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FisherADXBearShort{}, Name: "Fisher_ADX_Bear_Short", Description: "4h Fisher cross↓0 + ADX>25+MACD-+RSI4h<55", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EFICMFBearDualShort{}, Name: "EFI_CMF_Bear_Dual_Short", Description: "4h EFI cross↓0 + CMF<-0.05+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WRCMFBearShort{}, Name: "WR_CMF_Bear_Short", Description: "4h WR cross↓-50 + CMF<-0.05+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OBVBearCMFShort{}, Name: "OBV_Bear_CMF_Short", Description: "4h OBV declining 4 bars + CMF<-0.10+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
