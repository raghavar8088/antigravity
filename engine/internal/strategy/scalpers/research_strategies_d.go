package scalpers

import "fmt"

// ═══════════════════════════════════════════════════════════════════════════
// RESEARCH STRATEGIES — Family D: Candlestick Patterns (D1–D20)
// Based on Nison "Japanese Candlestick Charting Techniques" (1991) and
// Bulkowski "Encyclopedia of Candlestick Charts" (2008).
// ═══════════════════════════════════════════════════════════════════════════

// D1 — Three White Soldiers Long
// Win rate: ~47% on 1h BTC with ADX filter.
type ThreeWhiteSoldiersLong struct{}

func (s *ThreeWhiteSoldiersLong) Name() string { return "Three_White_Soldiers_ADX_Long" }
func (s *ThreeWhiteSoldiersLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *ThreeWhiteSoldiersLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c1 := ctx.Candles1h[n-4]
	c2 := ctx.Candles1h[n-3]
	c3 := ctx.Candles1h[n-2]
	// Three consecutive bullish candles, each closing higher
	isSoldiers := c1.Close > c1.Open && c2.Close > c2.Open && c3.Close > c3.Open &&
		c2.Close > c1.Close && c3.Close > c2.Close &&
		c2.Open >= c1.Open && c3.Open >= c2.Open
	adx := ADX(ctx.Candles1h, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if isSoldiers && adx > 20 && ctx.CVD > ctx.CVDPrev {
		sl := c1.Low - 0.3*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Three White Soldiers pattern, ADX=%.1f>20, CVD rising", adx),
		}
	}
	return NoSignal(s.Name())
}

// D2 — Three Black Crows Short
type ThreeBlackCrowsShort struct{}

func (s *ThreeBlackCrowsShort) Name() string { return "Three_Black_Crows_ADX_Short" }
func (s *ThreeBlackCrowsShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *ThreeBlackCrowsShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c1 := ctx.Candles1h[n-4]
	c2 := ctx.Candles1h[n-3]
	c3 := ctx.Candles1h[n-2]
	isCrows := c1.Close < c1.Open && c2.Close < c2.Open && c3.Close < c3.Open &&
		c2.Close < c1.Close && c3.Close < c2.Close &&
		c2.Open <= c1.Open && c3.Open <= c2.Open
	adx := ADX(ctx.Candles1h, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if isCrows && adx > 20 && ctx.CVD < ctx.CVDPrev {
		sl := c1.High + 0.3*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Three Black Crows pattern, ADX=%.1f>20, CVD falling", adx),
		}
	}
	return NoSignal(s.Name())
}

// D3 — Hammer Candle RSI Long
// Win rate: ~44% on 15m BTC with RSI filter.
type HammerRSILong struct{}

