package scalpers

import "time"

// ── S101: ZLEMA Momentum Scalp ────────────────────────────────────────────────

type ZLEMAMomentumScalp struct{}

func (s *ZLEMAMomentumScalp) Name() string { return "ZLEMA_Momentum_Scalp" }
func (s *ZLEMAMomentumScalp) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *ZLEMAMomentumScalp) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1m) < 40 || len(ctx.Candles5m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	fast := ZLEMA(ctx.Candles1m, 9)
	slow := ZLEMA(ctx.Candles1m, 21)
	rsi := RSI(ctx.Candles5m, 14)
	switch {
	case fast > slow && rsi > 50 && ctx.CVD > ctx.CVDPrev:
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.62,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "ZLEMA_cross_up+CVD+RSI_bull", Timestamp: ctx.Now}
	case fast < slow && rsi < 50 && ctx.CVD < ctx.CVDPrev:
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.62,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "ZLEMA_cross_dn+CVD+RSI_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S102: Hull MA Scalp ───────────────────────────────────────────────────────

type HullMAScalp struct{}

func (s *HullMAScalp) Name() string { return "Hull_MA_Scalp" }
func (s *HullMAScalp) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *HullMAScalp) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 30 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	fast := HMA(ctx.Candles5m, 9)
	slow := HMA(ctx.Candles5m, 16)
	imb := ctx.OrderBook.Imbalance
	switch {
	case fast > slow && imb > 0.1:
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.63,
			StopLoss: ctx.Price - 1.8*atr, TakeProfit: ctx.Price + 3.6*atr,
			Reason: "HMA_fast_above_slow+OB_bid_wall", Timestamp: ctx.Now}
	case fast < slow && imb < -0.1:
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.63,
			StopLoss: ctx.Price + 1.8*atr, TakeProfit: ctx.Price - 3.6*atr,
			Reason: "HMA_fast_below_slow+OB_ask_wall", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S103: Williams %R Hammer ──────────────────────────────────────────────────

type WilliamsRHammer struct{}

func (s *WilliamsRHammer) Name() string { return "WilliamsR_Hammer" }
func (s *WilliamsRHammer) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *WilliamsRHammer) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles1m) < 20 || len(ctx.Candles5m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles1m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	wr := WilliamsR(ctx.Candles1m, 14)
	macd := MACD(ctx.Candles5m)
	last := ctx.Candles1m[len(ctx.Candles1m)-1]
	isHammer := last.Close > last.Open && (last.Close-last.Low) > 2*(last.High-last.Close)
	if wr < -80 && isHammer && macd.Histogram > 0 {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "WilliamsR_oversold+hammer+MACD_bull", Timestamp: ctx.Now}
	}
	isShooter := last.Close < last.Open && (last.High-last.Close) > 2*(last.Close-last.Low)
	if wr > -20 && isShooter && macd.Histogram < 0 {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "WilliamsR_overbought+shooter+MACD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S104: Fisher Transform ────────────────────────────────────────────────────

type FisherTransformSignal struct{}

func (s *FisherTransformSignal) Name() string { return "Fisher_Transform_Signal" }
func (s *FisherTransformSignal) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *FisherTransformSignal) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	fisher := FisherTransform(ctx.Candles5m, 10)
	switch {
	case fisher < -2.5 && ctx.CVD > ctx.CVDPrev:
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.64,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "Fisher_extreme_low+CVD_bull", Timestamp: ctx.Now}
	case fisher > 2.5 && ctx.CVD < ctx.CVDPrev:
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.64,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "Fisher_extreme_high+CVD_bear", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S105: Stoch RSI Cross ─────────────────────────────────────────────────────

type StochRSICross struct{}

func (s *StochRSICross) Name() string { return "StochRSI_Cross" }
func (s *StochRSICross) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *StochRSICross) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 40 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	k, d := StochRSI(ctx.Candles5m, 14, 14, 3, 3)
	avgVol := AvgVolume(ctx.Candles5m, 20)
	lastVol := ctx.Candles5m[len(ctx.Candles5m)-1].Volume
	volOK := lastVol > avgVol
	switch {
	case k > d && k < 20 && volOK:
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.63,
			StopLoss: ctx.Price - 1.8*atr, TakeProfit: ctx.Price + 3.6*atr,
			Reason: "StochRSI_K_cross_D_oversold+vol", Timestamp: ctx.Now}
	case k < d && k > 80 && volOK:
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.63,
			StopLoss: ctx.Price + 1.8*atr, TakeProfit: ctx.Price - 3.6*atr,
			Reason: "StochRSI_K_cross_D_overbought+vol", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S106: Volatility Squeeze Entry ────────────────────────────────────────────

type VolatilitySqueezeEntry struct{}

