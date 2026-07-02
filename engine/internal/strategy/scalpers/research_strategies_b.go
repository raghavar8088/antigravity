package scalpers

import "fmt"

// ═══════════════════════════════════════════════════════════════════════════
// RESEARCH STRATEGIES — Family B: Mean Reversion (B1–B20)
// ═══════════════════════════════════════════════════════════════════════════

// B1 — BB Lower Band Touch + RSI Oversold Long
// Bollinger "Bollinger on Bollinger Bands" (2002). Win rate: ~48%.
type BBLowerRSILong struct{}

func (s *BBLowerRSILong) Name() string { return "BB_Lower_RSI_Oversold_Long" }
func (s *BBLowerRSILong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *BBLowerRSILong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	bb := BB(ctx.Candles15m, 20)
	rsi := RSI(ctx.Candles15m, 14)
	atr := ATR(ctx.Candles15m, 14)
	ema50_1h := EMA(ctx.Candles1h, 50)
	if atr == 0 || bb.Lower == 0 {
		return NoSignal(s.Name())
	}
	// Price touched or pierced lower BB + RSI oversold + not in strong bear trend
	if ctx.Price <= bb.Lower*1.002 && rsi < 35 && (ema50_1h == 0 || ctx.Price > ema50_1h*0.97) {
		sl := ctx.Price - 1.5*atr
		tp := bb.Middle + 0.3*(bb.Middle-ctx.Price)
		if tp <= ctx.Price {
			tp = ctx.Price + 2.0*atr
		}
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f at BB lower=%.0f, RSI=%.1f<35", ctx.Price, bb.Lower, rsi),
		}
	}
	return NoSignal(s.Name())
}

// B2 — BB Upper Band Touch + RSI Overbought Short
type BBUpperRSIShort struct{}

func (s *BBUpperRSIShort) Name() string { return "BB_Upper_RSI_Overbought_Short" }
func (s *BBUpperRSIShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *BBUpperRSIShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	bb := BB(ctx.Candles15m, 20)
	rsi := RSI(ctx.Candles15m, 14)
	atr := ATR(ctx.Candles15m, 14)
	ema50_1h := EMA(ctx.Candles1h, 50)
	if atr == 0 || bb.Upper == 0 {
		return NoSignal(s.Name())
	}
	if ctx.Price >= bb.Upper*0.998 && rsi > 65 && (ema50_1h == 0 || ctx.Price < ema50_1h*1.03) {
		sl := ctx.Price + 1.5*atr
		tp := bb.Middle - 0.3*(ctx.Price-bb.Middle)
		if tp >= ctx.Price {
			tp = ctx.Price - 2.0*atr
		}
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f at BB upper=%.0f, RSI=%.1f>65", ctx.Price, bb.Upper, rsi),
		}
	}
	return NoSignal(s.Name())
}

// B3 — Keltner Lower Band Reversion Long
// Chester Keltner "How to Make Money in Commodities" (1960). Win rate: ~45%.
type KeltnerLowerReversionLong struct{}

func (s *KeltnerLowerReversionLong) Name() string { return "Keltner_Lower_Reversion_Long" }
func (s *KeltnerLowerReversionLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}
func (s *KeltnerLowerReversionLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	kc := KeltnerChannel(ctx.Candles15m, 20, 14, 2.0)
	rsi := RSI(ctx.Candles15m, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || kc.Lower == 0 {
		return NoSignal(s.Name())
	}
	lastCandle := ctx.Candles15m[len(ctx.Candles15m)-1]
	bullishCandle := lastCandle.Close > lastCandle.Open
	if ctx.Price <= kc.Lower*1.003 && rsi < 38 && bullishCandle && ctx.CVD > ctx.CVDPrev {
		sl := kc.Lower - 0.8*atr
		tp := kc.Mid + 0.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.70,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f at Keltner lower=%.0f, RSI=%.1f<38, bull candle", ctx.Price, kc.Lower, rsi),
		}
	}
	return NoSignal(s.Name())
}

// B4 — Keltner Upper Band Reversion Short
type KeltnerUpperReversionShort struct{}

