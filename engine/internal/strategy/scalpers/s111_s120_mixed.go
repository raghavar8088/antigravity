package scalpers

// ── S111: Effective Spread Compression ───────────────────────────────────────

type EffectiveSpreadCompress struct{}

func (s *EffectiveSpreadCompress) Name() string { return "Effective_Spread_Compress" }
func (s *EffectiveSpreadCompress) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *EffectiveSpreadCompress) Evaluate(ctx MarketContext) Signal {
	if !ctx.MicrostructurePopulated || ctx.EffectiveSpreadAvg == 0 || len(ctx.Candles5m) < 30 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if ctx.EffectiveSpread > 0.5*ctx.EffectiveSpreadAvg {
		return NoSignal(s.Name())
	}
	macd := MACD(ctx.Candles5m)
	if macd.Histogram > 0 {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.62,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 3*atr,
			Reason: "spread_compressed+MACD_bull", Timestamp: ctx.Now}
	}
	if macd.Histogram < 0 {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.62,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 3*atr,
			Reason: "spread_compressed+MACD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S112: Last Trade Size Whale ───────────────────────────────────────────────

type LastTradeSizeWhale struct{}

func (s *LastTradeSizeWhale) Name() string { return "Last_Trade_Size_Whale" }
func (s *LastTradeSizeWhale) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *LastTradeSizeWhale) Evaluate(ctx MarketContext) Signal {
	if !ctx.MicrostructurePopulated || len(ctx.Candles5m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 || ctx.LastTradeSizeRatio < 5 {
		return NoSignal(s.Name())
	}
	imb := ctx.OrderBook.Imbalance
	switch {
	case ctx.CVD > ctx.CVDPrev && imb > 0.1:
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "whale_print+CVD_bull+OB_imb", Timestamp: ctx.Now}
	case ctx.CVD < ctx.CVDPrev && imb < -0.1:
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "whale_print+CVD_bear+OB_imb", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S113: Multi-TF Z-Score Long ──────────────────────────────────────────────

type MultiTFZScoreLong struct{}

func (s *MultiTFZScoreLong) Name() string { return "Multi_TF_ZScore_Long" }
func (s *MultiTFZScoreLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *MultiTFZScoreLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 25 || len(ctx.Candles15m) < 25 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	z5, z15, z1h := ZScoreMultiTimeframe(ctx.Candles5m, ctx.Candles15m, ctx.Candles1h)
	if z5 < -2.0 && z15 < -2.0 && z1h < -2.0 {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.68,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "multi_tf_zscore_all_extreme_low", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S114: Multi-TF Z-Score Short ─────────────────────────────────────────────

type MultiTFZScoreShort struct{}

func (s *MultiTFZScoreShort) Name() string { return "Multi_TF_ZScore_Short" }
func (s *MultiTFZScoreShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *MultiTFZScoreShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 25 || len(ctx.Candles15m) < 25 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	z5, z15, z1h := ZScoreMultiTimeframe(ctx.Candles5m, ctx.Candles15m, ctx.Candles1h)
	if z5 > 2.0 && z15 > 2.0 && z1h > 2.0 {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.68,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "multi_tf_zscore_all_extreme_high", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S115: Supertrend Rider ────────────────────────────────────────────────────

type SupertrendRider struct{}

func (s *SupertrendRider) Name() string { return "Supertrend_Rider" }
func (s *SupertrendRider) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *SupertrendRider) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 25 || len(ctx.Candles1h) < 25 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	st5m := Supertrend(ctx.Candles5m, 10, 3.0)
	st1h := Supertrend(ctx.Candles1h, 10, 3.0)
	if st5m.Direction != st1h.Direction {
		return NoSignal(s.Name())
	}
	if st5m.Direction == 1 {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: st5m.Level, TakeProfit: ctx.Price + 3*atr,
			Reason: "Supertrend_bull_5m+1h_aligned", Timestamp: ctx.Now}
	}
	return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
		StopLoss: st5m.Level, TakeProfit: ctx.Price - 3*atr,
		Reason: "Supertrend_bear_5m+1h_aligned", Timestamp: ctx.Now}
}

// ── S116: Chandelier Follow ───────────────────────────────────────────────────

type ChandelierFollow struct{}

func (s *ChandelierFollow) Name() string { return "Chandelier_Follow" }
func (s *ChandelierFollow) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ChandelierFollow) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 30 || len(ctx.Candles4h) < 10 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	adx := ADX(ctx.Candles15m, 14)
	if adx < 20 {
		return NoSignal(s.Name())
	}
	longStop, shortStop := ChandelierExit(ctx.Candles15m, 22, 3.0)
	h4bias := EMA(ctx.Candles4h, 20)
	switch {
	case ctx.Price > longStop && ctx.Price > h4bias:
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: longStop, TakeProfit: ctx.Price + 3*atr,
			Reason: "CE_above_long_stop+4h_bull_bias", Timestamp: ctx.Now}
	case ctx.Price < shortStop && ctx.Price < h4bias:
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: shortStop, TakeProfit: ctx.Price - 3*atr,
			Reason: "CE_below_short_stop+4h_bear_bias", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S117: PSAR Trend ──────────────────────────────────────────────────────────

type PSARTrend struct{}

func (s *PSARTrend) Name() string { return "PSAR_Trend" }
func (s *PSARTrend) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *PSARTrend) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	adx := ADX(ctx.Candles15m, 14)
	if adx < 20 {
		return NoSignal(s.Name())
	}
	sar, bullish := PSARValue(ctx.Candles15m, 0.02, 0.2)
	switch {
	case bullish && ctx.CVD > ctx.CVDPrev:
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.64,
			StopLoss: sar, TakeProfit: ctx.Price + 3*atr,
			Reason: "PSAR_bullish+ADX+CVD_bull", Timestamp: ctx.Now}
	case !bullish && ctx.CVD < ctx.CVDPrev:
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.64,
			StopLoss: sar, TakeProfit: ctx.Price - 3*atr,
			Reason: "PSAR_bearish+ADX+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S118: DEMA Pullback ───────────────────────────────────────────────────────

