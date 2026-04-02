package strategy

import "antigravity-engine/internal/marketdata"

func appendRollingFloat(values []float64, value float64, maxSize int) []float64 {
	values = append(values, value)
	if len(values) > maxSize {
		values = values[1:]
	}
	return values
}

func tailAverage(values []float64, window int) float64 {
	if len(values) == 0 {
		return 0
	}
	if len(values) < window {
		return SMA(values)
	}
	return SMA(values[len(values)-window:])
}

func clampConfidence(value float64) float64 {
	if value < 0.5 {
		return 0.5
	}
	if value > 1.5 {
		return 1.5
	}
	return value
}

func signalWithConfidence(symbol string, action Action, slPct, tpPct, confidence float64) []Signal {
	return []Signal{{
		Symbol:        symbol,
		Action:        action,
		TargetSize:    defaultQty,
		Confidence:    clampConfidence(confidence),
		StopLossPct:   slPct,
		TakeProfitPct: tpPct,
	}}
}

// VolumeWeightedTrendScalper trades with the trend only when momentum and
// candle volume confirm the move.
type VolumeWeightedTrendScalper struct {
	baseScalper
	volumes    []float64
	fastPeriod int
	slowPeriod int
}

func NewVolumeWeightedTrendScalper() *VolumeWeightedTrendScalper {
	return &VolumeWeightedTrendScalper{
		baseScalper: baseScalper{name: "VolumeWeighted_Trend_Scalp", maxBuf: defaultBufSize},
		fastPeriod:  12,
		slowPeriod:  34,
	}
}

func (s *VolumeWeightedTrendScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *VolumeWeightedTrendScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.volumes = appendRollingFloat(s.volumes, candle.Quantity, defaultBufSize)
	if len(s.prices) < s.slowPeriod+10 || len(s.volumes) < 20 {
		return holdSignal()
	}

	fastEMA := EMA(s.prices, s.fastPeriod)
	slowEMA := EMA(s.prices, s.slowPeriod)
	_, _, hist := MACD(s.prices, 12, 26, 9)
	avgVolume := tailAverage(s.volumes, 20)
	if avgVolume == 0 {
		return holdSignal()
	}

	volRatio := candle.Quantity / avgVolume
	if fastEMA > slowEMA && hist > 0 && candle.Price > fastEMA && volRatio > 1.15 {
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.35, 1.10, 0.9+volRatio*0.15)
	}
	if fastEMA < slowEMA && hist < 0 && candle.Price < fastEMA && volRatio > 1.15 {
		return signalWithConfidence(candle.Symbol, ActionSell, 0.35, 1.10, 0.9+volRatio*0.15)
	}
	return holdSignal()
}

// PullbackContinuationProScalper buys trend pullbacks only when the reversal
// candle reclaims direction with above-average volume.
type PullbackContinuationProScalper struct {
	baseScalper
	volumes []float64
}

func NewPullbackContinuationProScalper() *PullbackContinuationProScalper {
	return &PullbackContinuationProScalper{
		baseScalper: baseScalper{name: "Pullback_Continuation_Pro_Scalp", maxBuf: defaultBufSize},
	}
}

func (s *PullbackContinuationProScalper) OnTick(tick marketdata.Tick) []Signal {
	return s.OnCandle(tick)
}

func (s *PullbackContinuationProScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.volumes = appendRollingFloat(s.volumes, candle.Quantity, defaultBufSize)
	if len(s.prices) < 55 || len(s.volumes) < 20 {
		return holdSignal()
	}

	ema20 := EMA(s.prices, 20)
	ema50 := EMA(s.prices, 50)
	adx := ADX(s.prices, 14)
	if adx < 22 {
		return holdSignal()
	}

	avgVolume := tailAverage(s.volumes, 20)
	if avgVolume == 0 || candle.Quantity < avgVolume*1.40 {
		return holdSignal()
	}

	n := len(s.prices)
	longTrend := ema20 > ema50 && candle.Price > ema20
	shortTrend := ema20 < ema50 && candle.Price < ema20

	if longTrend &&
		s.prices[n-4] > s.prices[n-3] &&
		s.prices[n-3] > s.prices[n-2] &&
		s.prices[n-1] > s.prices[n-2] {
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.30, 0.95, 0.95+adx/100)
	}

	if shortTrend &&
		s.prices[n-4] < s.prices[n-3] &&
		s.prices[n-3] < s.prices[n-2] &&
		s.prices[n-1] < s.prices[n-2] {
		return signalWithConfidence(candle.Symbol, ActionSell, 0.30, 0.95, 0.95+adx/100)
	}

	return holdSignal()
}