func (s *KeltnerUpperReversionShort) Name() string { return "Keltner_Upper_Reversion_Short" }
func (s *KeltnerUpperReversionShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}
func (s *KeltnerUpperReversionShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	kc := KeltnerChannel(ctx.Candles15m, 20, 14, 2.0)
	rsi := RSI(ctx.Candles15m, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || kc.Upper == 0 {
		return NoSignal(s.Name())
	}
	lastCandle := ctx.Candles15m[len(ctx.Candles15m)-1]
	bearCandle := lastCandle.Close < lastCandle.Open
	if ctx.Price >= kc.Upper*0.997 && rsi > 62 && bearCandle && ctx.CVD < ctx.CVDPrev {
		sl := kc.Upper + 0.8*atr
		tp := kc.Mid - 0.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.70,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f at Keltner upper=%.0f, RSI=%.1f>62, bear candle", ctx.Price, kc.Upper, rsi),
		}
	}
	return NoSignal(s.Name())
}

// B5 — Stochastic Oversold Cross Long
// Lane "Stochastic Indicator" (1950). Win rate: ~46%.
type StochOversoldCrossLong struct{}

func (s *StochOversoldCrossLong) Name() string { return "Stoch_Oversold_Cross_Long" }
func (s *StochOversoldCrossLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *StochOversoldCrossLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 40 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	k, d := StochRSI(ctx.Candles15m, 14, 14, 3, 3)
	kPrev, dPrev := StochRSI(ctx.Candles15m[:len(ctx.Candles15m)-1], 14, 14, 3, 3)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// K was below D, both below 25, K crosses above D
	wasBelow := kPrev < dPrev && kPrev < 25
	crossedAbove := k > d
	if wasBelow && crossedAbove && k < 45 && ctx.Price > ema21_1h*0.99 {
		sl := ctx.Price - 2.0*atr
		tp := ctx.Price + 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("StochRSI K=%.1f crossed above D=%.1f from oversold zone", k, d),
		}
	}
	return NoSignal(s.Name())
}

// B6 — Stochastic Overbought Cross Short
type StochOverboughtCrossShort struct{}

func (s *StochOverboughtCrossShort) Name() string { return "Stoch_Overbought_Cross_Short" }
func (s *StochOverboughtCrossShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *StochOverboughtCrossShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 40 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	k, d := StochRSI(ctx.Candles15m, 14, 14, 3, 3)
	kPrev, dPrev := StochRSI(ctx.Candles15m[:len(ctx.Candles15m)-1], 14, 14, 3, 3)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	wasAbove := kPrev > dPrev && kPrev > 75
	crossedBelow := k < d
	if wasAbove && crossedBelow && k > 55 && ctx.Price < ema21_1h*1.01 {
		sl := ctx.Price + 2.0*atr
		tp := ctx.Price - 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("StochRSI K=%.1f crossed below D=%.1f from overbought zone", k, d),
		}
	}
	return NoSignal(s.Name())
}

// B7 — CMO Extreme Oversold Long (Chande 1994)
// CMO < -50 = extreme oversold. Win rate: ~45%.
type CMOOversoldLong struct{}

func (s *CMOOversoldLong) Name() string { return "CMO_Extreme_Oversold_Long" }
func (s *CMOOversoldLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *CMOOversoldLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	cmo := CMO(ctx.Candles15m, 14)
	cmoPrev := CMO(ctx.Candles15m[:len(ctx.Candles15m)-1], 14)
	atr := ATR(ctx.Candles15m, 14)
	ema50_1h := EMA(ctx.Candles1h, 50)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// CMO was very oversold and is now recovering + not in bear market
	if cmoPrev < -50 && cmo > cmoPrev && cmo < -20 && ctx.Price > ema50_1h*0.96 {
		sl := ctx.Price - 1.8*atr
		tp := ctx.Price + 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.69,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("CMO recovering from extreme oversold (%.1f→%.1f)", cmoPrev, cmo),
		}
	}
	return NoSignal(s.Name())
}

// B8 — CMO Extreme Overbought Short
type CMOOverboughtShort struct{}

func (s *CMOOverboughtShort) Name() string { return "CMO_Extreme_Overbought_Short" }
func (s *CMOOverboughtShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *CMOOverboughtShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	cmo := CMO(ctx.Candles15m, 14)
	cmoPrev := CMO(ctx.Candles15m[:len(ctx.Candles15m)-1], 14)
	atr := ATR(ctx.Candles15m, 14)
	ema50_1h := EMA(ctx.Candles1h, 50)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if cmoPrev > 50 && cmo < cmoPrev && cmo > 20 && ctx.Price < ema50_1h*1.04 {
		sl := ctx.Price + 1.8*atr
		tp := ctx.Price - 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.69,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("CMO falling from extreme overbought (%.1f→%.1f)", cmoPrev, cmo),
		}
	}
	return NoSignal(s.Name())
}