type DEMAPullback struct{}

func (s *DEMAPullback) Name() string { return "DEMA_Pullback" }
func (s *DEMAPullback) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *DEMAPullback) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 60 || len(ctx.Candles15m) < 30 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	fast := DEMA(ctx.Candles1h, 21)
	slow := DEMA(ctx.Candles1h, 55)
	if fast <= slow {
		return NoSignal(s.Name())
	}
	// Pullback: 15m price near the 1h DEMA(21)
	if ctx.Price > fast*1.005 || ctx.Price < fast*0.995 {
		return NoSignal(s.Name())
	}
	return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
		StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
		Reason: "DEMA_fast_above_slow+pullback_to_DEMA21", Timestamp: ctx.Now}
}

// ── S119: Keltner Breakout ────────────────────────────────────────────────────

type KeltnerBreakout struct{}

func (s *KeltnerBreakout) Name() string { return "Keltner_Breakout" }
func (s *KeltnerBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *KeltnerBreakout) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 25 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	adx := ADX(ctx.Candles15m, 14)
	if adx < 20 {
		return NoSignal(s.Name())
	}
	kc := KeltnerChannel(ctx.Candles15m, 20, 14, 2.0)
	switch {
	case ctx.Price > kc.Upper:
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: kc.Mid, TakeProfit: ctx.Price + 3*atr,
			Reason: "Keltner_breakout_upper+ADX", Timestamp: ctx.Now}
	case ctx.Price < kc.Lower:
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: kc.Mid, TakeProfit: ctx.Price - 3*atr,
			Reason: "Keltner_breakout_lower+ADX", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S120: Pivot Level Break ───────────────────────────────────────────────────

type PivotLevelBreak struct{}

func (s *PivotLevelBreak) Name() string { return "Pivot_Level_Break" }
func (s *PivotLevelBreak) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *PivotLevelBreak) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles4h) < 3 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	prev4h := ctx.Candles4h[len(ctx.Candles4h)-2]
	piv := Pivots(prev4h)
	avgVol := AvgVolume(ctx.Candles15m, 10)
	lastVol := ctx.Candles15m[len(ctx.Candles15m)-1].Volume
	volSurge := lastVol > 1.5*avgVol
	switch {
	case ctx.Price > piv.R1 && volSurge && ctx.CVD > ctx.CVDPrev:
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: piv.PP, TakeProfit: piv.R2,
			Reason: "pivot_R1_break+vol_surge+CVD_bull", Timestamp: ctx.Now}
	case ctx.Price < piv.S1 && volSurge && ctx.CVD < ctx.CVDPrev:
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: piv.PP, TakeProfit: piv.S2,
			Reason: "pivot_S1_break+vol_surge+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}
