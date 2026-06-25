package scalpers

// ── S121: EFI Momentum ────────────────────────────────────────────────────────

type EFIMomentum struct{}

func (s *EFIMomentum) Name() string { return "EFI_Momentum" }
func (s *EFIMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *EFIMomentum) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	efi := ElderForceIndex(ctx.Candles15m, 13)
	adx := ADX(ctx.Candles15m, 14)
	if adx < 20 {
		return NoSignal(s.Name())
	}
	if efi > 0 && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.63,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 3.5*atr,
			Reason: "EFI_pos+CVD_bull+ADX", Timestamp: ctx.Now}
	}
	if efi < 0 && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.63,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 3.5*atr,
			Reason: "EFI_neg+CVD_bear+ADX", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S122: Donchian Squeeze ────────────────────────────────────────────────────

type DonchianSqueeze struct{}

func (s *DonchianSqueeze) Name() string { return "Donchian_Squeeze" }
func (s *DonchianSqueeze) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *DonchianSqueeze) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 25 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	dc20 := Donchian(ctx.Candles1h, 20)
	dc55 := Donchian(ctx.Candles1h, 55)
	channelRatio := (dc20.Upper - dc20.Lower) / (dc55.Upper - dc55.Lower + 1e-9)
	if channelRatio > 0.4 {
		return NoSignal(s.Name())
	}
	if ctx.Price > dc20.Upper {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: dc20.Mid, TakeProfit: ctx.Price + 3*atr,
			Reason: "Donchian_squeeze_upper_break", Timestamp: ctx.Now}
	}
	if ctx.Price < dc20.Lower {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: dc20.Mid, TakeProfit: ctx.Price - 3*atr,
			Reason: "Donchian_squeeze_lower_break", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S123: OBV Divergence 1h ───────────────────────────────────────────────────

type OBVDivergence1h struct{}

func (s *OBVDivergence1h) Name() string { return "OBV_Divergence_1h" }
func (s *OBVDivergence1h) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *OBVDivergence1h) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 30 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	obvSlice := OBVSlice(ctx.Candles1h)
	n := len(obvSlice)
	if n < 20 {
		return NoSignal(s.Name())
	}
	recentObvUp := obvSlice[n-1] > obvSlice[n-10]
	recentPriceDown := ctx.Candles1h[len(ctx.Candles1h)-1].Close < ctx.Candles1h[len(ctx.Candles1h)-10].Close
	recentObvDown := obvSlice[n-1] < obvSlice[n-10]
	recentPriceUp := ctx.Candles1h[len(ctx.Candles1h)-1].Close > ctx.Candles1h[len(ctx.Candles1h)-10].Close
	if recentObvUp && recentPriceDown {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "OBV_bull_divergence_1h", Timestamp: ctx.Now}
	}
	if recentObvDown && recentPriceUp {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "OBV_bear_divergence_1h", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S124: WilliamsR Swing ─────────────────────────────────────────────────────

type WilliamsRSwing struct{}

func (s *WilliamsRSwing) Name() string { return "WilliamsR_Swing" }
func (s *WilliamsRSwing) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *WilliamsRSwing) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || len(ctx.Candles4h) < 15 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	wr1h := WilliamsR(ctx.Candles1h, 14)
	wr4h := WilliamsR(ctx.Candles4h, 14)
	if wr1h < -80 && wr4h < -70 {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "WilliamsR_oversold_1h+4h", Timestamp: ctx.Now}
	}
	if wr1h > -20 && wr4h > -30 {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "WilliamsR_overbought_1h+4h", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S125: 4H Momentum Burst ───────────────────────────────────────────────────

type FourHMomentumBurst struct{}

func (s *FourHMomentumBurst) Name() string { return "4H_Momentum_Burst" }
func (s *FourHMomentumBurst) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *FourHMomentumBurst) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles4h) < 20 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atr4h := ATR(ctx.Candles4h, 14)
	atr15m := ATR(ctx.Candles15m, 14)
	if atr4h == 0 || atr15m == 0 {
		return NoSignal(s.Name())
	}
	adx := ADX(ctx.Candles4h, 14)
	if adx < 25 {
		return NoSignal(s.Name())
	}
	macd4h := MACD(ctx.Candles4h)
	if macd4h.Histogram > 0 && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.68,
			StopLoss: ctx.Price - 1.5*atr15m, TakeProfit: ctx.Price + 5*atr15m,
			Reason: "4H_MACD_bull_burst+CVD+ADX25", Timestamp: ctx.Now}
	}
	if macd4h.Histogram < 0 && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.68,
			StopLoss: ctx.Price + 1.5*atr15m, TakeProfit: ctx.Price - 5*atr15m,
			Reason: "4H_MACD_bear_burst+CVD+ADX25", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S126: Ichimoku Cloud Break ────────────────────────────────────────────────