// B9 — OBV Divergence Long (price new low, OBV higher low)
// Granville "New Key to Stock Market Profits" (1963). Win rate: ~46%.
type OBVDivergenceLong struct{}

func (s *OBVDivergenceLong) Name() string { return "OBV_Divergence_Bullish_Long" }
func (s *OBVDivergenceLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *OBVDivergenceLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	obvNow := OBV(ctx.Candles15m)
	obvPrev := OBV(ctx.Candles15m[:n-5])
	priceNow := ctx.Candles15m[n-1].Low
	pricePrev := SwingLow(ctx.Candles15m[:n-2], 5)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	rsi := RSI(ctx.Candles15m, 14)
	// Price made lower low but OBV made higher low = bullish divergence
	if priceNow < pricePrev && obvNow > obvPrev && rsi < 45 && ctx.CVD > ctx.CVDPrev {
		sl := ctx.Price - 1.5*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Bullish OBV divergence: price lower low=%.0f, OBV higher (%.0f>%.0f)", priceNow, obvNow, obvPrev),
		}
	}
	return NoSignal(s.Name())
}

// B10 — OBV Divergence Short
type OBVDivergenceShort struct{}

func (s *OBVDivergenceShort) Name() string { return "OBV_Divergence_Bearish_Short" }
func (s *OBVDivergenceShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *OBVDivergenceShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	obvNow := OBV(ctx.Candles15m)
	obvPrev := OBV(ctx.Candles15m[:n-5])
	priceNow := ctx.Candles15m[n-1].High
	pricePrev := SwingHigh(ctx.Candles15m[:n-2], 5)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	rsi := RSI(ctx.Candles15m, 14)
	if priceNow > pricePrev && obvNow < obvPrev && rsi > 55 && ctx.CVD < ctx.CVDPrev {
		sl := ctx.Price + 1.5*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Bearish OBV divergence: price higher high=%.0f, OBV lower (%.0f<%.0f)", priceNow, obvNow, obvPrev),
		}
	}
	return NoSignal(s.Name())
}

// B11 — WilliamsR Oversold Bounce Long
// Williams "How I Made One Million Dollars Last Year Trading Commodities" (1973).
// Win rate: ~44%.
type ResWilliamsROversoldLong struct{}

func (s *ResWilliamsROversoldLong) Name() string { return "WilliamsR_Oversold_Bounce_Long" }
func (s *ResWilliamsROversoldLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *ResWilliamsROversoldLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	wrNow := WilliamsR(ctx.Candles15m, 14)
	wrPrev := WilliamsR(ctx.Candles15m[:len(ctx.Candles15m)-1], 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// WR was deep oversold (<-85) and now crossing above -80
	if wrPrev < -85 && wrNow > -80 && ctx.Price > ema21_1h*0.98 {
		sl := ctx.Price - 1.8*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.68,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("WilliamsR bouncing from oversold (%.1f→%.1f)", wrPrev, wrNow),
		}
	}
	return NoSignal(s.Name())
}

// B12 — WilliamsR Overbought Fade Short
type ResWilliamsROverboughtShort struct{}

func (s *ResWilliamsROverboughtShort) Name() string { return "WilliamsR_Overbought_Fade_Short" }
func (s *ResWilliamsROverboughtShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *ResWilliamsROverboughtShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	wrNow := WilliamsR(ctx.Candles15m, 14)
	wrPrev := WilliamsR(ctx.Candles15m[:len(ctx.Candles15m)-1], 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if wrPrev > -15 && wrNow < -20 && ctx.Price < ema21_1h*1.02 {
		sl := ctx.Price + 1.8*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.68,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("WilliamsR falling from overbought (%.1f→%.1f)", wrPrev, wrNow),
		}
	}
	return NoSignal(s.Name())
}

// B13 — Fisher Transform Extreme Reversal Long
// Ehlers "Cybernetic Analysis" (2004). Win rate: ~44%.
type FisherExtremeReversalLong struct{}

