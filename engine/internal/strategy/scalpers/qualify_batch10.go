package scalpers

import "fmt"

// qualify_batch10.go — 10 new candidate strategies built for the "10 more
// qualified strategies" follow-up task. Each is a genuinely new combination
// of already-implemented indicator primitives (RSI, CMF, ADX, OBV, HMA,
// Donchian, Keltner, Fisher, StochRSI, WMA, MACD, BB) on the 4h/1h timeframe
// pair — the same pair used by nearly all 22 strategies that already passed
// the strict OOS bar. No new math is implemented; all indicator calls reuse
// existing functions in indicators.go. Registered via buildQualifyBatch10()
// and wired into BuildAllScalpers() so they flow through the normal
// backtest/qualification harness like every other candidate.

// ── QB1: RSI 55-break + CMF + ADX Bear Short ────────────────────────────────
type RSI55CMFADXBearShortV2 struct{}

func (s *RSI55CMFADXBearShortV2) Name() string           { return "RSI55_CMF_ADX_Bear_V2_Short" }
func (s *RSI55CMFADXBearShortV2) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *RSI55CMFADXBearShortV2) Evaluate(ctx MarketContext) Signal {
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
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.02 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 16 {
		return NoSignal(name)
	}
	if EMA(ctx.Candles4h, 8) >= EMA(ctx.Candles4h, 21) {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("RSI cross↓55(%.1f)+CMF(%.3f)<-0.02+ADX+EMA down. SL=%.2f%%", rsi, cmf, slDist/ctx.Price*100)}
}

// ── QB2: HMA slope-turn + ADX + CMF Bear Short ──────────────────────────────
type HMAADXCMFBearShort struct{}

func (s *HMAADXCMFBearShort) Name() string           { return "HMA_ADX_CMF_Bear_Short" }
func (s *HMAADXCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *HMAADXCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	h := HMA(ctx.Candles4h, 9)
	hPrev := HMA(ctx.Candles4h[:n4h-1], 9)
	hPrev2 := HMA(ctx.Candles4h[:n4h-2], 9)
	if !(hPrev2 <= hPrev && hPrev > h) { // slope turned down
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 18 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= 0 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("HMA9 slope turn down+ADX+CMF(%.3f)<0. SL=%.2f%%", cmf, slDist/ctx.Price*100)}
}

// ── QB3: Donchian mid-break + ADX + CMF Bear Short ──────────────────────────
type DonchianADXCMFBearShort struct{}

func (s *DonchianADXCMFBearShort) Name() string           { return "Donchian_ADX_CMF_Bear_Short" }
func (s *DonchianADXCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *DonchianADXCMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	if ADX(ctx.Candles4h, 14) < 17 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.01 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Donchian mid-break down+ADX+CMF(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100)}
}

// ── QB4: Keltner mid-break + ADX + CMF Bear Short ───────────────────────────
type KeltnerADXCMFBearShort struct{}

func (s *KeltnerADXCMFBearShort) Name() string           { return "Keltner_ADX_CMF_Bear_Short" }
func (s *KeltnerADXCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *KeltnerADXCMFBearShort) Evaluate(ctx MarketContext) Signal {
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
	if ADX(ctx.Candles4h, 14) < 17 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.01 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Keltner mid-break down+ADX+CMF(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100)}
}

// ── QB5: Fisher zero-cross + ADX + OBV slope Bear Short ─────────────────────
type FisherADXOBVBearShort struct{}

func (s *FisherADXOBVBearShort) Name() string           { return "Fisher_ADX_OBV_Bear_Short" }
func (s *FisherADXOBVBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *FisherADXOBVBearShort) Evaluate(ctx MarketContext) Signal {
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
	if ADX(ctx.Candles4h, 14) < 16 {
		return NoSignal(name)
	}
	obvNow := OBV(ctx.Candles4h)
	obvPrev := OBV(ctx.Candles4h[:n4h-3])
	if obvNow >= obvPrev {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("Fisher cross↓0(%.2f)+ADX+OBV declining. SL=%.2f%%", fish, slDist/ctx.Price*100)}
}

// ── QB6: WMA5/WMA13 cross + ADX + OBV Bear Short ────────────────────────────
type WMAADXOBVBearShort struct{}

func (s *WMAADXOBVBearShort) Name() string           { return "WMA_ADX_OBV_Bear_Short" }
func (s *WMAADXOBVBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *WMAADXOBVBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	w5 := WMA(ctx.Candles4h, 5)
	w13 := WMA(ctx.Candles4h, 13)
	w5Prev := WMA(ctx.Candles4h[:n4h-1], 5)
	w13Prev := WMA(ctx.Candles4h[:n4h-1], 13)
	if !(w5Prev >= w13Prev && w5 < w13) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 17 {
		return NoSignal(name)
	}
	obvNow := OBV(ctx.Candles4h)
	obvPrev := OBV(ctx.Candles4h[:n4h-3])
	if obvNow >= obvPrev {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("WMA5 cross↓WMA13+ADX+OBV declining. SL=%.2f%%", slDist/ctx.Price*100)}
}

// ── QB7: EFI zero-cross + ADX + CMF Bear Short (relaxed variant) ───────────
type EFIADXCMFBearShortV2 struct{}

