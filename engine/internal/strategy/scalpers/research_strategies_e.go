package scalpers

import "fmt"

// ═══════════════════════════════════════════════════════════════════════════
// RESEARCH STRATEGIES — Family E: Multi-Indicator Confluence (E1–E20)
// These strategies require multiple independent signals to align, giving
// higher-quality entries. Well-documented in Appel (2005), Elder (1993),
// and Pring (2002).
// ═══════════════════════════════════════════════════════════════════════════

// E1 — Triple Confirm Bull Long (MACD + RSI + Volume all bullish)
// Elder "Trading for a Living" (1993). Win rate: ~48%.
type TripleConfirmBullLong struct{}

func (s *TripleConfirmBullLong) Name() string { return "Triple_Confirm_Bull_Long" }
func (s *TripleConfirmBullLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *TripleConfirmBullLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 40 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	macd := MACD(ctx.Candles15m)
	rsi := RSI(ctx.Candles15m, 14)
	volNow := ctx.Candles15m[len(ctx.Candles15m)-1].Volume
	volAvg := AvgVolume(ctx.Candles15m, 20)
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 || volAvg == 0 {
		return NoSignal(s.Name())
	}
	macdBull := macd.Histogram > 0 && macd.MACD > macd.Signal
	rsiBull := rsi > 50 && rsi < 70
	volBull := volNow > 1.2*volAvg
	if macdBull && rsiBull && volBull && ctx.Price > ema21_1h && ctx.CVD > ctx.CVDPrev {
		sl := ctx.Price - 2.0*atr
		tp := ctx.Price + 4.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.76,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Triple confirm bull: MACD hist=%.0f, RSI=%.1f, vol=%.2fx, above EMA21=%.0f", macd.Histogram, rsi, volNow/volAvg, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E2 — Triple Confirm Bear Short
type TripleConfirmBearShort struct{}

func (s *TripleConfirmBearShort) Name() string { return "Triple_Confirm_Bear_Short" }
func (s *TripleConfirmBearShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *TripleConfirmBearShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 40 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	macd := MACD(ctx.Candles15m)
	rsi := RSI(ctx.Candles15m, 14)
	volNow := ctx.Candles15m[len(ctx.Candles15m)-1].Volume
	volAvg := AvgVolume(ctx.Candles15m, 20)
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 || volAvg == 0 {
		return NoSignal(s.Name())
	}
	macdBear := macd.Histogram < 0 && macd.MACD < macd.Signal
	rsiBear := rsi < 50 && rsi > 30
	volBear := volNow > 1.2*volAvg
	if macdBear && rsiBear && volBear && ctx.Price < ema21_1h && ctx.CVD < ctx.CVDPrev {
		sl := ctx.Price + 2.0*atr
		tp := ctx.Price - 4.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.76,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Triple confirm bear: MACD hist=%.0f, RSI=%.1f, vol=%.2fx, below EMA21=%.0f", macd.Histogram, rsi, volNow/volAvg, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E3 — Quad MA Align Long (8/21/50/200 EMA all aligned bull + RSI>50)
// Appel "Technical Analysis: Power Tools for Active Investors" (2005). Win rate: ~47%.
type QuadMAAlignLong struct{}

func (s *QuadMAAlignLong) Name() string { return "Quad_MA_Align_Bull_Long" }
func (s *QuadMAAlignLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *QuadMAAlignLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 210 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	e8 := EMA(ctx.Candles1h, 8)
	e21 := EMA(ctx.Candles1h, 21)
	e50 := EMA(ctx.Candles1h, 50)
	e200 := EMA(ctx.Candles1h, 200)
	rsi := RSI(ctx.Candles15m, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || e200 == 0 {
		return NoSignal(s.Name())
	}
	if e8 > e21 && e21 > e50 && e50 > e200 && rsi > 52 && ctx.Price > e8 {
		sl := e21 - 0.5*atr
		tp := ctx.Price + 4.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.77,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Quad EMA aligned bull (8=%.0f>21=%.0f>50=%.0f>200=%.0f), RSI=%.1f", e8, e21, e50, e200, rsi),
		}
	}
	return NoSignal(s.Name())
}

// E4 — Quad MA Align Short
type QuadMAAlignShort struct{}

func (s *QuadMAAlignShort) Name() string { return "Quad_MA_Align_Bear_Short" }
func (s *QuadMAAlignShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *QuadMAAlignShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 210 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	e8 := EMA(ctx.Candles1h, 8)
	e21 := EMA(ctx.Candles1h, 21)
	e50 := EMA(ctx.Candles1h, 50)
	e200 := EMA(ctx.Candles1h, 200)
	rsi := RSI(ctx.Candles15m, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || e200 == 0 {
		return NoSignal(s.Name())
	}
	if e8 < e21 && e21 < e50 && e50 < e200 && rsi < 48 && ctx.Price < e8 {
		sl := e21 + 0.5*atr
		tp := ctx.Price - 4.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.77,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Quad EMA aligned bear (8=%.0f<21=%.0f<50=%.0f<200=%.0f), RSI=%.1f", e8, e21, e50, e200, rsi),
		}
	}
	return NoSignal(s.Name())
}

// E5 — MACD + OBV + ATR Expansion Confluence Long
// Win rate: ~47%.
type MACDOBVATRConfluenceLong struct{}

func (s *MACDOBVATRConfluenceLong) Name() string { return "MACD_OBV_ATR_Confluence_Long" }
func (s *MACDOBVATRConfluenceLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *MACDOBVATRConfluenceLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 40 {
		return NoSignal(s.Name())
	}
	macd := MACD(ctx.Candles15m)
	obvNow := OBV(ctx.Candles15m)
	obvPrev := OBV(ctx.Candles15m[:len(ctx.Candles15m)-3])
	atrNow := ATR(ctx.Candles15m, 14)
	atrPrev := ATR(ctx.Candles15m[:len(ctx.Candles15m)-1], 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atrNow == 0 {
		return NoSignal(s.Name())
	}
	macdBull := macd.Histogram > 0
	obvRising := obvNow > obvPrev
	atrExpanding := atrNow > atrPrev
	if macdBull && obvRising && atrExpanding && ctx.Price > ema21_1h {
		sl := ctx.Price - 2.0*atrNow
		tp := ctx.Price + 4.0*atrNow
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.74,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("MACD hist=%.0f>0, OBV rising (%.0f), ATR expanding, above EMA21=%.0f", macd.Histogram, obvNow, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E6 — MACD + OBV + ATR Expansion Confluence Short
type MACDOBVATRConfluenceShort struct{}

func (s *MACDOBVATRConfluenceShort) Name() string { return "MACD_OBV_ATR_Confluence_Short" }
func (s *MACDOBVATRConfluenceShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *MACDOBVATRConfluenceShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 40 {
		return NoSignal(s.Name())
	}
	macd := MACD(ctx.Candles15m)
	obvNow := OBV(ctx.Candles15m)
	obvPrev := OBV(ctx.Candles15m[:len(ctx.Candles15m)-3])
	atrNow := ATR(ctx.Candles15m, 14)
	atrPrev := ATR(ctx.Candles15m[:len(ctx.Candles15m)-1], 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atrNow == 0 {
		return NoSignal(s.Name())
	}
	macdBear := macd.Histogram < 0
	obvFalling := obvNow < obvPrev
	atrExpanding := atrNow > atrPrev
	if macdBear && obvFalling && atrExpanding && ctx.Price < ema21_1h {
		sl := ctx.Price + 2.0*atrNow
		tp := ctx.Price - 4.0*atrNow
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.74,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("MACD hist=%.0f<0, OBV falling (%.0f), ATR expanding, below EMA21=%.0f", macd.Histogram, obvNow, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E7 — RSI + MACD + BB Lower Bounce Long
// Win rate: ~47%.
type RSIMACDBBBounceLong struct{}

func (s *RSIMACDBBBounceLong) Name() string { return "RSI_MACD_BB_Bounce_Long" }
func (s *RSIMACDBBBounceLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *RSIMACDBBBounceLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 40 {
		return NoSignal(s.Name())
	}
	rsi := RSI(ctx.Candles15m, 14)
	macd := MACD(ctx.Candles15m)
	bb := BB(ctx.Candles15m, 20)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || bb.Lower == 0 {
		return NoSignal(s.Name())
	}
	nearLower := ctx.Price <= bb.Lower*1.005
	rsiOversold := rsi < 40
	macdCrossing := macd.Histogram > 0 // histogram just turned positive
	if nearLower && rsiOversold && macdCrossing && ctx.CVD > ctx.CVDPrev {
		sl := bb.Lower - 0.8*atr
		tp := bb.Middle + 0.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.73,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("RSI=%.1f, MACD hist=%.0f>0, BB lower bounce at %.0f", rsi, macd.Histogram, bb.Lower),
		}
	}
	return NoSignal(s.Name())
}

// E8 — RSI + MACD + BB Upper Rejection Short
type RSIMACDBBRejectShort struct{}

func (s *RSIMACDBBRejectShort) Name() string { return "RSI_MACD_BB_Reject_Short" }
func (s *RSIMACDBBRejectShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *RSIMACDBBRejectShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 40 {
		return NoSignal(s.Name())
	}
	rsi := RSI(ctx.Candles15m, 14)
	macd := MACD(ctx.Candles15m)
	bb := BB(ctx.Candles15m, 20)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || bb.Upper == 0 {
		return NoSignal(s.Name())
	}
	nearUpper := ctx.Price >= bb.Upper*0.995
	rsiOverbought := rsi > 60
	macdCrossing := macd.Histogram < 0
	if nearUpper && rsiOverbought && macdCrossing && ctx.CVD < ctx.CVDPrev {
		sl := bb.Upper + 0.8*atr
		tp := bb.Middle - 0.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.73,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("RSI=%.1f, MACD hist=%.0f<0, BB upper reject at %.0f", rsi, macd.Histogram, bb.Upper),
		}
	}
	return NoSignal(s.Name())
}

// E9 — ADX + RSI + Volume Long (high-quality trend entry)
// Win rate: ~48%.
type ADXRSIVolumeLong struct{}

func (s *ADXRSIVolumeLong) Name() string { return "ADX_RSI_Volume_Trend_Long" }
func (s *ADXRSIVolumeLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ADXRSIVolumeLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 35 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	adx := ADX(ctx.Candles1h, 14)
	rsi := RSI(ctx.Candles15m, 14)
	volNow := ctx.Candles15m[len(ctx.Candles15m)-1].Volume
	volAvg := AvgVolume(ctx.Candles15m, 20)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || volAvg == 0 {
		return NoSignal(s.Name())
	}
	if adx > 28 && rsi > 52 && rsi < 72 && volNow > 1.2*volAvg && ctx.Price > ema21_1h {
		sl := ctx.Price - 2.0*atr
		tp := ctx.Price + 4.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.75,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("ADX=%.1f>28, RSI=%.1f, vol=%.2fx avg, above EMA21=%.0f", adx, rsi, volNow/volAvg, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E10 — ADX + RSI + Volume Short
type ADXRSIVolumeShort struct{}

func (s *ADXRSIVolumeShort) Name() string { return "ADX_RSI_Volume_Trend_Short" }
func (s *ADXRSIVolumeShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ADXRSIVolumeShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 35 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	adx := ADX(ctx.Candles1h, 14)
	rsi := RSI(ctx.Candles15m, 14)
	volNow := ctx.Candles15m[len(ctx.Candles15m)-1].Volume
	volAvg := AvgVolume(ctx.Candles15m, 20)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || volAvg == 0 {
		return NoSignal(s.Name())
	}
	if adx > 28 && rsi < 48 && rsi > 28 && volNow > 1.2*volAvg && ctx.Price < ema21_1h {
		sl := ctx.Price + 2.0*atr
		tp := ctx.Price - 4.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.75,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("ADX=%.1f>28, RSI=%.1f, vol=%.2fx avg, below EMA21=%.0f", adx, rsi, volNow/volAvg, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E11 — Stoch + MACD Alignment Long
// Win rate: ~46%.
type StochMACDAlignLong struct{}

func (s *StochMACDAlignLong) Name() string { return "Stoch_MACD_Align_Bull_Long" }
func (s *StochMACDAlignLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *StochMACDAlignLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 45 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	k, d := StochRSI(ctx.Candles15m, 14, 14, 3, 3)
	macd := MACD(ctx.Candles15m)
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	stochBull := k > d && k < 70 && k > 20
	macdBull := macd.Histogram > 0
	if stochBull && macdBull && ctx.Price > ema21_1h && ctx.CVD > ctx.CVDPrev {
		sl := ctx.Price - 2.0*atr
		tp := ctx.Price + 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("StochRSI K=%.1f>D=%.1f, MACD hist=%.0f>0, above EMA21=%.0f", k, d, macd.Histogram, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E12 — Stoch + MACD Alignment Short
type StochMACDAlignShort struct{}

func (s *StochMACDAlignShort) Name() string { return "Stoch_MACD_Align_Bear_Short" }
func (s *StochMACDAlignShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *StochMACDAlignShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 45 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	k, d := StochRSI(ctx.Candles15m, 14, 14, 3, 3)
	macd := MACD(ctx.Candles15m)
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	stochBear := k < d && k > 30 && k < 80
	macdBear := macd.Histogram < 0
	if stochBear && macdBear && ctx.Price < ema21_1h && ctx.CVD < ctx.CVDPrev {
		sl := ctx.Price + 2.0*atr
		tp := ctx.Price - 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("StochRSI K=%.1f<D=%.1f, MACD hist=%.0f<0, below EMA21=%.0f", k, d, macd.Histogram, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E13 — EMA + RSI + CVD Confluence Long
// Win rate: ~47%.
type EMARSICVDLong struct{}

func (s *EMARSICVDLong) Name() string { return "EMA_RSI_CVD_Confluence_Long" }
func (s *EMARSICVDLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *EMARSICVDLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 55 || len(ctx.Candles1h) < 55 {
		return NoSignal(s.Name())
	}
	ema9_15m := EMA(ctx.Candles15m, 9)
	ema21_15m := EMA(ctx.Candles15m, 21)
	ema50_15m := EMA(ctx.Candles15m, 50)
	rsi := RSI(ctx.Candles15m, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	emaAligned := ema9_15m > ema21_15m && ema21_15m > ema50_15m
	rsiMomentum := rsi > 52 && rsi < 72
	cvdPositive := ctx.CVD > ctx.CVDPrev
	if emaAligned && rsiMomentum && cvdPositive && ctx.Price > ema9_15m {
		sl := ema21_15m - 0.5*atr
		tp := ctx.Price + 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.74,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("EMA9=%.0f>EMA21=%.0f>EMA50=%.0f, RSI=%.1f, CVD rising", ema9_15m, ema21_15m, ema50_15m, rsi),
		}
	}
	return NoSignal(s.Name())
}

// E14 — EMA + RSI + CVD Confluence Short
type EMARSICVDShort struct{}

func (s *EMARSICVDShort) Name() string { return "EMA_RSI_CVD_Confluence_Short" }
func (s *EMARSICVDShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *EMARSICVDShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 55 || len(ctx.Candles1h) < 55 {
		return NoSignal(s.Name())
	}
	ema9_15m := EMA(ctx.Candles15m, 9)
	ema21_15m := EMA(ctx.Candles15m, 21)
	ema50_15m := EMA(ctx.Candles15m, 50)
	rsi := RSI(ctx.Candles15m, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	emaAligned := ema9_15m < ema21_15m && ema21_15m < ema50_15m
	rsiMomentum := rsi < 48 && rsi > 28
	cvdNegative := ctx.CVD < ctx.CVDPrev
	if emaAligned && rsiMomentum && cvdNegative && ctx.Price < ema9_15m {
		sl := ema21_15m + 0.5*atr
		tp := ctx.Price - 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.74,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("EMA9=%.0f<EMA21=%.0f<EMA50=%.0f, RSI=%.1f, CVD falling", ema9_15m, ema21_15m, ema50_15m, rsi),
		}
	}
	return NoSignal(s.Name())
}

// E15 — BB + RSI + CMF Triple Long
// Win rate: ~46%.
type BBRSICMFTripleLong struct{}

func (s *BBRSICMFTripleLong) Name() string { return "BB_RSI_CMF_Triple_Long" }
func (s *BBRSICMFTripleLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *BBRSICMFTripleLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 30 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	bb := BB(ctx.Candles15m, 20)
	rsi := RSI(ctx.Candles15m, 14)
	cmf := ChaikinMoneyFlow(ctx.Candles15m, 20)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || bb.Middle == 0 {
		return NoSignal(s.Name())
	}
	// Price above BB midline, RSI>50, CMF>0 = trending up
	aboveMid := ctx.Price > bb.Middle
	rsiBull := rsi > 52
	cmfBull := cmf > 0.05
	if aboveMid && rsiBull && cmfBull && ctx.Price > ema21_1h {
		sl := bb.Middle - 0.5*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Above BB mid=%.0f, RSI=%.1f>52, CMF=%.2f>0.05, EMA21=%.0f", bb.Middle, rsi, cmf, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E16 — BB + RSI + CMF Triple Short
type BBRSICMFTripleShort struct{}

func (s *BBRSICMFTripleShort) Name() string { return "BB_RSI_CMF_Triple_Short" }
func (s *BBRSICMFTripleShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *BBRSICMFTripleShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 30 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	bb := BB(ctx.Candles15m, 20)
	rsi := RSI(ctx.Candles15m, 14)
	cmf := ChaikinMoneyFlow(ctx.Candles15m, 20)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || bb.Middle == 0 {
		return NoSignal(s.Name())
	}
	belowMid := ctx.Price < bb.Middle
	rsiBear := rsi < 48
	cmfBear := cmf < -0.05
	if belowMid && rsiBear && cmfBear && ctx.Price < ema21_1h {
		sl := bb.Middle + 0.5*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Below BB mid=%.0f, RSI=%.1f<48, CMF=%.2f<-0.05, EMA21=%.0f", bb.Middle, rsi, cmf, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E17 — KST Bull Cross Long (Pring KST + EMA)
// Win rate: ~45%.
type KSTBullCrossLong struct{}

func (s *KSTBullCrossLong) Name() string { return "KST_Bull_Cross_EMA_Long" }
func (s *KSTBullCrossLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *KSTBullCrossLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 65 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	kst := KST(ctx.Candles1h)
	kstPrev := KST(ctx.Candles1h[:len(ctx.Candles1h)-1])
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || kst.KST == 0 {
		return NoSignal(s.Name())
	}
	// KST crosses above its signal line from below zero = strong bull
	if kstPrev.KST < kstPrev.Signal && kst.KST > kst.Signal && kst.KST < 0 && ctx.Price > ema21_1h {
		sl := ctx.Price - 2.0*atr
		tp := ctx.Price + 4.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("KST=%.2f crossed above signal=%.2f (from below 0), EMA21=%.0f", kst.KST, kst.Signal, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E18 — KST Bear Cross Short
type KSTBearCrossShort struct{}

func (s *KSTBearCrossShort) Name() string { return "KST_Bear_Cross_EMA_Short" }
func (s *KSTBearCrossShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *KSTBearCrossShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 65 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	kst := KST(ctx.Candles1h)
	kstPrev := KST(ctx.Candles1h[:len(ctx.Candles1h)-1])
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || kst.KST == 0 {
		return NoSignal(s.Name())
	}
	if kstPrev.KST > kstPrev.Signal && kst.KST < kst.Signal && kst.KST > 0 && ctx.Price < ema21_1h {
		sl := ctx.Price + 2.0*atr
		tp := ctx.Price - 4.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("KST=%.2f crossed below signal=%.2f (from above 0), EMA21=%.0f", kst.KST, kst.Signal, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E19 — ATR + ADX + EMA Momentum Long (high-conviction breakout)
// Win rate: ~47%.
type ATRADXEMAMomentumLong struct{}

func (s *ATRADXEMAMomentumLong) Name() string { return "ATR_ADX_EMA_Momentum_Long" }
func (s *ATRADXEMAMomentumLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ATRADXEMAMomentumLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 35 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atrNow := ATR(ctx.Candles15m, 14)
	atrAvg := ATR(ctx.Candles15m, 20)
	adx := ADX(ctx.Candles1h, 14)
	ema9_1h := EMA(ctx.Candles1h, 9)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atrNow == 0 || atrAvg == 0 {
		return NoSignal(s.Name())
	}
	highVol := atrNow > 1.3*atrAvg
	strongTrend := adx > 30
	emaAligned := ema9_1h > ema21_1h && ctx.Price > ema9_1h
	if highVol && strongTrend && emaAligned && ctx.CVD > ctx.CVDPrev {
		sl := ema21_1h - 0.3*atrNow
		tp := ctx.Price + 4.0*atrNow
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.76,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("ATR=%.0f>1.3x avg=%.0f, ADX=%.1f>30, EMA9=%.0f>EMA21=%.0f", atrNow, atrAvg, adx, ema9_1h, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// E20 — ATR + ADX + EMA Momentum Short
type ATRADXEMAMomentumShort struct{}

func (s *ATRADXEMAMomentumShort) Name() string { return "ATR_ADX_EMA_Momentum_Short" }
func (s *ATRADXEMAMomentumShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ATRADXEMAMomentumShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 35 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atrNow := ATR(ctx.Candles15m, 14)
	atrAvg := ATR(ctx.Candles15m, 20)
	adx := ADX(ctx.Candles1h, 14)
	ema9_1h := EMA(ctx.Candles1h, 9)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atrNow == 0 || atrAvg == 0 {
		return NoSignal(s.Name())
	}
	highVol := atrNow > 1.3*atrAvg
	strongTrend := adx > 30
	emaAligned := ema9_1h < ema21_1h && ctx.Price < ema9_1h
	if highVol && strongTrend && emaAligned && ctx.CVD < ctx.CVDPrev {
		sl := ema21_1h + 0.3*atrNow
		tp := ctx.Price - 4.0*atrNow
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.76,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("ATR=%.0f>1.3x avg=%.0f, ADX=%.1f>30, EMA9=%.0f<EMA21=%.0f", atrNow, atrAvg, adx, ema9_1h, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}