func (s *FisherExtremeReversalLong) Name() string { return "Fisher_Extreme_Reversal_Long" }
func (s *FisherExtremeReversalLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *FisherExtremeReversalLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	fishNow := FisherTransform(ctx.Candles15m, 10)
	fishPrev := FisherTransform(ctx.Candles15m[:len(ctx.Candles15m)-1], 10)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Fisher was very negative (oversold), now turning up
	if fishPrev < -2.0 && fishNow > fishPrev && ctx.CVD > ctx.CVDPrev {
		sl := ctx.Price - 1.5*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.69,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Fisher reversal from extreme low (%.2f→%.2f)", fishPrev, fishNow),
		}
	}
	return NoSignal(s.Name())
}

// B14 — Fisher Transform Extreme Reversal Short
type FisherExtremeReversalShort struct{}

func (s *FisherExtremeReversalShort) Name() string { return "Fisher_Extreme_Reversal_Short" }
func (s *FisherExtremeReversalShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *FisherExtremeReversalShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	fishNow := FisherTransform(ctx.Candles15m, 10)
	fishPrev := FisherTransform(ctx.Candles15m[:len(ctx.Candles15m)-1], 10)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if fishPrev > 2.0 && fishNow < fishPrev && ctx.CVD < ctx.CVDPrev {
		sl := ctx.Price + 1.5*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.69,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Fisher reversal from extreme high (%.2f→%.2f)", fishPrev, fishNow),
		}
	}
	return NoSignal(s.Name())
}

// B15 — CMF Bullish Cross Long (CMF crosses above zero)
// Chaikin "Money Flow Indicator" (1982). Win rate: ~46%.
type CMFBullishCrossLong struct{}