func (s *EFIADXCMFBearShortV2) Name() string           { return "EFI_ADX_CMF_Bear_V2_Short" }
func (s *EFIADXCMFBearShortV2) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *EFIADXCMFBearShortV2) Evaluate(ctx MarketContext) Signal {
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
	if ADX(ctx.Candles4h, 14) < 16 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.01 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("EFI cross↓0+ADX+CMF(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100)}
}

// ── QB8: StochRSI K/D cross (relaxed 55 trigger, ADX>=15) Bear Short ────────
type StochRSIADXCMFBearShortV2 struct{}

func (s *StochRSIADXCMFBearShortV2) Name() string           { return "StochRSI_ADX_CMF_Bear_V2_Short" }
func (s *StochRSIADXCMFBearShortV2) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *StochRSIADXCMFBearShortV2) Evaluate(ctx MarketContext) Signal {
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
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 15 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("StochRSI K cross↓D from>55+CMF(%.3f)+ADX. SL=%.2f%%", cmf, slDist/ctx.Price*100)}
}

// ── QB9: MACD histogram zero-cross + ADX + CMF Bear Short ──────────────────
type MACDADXCMFBearShort struct{}

func (s *MACDADXCMFBearShort) Name() string           { return "MACD_ADX_CMF_Bear_Short" }
func (s *MACDADXCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *MACDADXCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	m := MACD(ctx.Candles4h)
	mPrev := MACD(ctx.Candles4h[:n4h-1])
	if mPrev.Histogram < 0 || m.Histogram >= 0 {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 16 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.01 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("MACD hist cross↓0+ADX+CMF(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100)}
}

// ── QB10: BB mid-break + ADX + CMF Bear Short ───────────────────────────────
type BBMidADXCMFBearShort struct{}

func (s *BBMidADXCMFBearShort) Name() string           { return "BB_Mid_ADX_CMF_Bear_V2_Short" }
func (s *BBMidADXCMFBearShort) ValidRegimes() []Regime { return []Regime{RegimeTrending} }
func (s *BBMidADXCMFBearShort) Evaluate(ctx MarketContext) Signal {
	name := s.Name()
	n4h := len(ctx.Candles4h)
	if n4h < 40 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}
	mid := BB(ctx.Candles4h, 20).Middle
	closePrev := ctx.Candles4h[n4h-2].Close
	closeNow := ctx.Candles4h[n4h-1].Close
	if !(closePrev >= mid && closeNow < mid) {
		return NoSignal(name)
	}
	if ADX(ctx.Candles4h, 14) < 16 {
		return NoSignal(name)
	}
	cmf := ChaikinMoneyFlow(ctx.Candles4h, 20)
	if cmf >= -0.01 {
		return NoSignal(name)
	}
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	sl, tp, slDist := hwSLTPShort(atr1h, ctx.Price)
	return Signal{Strategy: name, Direction: DirectionShort, Confidence: 0.8, StopLoss: sl, TakeProfit: tp,
		Reason: fmt.Sprintf("BB20-mid break down+ADX+CMF(%.3f). SL=%.2f%%", cmf, slDist/ctx.Price*100)}
}

// buildQualifyBatch10 registers the 10 new candidate strategies above.
func buildQualifyBatch10() []RegistryEntry {
	return []RegistryEntry{
		{Strategy: &RSI55CMFADXBearShortV2{}, Name: "RSI55_CMF_ADX_Bear_V2_Short", Description: "4h RSI cross↓55 + CMF<-0.02 + ADX>16 + EMA down", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &HMAADXCMFBearShort{}, Name: "HMA_ADX_CMF_Bear_Short", Description: "4h HMA9 slope turns down + ADX>18 + CMF<0", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &DonchianADXCMFBearShort{}, Name: "Donchian_ADX_CMF_Bear_Short", Description: "4h close breaks below Donchian(20) mid + ADX>17 + CMF<-0.01", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &KeltnerADXCMFBearShort{}, Name: "Keltner_ADX_CMF_Bear_Short", Description: "4h close breaks below Keltner mid + ADX>17 + CMF<-0.01", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &FisherADXOBVBearShort{}, Name: "Fisher_ADX_OBV_Bear_Short", Description: "4h Fisher cross↓0 + ADX>16 + OBV declining", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &WMAADXOBVBearShort{}, Name: "WMA_ADX_OBV_Bear_Short", Description: "4h WMA5 cross↓WMA13 + ADX>17 + OBV declining", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &EFIADXCMFBearShortV2{}, Name: "EFI_ADX_CMF_Bear_V2_Short", Description: "4h EFI(13) cross↓0 + ADX>16 + CMF<-0.01", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &StochRSIADXCMFBearShortV2{}, Name: "StochRSI_ADX_CMF_Bear_V2_Short", Description: "4h StochRSI K cross↓D from >55 + CMF<0 + ADX>15 (relaxed from StochRSI_CMF_Bear_Short)", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &MACDADXCMFBearShort{}, Name: "MACD_ADX_CMF_Bear_Short", Description: "4h MACD histogram cross↓0 + ADX>16 + CMF<-0.01", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
		{Strategy: &BBMidADXCMFBearShort{}, Name: "BB_Mid_ADX_CMF_Bear_V2_Short", Description: "4h close breaks below BB20 mid (SMA) + ADX>16 + CMF<-0.01", Regimes: []Regime{RegimeTrending}, Timeframes: []string{"4h", "1h"}, MaxPositions: 1, OHLCVCompatible: true},
	}
}
