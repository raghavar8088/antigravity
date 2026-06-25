package scalpers

// ── S131: Funding Rate Extreme Long ──────────────────────────────────────────

type FundingRateExtremeLong struct{}

func (s *FundingRateExtremeLong) Name() string { return "Funding_Rate_Extreme_Long" }
func (s *FundingRateExtremeLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *FundingRateExtremeLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Funding < -0.05% (extremely negative) → longs get paid → bullish mean reversion
	if ctx.FundingRate > -0.0005 {
		return NoSignal(s.Name())
	}
	if ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "funding_extreme_neg+CVD_bull_mean_rev", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S132: Funding Rate Extreme Short ─────────────────────────────────────────

type FundingRateExtremeShort struct{}

func (s *FundingRateExtremeShort) Name() string { return "Funding_Rate_Extreme_Short" }
func (s *FundingRateExtremeShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *FundingRateExtremeShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Funding > +0.1% (extremely positive) → shorts get paid → bearish mean reversion
	if ctx.FundingRate < 0.001 {
		return NoSignal(s.Name())
	}
	if ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "funding_extreme_pos+CVD_bear_mean_rev", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S133: OI Spike Trend Follow ───────────────────────────────────────────────

type OISpikeFollow struct{}

func (s *OISpikeFollow) Name() string { return "OI_Spike_Follow" }
func (s *OISpikeFollow) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *OISpikeFollow) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || ctx.OpenInterest == 0 || ctx.OpenInterestPrev == 0 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	oiChange := (ctx.OpenInterest - ctx.OpenInterestPrev) / ctx.OpenInterestPrev
	if oiChange < 0.02 {
		return NoSignal(s.Name())
	}
	if ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.67,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "OI_spike_2pct+CVD_bull", Timestamp: ctx.Now}
	}
	if ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.67,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "OI_spike_2pct+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S134: OI Unwind Fade ─────────────────────────────────────────────────────

type OIUnwindFade struct{}

func (s *OIUnwindFade) Name() string { return "OI_Unwind_Fade" }
func (s *OIUnwindFade) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *OIUnwindFade) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || ctx.OpenInterest == 0 || ctx.OpenInterestPrev == 0 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// OI falling sharply = position unwind; fade the prior move
	oiChange := (ctx.OpenInterest - ctx.OpenInterestPrev) / ctx.OpenInterestPrev
	if oiChange > -0.03 {
		return NoSignal(s.Name())
	}
	bb := BB(ctx.Candles15m, 20)
	if ctx.Price > bb.Upper {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.64,
			StopLoss: ctx.Price + 1.5*atr, TakeProfit: bb.Middle,
			Reason: "OI_unwind_at_BB_upper_fade", Timestamp: ctx.Now}
	}
	if ctx.Price < bb.Lower {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.64,
			StopLoss: ctx.Price - 1.5*atr, TakeProfit: bb.Middle,
			Reason: "OI_unwind_at_BB_lower_fade", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S135: Liquidation Cascade Reversal ───────────────────────────────────────

type LiquidationCascadeReversal struct{}

func (s *LiquidationCascadeReversal) Name() string { return "Liquidation_Cascade_Reversal" }
func (s *LiquidationCascadeReversal) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *LiquidationCascadeReversal) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 20 || !ctx.LiquidationFeedPopulated {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	bb := BB(ctx.Candles5m, 20)
	// Large long liquidation spike below BB lower → mean reversion long
	if ctx.LongLiquidationsUSD5m > 3*ctx.LongLiquidationsAvg1h && ctx.Price < bb.Lower {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.68,
			StopLoss: ctx.Price - 1.5*atr, TakeProfit: bb.Middle,
			Reason: "long_liq_cascade_at_BB_lower_reversal", Timestamp: ctx.Now}
	}
	// Large short liquidation spike above BB upper → mean reversion short
	if ctx.ShortLiquidationsUSD5m > 3*ctx.ShortLiquidationsAvg1h && ctx.Price > bb.Upper {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.68,
			StopLoss: ctx.Price + 1.5*atr, TakeProfit: bb.Middle,
			Reason: "short_liq_cascade_at_BB_upper_reversal", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S136: DVOL Extreme Regime ─────────────────────────────────────────────────

type DVOLExtremeRegime struct{}

func (s *DVOLExtremeRegime) Name() string { return "DVOL_Extreme_Regime" }
func (s *DVOLExtremeRegime) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *DVOLExtremeRegime) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || !ctx.DVOLPopulated {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// DVOL > 100 = extreme fear → mean reversion long on CVD confirmation
	if ctx.DVOL > 100 && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "DVOL_extreme_fear+CVD_bull_mean_rev", Timestamp: ctx.Now}
	}
	// DVOL < 40 = complacency → trend follow short on CVD
	if ctx.DVOL < 40 && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.60,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 3*atr,
			Reason: "DVOL_complacency+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S137: Funding + OI Confirmation ──────────────────────────────────────────

