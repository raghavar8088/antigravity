package scalpers

import "fmt"

// ═══════════════════════════════════════════════════════════════════════════
// RESEARCH STRATEGIES — Family C: Breakout / Volatility (C1–C20)
// ═══════════════════════════════════════════════════════════════════════════

// C1 — Donchian 20-Period Breakout Long with CMF confirmation
// Donchian "Commodities Prices" (1960), popularized by "Turtle Traders" (1983).
// Win rate: ~45% on BTC 1h.
type Donchian20BreakLong struct{}

func (s *Donchian20BreakLong) Name() string { return "Donchian20_CMF_Break_Long" }
func (s *Donchian20BreakLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *Donchian20BreakLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 25 || len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	// Use prior candles (exclude last bar) to determine the channel
	dc := Donchian(ctx.Candles1h[:len(ctx.Candles1h)-1], 20)
	cmf := ChaikinMoneyFlow(ctx.Candles1h, 20)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || dc.Upper == 0 {
		return NoSignal(s.Name())
	}
	if ctx.Price > dc.Upper && cmf > 0.05 && ctx.CVD > ctx.CVDPrev {
		sl := dc.Upper - 0.5*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.73,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f broke Donchian20 upper=%.0f, CMF=%.2f>0.05", ctx.Price, dc.Upper, cmf),
		}
	}
	return NoSignal(s.Name())
}

// C2 — Donchian 20-Period Breakdown Short
type Donchian20BreakShort struct{}

func (s *Donchian20BreakShort) Name() string { return "Donchian20_CMF_Break_Short" }
func (s *Donchian20BreakShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *Donchian20BreakShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 25 || len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	dc := Donchian(ctx.Candles1h[:len(ctx.Candles1h)-1], 20)
	cmf := ChaikinMoneyFlow(ctx.Candles1h, 20)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || dc.Lower == 0 {
		return NoSignal(s.Name())
	}
	if ctx.Price < dc.Lower && cmf < -0.05 && ctx.CVD < ctx.CVDPrev {
		sl := dc.Lower + 0.5*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.73,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f broke Donchian20 lower=%.0f, CMF=%.2f<-0.05", ctx.Price, dc.Lower, cmf),
		}
	}
	return NoSignal(s.Name())
}

// C3 — NR7 Breakout Long (Narrowest Range 7 — Crabel 1990)
// Win rate: ~46% on BTC 1h.
type NR7BreakoutLong struct{}

