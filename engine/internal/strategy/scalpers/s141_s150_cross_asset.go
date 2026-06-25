package scalpers

// ── S141: Sentiment + Trend Combo ────────────────────────────────────────────

type SentimentTrendCombo struct{}

func (s *SentimentTrendCombo) Name() string { return "Sentiment_Trend_Combo" }
func (s *SentimentTrendCombo) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *SentimentTrendCombo) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles4h) < 20 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Use Fear&Greed-like proxy: funding above zero + OI rising = greed → trend long
	if ctx.FundingRate > 0 && ctx.OpenInterest > ctx.OpenInterestPrev {
		adx := ADX(ctx.Candles4h, 14)
		if adx > 25 && ctx.CVD > ctx.CVDPrev {
			return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
				StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
				Reason: "sentiment_greed+ADX25+CVD_bull", Timestamp: ctx.Now}
		}
	}
	// Fear: funding negative + OI falling = fear → trend short
	if ctx.FundingRate < 0 && ctx.OpenInterest < ctx.OpenInterestPrev {
		adx := ADX(ctx.Candles4h, 14)
		if adx > 25 && ctx.CVD < ctx.CVDPrev {
			return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
				StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
				Reason: "sentiment_fear+ADX25+CVD_bear", Timestamp: ctx.Now}
		}
	}
	return NoSignal(s.Name())
}

// ── S142: DVOL + Funding Macro Long ──────────────────────────────────────────

type DVOLFundingMacroLong struct{}