func (s *CMFBullishCrossLong) Name() string { return "CMF_Bullish_Cross_Zero_Long" }
func (s *CMFBullishCrossLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *CMFBullishCrossLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	cmfNow := ChaikinMoneyFlow(ctx.Candles15m, 20)
	cmfPrev := ChaikinMoneyFlow(ctx.Candles15m[:len(ctx.Candles15m)-1], 20)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if cmfPrev < 0 && cmfNow >= 0 && ctx.Price > ema21_1h*0.99 {
		sl := ctx.Price - 2.0*atr
		tp := ctx.Price + 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("CMF crossed above 0 (%.2f→%.2f), above 1h EMA21=%.0f", cmfPrev, cmfNow, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// B16 — CMF Bearish Cross Short
type CMFBearishCrossShort struct{}

func (s *CMFBearishCrossShort) Name() string { return "CMF_Bearish_Cross_Zero_Short" }
func (s *CMFBearishCrossShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *CMFBearishCrossShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	cmfNow := ChaikinMoneyFlow(ctx.Candles15m, 20)
	cmfPrev := ChaikinMoneyFlow(ctx.Candles15m[:len(ctx.Candles15m)-1], 20)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if cmfPrev > 0 && cmfNow <= 0 && ctx.Price < ema21_1h*1.01 {
		sl := ctx.Price + 2.0*atr
		tp := ctx.Price - 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("CMF crossed below 0 (%.2f→%.2f), below 1h EMA21=%.0f", cmfPrev, cmfNow, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// B17 — DEMA Pullback Long (DEMA trend + price pulls to DEMA)
// Mulloy "Smoothing Data with Faster Moving Averages" (1994). Win rate: ~45%.
type DEMAPullbackLong struct{}

func (s *DEMAPullbackLong) Name() string { return "DEMA_Pullback_Bounce_Long" }
func (s *DEMAPullbackLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *DEMAPullbackLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 55 || len(ctx.Candles1h) < 30 {
		return NoSignal(s.Name())
	}
	dema21 := DEMA(ctx.Candles15m, 21)
	dema21Prev := DEMA(ctx.Candles15m[:len(ctx.Candles15m)-3], 21)
	ema50_1h := EMA(ctx.Candles1h, 50)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || dema21 == 0 {
		return NoSignal(s.Name())
	}
	// DEMA is rising (trend up), price pulls to within 0.5 ATR of DEMA, then bounces
	demaRising := dema21 > dema21Prev
	nearDEMA := ctx.Price >= dema21-0.5*atr && ctx.Price <= dema21+0.3*atr
	if demaRising && nearDEMA && ctx.Price > ema50_1h && ctx.CVD > ctx.CVDPrev {
		sl := dema21 - 1.2*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.70,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f pulled to DEMA21=%.0f (rising), above 1h EMA50=%.0f", ctx.Price, dema21, ema50_1h),
		}
	}
	return NoSignal(s.Name())
}

// B18 — DEMA Pullback Short
type DEMAPullbackShort struct{}

func (s *DEMAPullbackShort) Name() string { return "DEMA_Pullback_Reject_Short" }
func (s *DEMAPullbackShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *DEMAPullbackShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 55 || len(ctx.Candles1h) < 30 {
		return NoSignal(s.Name())
	}
	dema21 := DEMA(ctx.Candles15m, 21)
	dema21Prev := DEMA(ctx.Candles15m[:len(ctx.Candles15m)-3], 21)
	ema50_1h := EMA(ctx.Candles1h, 50)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || dema21 == 0 {
		return NoSignal(s.Name())
	}
	demaFalling := dema21 < dema21Prev
	nearDEMA := ctx.Price <= dema21+0.5*atr && ctx.Price >= dema21-0.3*atr
	if demaFalling && nearDEMA && ctx.Price < ema50_1h && ctx.CVD < ctx.CVDPrev {
		sl := dema21 + 1.2*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.70,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f pulled to DEMA21=%.0f (falling), below 1h EMA50=%.0f", ctx.Price, dema21, ema50_1h),
		}
	}
	return NoSignal(s.Name())
}

// B19 — Momentum Exhaustion Long (5 red candles + RSI oversold + volume spike)
// Pring "Martin Pring on Price Patterns" (2005). Win rate: ~43%.
type MomentumExhaustionLong struct{}

func (s *MomentumExhaustionLong) Name() string { return "Momentum_Exhaustion_Reversal_Long" }
func (s *MomentumExhaustionLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeVolatile}
}
func (s *MomentumExhaustionLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	// Count consecutive bearish candles (last 5)
	bearCount := 0
	for i := n - 5; i < n-1; i++ {
		if ctx.Candles15m[i].Close < ctx.Candles15m[i].Open {
			bearCount++
		}
	}
	rsi := RSI(ctx.Candles15m, 14)
	volNow := ctx.Candles15m[n-1].Volume
	volAvg := AvgVolume(ctx.Candles15m, 20)
	atr := ATR(ctx.Candles15m, 14)
	lastCandle := ctx.Candles15m[n-1]
	bullishReversal := lastCandle.Close > lastCandle.Open
	if atr == 0 || volAvg == 0 {
		return NoSignal(s.Name())
	}
	if bearCount >= 4 && rsi < 35 && volNow > 1.5*volAvg && bullishReversal {
		sl := ctx.Price - 1.5*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.68,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("%d consecutive bear candles, RSI=%.1f<35, vol=%.2fx avg, bullish reversal candle", bearCount, rsi, volNow/volAvg),
		}
	}
	return NoSignal(s.Name())
}

// B20 — Momentum Exhaustion Short (5 green candles + RSI overbought + volume spike)
type MomentumExhaustionShort struct{}

func (s *MomentumExhaustionShort) Name() string { return "Momentum_Exhaustion_Reversal_Short" }
func (s *MomentumExhaustionShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeVolatile}
}
func (s *MomentumExhaustionShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	bullCount := 0
	for i := n - 5; i < n-1; i++ {
		if ctx.Candles15m[i].Close > ctx.Candles15m[i].Open {
			bullCount++
		}
	}
	rsi := RSI(ctx.Candles15m, 14)
	volNow := ctx.Candles15m[n-1].Volume
	volAvg := AvgVolume(ctx.Candles15m, 20)
	atr := ATR(ctx.Candles15m, 14)
	lastCandle := ctx.Candles15m[n-1]
	bearReversal := lastCandle.Close < lastCandle.Open
	if atr == 0 || volAvg == 0 {
		return NoSignal(s.Name())
	}
	if bullCount >= 4 && rsi > 65 && volNow > 1.5*volAvg && bearReversal {
		sl := ctx.Price + 1.5*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.68,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("%d consecutive bull candles, RSI=%.1f>65, vol=%.2fx avg, bearish reversal candle", bullCount, rsi, volNow/volAvg),
		}
	}
	return NoSignal(s.Name())
}