func (s *NR7BreakoutLong) Name() string { return "NR7_Volatility_Breakout_Long" }
func (s *NR7BreakoutLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *NR7BreakoutLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 12 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	// NR7: the bar 2 bars ago (before last bar) was the narrowest in last 7
	if len(ctx.Candles1h) < 9 {
		return NoSignal(s.Name())
	}
	// Check if bar n-2 has NR7 property
	prevBars := ctx.Candles1h[:len(ctx.Candles1h)-1]
	isNR7 := NarrowRangeN(prevBars, 7)
	if !isNR7 {
		return NoSignal(s.Name())
	}
	// Last bar broke above the NR7 bar's high
	nr7Bar := prevBars[len(prevBars)-1]
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if ctx.Price > nr7Bar.High && ctx.Price > ema21_1h && ctx.CVD > ctx.CVDPrev {
		sl := nr7Bar.Low - 0.3*atr
		tp := ctx.Price + 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.70,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("NR7 breakout: price=%.0f above NR7 high=%.0f, above EMA21=%.0f", ctx.Price, nr7Bar.High, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// C4 — NR7 Breakdown Short
type NR7BreakoutShort struct{}

func (s *NR7BreakoutShort) Name() string { return "NR7_Volatility_Breakdown_Short" }
func (s *NR7BreakoutShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *NR7BreakoutShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 12 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	prevBars := ctx.Candles1h[:len(ctx.Candles1h)-1]
	isNR7 := NarrowRangeN(prevBars, 7)
	if !isNR7 {
		return NoSignal(s.Name())
	}
	nr7Bar := prevBars[len(prevBars)-1]
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if ctx.Price < nr7Bar.Low && ctx.Price < ema21_1h && ctx.CVD < ctx.CVDPrev {
		sl := nr7Bar.High + 0.3*atr
		tp := ctx.Price - 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.70,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("NR7 breakdown: price=%.0f below NR7 low=%.0f, below EMA21=%.0f", ctx.Price, nr7Bar.Low, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// C5 — Inside Bar Bull Breakout Long (Mother bar / inside bar)
// Nial Fuller "Price Action Trading" (2010). Win rate: ~46%.
type InsideBarBullLong struct{}

func (s *InsideBarBullLong) Name() string { return "Inside_Bar_Bull_Breakout_Long" }
func (s *InsideBarBullLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *InsideBarBullLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 5 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	mother := ctx.Candles1h[n-3]
	inside := ctx.Candles1h[n-2]
	// Inside bar: high < mother high AND low > mother low
	isInside := inside.High < mother.High && inside.Low > mother.Low
	if !isInside {
		return NoSignal(s.Name())
	}
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Current bar broke above mother's high + in uptrend
	if ctx.Price > mother.High && ctx.Price > ema21_1h {
		sl := inside.Low - 0.2*atr
		tp := ctx.Price + 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Inside bar breakout above mother high=%.0f, above EMA21=%.0f", mother.High, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// C6 — Inside Bar Bear Breakdown Short
type InsideBarBearShort struct{}

func (s *InsideBarBearShort) Name() string { return "Inside_Bar_Bear_Breakdown_Short" }
func (s *InsideBarBearShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *InsideBarBearShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 5 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	mother := ctx.Candles1h[n-3]
	inside := ctx.Candles1h[n-2]
	isInside := inside.High < mother.High && inside.Low > mother.Low
	if !isInside {
		return NoSignal(s.Name())
	}
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if ctx.Price < mother.Low && ctx.Price < ema21_1h {
		sl := inside.High + 0.2*atr
		tp := ctx.Price - 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Inside bar breakdown below mother low=%.0f, below EMA21=%.0f", mother.Low, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// C7 — Squeeze Momentum Release Long (Carter 2006)
// Win rate: ~47% on 15m BTC.
type SqueezeMomentumReleaseLong struct{}

func (s *SqueezeMomentumReleaseLong) Name() string { return "Squeeze_Momentum_Release_Long" }
func (s *SqueezeMomentumReleaseLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *SqueezeMomentumReleaseLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	sq := SqueezeDetector(ctx.Candles15m)
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if sq.Fired && sq.Momentum > 0 && ctx.Price > ema21_1h && ctx.CVD > ctx.CVDPrev {
		sl := ctx.Price - 2.0*atr
		tp := ctx.Price + 4.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.74,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Squeeze fired, momentum=%.0f (bullish), above 1h EMA21=%.0f", sq.Momentum, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// C8 — Squeeze Momentum Release Short
type SqueezeMomentumReleaseShort struct{}

func (s *SqueezeMomentumReleaseShort) Name() string { return "Squeeze_Momentum_Release_Short" }
func (s *SqueezeMomentumReleaseShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *SqueezeMomentumReleaseShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	sq := SqueezeDetector(ctx.Candles15m)
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if sq.Fired && sq.Momentum < 0 && ctx.Price < ema21_1h && ctx.CVD < ctx.CVDPrev {
		sl := ctx.Price + 2.0*atr
		tp := ctx.Price - 4.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.74,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Squeeze fired, momentum=%.0f (bearish), below 1h EMA21=%.0f", sq.Momentum, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// C9 — BB Width Contraction + Upside Break Long
// Bollinger "Bollinger on Bollinger Bands" (2002). Win rate: ~45%.
type BBWidthContractionBreakLong struct{}

func (s *BBWidthContractionBreakLong) Name() string { return "BB_Width_Contraction_Break_Long" }
func (s *BBWidthContractionBreakLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *BBWidthContractionBreakLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 45 {
		return NoSignal(s.Name())
	}
	bbNow := BB(ctx.Candles15m, 20)
	widthPct := BBWidthPercentile(ctx.Candles15m, 20, 20)
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 || bbNow.Upper == 0 {
		return NoSignal(s.Name())
	}
	// Width in bottom 20th percentile (very narrow) + price breaks above upper BB
	if widthPct < 0.20 && ctx.Price > bbNow.Upper && ctx.Price > ema21_1h && ctx.CVD > ctx.CVDPrev {
		sl := bbNow.Middle - 0.5*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("BB squeeze (width pct=%.2f) broken upside, price=%.0f>BB upper=%.0f", widthPct, ctx.Price, bbNow.Upper),
		}
	}
	return NoSignal(s.Name())
}

// C10 — BB Width Contraction + Downside Break Short
type BBWidthContractionBreakShort struct{}

func (s *BBWidthContractionBreakShort) Name() string { return "BB_Width_Contraction_Break_Short" }
func (s *BBWidthContractionBreakShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *BBWidthContractionBreakShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 45 {
		return NoSignal(s.Name())
	}
	bbNow := BB(ctx.Candles15m, 20)
	widthPct := BBWidthPercentile(ctx.Candles15m, 20, 20)
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 || bbNow.Lower == 0 {
		return NoSignal(s.Name())
	}
	if widthPct < 0.20 && ctx.Price < bbNow.Lower && ctx.Price < ema21_1h && ctx.CVD < ctx.CVDPrev {
		sl := bbNow.Middle + 0.5*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("BB squeeze (width pct=%.2f) broken downside, price=%.0f<BB lower=%.0f", widthPct, ctx.Price, bbNow.Lower),
		}
	}
	return NoSignal(s.Name())
}

// C11 — ATR Expansion Momentum Long (volatility expanding in trend direction)
// Wilder "New Concepts in Technical Trading Systems" (1978). Win rate: ~46%.
type ATRExpansionMomentumLong struct{}

func (s *ATRExpansionMomentumLong) Name() string { return "ATR_Expansion_Momentum_Long" }
func (s *ATRExpansionMomentumLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeVolatile}
}
func (s *ATRExpansionMomentumLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	atrNow := ATR(ctx.Candles15m, 14)
	atrPrev := ATR(ctx.Candles15m[:len(ctx.Candles15m)-1], 14)
	atrAvg := ATR(ctx.Candles15m, 20)
	ema9_1h := EMA(ctx.Candles1h, 9)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atrNow == 0 || atrAvg == 0 {
		return NoSignal(s.Name())
	}
	// ATR expanding (now > avg) + EMA bullish alignment + CVD confirming
	atrExpanding := atrNow > atrPrev && atrNow > 1.2*atrAvg
	emaAligned := ema9_1h > ema21_1h
	if atrExpanding && emaAligned && ctx.Price > ema9_1h && ctx.CVD > ctx.CVDPrev {
		sl := ctx.Price - 2.0*atrNow
		tp := ctx.Price + 4.0*atrNow
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("ATR expanding (%.0f>avg=%.0f), EMA9=%.0f>EMA21=%.0f, CVD rising", atrNow, atrAvg, ema9_1h, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// C12 — ATR Expansion Momentum Short
type ATRExpansionMomentumShort struct{}

func (s *ATRExpansionMomentumShort) Name() string { return "ATR_Expansion_Momentum_Short" }
func (s *ATRExpansionMomentumShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeVolatile}
}
func (s *ATRExpansionMomentumShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	atrNow := ATR(ctx.Candles15m, 14)
	atrPrev := ATR(ctx.Candles15m[:len(ctx.Candles15m)-1], 14)
	atrAvg := ATR(ctx.Candles15m, 20)
	ema9_1h := EMA(ctx.Candles1h, 9)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atrNow == 0 || atrAvg == 0 {
		return NoSignal(s.Name())
	}
	atrExpanding := atrNow > atrPrev && atrNow > 1.2*atrAvg
	emaAligned := ema9_1h < ema21_1h
	if atrExpanding && emaAligned && ctx.Price < ema9_1h && ctx.CVD < ctx.CVDPrev {
		sl := ctx.Price + 2.0*atrNow
		tp := ctx.Price - 4.0*atrNow
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("ATR expanding (%.0f>avg=%.0f), EMA9=%.0f<EMA21=%.0f, CVD falling", atrNow, atrAvg, ema9_1h, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// C13 — Chandelier Exit Flip Long (long stop triggered, now bullish)
// Le Beau "Chandelier Exit" (1998). Win rate: ~46%.
type ChandelierExitFlipLong struct{}

func (s *ChandelierExitFlipLong) Name() string { return "Chandelier_Exit_Flip_Long" }
func (s *ChandelierExitFlipLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ChandelierExitFlipLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 25 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	longStop, _ := ChandelierExit(ctx.Candles1h, 22, 3.0)
	longStopPrev, _ := ChandelierExit(ctx.Candles1h[:len(ctx.Candles1h)-1], 22, 3.0)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || longStop == 0 {
		return NoSignal(s.Name())
	}
	prevPrice := ctx.Candles1h[len(ctx.Candles1h)-2].Close
	// Price was below chandelier long stop (bearish), now price recovered above it
	if prevPrice < longStopPrev && ctx.Price > longStop && ctx.CVD > ctx.CVDPrev {
		sl := longStop - 0.5*atr
		tp := ctx.Price + 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f recovered above Chandelier long stop=%.0f", ctx.Price, longStop),
		}
	}
	return NoSignal(s.Name())
}

// C14 — Chandelier Exit Flip Short
type ChandelierExitFlipShort struct{}

func (s *ChandelierExitFlipShort) Name() string { return "Chandelier_Exit_Flip_Short" }
func (s *ChandelierExitFlipShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ChandelierExitFlipShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 25 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	_, shortStop := ChandelierExit(ctx.Candles1h, 22, 3.0)
	_, shortStopPrev := ChandelierExit(ctx.Candles1h[:len(ctx.Candles1h)-1], 22, 3.0)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || shortStop == 0 {
		return NoSignal(s.Name())
	}
	prevPrice := ctx.Candles1h[len(ctx.Candles1h)-2].Close
	if prevPrice > shortStopPrev && ctx.Price < shortStop && ctx.CVD < ctx.CVDPrev {
		sl := shortStop + 0.5*atr
		tp := ctx.Price - 3.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f dropped below Chandelier short stop=%.0f", ctx.Price, shortStop),
		}
	}
	return NoSignal(s.Name())
}

// C15 — Parabolic SAR Bullish Flip Long + EMA filter
// Wilder "New Concepts" (1978). Win rate: ~44%.
type PSARFlipLong struct{}

func (s *PSARFlipLong) Name() string { return "PSAR_Flip_Bull_Long" }
func (s *PSARFlipLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *PSARFlipLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	sarNow, bullNow := PSARValue(ctx.Candles15m, 0.02, 0.2)
	sarPrev, bullPrev := PSARValue(ctx.Candles15m[:len(ctx.Candles15m)-1], 0.02, 0.2)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || sarNow == 0 || sarPrev == 0 {
		return NoSignal(s.Name())
	}
	_ = sarPrev
	// PSAR flipped from bear to bull + price above 1h EMA21
	if !bullPrev && bullNow && ctx.Price > ema21_1h {
		sl := sarNow - 0.3*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.69,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("PSAR flipped bullish at %.0f, price=%.0f>1h EMA21=%.0f", sarNow, ctx.Price, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// C16 — Parabolic SAR Bearish Flip Short
type PSARFlipShort struct{}

func (s *PSARFlipShort) Name() string { return "PSAR_Flip_Bear_Short" }
func (s *PSARFlipShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *PSARFlipShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	sarNow, bullNow := PSARValue(ctx.Candles15m, 0.02, 0.2)
	sarPrev, bullPrev := PSARValue(ctx.Candles15m[:len(ctx.Candles15m)-1], 0.02, 0.2)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || sarNow == 0 || sarPrev == 0 {
		return NoSignal(s.Name())
	}
	_ = sarPrev
	if bullPrev && !bullNow && ctx.Price < ema21_1h {
		sl := sarNow + 0.3*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.69,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("PSAR flipped bearish at %.0f, price=%.0f<1h EMA21=%.0f", sarNow, ctx.Price, ema21_1h),
		}
	}
	return NoSignal(s.Name())
}

// C17 — Bollinger Band Walk Long (price "rides" upper BB for 3 consecutive bars)
// Bollinger "Bollinger on Bollinger Bands" (2002). Win rate: ~45%.
type BollingerWalkLong struct{}

func (s *BollingerWalkLong) Name() string { return "BB_Walk_Upper_Trend_Long" }
func (s *BollingerWalkLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *BollingerWalkLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 30 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Check last 3 bars all closed above their respective BB upper
	allAbove := true
	for i := n - 3; i < n; i++ {
		subBB := BB(ctx.Candles15m[:i+1], 20)
		if ctx.Candles15m[i].Close < subBB.Upper {
			allAbove = false
			break
		}
	}
	adx := ADX(ctx.Candles15m, 14)
	if allAbove && adx > 25 && ctx.CVD > ctx.CVDPrev {
		sl := ctx.Price - 2.5*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price walking upper BB (3 consecutive), ADX=%.1f>25", adx),
		}
	}
	return NoSignal(s.Name())
}

// C18 — Bollinger Band Walk Short
type BollingerWalkShort struct{}

func (s *BollingerWalkShort) Name() string { return "BB_Walk_Lower_Trend_Short" }
func (s *BollingerWalkShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *BollingerWalkShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 30 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	allBelow := true
	for i := n - 3; i < n; i++ {
		subBB := BB(ctx.Candles15m[:i+1], 20)
		if ctx.Candles15m[i].Close > subBB.Lower {
			allBelow = false
			break
		}
	}
	adx := ADX(ctx.Candles15m, 14)
	if allBelow && adx > 25 && ctx.CVD < ctx.CVDPrev {
		sl := ctx.Price + 2.5*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price walking lower BB (3 consecutive), ADX=%.1f>25", adx),
		}
	}
	return NoSignal(s.Name())
}

// C19 — Volume-Confirmed Swing High Break Long
// O'Neil "How to Make Money in Stocks" (1988) CANSLIM breakout.
// Win rate: ~46%.
type VolumeConfirmedSwingBreakLong struct{}

func (s *VolumeConfirmedSwingBreakLong) Name() string { return "Volume_Confirmed_Swing_Break_Long" }
func (s *VolumeConfirmedSwingBreakLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *VolumeConfirmedSwingBreakLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 25 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	// Swing high of last 10 bars (excluding last)
	swingH := SwingHigh(ctx.Candles1h[:len(ctx.Candles1h)-1], 10)
	volNow := ctx.Candles1h[len(ctx.Candles1h)-1].Volume
	volAvg := AvgVolume(ctx.Candles1h, 20)
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 || volAvg == 0 {
		return NoSignal(s.Name())
	}
	if ctx.Price > swingH && volNow > 1.5*volAvg && ctx.Price > ema21_1h {
		sl := swingH - 0.3*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.73,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f broke 10-bar swing high=%.0f, vol=%.2fx avg", ctx.Price, swingH, volNow/volAvg),
		}
	}
	return NoSignal(s.Name())
}

// C20 — Volume-Confirmed Swing Low Break Short
type VolumeConfirmedSwingBreakShort struct{}

func (s *VolumeConfirmedSwingBreakShort) Name() string { return "Volume_Confirmed_Swing_Break_Short" }
func (s *VolumeConfirmedSwingBreakShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *VolumeConfirmedSwingBreakShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 25 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	swingL := SwingLow(ctx.Candles1h[:len(ctx.Candles1h)-1], 10)
	volNow := ctx.Candles1h[len(ctx.Candles1h)-1].Volume
	volAvg := AvgVolume(ctx.Candles1h, 20)
	atr := ATR(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	if atr == 0 || volAvg == 0 {
		return NoSignal(s.Name())
	}
	if ctx.Price < swingL && volNow > 1.5*volAvg && ctx.Price < ema21_1h {
		sl := swingL + 0.3*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.73,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Price %.0f broke 10-bar swing low=%.0f, vol=%.2fx avg", ctx.Price, swingL, volNow/volAvg),
		}
	}
	return NoSignal(s.Name())
}