func (s *VolatilitySqueezeEntry) Name() string { return "Volatility_Squeeze_Entry" }
func (s *VolatilitySqueezeEntry) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *VolatilitySqueezeEntry) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 60 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	pct := BBWidthPercentile(ctx.Candles5m, 20, 40)
	if pct > 0.20 {
		return NoSignal(s.Name())
	}
	last := ctx.Candles5m[len(ctx.Candles5m)-1]
	prev := ctx.Candles5m[len(ctx.Candles5m)-2]
	expansion := (last.High - last.Low) > 1.2*(prev.High-prev.Low)
	if !expansion {
		return NoSignal(s.Name())
	}
	if last.Close > prev.Close {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.65,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "BB_squeeze_expansion_up", Timestamp: ctx.Now}
	}
	return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.65,
		StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
		Reason: "BB_squeeze_expansion_dn", Timestamp: ctx.Now}
}

// ── S107: Narrow Range Breakout ───────────────────────────────────────────────

type NarrowRangeBreakout struct{}

func (s *NarrowRangeBreakout) Name() string { return "Narrow_Range_Breakout" }
func (s *NarrowRangeBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *NarrowRangeBreakout) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if !NarrowRangeN(ctx.Candles5m, 7) {
		return NoSignal(s.Name())
	}
	avgVol := AvgVolume(ctx.Candles5m, 10)
	last := ctx.Candles5m[len(ctx.Candles5m)-1]
	if last.Volume < 1.5*avgVol {
		return NoSignal(s.Name())
	}
	prev := ctx.Candles5m[len(ctx.Candles5m)-2]
	if last.Close > prev.High {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.63,
			StopLoss: ctx.Price - 1.8*atr, TakeProfit: ctx.Price + 3.6*atr,
			Reason: "NR7_breakout_up+vol_surge", Timestamp: ctx.Now}
	}
	if last.Close < prev.Low {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.63,
			StopLoss: ctx.Price + 1.8*atr, TakeProfit: ctx.Price - 3.6*atr,
			Reason: "NR7_breakout_dn+vol_surge", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S108: CMF + BB Touch ──────────────────────────────────────────────────────

type CMFBBTouch struct{}

func (s *CMFBBTouch) Name() string { return "CMF_BB_Touch" }
func (s *CMFBBTouch) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *CMFBBTouch) Evaluate(ctx MarketContext) Signal {
	if len(ctx.Candles5m) < 30 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	cmf := ChaikinMoneyFlow(ctx.Candles5m, 20)
	bb := BB(ctx.Candles5m, 20)
	if cmf > 0.15 && ctx.Price <= bb.Lower*1.005 {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.64,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "CMF_positive+price_at_BB_lower", Timestamp: ctx.Now}
	}
	if cmf < -0.15 && ctx.Price >= bb.Upper*0.995 {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.64,
			StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
			Reason: "CMF_negative+price_at_BB_upper", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S109: Aggressor Exhaustion ────────────────────────────────────────────────

type AggressorExhaustion struct{}

func (s *AggressorExhaustion) Name() string { return "Aggressor_Exhaustion" }
func (s *AggressorExhaustion) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *AggressorExhaustion) Evaluate(ctx MarketContext) Signal {
	if !ctx.MicrostructurePopulated || len(ctx.Candles5m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	bb := BB(ctx.Candles5m, 20)
	if ctx.AggressorBuyRatio100 > 0.75 && ctx.Price >= bb.Upper*0.995 {
		return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.66,
			StopLoss: ctx.Price + 1.8*atr, TakeProfit: ctx.Price - 3.6*atr,
			Reason: "aggressor_buy_ratio_high+BB_upper_touch", Timestamp: ctx.Now}
	}
	if ctx.AggressorBuyRatio100 < 0.25 && ctx.Price <= bb.Lower*1.005 {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.66,
			StopLoss: ctx.Price - 1.8*atr, TakeProfit: ctx.Price + 3.6*atr,
			Reason: "aggressor_sell_ratio_high+BB_lower_touch", Timestamp: ctx.Now}
	}
	return NoSignal(s.Name())
}

// ── S110: Trade Count Spike ───────────────────────────────────────────────────

type TradeCountSpike struct{}

func (s *TradeCountSpike) Name() string { return "Trade_Count_Spike" }
func (s *TradeCountSpike) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}
func (s *TradeCountSpike) Evaluate(ctx MarketContext) Signal {
	if !ctx.MicrostructurePopulated || ctx.TradeCountAvg20m == 0 || len(ctx.Candles5m) < 20 {
		return NoSignal(s.Name())
	}
	atr := ATR(ctx.Candles5m, 14)
	if atr == 0 {
		return NoSignal(s.Name())
	}
	if ctx.TradeCountPerMin1m < 3*ctx.TradeCountAvg20m {
		return NoSignal(s.Name())
	}
	if ctx.CVD > ctx.CVDPrev {
		return Signal{Strategy: s.Name(), Direction: DirectionLong, Confidence: 0.63,
			StopLoss: ctx.Price - 2*atr, TakeProfit: ctx.Price + 4*atr,
			Reason: "trade_count_spike+CVD_bull", Timestamp: time.Now().UTC()}
	}
	return Signal{Strategy: s.Name(), Direction: DirectionShort, Confidence: 0.63,
		StopLoss: ctx.Price + 2*atr, TakeProfit: ctx.Price - 4*atr,
		Reason: "trade_count_spike+CVD_bear", Timestamp: time.Now().UTC()}
}