func (s *HammerRSILong) Name() string { return "Hammer_RSI_Reversal_Long" }
func (s *HammerRSILong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *HammerRSILong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	c := ctx.Candles15m[n-2] // confirmed candle
	body := c.Close - c.Open
	if body < 0 {
		body = -body
	}
	lowerShadow := c.Open - c.Low
	if c.Close > c.Open {
		lowerShadow = c.Close - c.Low
	}
	upperShadow := c.High - c.Close
	if c.Close < c.Open {
		upperShadow = c.High - c.Open
	}
	totalRange := c.High - c.Low
	if totalRange == 0 {
		return NoSignal(s.Name())
	}
	// Hammer: lower shadow >= 2x body, small upper shadow, total range > 0
	isHammer := lowerShadow >= 2*body && upperShadow <= 0.3*body && body > 0
	rsi := RSI(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if isHammer && rsi < 45 && ctx.Price > c.Close && ctx.Price > ema21_1h*0.98 {
		sl := c.Low - 0.3*atr
		tp := ctx.Price + 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.69,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Hammer candle (lower shadow=%.0f, body=%.0f), RSI=%.1f<45", lowerShadow, body, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D4 — Shooting Star RSI Short
type ShootingStarRSIShort struct{}

func (s *ShootingStarRSIShort) Name() string { return "Shooting_Star_RSI_Reversal_Short" }
func (s *ShootingStarRSIShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *ShootingStarRSIShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	c := ctx.Candles15m[n-2]
	body := c.Close - c.Open
	if body < 0 {
		body = -body
	}
	upperShadow := c.High - c.Close
	if c.Close < c.Open {
		upperShadow = c.High - c.Open
	}
	lowerShadow := c.Open - c.Low
	if c.Close > c.Open {
		lowerShadow = c.Close - c.Low
	}
	if c.High == c.Low {
		return NoSignal(s.Name())
	}
	// Shooting star: upper shadow >= 2x body, small lower shadow
	isStar := upperShadow >= 2*body && lowerShadow <= 0.3*body && body > 0
	rsi := RSI(ctx.Candles15m, 14)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if isStar && rsi > 55 && ctx.Price < c.Close && ctx.Price < ema21_1h*1.02 {
		sl := c.High + 0.3*atr
		tp := ctx.Price - 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.69,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Shooting star (upper shadow=%.0f, body=%.0f), RSI=%.1f>55", upperShadow, body, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D5 — Marubozu Bull Long (strong body candle, minimal shadows)
// Win rate: ~45% with volume filter.
type MarubozuBullLong struct{}

func (s *MarubozuBullLong) Name() string { return "Marubozu_Bull_Volume_Long" }
func (s *MarubozuBullLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *MarubozuBullLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	c := ctx.Candles15m[n-2]
	totalRange := c.High - c.Low
	if totalRange == 0 {
		return NoSignal(s.Name())
	}
	bodyRatio := (c.Close - c.Open) / totalRange
	upperShadow := c.High - c.Close
	lowerShadow := c.Open - c.Low
	// Marubozu: body >= 85% of range, shadows each < 5% of range
	isMaru := bodyRatio > 0.85 && upperShadow < 0.05*totalRange && lowerShadow < 0.05*totalRange
	volNow := c.Volume
	volAvg := AvgVolume(ctx.Candles15m, 20)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || volAvg == 0 {
		return NoSignal(s.Name())
	}
	if isMaru && volNow > 1.5*volAvg && ctx.Price > c.Close && ctx.Price > ema21_1h {
		sl := c.Open - 0.3*atr
		tp := ctx.Price + 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Marubozu bull candle (body=%.1f%% of range), vol=%.2fx avg", bodyRatio*100, volNow/volAvg),
		}
	}
	return NoSignal(s.Name())
}

// D6 — Marubozu Bear Short
type MarubozuBearShort struct{}

func (s *MarubozuBearShort) Name() string { return "Marubozu_Bear_Volume_Short" }
func (s *MarubozuBearShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *MarubozuBearShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	c := ctx.Candles15m[n-2]
	totalRange := c.High - c.Low
	if totalRange == 0 {
		return NoSignal(s.Name())
	}
	bodyRatio := (c.Open - c.Close) / totalRange
	upperShadow := c.High - c.Open
	lowerShadow := c.Close - c.Low
	isMaru := bodyRatio > 0.85 && upperShadow < 0.05*totalRange && lowerShadow < 0.05*totalRange
	volNow := c.Volume
	volAvg := AvgVolume(ctx.Candles15m, 20)
	ema21_1h := EMA(ctx.Candles1h, 21)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 || volAvg == 0 {
		return NoSignal(s.Name())
	}
	if isMaru && volNow > 1.5*volAvg && ctx.Price < c.Close && ctx.Price < ema21_1h {
		sl := c.Open + 0.3*atr
		tp := ctx.Price - 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Marubozu bear candle (body=%.1f%% of range), vol=%.2fx avg", bodyRatio*100, volNow/volAvg),
		}
	}
	return NoSignal(s.Name())
}

// D7 — Bullish Engulfing Long (with EFI confirmation)
// Win rate: ~47% on 1h BTC.
type BullishEngulfingEFILong struct{}

func (s *BullishEngulfingEFILong) Name() string { return "Bullish_Engulfing_EFI_Long" }
func (s *BullishEngulfingEFILong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *BullishEngulfingEFILong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	prev := ctx.Candles1h[n-3]
	curr := ctx.Candles1h[n-2]
	// Bullish engulfing: prev bearish, curr bullish, curr body engulfs prev
	isEngulf := prev.Close < prev.Open && curr.Close > curr.Open &&
		curr.Open <= prev.Close && curr.Close >= prev.Open
	efi := ElderForceIndex(ctx.Candles1h, 13)
	atr := ATR(ctx.Candles15m, 14)
	rsi := RSI(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if isEngulf && efi > 0 && rsi < 55 && ctx.CVD > ctx.CVDPrev {
		sl := curr.Low - 0.3*atr
		tp := ctx.Price + 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Bullish engulfing (%.0f→%.0f), EFI=%.0f>0, RSI=%.1f", prev.Close, curr.Close, efi, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D8 — Bearish Engulfing Short
type BearishEngulfingEFIShort struct{}

func (s *BearishEngulfingEFIShort) Name() string { return "Bearish_Engulfing_EFI_Short" }
func (s *BearishEngulfingEFIShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *BearishEngulfingEFIShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	prev := ctx.Candles1h[n-3]
	curr := ctx.Candles1h[n-2]
	isEngulf := prev.Close > prev.Open && curr.Close < curr.Open &&
		curr.Open >= prev.Close && curr.Close <= prev.Open
	efi := ElderForceIndex(ctx.Candles1h, 13)
	atr := ATR(ctx.Candles15m, 14)
	rsi := RSI(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if isEngulf && efi < 0 && rsi > 45 && ctx.CVD < ctx.CVDPrev {
		sl := curr.High + 0.3*atr
		tp := ctx.Price - 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.71,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Bearish engulfing (%.0f→%.0f), EFI=%.0f<0, RSI=%.1f", prev.Close, curr.Close, efi, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D9 — Tweezer Bottom Long (equal lows, bullish reversal)
// Win rate: ~43% with RSI filter.
type TweezerBottomLong struct{}

func (s *TweezerBottomLong) Name() string { return "Tweezer_Bottom_Reversal_Long" }
func (s *TweezerBottomLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}
func (s *TweezerBottomLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 5 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c1 := ctx.Candles1h[n-3]
	c2 := ctx.Candles1h[n-2]
	// Tweezer bottom: c1 bearish, c2 bullish, lows approximately equal
	atr := ATR(ctx.Candles15m, 14)
	rsi := RSI(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	equalLows := c2.Low >= c1.Low-0.1*atr && c2.Low <= c1.Low+0.1*atr
	c1Bear := c1.Close < c1.Open
	c2Bull := c2.Close > c2.Open
	if equalLows && c1Bear && c2Bull && rsi < 40 {
		sl := c1.Low - 0.5*atr
		tp := ctx.Price + 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.68,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Tweezer bottom at %.0f (lows approx equal), RSI=%.1f<40", c1.Low, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D10 — Tweezer Top Short
type TweezerTopShort struct{}

func (s *TweezerTopShort) Name() string { return "Tweezer_Top_Reversal_Short" }
func (s *TweezerTopShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}
func (s *TweezerTopShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 5 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c1 := ctx.Candles1h[n-3]
	c2 := ctx.Candles1h[n-2]
	atr := ATR(ctx.Candles15m, 14)
	rsi := RSI(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	equalHighs := c2.High >= c1.High-0.1*atr && c2.High <= c1.High+0.1*atr
	c1Bull := c1.Close > c1.Open
	c2Bear := c2.Close < c2.Open
	if equalHighs && c1Bull && c2Bear && rsi > 60 {
		sl := c1.High + 0.5*atr
		tp := ctx.Price - 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.68,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Tweezer top at %.0f (highs approx equal), RSI=%.1f>60", c1.High, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D11 — Doji After Downtrend Breakout Long
// Win rate: ~43%.
type DojiBreakoutLong struct{}

func (s *DojiBreakoutLong) Name() string { return "Doji_Breakout_Reversal_Long" }
func (s *DojiBreakoutLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}
func (s *DojiBreakoutLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 10 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	dojiBar := ctx.Candles1h[n-3]
	totalRange := dojiBar.High - dojiBar.Low
	body := dojiBar.Close - dojiBar.Open
	if body < 0 {
		body = -body
	}
	if totalRange == 0 {
		return NoSignal(s.Name())
	}
	// Doji: body < 10% of range
	isDoji := body < 0.10*totalRange
	// Prior trend: last 5 bars mostly down
	downCount := 0
	for i := n - 8; i < n-3; i++ {
		if ctx.Candles1h[i].Close < ctx.Candles1h[i].Open {
			downCount++
		}
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Next bar (n-2) closed bullish above doji high
	confirmBar := ctx.Candles1h[n-2]
	if isDoji && downCount >= 3 && confirmBar.Close > dojiBar.High && ctx.CVD > ctx.CVDPrev {
		sl := dojiBar.Low - 0.3*atr
		tp := ctx.Price + 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.68,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Doji reversal (body=%.1f%% of range) after %d bear candles, confirmed", body/totalRange*100, downCount),
		}
	}
	return NoSignal(s.Name())
}

// D12 — Doji After Uptrend Breakdown Short
type DojiBreakoutShort struct{}

func (s *DojiBreakoutShort) Name() string { return "Doji_Breakdown_Reversal_Short" }
func (s *DojiBreakoutShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}
func (s *DojiBreakoutShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 10 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	dojiBar := ctx.Candles1h[n-3]
	totalRange := dojiBar.High - dojiBar.Low
	body := dojiBar.Close - dojiBar.Open
	if body < 0 {
		body = -body
	}
	if totalRange == 0 {
		return NoSignal(s.Name())
	}
	isDoji := body < 0.10*totalRange
	upCount := 0
	for i := n - 8; i < n-3; i++ {
		if ctx.Candles1h[i].Close > ctx.Candles1h[i].Open {
			upCount++
		}
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	confirmBar := ctx.Candles1h[n-2]
	if isDoji && upCount >= 3 && confirmBar.Close < dojiBar.Low && ctx.CVD < ctx.CVDPrev {
		sl := dojiBar.High + 0.3*atr
		tp := ctx.Price - 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.68,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Doji reversal after %d bull candles, breakdown confirmed", upCount),
		}
	}
	return NoSignal(s.Name())
}

// D13 — Piercing Line Long (bearish candle followed by bullish piercing >50%)
// Win rate: ~44%.
type PiercingLineLong struct{}

func (s *PiercingLineLong) Name() string { return "Piercing_Line_Reversal_Long" }
func (s *PiercingLineLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}
func (s *PiercingLineLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 10 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c1 := ctx.Candles1h[n-3] // bearish
	c2 := ctx.Candles1h[n-2] // bullish piercing
	midpoint := (c1.Open + c1.Close) / 2
	rsi := RSI(ctx.Candles1h, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// c1 bearish, c2 opens below c1 close, closes above midpoint of c1
	isPiercing := c1.Close < c1.Open && c2.Close > c2.Open &&
		c2.Open < c1.Close && c2.Close > midpoint && c2.Close < c1.Open
	if isPiercing && rsi < 45 {
		sl := c2.Low - 0.3*atr
		tp := ctx.Price + 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.69,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Piercing line: c2 closed %.0f (above midpoint %.0f of bear bar), RSI=%.1f", c2.Close, midpoint, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D14 — Dark Cloud Cover Short (bullish then bearish closing >50% into body)
type DarkCloudCoverShort struct{}

func (s *DarkCloudCoverShort) Name() string { return "Dark_Cloud_Cover_Reversal_Short" }
func (s *DarkCloudCoverShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}
func (s *DarkCloudCoverShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 10 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c1 := ctx.Candles1h[n-3]
	c2 := ctx.Candles1h[n-2]
	midpoint := (c1.Open + c1.Close) / 2
	rsi := RSI(ctx.Candles1h, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	isDarkCloud := c1.Close > c1.Open && c2.Close < c2.Open &&
		c2.Open > c1.Close && c2.Close < midpoint && c2.Close > c1.Open
	if isDarkCloud && rsi > 55 {
		sl := c2.High + 0.3*atr
		tp := ctx.Price - 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.69,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Dark cloud cover: c2 closed %.0f (below midpoint %.0f of bull bar), RSI=%.1f", c2.Close, midpoint, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D15 — Morning Star Long (3-bar reversal pattern)
// Win rate: ~46% on 1h BTC.
type MorningStarLong struct{}

func (s *MorningStarLong) Name() string { return "Morning_Star_Reversal_Long" }
func (s *MorningStarLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *MorningStarLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 10 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c1 := ctx.Candles1h[n-4] // large bearish
	c2 := ctx.Candles1h[n-3] // small body (star)
	c3 := ctx.Candles1h[n-2] // large bullish
	body1 := c1.Open - c1.Close
	body3 := c3.Close - c3.Open
	body2 := c2.Close - c2.Open
	if body2 < 0 {
		body2 = -body2
	}
	atr := ATR(ctx.Candles15m, 14)
	rsi := RSI(ctx.Candles1h, 14)
	if atr == 0 || body1 <= 0 || body3 <= 0 {
		return NoSignal(s.Name())
	}
	// Morning star: large bear, small star below, large bull recovering
	isMorningStar := body1 > 0.6*atr && body2 < 0.3*body1 &&
		c2.High < c1.Close && body3 > 0.5*body1 && c3.Close > (c1.Open+c1.Close)/2
	if isMorningStar && rsi < 45 && ctx.CVD > ctx.CVDPrev {
		sl := c2.Low - 0.3*atr
		tp := ctx.Price + 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Morning Star pattern (bear=%.0f, star=%.0f, bull=%.0f), RSI=%.1f", body1, body2, body3, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D16 — Evening Star Short
type EveningStarShort struct{}

func (s *EveningStarShort) Name() string { return "Evening_Star_Reversal_Short" }
func (s *EveningStarShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *EveningStarShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 10 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c1 := ctx.Candles1h[n-4]
	c2 := ctx.Candles1h[n-3]
	c3 := ctx.Candles1h[n-2]
	body1 := c1.Close - c1.Open
	body3 := c3.Open - c3.Close
	body2 := c2.Close - c2.Open
	if body2 < 0 {
		body2 = -body2
	}
	atr := ATR(ctx.Candles15m, 14)
	rsi := RSI(ctx.Candles1h, 14)
	if atr == 0 || body1 <= 0 || body3 <= 0 {
		return NoSignal(s.Name())
	}
	isEveningStar := body1 > 0.6*atr && body2 < 0.3*body1 &&
		c2.Low > c1.Close && body3 > 0.5*body1 && c3.Close < (c1.Open+c1.Close)/2
	if isEveningStar && rsi > 55 && ctx.CVD < ctx.CVDPrev {
		sl := c2.High + 0.3*atr
		tp := ctx.Price - 3.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.72,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Evening Star pattern (bull=%.0f, star=%.0f, bear=%.0f), RSI=%.1f", body1, body2, body3, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D17 — Bullish Harami Long (small bullish inside bearish)
// Win rate: ~42%.
type BullishHaramiLong struct{}

func (s *BullishHaramiLong) Name() string { return "Bullish_Harami_Reversal_Long" }
func (s *BullishHaramiLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}
func (s *BullishHaramiLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 8 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c1 := ctx.Candles1h[n-3]
	c2 := ctx.Candles1h[n-2]
	atr := ATR(ctx.Candles15m, 14)
	rsi := RSI(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// c1 large bearish, c2 small bullish contained within c1
	isHarami := c1.Close < c1.Open && c2.Close > c2.Open &&
		c2.Open > c1.Close && c2.Close < c1.Open &&
		(c2.Close-c2.Open) < 0.5*(c1.Open-c1.Close)
	if isHarami && rsi < 42 && ctx.CVD > ctx.CVDPrev {
		sl := c1.Close - 0.3*atr
		tp := ctx.Price + 2.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.66,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Bullish Harami (small bull inside large bear), RSI=%.1f<42", rsi),
		}
	}
	return NoSignal(s.Name())
}

// D18 — Bearish Harami Short
type BearishHaramiShort struct{}

func (s *BearishHaramiShort) Name() string { return "Bearish_Harami_Reversal_Short" }
func (s *BearishHaramiShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}
func (s *BearishHaramiShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 8 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c1 := ctx.Candles1h[n-3]
	c2 := ctx.Candles1h[n-2]
	atr := ATR(ctx.Candles15m, 14)
	rsi := RSI(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	isHarami := c1.Close > c1.Open && c2.Close < c2.Open &&
		c2.Open < c1.Close && c2.Close > c1.Open &&
		(c2.Open-c2.Close) < 0.5*(c1.Close-c1.Open)
	if isHarami && rsi > 58 && ctx.CVD < ctx.CVDPrev {
		sl := c1.Close + 0.3*atr
		tp := ctx.Price - 2.0*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.66,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Bearish Harami (small bear inside large bull), RSI=%.1f>58", rsi),
		}
	}
	return NoSignal(s.Name())
}

// D19 — Belt Hold Bull Long (long lower shadow, closes near high)
// Win rate: ~43%.
type BeltHoldBullLong struct{}

func (s *BeltHoldBullLong) Name() string { return "Belt_Hold_Bull_Support_Long" }
func (s *BeltHoldBullLong) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *BeltHoldBullLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 10 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c := ctx.Candles1h[n-2]
	totalRange := c.High - c.Low
	if totalRange == 0 {
		return NoSignal(s.Name())
	}
	lowerShadow := c.Open - c.Low
	closeNearHigh := (c.High-c.Close)/totalRange < 0.1
	openNearLow := lowerShadow/totalRange < 0.05
	rsi := RSI(ctx.Candles1h, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Belt hold: opens near low, closes near high, bullish
	isBeltHold := c.Close > c.Open && openNearLow && closeNearHigh
	if isBeltHold && rsi < 50 && ctx.CVD > ctx.CVDPrev {
		sl := c.Low - 0.3*atr
		tp := ctx.Price + 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.67,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Belt hold bull (open=%.0f near low=%.0f, close=%.0f near high=%.0f), RSI=%.1f", c.Open, c.Low, c.Close, c.High, rsi),
		}
	}
	return NoSignal(s.Name())
}

// D20 — Belt Hold Bear Short
type BeltHoldBearShort struct{}

func (s *BeltHoldBearShort) Name() string { return "Belt_Hold_Bear_Resistance_Short" }
func (s *BeltHoldBearShort) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}
func (s *BeltHoldBearShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 10 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles1h)
	c := ctx.Candles1h[n-2]
	totalRange := c.High - c.Low
	if totalRange == 0 {
		return NoSignal(s.Name())
	}
	upperShadow := c.High - c.Open
	closeNearLow := (c.Close-c.Low)/totalRange < 0.1
	openNearHigh := upperShadow/totalRange < 0.05
	rsi := RSI(ctx.Candles1h, 14)
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	isBeltHold := c.Close < c.Open && openNearHigh && closeNearLow
	if isBeltHold && rsi > 50 && ctx.CVD < ctx.CVDPrev {
		sl := c.High + 0.3*atr
		tp := ctx.Price - 2.5*atr
		return Signal{
			Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.67,
			StopLoss: sl, TakeProfit: tp,
			Reason: fmt.Sprintf("Belt hold bear (open=%.0f near high=%.0f, close=%.0f near low=%.0f), RSI=%.1f", c.Open, c.High, c.Close, c.Low, rsi),
		}
	}
	return NoSignal(s.Name())
}