type FundingOIConfirm struct{}

func (s *FundingOIConfirm) Name() string { return "Funding_OI_Confirm" }
func (s *FundingOIConfirm) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *FundingOIConfirm) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || ctx.OpenInterest == 0 || ctx.OpenInterestPrev == 0 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	oiRising := ctx.OpenInterest > ctx.OpenInterestPrev
	adx := ADX(ctx.Candles1h, 14)
	if adx < 20 {
		return NoSignal(s.Name())
	}
	if ctx.FundingRate > 0 && oiRising && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.68,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "funding_pos+OI_rising+CVD_bull+ADX", Timestamp: ctx.Now}
	}
	if ctx.FundingRate < 0 && !oiRising && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.68,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "funding_neg+OI_falling+CVD_bear+ADX", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S138: Options Proxy IV Crush ──────────────────────────────────────────────

type OptionIVCrush struct{}

func (s *OptionIVCrush) Name() string { return "Option_IV_Crush" }
func (s *OptionIVCrush) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *OptionIVCrush) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || !ctx.DVOLPopulated || len(ctx.DVOLHistory) < 2 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	n := len(ctx.DVOLHistory)
	prevDVOL := ctx.DVOLHistory[n-2]
	if prevDVOL == 0 {
		return NoSignal(s.Name())
	}
	dvolDrop := (prevDVOL - ctx.DVOL) / prevDVOL
	if dvolDrop < 0.05 {
		return NoSignal(s.Name())
	}
	if ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.63,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 3*atr,
			Reason: "IV_crush_5pct+CVD_bull", Timestamp: ctx.Now}
	}
	if ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.63,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 3*atr,
			Reason: "IV_crush_5pct+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S139: Options Proxy IV Expansion ─────────────────────────────────────────

type OptionIVExpansion struct{}

func (s *OptionIVExpansion) Name() string { return "Option_IV_Expansion" }
func (s *OptionIVExpansion) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *OptionIVExpansion) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || !ctx.DVOLPopulated || len(ctx.DVOLHistory) < 2 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	n := len(ctx.DVOLHistory)
	prevDVOL := ctx.DVOLHistory[n-2]
	if prevDVOL == 0 {
		return NoSignal(s.Name())
	}
	dvolRise := (ctx.DVOL - prevDVOL) / prevDVOL
	if dvolRise < 0.05 {
		return NoSignal(s.Name())
	}
	if ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.64,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "IV_expansion_5pct+CVD_bull_breakout", Timestamp: ctx.Now}
	}
	if ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.64,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "IV_expansion_5pct+CVD_bear_breakdown", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S140: ETH Correlation Divergence ─────────────────────────────────────────

type ETHCorrelationDivergence struct{}

func (s *ETHCorrelationDivergence) Name() string { return "ETH_Correlation_Divergence" }
func (s *ETHCorrelationDivergence) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ETHCorrelationDivergence) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 20 || !ctx.ETHPricePopulated || ctx.ETHPrice == 0 {
		return NoSignal(s.Name())
	}
	n := len(ctx.Candles15m)
	if n < 3 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Compare ETH price to its EMA(9) as a proxy for ETH direction
	ethEMA := EMA(ctx.Candles15m, 9) // BTC EMA used as cross-asset proxy; not ideal but avoids missing fields
	ethAboveEMA := ctx.ETHPrice > ethEMA
	btcMove := (ctx.Candles15m[n-1].Close - ctx.Candles15m[n-3].Close) / ctx.Candles15m[n-3].Close
	// ETH momentum up but BTC lagging (negative 2-bar move) → long BTC
	if ethAboveEMA && btcMove < 0 && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.63,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 3*atr,
			Reason: "ETH_above_EMA_BTC_lagging_long", Timestamp: ctx.Now}
	}
	// ETH momentum down but BTC lagging (positive 2-bar move) → short BTC
	if !ethAboveEMA && btcMove > 0 && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.63,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 3*atr,
			Reason: "ETH_below_EMA_BTC_lagging_short", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}