func (s *DVOLFundingMacroLong) Name() string { return "DVOL_Funding_Macro_Long" }
func (s *DVOLFundingMacroLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *DVOLFundingMacroLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || !ctx.DVOLPopulated {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// High vol fear + negative funding = forced shorts; buy the fear
	if ctx.DVOL > 80 && ctx.FundingRate < -0.0003 && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.68,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 5*atr,
			Reason: "DVOL_fear+neg_funding+CVD_bull", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S143: DVOL + Funding Macro Short ─────────────────────────────────────────

type DVOLFundingMacroShort struct{}

func (s *DVOLFundingMacroShort) Name() string { return "DVOL_Funding_Macro_Short" }
func (s *DVOLFundingMacroShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *DVOLFundingMacroShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || !ctx.DVOLPopulated {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Low vol complacency + very positive funding = over-levered longs; short
	if ctx.DVOL < 45 && ctx.FundingRate > 0.001 && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "DVOL_complacency+high_funding+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S144: ETH Strength BTC Lag ────────────────────────────────────────────────

type ETHStrengthBTCLag struct{}

func (s *ETHStrengthBTCLag) Name() string { return "ETH_Strength_BTC_Lag" }
func (s *ETHStrengthBTCLag) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ETHStrengthBTCLag) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 20 || !ctx.ETHPricePopulated || ctx.ETHPrice == 0 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	btcEMA21 := EMA(ctx.Candles1h, 21)
	btcEMA55 := EMA(ctx.Candles1h, 55)
	// ETH price above BTC EMA ratios proxy: both EMAs bull + ETH > EMA
	if btcEMA21 > btcEMA55 && ctx.ETHPrice > btcEMA21 && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "ETH_strength_BTC_1h_EMA_bull", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S145: Liquidation Hunt Long ───────────────────────────────────────────────

type LiquidationHuntLong struct{}

func (s *LiquidationHuntLong) Name() string { return "Liquidation_Hunt_Long" }
func (s *LiquidationHuntLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *LiquidationHuntLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 20 || !ctx.LiquidationFeedPopulated {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Large short liquidations = shorts being squeezed → ride the squeeze long
	if ctx.ShortLiquidationsUSD5m > 2*ctx.ShortLiquidationsAvg1h && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.68,
			StopLoss: ctx.Price - 1.5*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "short_liq_squeeze+CVD_bull", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S146: Liquidation Hunt Short ─────────────────────────────────────────────

type LiquidationHuntShort struct{}

func (s *LiquidationHuntShort) Name() string { return "Liquidation_Hunt_Short" }
func (s *LiquidationHuntShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *LiquidationHuntShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 20 || !ctx.LiquidationFeedPopulated {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	// Large long liquidations = longs being flushed → ride the flush short
	if ctx.LongLiquidationsUSD5m > 2*ctx.LongLiquidationsAvg1h && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.68,
			StopLoss: ctx.Price + 1.5*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "long_liq_flush+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S147: OI Trend + Funding Filter ──────────────────────────────────────────

type OITrendFundingFilter struct{}

func (s *OITrendFundingFilter) Name() string { return "OI_Trend_Funding_Filter" }
func (s *OITrendFundingFilter) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *OITrendFundingFilter) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles4h) < 20 || ctx.OpenInterest == 0 || ctx.OpenInterestPrev == 0 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles4h, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	adx := ADX(ctx.Candles4h, 14)
	if adx < 20 {
		return NoSignal(s.Name())
	}
	oiUp := ctx.OpenInterest > ctx.OpenInterestPrev
	// OI rising + funding positive + ADX trending = established bull trend
	if oiUp && ctx.FundingRate > 0 && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.67,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "OI_trend_up+funding_pos+ADX+CVD_bull", Timestamp: ctx.Now}
	}
	// OI rising + funding negative + ADX trending = short squeeze → short
	if oiUp && ctx.FundingRate < -0.0005 && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "OI_trend_up+funding_neg+ADX+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S148: Multi-Signal Confluence Long ────────────────────────────────────────

type MultiSignalConfluenceLong struct{}

func (s *MultiSignalConfluenceLong) Name() string { return "Multi_Signal_Confluence_Long" }
func (s *MultiSignalConfluenceLong) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *MultiSignalConfluenceLong) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 30 || len(ctx.Candles1h) < 30 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	score := 0
	// 1. RSI oversold on 15m
	if RSI(ctx.Candles15m, 14) < 35 {
		score++
	}
	// 2. MACD hist positive on 15m
	if MACD(ctx.Candles15m).Histogram > 0 {
		score++
	}
	// 3. Price above 1h EMA(21)
	if ctx.Price > EMA(ctx.Candles1h, 21) {
		score++
	}
	// 4. CVD bullish
	if ctx.CVD > ctx.CVDPrev {
		score++
	}
	// 5. ADX trending
	if ADX(ctx.Candles15m, 14) > 20 {
		score++
	}
	if score >= 4 {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.60 + float64(score)*0.02,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "multi_signal_confluence_long", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S149: Multi-Signal Confluence Short ───────────────────────────────────────

type MultiSignalConfluenceShort struct{}

func (s *MultiSignalConfluenceShort) Name() string { return "Multi_Signal_Confluence_Short" }
func (s *MultiSignalConfluenceShort) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *MultiSignalConfluenceShort) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles15m) < 30 || len(ctx.Candles1h) < 30 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles15m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	score := 0
	// 1. RSI overbought on 15m
	if RSI(ctx.Candles15m, 14) > 65 {
		score++
	}
	// 2. MACD hist negative on 15m
	if MACD(ctx.Candles15m).Histogram < 0 {
		score++
	}
	// 3. Price below 1h EMA(21)
	if ctx.Price < EMA(ctx.Candles1h, 21) {
		score++
	}
	// 4. CVD bearish
	if ctx.CVD < ctx.CVDPrev {
		score++
	}
	// 5. ADX trending
	if ADX(ctx.Candles15m, 14) > 20 {
		score++
	}
	if score >= 4 {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.60 + float64(score)*0.02,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "multi_signal_confluence_short", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S150: Volatility Regime Shift ─────────────────────────────────────────────

type VolatilityRegimeShift struct{}

func (s *VolatilityRegimeShift) Name() string { return "Volatility_Regime_Shift" }
func (s *VolatilityRegimeShift) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}
func (s *VolatilityRegimeShift) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1h) < 40 || len(ctx.Candles15m) < 20 {
		return NoSignal(s.Name())
	}
	atr15m := ATR(ctx.Candles15m, 14)
	atr1h := ATR(ctx.Candles1h, 14)
	if atr15m == 0 || atr1h == 0 {
		return NoSignal(s.Name())
	}
	// ATR(15m) expanding vs ATR(1h) normalised = vol regime expanding
	// Proxy: current ATR15m vs its own 20-period SMA
	avgATR15m := 0.0
	n15 := len(ctx.Candles15m)
	lookback := 20
	if n15 < lookback+14 {
		return NoSignal(s.Name())
	}
	for i := n15 - lookback - 14; i < n15-14; i++ {
		avgATR15m += ATR(ctx.Candles15m[:i+14], 14)
	}
	avgATR15m /= float64(lookback)
	if avgATR15m == 0 {
		return NoSignal(s.Name())
	}
	expanding := atr15m > 1.5*avgATR15m
	if !expanding {
		return NoSignal(s.Name())
	}
	bb := BB(ctx.Candles1h, 20)
	if ctx.Price > bb.Middle && ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr15m, TakeProfit: ctx.Price + 4*atr15m,
			Reason: "vol_regime_expanding_breakout_long", Timestamp: ctx.Now}
	}
	if ctx.Price < bb.Middle && ctx.CVD < ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: ctx.Price + 2*atr15m, TakeProfit: ctx.Price - 4*atr15m,
			Reason: "vol_regime_expanding_breakdown_short", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}