// OrderFlowPressureProScalper uses signed tick volume and price alignment to
// capture directional pressure in the live tape.
type OrderFlowPressureProScalper struct {
	baseScalper
	signedFlow []float64
	sizeHist   []float64
	window     int
}

func NewOrderFlowPressureProScalper(window int) *OrderFlowPressureProScalper {
	return &OrderFlowPressureProScalper{
		baseScalper: baseScalper{name: "OrderFlow_Pressure_Pro_Scalp", maxBuf: defaultBufSize},
		window:      window,
	}
}

func (s *OrderFlowPressureProScalper) OnTick(tick marketdata.Tick) []Signal { return s.evaluate(tick) }
func (s *OrderFlowPressureProScalper) OnCandle(candle marketdata.Tick) []Signal {
	return s.evaluate(candle)
}

func (s *OrderFlowPressureProScalper) evaluate(point marketdata.Tick) []Signal {
	s.feed(point.Price)
	s.sizeHist = appendRollingFloat(s.sizeHist, point.Quantity, defaultBufSize)

	signedQty := 0.0
	if point.Side == "BUY" {
		signedQty = point.Quantity
	} else if point.Side == "SELL" {
		signedQty = -point.Quantity
	}
	s.signedFlow = appendRollingFloat(s.signedFlow, signedQty, defaultBufSize)

	if point.Side == "" || len(s.prices) < s.window+21 || len(s.signedFlow) < s.window {
		return holdSignal()
	}

	flowWindow := s.signedFlow[len(s.signedFlow)-s.window:]
	flowSum := 0.0
	flowAbs := 0.0
	for _, flow := range flowWindow {
		flowSum += flow
		if flow < 0 {
			flowAbs -= flow
		} else {
			flowAbs += flow
		}
	}
	if flowAbs == 0 {
		return holdSignal()
	}

	imbalance := flowSum / flowAbs
	fastEMA := EMA(s.prices, 8)
	slowEMA := EMA(s.prices, 21)
	momentum := ROC(s.prices, 10)
	avgSize := tailAverage(s.sizeHist, 40)
	sizeRatio := 1.0
	if avgSize > 0 {
		sizeRatio = point.Quantity / avgSize
	}

	if imbalance > 0.22 && fastEMA > slowEMA && momentum > 0.02 && sizeRatio > 0.85 {
		return signalWithConfidence(point.Symbol, ActionBuy, 0.20, 0.60, 0.95+imbalance)
	}
	if imbalance < -0.22 && fastEMA < slowEMA && momentum < -0.02 && sizeRatio > 0.85 {
		return signalWithConfidence(point.Symbol, ActionSell, 0.20, 0.60, 0.95-imbalance)
	}

	return holdSignal()
}

// VolumeBreakoutImpulseScalper looks for compressed markets resolving with a
// real volume expansion through the recent range.
type VolumeBreakoutImpulseScalper struct {
	baseScalper
	volumes []float64
	period  int
}

func NewVolumeBreakoutImpulseScalper(period int) *VolumeBreakoutImpulseScalper {
	return &VolumeBreakoutImpulseScalper{
		baseScalper: baseScalper{name: "VolumeBreakout_Impulse_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *VolumeBreakoutImpulseScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *VolumeBreakoutImpulseScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.volumes = appendRollingFloat(s.volumes, candle.Quantity, defaultBufSize)
	if len(s.prices) < s.period+2 || len(s.volumes) < s.period {
		return holdSignal()
	}

	upper, lower := DonchianChannel(s.prices[:len(s.prices)-1], s.period)
	bbUpper, mid, bbLower := BollingerBands(s.prices, s.period, 2.0)
	if mid == 0 {
		return holdSignal()
	}

	widthPct := ((bbUpper - bbLower) / mid) * 100
	avgVolume := tailAverage(s.volumes, s.period)
	if avgVolume == 0 {
		return holdSignal()
	}
	volRatio := candle.Quantity / avgVolume

	if widthPct < 1.2 && volRatio > 1.30 && candle.Price > upper {
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.40, 1.20, 0.9+volRatio*0.20)
	}
	if widthPct < 1.2 && volRatio > 1.30 && candle.Price < lower {
		return signalWithConfidence(candle.Symbol, ActionSell, 0.40, 1.20, 0.9+volRatio*0.20)
	}
	return holdSignal()
}