type IchimokuCloudBreak struct{}

func (s *IchimokuCloudBreak) Name() string { return "Ichimoku_Cloud_Break" }
func (s *IchimokuCloudBreak) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *IchimokuCloudBreak) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles4h) < 60 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	ichi := Ichimoku(ctx.Candles4h)
	kumoTop := ichi.SpanA
	kumoBot := ichi.SpanB
	if ichi.SpanB > ichi.SpanA {
		kumoTop = ichi.SpanB
		kumoBot = ichi.SpanA
	}
	if ctx.Price > kumoTop && ichi.Tenkan > ichi.Kijun {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.67,
			StopLoss: kumoTop, TakeProfit: ctx.Price + 4*atr,
			Reason: "Ichimoku_cloud_break_bull_4h", Timestamp: ctx.Now}
	}
	if ctx.Price < kumoBot && ichi.Tenkan < ichi.Kijun {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.67,
			StopLoss: kumoBot, TakeProfit: ctx.Price - 4*atr,
			Reason: "Ichimoku_cloud_break_bear_4h", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S127: HMA Slope Shift ─────────────────────────────────────────────────────

type HMASlopeShift struct{}

func (s *HMASlopeShift) Name() string { return "HMA_Slope_Shift" }
func (s *HMASlopeShift) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *HMASlopeShift) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 30 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	nowHMA := HMA(ctx.Candles15m, 20)
	prevCandles := ctx.Candles15m[:len(ctx.Candles15m)-1]
	prevHMA := HMA(prevCandles, 20)
	slope := nowHMA - prevHMA
	if slope > 0.1*atr && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.64,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 3*atr,
			Reason: "HMA_slope_turned_positive+CVD_bull", Timestamp: ctx.Now}
	}
	if slope < -0.1*atr && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.64,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 3*atr,
			Reason: "HMA_slope_turned_negative+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S128: StochRSI 1h Reversal ────────────────────────────────────────────────

type StochRSI1hReversal struct{}

func (s *StochRSI1hReversal) Name() string { return "StochRSI_1h_Reversal" }
func (s *StochRSI1hReversal) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *StochRSI1hReversal) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 40 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	k, d := StochRSI(ctx.Candles1h, 14, 14, 3, 3)
	switch {
	case k < 20 && d < 20 && k > d:
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "StochRSI_1h_oversold_cross_up", Timestamp: ctx.Now}
	case k > 80 && d > 80 && k < d:
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "StochRSI_1h_overbought_cross_down", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S129: ZLEMA 1h Cross ──────────────────────────────────────────────────────

type ZLEMA1hCross struct{}

func (s *ZLEMA1hCross) Name() string { return "ZLEMA_1h_Cross" }
func (s *ZLEMA1hCross) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ZLEMA1hCross) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 55 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	fast := ZLEMA(ctx.Candles1h, 21)
	slow := ZLEMA(ctx.Candles1h, 55)
	if fast > slow && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "ZLEMA21_above_55_1h+CVD_bull", Timestamp: ctx.Now}
	}
	if fast < slow && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "ZLEMA21_below_55_1h+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S130: Multi-TF Supertrend Cascade ────────────────────────────────────────

type MultiTFSupertrendCascade struct{}

func (s *MultiTFSupertrendCascade) Name() string { return "Multi_TF_Supertrend_Cascade" }
func (s *MultiTFSupertrendCascade) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *MultiTFSupertrendCascade) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 || len(ctx.Candles1h) < 25 || len(ctx.Candles4h) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	st15m := Supertrend(ctx.Candles15m, 10, 3.0)
	st1h := Supertrend(ctx.Candles1h, 10, 3.0)
	st4h := Supertrend(ctx.Candles4h, 10, 3.0)
	if st15m.Direction == 1 && st1h.Direction == 1 && st4h.Direction == 1 {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.72,
			StopLoss: st15m.Level, TakeProfit: ctx.Price + 5*atr,
			Reason: "Supertrend_cascade_bull_15m+1h+4h", Timestamp: ctx.Now}
	}
	if st15m.Direction == -1 && st1h.Direction == -1 && st4h.Direction == -1 {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.72,
			StopLoss: st15m.Level, TakeProfit: ctx.Price - 5*atr,
			Reason: "Supertrend_cascade_bear_15m+1h+4h", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}
