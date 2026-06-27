package scalpers

import "fmt"

// ─── High-Win-Rate Strategy Family v7 (HW106–HW125) ──────────────────────────
// 20 SHORT strategies using multi-timeframe confirmation (4h + 1h signals).
// ─────────────────────────────────────────────────────────────────────────────

// ── HW106: MTF RSI Dual Bear Short ───────────────────────────────────────────
type MTFRSIDualBearShort struct{}

func (s *MTFRSIDualBearShort) Name() string           { return "MTF_RSI_Dual_Bear_Short" }
func (s *MTFRSIDualBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MTFRSIDualBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi1h := RSI(ctx.Candles1h, 14)
	// Both 4h RSI<50 AND 1h RSI<48 (dual-timeframe bear)
	if rsi4h >= 50 || rsi1h >= 48 {
		return NoSignal(name)
	}
	n4hPrev := n4h - 1
	rsi4hPrev := RSI(ctx.Candles4h[:n4hPrev], 14)
	// 4h RSI just crossed below 50 (fresh signal)
	if rsi4hPrev <= 50 {
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
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("MTF: 4h RSI(%.1f)<50+1h RSI(%.1f)<48, EMA down. SL=%.2f%%", rsi4h, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW107: MTF EMA Confirm Bear Short ────────────────────────────────────────
type MTFEMAConfirmBearShort struct{}

func (s *MTFEMAConfirmBearShort) Name() string           { return "MTF_EMA_Confirm_Bear_Short" }
func (s *MTFEMAConfirmBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MTFEMAConfirmBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 30 {
		return NoSignal(name)
	}
	// Both 4h EMA8<EMA21 AND 1h EMA8<EMA21
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if EMA(ctx.Candles1h, 8) >= EMA(ctx.Candles1h, 21) {
		return NoSignal(name)
	}
	macd4h := MACD(ctx.Candles4h)
	if macd4h.Histogram >= 0 {
		return NoSignal(name)
	}
	// 4h MACD line just crossed below signal (fresh bear signal)
	macd4hPrev := MACD(ctx.Candles4h[:n4h-1])
	if macd4hPrev.MACD <= macd4hPrev.Signal || macd4h.MACD >= macd4h.Signal {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
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
		Reason: fmt.Sprintf("MTF: 4h+1h both EMA8<EMA21, MACD line cross↓signal. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW108: 1h MACD Cross Bear Short ─────────────────────────────────────────
type OnehMACDCrossBearShort struct{}

func (s *OnehMACDCrossBearShort) Name() string           { return "1h_MACD_Cross_Bear_Short" }
func (s *OnehMACDCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehMACDCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 30 {
		return NoSignal(name)
	}
	// 1h MACD line crosses below signal
	macd1h := MACD(ctx.Candles1h)
	macd1hPrev := MACD(ctx.Candles1h[:n1h-1])
	if macd1hPrev.MACD <= macd1hPrev.Signal || macd1h.MACD >= macd1h.Signal {
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
		Reason: fmt.Sprintf("1h MACD line cross↓signal+4h EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW109: 1h RSI 60 Break Bear Short ────────────────────────────────────────
type OnehRSI60BreakBearShort struct{}

func (s *OnehRSI60BreakBearShort) Name() string           { return "1h_RSI_60_Break_Bear_Short" }
func (s *OnehRSI60BreakBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehRSI60BreakBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	rsi1hPrev := RSI(ctx.Candles1h[:n1h-1], 14)
	// 1h RSI crosses below 60 from above
	if rsi1hPrev <= 60 || rsi1h >= 60 {
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
	if ADX(ctx.Candles4h, 14) < 18 {
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
		Reason: fmt.Sprintf("1h RSI cross↓60(%.1f→%.1f)+4h EMA down+MACD-. SL=%.2f%%", rsi1hPrev, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW110: 1h EMA Death Cross Bear Short ─────────────────────────────────────
type OnehEMADeathCrossBearShort struct{}

func (s *OnehEMADeathCrossBearShort) Name() string           { return "1h_EMA_Death_Cross_Bear_Short" }
func (s *OnehEMADeathCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehEMADeathCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 30 {
		return NoSignal(name)
	}
	// 1h EMA9 crosses below EMA21
	ema9_1h := EMA(ctx.Candles1h, 9)
	ema21_1h := EMA(ctx.Candles1h, 21)
	ema9_1hPrev := EMA(ctx.Candles1h[:n1h-1], 9)
	ema21_1hPrev := EMA(ctx.Candles1h[:n1h-1], 21)
	if ema9_1hPrev <= ema21_1hPrev || ema9_1h >= ema21_1h {
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
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h EMA9 cross↓EMA21+4h EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW111: MTF CMF Both Negative Short ───────────────────────────────────────
type MTFCMFBothNegShort struct{}

func (s *MTFCMFBothNegShort) Name() string           { return "MTF_CMF_Both_Neg_Short" }
func (s *MTFCMFBothNegShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MTFCMFBothNegShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 25 || n1h < 25 {
		return NoSignal(name)
	}
	cmf4h := ChaikinMoneyFlow(ctx.Candles4h, 20)
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	// Both 4h and 1h CMF negative with some depth
	if cmf4h >= -0.03 || cmf1h >= -0.03 {
		return NoSignal(name)
	}
	// 4h CMF just crossed below 0 (fresh signal)
	cmf4hPrev := ChaikinMoneyFlow(ctx.Candles4h[:n4h-1], 20)
	if cmf4hPrev <= 0 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("MTF CMF: 4h(%.3f)<0+1h(%.3f)<0+EFI<0, EMA down. SL=%.2f%%", cmf4h, cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW112: 1h Fisher Zero Cross Bear Short ────────────────────────────────────
type OnehFisherZeroCrossBearShort struct{}

func (s *OnehFisherZeroCrossBearShort) Name() string           { return "1h_Fisher_Zero_Cross_Short" }
func (s *OnehFisherZeroCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehFisherZeroCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	fish1h := FisherTransform(ctx.Candles1h, 14)
	fish1hPrev := FisherTransform(ctx.Candles1h[:n1h-1], 14)
	// 1h Fisher crosses below 0
	if fish1hPrev <= 0 || fish1h >= 0 {
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
		Reason: fmt.Sprintf("1h Fisher cross↓0(%.2f)+4h EMA down+MACD-. SL=%.2f%%", fish1h, slDist/ctx.Price*100),
	}
}

// ── HW113: 1h WilliamsR OB Cross Bear Short ──────────────────────────────────
type OnehWROBCrossBearShort struct{}

func (s *OnehWROBCrossBearShort) Name() string           { return "1h_WR_OB_Cross_Bear_Short" }
func (s *OnehWROBCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehWROBCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	wr1h := WilliamsR(ctx.Candles1h, 14)
	wr1hPrev := WilliamsR(ctx.Candles1h[:n1h-1], 14)
	// 1h WR crosses below -20 (exits overbought)
	if wr1hPrev <= -20 || wr1h >= -20 {
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
	if ADX(ctx.Candles4h, 14) < 18 {
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
		Reason: fmt.Sprintf("1h WR(%.1f) cross↓-20+4h EMA down+MACD-. SL=%.2f%%", wr1h, slDist/ctx.Price*100),
	}
}

// ── HW114: MTF MACD Both Negative Short ──────────────────────────────────────
type MTFMACDBothNegShort struct{}

func (s *MTFMACDBothNegShort) Name() string           { return "MTF_MACD_Both_Neg_Short" }
func (s *MTFMACDBothNegShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MTFMACDBothNegShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 30 || n1h < 30 {
		return NoSignal(name)
	}
	// Both 4h and 1h MACD histograms negative
	if MACD(ctx.Candles4h).Histogram >= 0 || MACD(ctx.Candles1h).Histogram >= 0 {
		return NoSignal(name)
	}
	// 1h MACD histogram just turned negative (fresh)
	if MACD(ctx.Candles1h[:n1h-1]).Histogram <= 0 {
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
		Reason: fmt.Sprintf("MTF MACD: 4h+1h both hist<0, 1h just turned neg, EMA down. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW115: 1h CMF Zero Cross Bear Short ──────────────────────────────────────
type OnehCMFZeroCrossBearShort struct{}

func (s *OnehCMFZeroCrossBearShort) Name() string           { return "1h_CMF_Zero_Cross_Bear_Short" }
func (s *OnehCMFZeroCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehCMFZeroCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 25 {
		return NoSignal(name)
	}
	cmf1h := ChaikinMoneyFlow(ctx.Candles1h, 20)
	cmf1hPrev := ChaikinMoneyFlow(ctx.Candles1h[:n1h-1], 20)
	// 1h CMF crosses below 0
	if cmf1hPrev <= 0 || cmf1h >= 0 {
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
		Reason: fmt.Sprintf("1h CMF cross↓0(%.3f)+4h EMA down+MACD-. SL=%.2f%%", cmf1h, slDist/ctx.Price*100),
	}
}

// ── HW116: 1h StochRSI OB Cross Bear Short ───────────────────────────────────
type OnehStochRSIOBCrossBearShort struct{}

func (s *OnehStochRSIOBCrossBearShort) Name() string           { return "1h_StochRSI_OB_Cross_Bear_Short" }
func (s *OnehStochRSIOBCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehStochRSIOBCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 40 {
		return NoSignal(name)
	}
	k1h, d1h := StochRSI(ctx.Candles1h, 14, 14, 3, 3)
	k1hPrev, d1hPrev := StochRSI(ctx.Candles1h[:n1h-1], 14, 14, 3, 3)
	// 1h StochRSI K crosses below D from >75 (overbought exit)
	if k1hPrev <= d1hPrev || k1h >= d1h || k1hPrev < 75 {
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
	if ADX(ctx.Candles4h, 14) < 18 {
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
		Reason: fmt.Sprintf("1h StochRSI K(%.1f)<D(%.1f) from OB+4h EMA down. SL=%.2f%%", k1h, d1h, slDist/ctx.Price*100),
	}
}

// ── HW117: MTF RSI Cross Confirm Short ───────────────────────────────────────
type MTFRSICrossConfirmShort struct{}

func (s *MTFRSICrossConfirmShort) Name() string           { return "MTF_RSI_Cross_Confirm_Short" }
func (s *MTFRSICrossConfirmShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *MTFRSICrossConfirmShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	// 4h RSI crosses below 50 + 1h RSI already < 45
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev <= 50 || rsi4h >= 50 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	if rsi1h >= 45 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	if MACD(ctx.Candles4h).Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 15 {
		return NoSignal(name)
	}
	if ChaikinMoneyFlow(ctx.Candles4h, 20) >= 0 {
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
		Reason: fmt.Sprintf("4h RSI cross↓50(%.1f)+1h RSI(%.1f)<45+CMF<0, EMA down. SL=%.2f%%", rsi4h, rsi1h, slDist/ctx.Price*100),
	}
}

// ── HW118: 1h BB Midline Break Bear Short ────────────────────────────────────
type OnehBBMidBreakBearShort struct{}

func (s *OnehBBMidBreakBearShort) Name() string           { return "1h_BB_Mid_Break_Bear_Short" }
func (s *OnehBBMidBreakBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehBBMidBreakBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 25 {
		return NoSignal(name)
	}
	bb1h := BB(ctx.Candles1h, 20)
	bb1hPrev := BB(ctx.Candles1h[:n1h-1], 20)
	cur1h := ctx.Candles1h[n1h-1]
	prev1h := ctx.Candles1h[n1h-2]
	// 1h close crosses below 1h BB midline
	if prev1h.Close <= bb1hPrev.Middle || cur1h.Close >= bb1h.Middle {
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
	if ADX(ctx.Candles4h, 14) < 18 {
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
		Reason: fmt.Sprintf("1h close cross↓BB mid(%.0f)+4h EMA down+MACD-. SL=%.2f%%", bb1h.Middle, slDist/ctx.Price*100),
	}
}

// ── HW119: 1h Supertrend Bear Short ──────────────────────────────────────────
type OnehSupertrendBearShort struct{}

func (s *OnehSupertrendBearShort) Name() string           { return "1h_Supertrend_Bear_Short" }
func (s *OnehSupertrendBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehSupertrendBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	// 1h Supertrend flips bearish (was bullish)
	st1h := Supertrend(ctx.Candles1h, 10, 3.0)
	st1hPrev := Supertrend(ctx.Candles1h[:n1h-1], 10, 3.0)
	if st1hPrev.Direction != 1 || st1h.Direction != -1 {
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
		Reason: fmt.Sprintf("1h ST flip bear+4h EMA down+MACD-+ADX>20. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW120: 1h EFI Zero Cross Bear Short ──────────────────────────────────────
type OnehEFIZeroCrossBearShort struct{}

func (s *OnehEFIZeroCrossBearShort) Name() string           { return "1h_EFI_Zero_Cross_Bear_Short" }
func (s *OnehEFIZeroCrossBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehEFIZeroCrossBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	efi1h := ElderForceIndex(ctx.Candles1h, 13)
	efi1hPrev := ElderForceIndex(ctx.Candles1h[:n1h-1], 13)
	// 1h EFI crosses below 0
	if efi1hPrev <= 0 || efi1h >= 0 {
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
	if ADX(ctx.Candles4h, 14) < 18 {
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
		Reason: fmt.Sprintf("1h EFI cross↓0(%.0f)+4h EMA down+MACD-. SL=%.2f%%", efi1h, slDist/ctx.Price*100),
	}
}

// ── HW121: 1h RSI Mid Bear Cross Short ───────────────────────────────────────
type OnehRSIMidBearCrossShort struct{}

func (s *OnehRSIMidBearCrossShort) Name() string           { return "1h_RSI_Mid_Bear_Cross_Short" }
func (s *OnehRSIMidBearCrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehRSIMidBearCrossShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	rsi1h := RSI(ctx.Candles1h, 14)
	rsi1hPrev := RSI(ctx.Candles1h[:n1h-1], 14)
	// 1h RSI crosses below 50
	if rsi1hPrev <= 50 || rsi1h >= 50 {
		return NoSignal(name)
	}
	rsi4h := RSI(ctx.Candles4h, 14)
	if rsi4h >= 55 {
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
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h RSI cross↓50(%.1f)+4h RSI<55(%.1f), EMA down. SL=%.2f%%", rsi1h, rsi4h, slDist/ctx.Price*100),
	}
}

// ── HW122: 1h OBV Bear SMA Cross Short ───────────────────────────────────────
type OnehOBVBearSMACrossShort struct{}

func (s *OnehOBVBearSMACrossShort) Name() string           { return "1h_OBV_Bear_SMA_Cross_Short" }
func (s *OnehOBVBearSMACrossShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehOBVBearSMACrossShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 22 {
		return NoSignal(name)
	}
	// 1h OBV vs its 14-bar SMA
	obvCalc := func(cs []Candle) (obv, sma float64) {
		period := 14
		if len(cs) < period+1 {
			return 0, 0
		}
		var cum float64
		vals := make([]float64, len(cs))
		for i := 1; i < len(cs); i++ {
			if cs[i].Close > cs[i-1].Close {
				cum += cs[i].Volume
			} else if cs[i].Close < cs[i-1].Close {
				cum -= cs[i].Volume
			}
			vals[i] = cum
		}
		obv = vals[len(cs)-1]
		var sum float64
		for _, v := range vals[len(cs)-period:] {
			sum += v
		}
		sma = sum / float64(period)
		return
	}
	obv1h, sma1h := obvCalc(ctx.Candles1h)
	obv1hPrev, sma1hPrev := obvCalc(ctx.Candles1h[:n1h-1])
	// 1h OBV crosses below its SMA
	if obv1hPrev <= sma1hPrev || obv1h >= sma1h {
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
		Reason: fmt.Sprintf("1h OBV cross↓SMA+4h EMA down+MACD-. SL=%.2f%%", slDist/ctx.Price*100),
	}
}

// ── HW123: 1h Keltner Break Bear Short ───────────────────────────────────────
type OnehKeltnerBreakBearShort struct{}

func (s *OnehKeltnerBreakBearShort) Name() string           { return "1h_Keltner_Break_Bear_Short" }
func (s *OnehKeltnerBreakBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehKeltnerBreakBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 25 {
		return NoSignal(name)
	}
	kc1h := KeltnerChannel(ctx.Candles1h, 20, 14, 1.5)
	kc1hPrev := KeltnerChannel(ctx.Candles1h[:n1h-1], 20, 14, 1.5)
	cur1h := ctx.Candles1h[n1h-1]
	prev1h := ctx.Candles1h[n1h-2]
	// 1h close crosses below Keltner lower
	if prev1h.Close <= kc1hPrev.Lower || cur1h.Close >= kc1h.Lower {
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
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{
		Strategy: name, Direction: DirectionShort, Confidence: 0.83,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h close cross↓KC lower(%.0f)+4h EMA down+MACD-. SL=%.2f%%", kc1h.Lower, slDist/ctx.Price*100),
	}
}

// ── HW124: 1h Donchian Break Bear Short ──────────────────────────────────────
type OnehDonchianBreakBearShort struct{}

func (s *OnehDonchianBreakBearShort) Name() string           { return "1h_Donchian_Break_Bear_Short" }
func (s *OnehDonchianBreakBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnehDonchianBreakBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 22 || n1h < 25 {
		return NoSignal(name)
	}
	don1h := Donchian(ctx.Candles1h[:n1h-1], 20)
	cur1h := ctx.Candles1h[n1h-1]
	prev1h := ctx.Candles1h[n1h-2]
	don1hPrev := Donchian(ctx.Candles1h[:n1h-2], 20)
	// 1h close breaks below Donchian lower
	if prev1h.Close <= don1hPrev.Lower || cur1h.Close >= don1h.Lower {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.82,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1h close breaks Donchian lower(%.0f)+4h EMA down. SL=%.2f%%", don1h.Lower, slDist/ctx.Price*100),
	}
}

// ── HW125: 1d MACD Neg 4h Entry Short ────────────────────────────────────────
type OnedMACDNeg4hEntryShort struct{}

func (s *OnedMACDNeg4hEntryShort) Name() string           { return "1d_MACD_Neg_4h_Entry_Short" }
func (s *OnedMACDNeg4hEntryShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }

func (s *OnedMACDNeg4hEntryShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	n1h := len(ctx.Candles1h)
	if n4h < 50 || n1h < 22 {
		return NoSignal(name)
	}
	// Use last 30 4h candles as proxy for longer-term MACD context
	// Require both MACD line AND histogram negative (bearish macro state)
	macdLong := MACD(ctx.Candles4h)
	if macdLong.MACD >= 0 || macdLong.Histogram >= 0 {
		return NoSignal(name)
	}
	// 4h RSI crosses below 55 (entry trigger)
	rsi4h := RSI(ctx.Candles4h, 14)
	rsi4hPrev := RSI(ctx.Candles4h[:n4h-1], 14)
	if rsi4hPrev <= 55 || rsi4h >= 55 {
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
		Strategy: name, Direction: DirectionShort, Confidence: 0.84,
		StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("1d MACD-+4h RSI cross↓55(%.1f), EMA down. SL=%.2f%%", rsi4h, slDist/ctx.Price*100),
	}
}

func BuildHWV7Strategies() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &MTFRSIDualBearShort{}, Name: "MTF_RSI_Dual_Bear_Short", Description: "4h RSI cross↓50+1h RSI<48+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MTFEMAConfirmBearShort{}, Name: "MTF_EMA_Confirm_Bear_Short", Description: "4h+1h both EMA8<EMA21+4h MACD line cross↓signal", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehMACDCrossBearShort{}, Name: "1h_MACD_Cross_Bear_Short", Description: "1h MACD line cross↓signal+4h EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehRSI60BreakBearShort{}, Name: "1h_RSI_60_Break_Bear_Short", Description: "1h RSI cross↓60+4h EMA down+MACD-+RSI4h<60", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehEMADeathCrossBearShort{}, Name: "1h_EMA_Death_Cross_Bear_Short", Description: "1h EMA9 cross↓EMA21+4h EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MTFCMFBothNegShort{}, Name: "MTF_CMF_Both_Neg_Short", Description: "4h CMF cross↓0+1h CMF<0+EMA down+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehFisherZeroCrossBearShort{}, Name: "1h_Fisher_Zero_Cross_Short", Description: "1h Fisher cross↓0+4h EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehWROBCrossBearShort{}, Name: "1h_WR_OB_Cross_Bear_Short", Description: "1h WR cross↓-20+4h EMA down+MACD-+RSI4h<62", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MTFMACDBothNegShort{}, Name: "MTF_MACD_Both_Neg_Short", Description: "4h+1h both MACD hist<0, 1h just turned neg+EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehCMFZeroCrossBearShort{}, Name: "1h_CMF_Zero_Cross_Bear_Short", Description: "1h CMF cross↓0+4h EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehStochRSIOBCrossBearShort{}, Name: "1h_StochRSI_OB_Cross_Bear_Short", Description: "1h StochRSI K cross↓D from>75+4h EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MTFRSICrossConfirmShort{}, Name: "MTF_RSI_Cross_Confirm_Short", Description: "4h RSI cross↓50+1h RSI<45+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehBBMidBreakBearShort{}, Name: "1h_BB_Mid_Break_Bear_Short", Description: "1h close cross↓BB mid+4h EMA down+MACD-+RSI4h<60", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehSupertrendBearShort{}, Name: "1h_Supertrend_Bear_Short", Description: "1h ST flips bearish+4h EMA down+MACD-+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehEFIZeroCrossBearShort{}, Name: "1h_EFI_Zero_Cross_Bear_Short", Description: "1h EFI cross↓0+4h EMA down+MACD-+RSI4h<60", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehRSIMidBearCrossShort{}, Name: "1h_RSI_Mid_Bear_Cross_Short", Description: "1h RSI cross↓50+4h RSI<55+EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehOBVBearSMACrossShort{}, Name: "1h_OBV_Bear_SMA_Cross_Short", Description: "1h OBV cross↓14-SMA+4h EMA down+MACD-+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehKeltnerBreakBearShort{}, Name: "1h_Keltner_Break_Bear_Short", Description: "1h close cross↓KC lower+4h EMA down+MACD-+ADX>20", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnehDonchianBreakBearShort{}, Name: "1h_Donchian_Break_Bear_Short", Description: "1h close breaks Donchian lower+4h EMA down+MACD-", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &OnedMACDNeg4hEntryShort{}, Name: "1d_MACD_Neg_4h_Entry_Short", Description: "4h MACD line+hist both neg+RSI cross↓55+EMA down+ADX>18", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
