package strategy

import (
	"math"

	"antigravity-engine/internal/marketdata"
)

// ChartWedgeBreakoutScalper looks for range compression on the chart followed
// by a directional breakout with trend and volume confirmation.
type ChartWedgeBreakoutScalper struct {
	baseScalper
	volumes       []float64
	lookback      int
	cooldownBars  int
	lastSignalBar int
}

func NewChartWedgeBreakoutScalper() *ChartWedgeBreakoutScalper {
	return &ChartWedgeBreakoutScalper{
		baseScalper:  baseScalper{name: "Chart_Wedge_Breakout_Scalp", maxBuf: defaultBufSize},
		lookback:     22,
		cooldownBars: 8,
	}
}

func (s *ChartWedgeBreakoutScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *ChartWedgeBreakoutScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.volumes = appendRollingFloat(s.volumes, candle.Quantity, defaultBufSize)

	n := len(s.prices)
	if n < s.lookback*2+2 || len(s.volumes) < 20 {
		return holdSignal()
	}
	if n-s.lastSignalBar < s.cooldownBars {
		return holdSignal()
	}

	recent := s.prices[n-s.lookback : n-1]
	prior := s.prices[n-2*s.lookback : n-s.lookback]
	recentHigh, recentLow := sliceHighLow(recent)
	priorHigh, priorLow := sliceHighLow(prior)

	recentRange := recentHigh - recentLow
	priorRange := priorHigh - priorLow
	if recentRange <= 0 || priorRange <= 0 {
		return holdSignal()
	}

	// Price action should visibly compress before we trust the breakout.
	if recentRange >= priorRange*0.82 {
		return holdSignal()
	}

	atr := ATR(s.prices, 14)
	avgVolume := tailAverage(s.volumes, 20)
	fastEMA := EMA(s.prices, 9)
	slowEMA := EMA(s.prices, 21)
	if atr == 0 || avgVolume == 0 {
		return holdSignal()
	}

	volRatio := candle.Quantity / avgVolume
	if volRatio < 1.10 {
		return holdSignal()
	}

	breakoutBuffer := math.Max(atr*0.18, candle.Price*0.00035)
	confidence := 0.95 + math.Min((volRatio-1.0)*0.25, 0.30)

	if candle.Price > recentHigh+breakoutBuffer && fastEMA > slowEMA {
		s.lastSignalBar = n
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.32, 1.00, confidence)
	}
	if candle.Price < recentLow-breakoutBuffer && fastEMA < slowEMA {
		s.lastSignalBar = n
		return signalWithConfidence(candle.Symbol, ActionSell, 0.32, 1.00, confidence)
	}

	return holdSignal()
}

// ChartDoubleTapReversalScalper hunts for double-bottom / double-top chart
// structures and enters only when the second touch starts to bounce.
type ChartDoubleTapReversalScalper struct {
	baseScalper
	volumes       []float64
	lookback      int
	cooldownBars  int
	lastSignalBar int
}

func NewChartDoubleTapReversalScalper() *ChartDoubleTapReversalScalper {
	return &ChartDoubleTapReversalScalper{
		baseScalper:  baseScalper{name: "Chart_DoubleTap_Reversal_Scalp", maxBuf: defaultBufSize},
		lookback:     28,
		cooldownBars: 7,
	}
}

func (s *ChartDoubleTapReversalScalper) OnTick(tick marketdata.Tick) []Signal {
	return s.OnCandle(tick)
}

func (s *ChartDoubleTapReversalScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.volumes = appendRollingFloat(s.volumes, candle.Quantity, defaultBufSize)

	n := len(s.prices)
	if n < s.lookback+4 || len(s.volumes) < 20 {
		return holdSignal()
	}
	if n-s.lastSignalBar < s.cooldownBars {
		return holdSignal()
	}

	window := s.prices[n-s.lookback : n-1]
	resistance, support := sliceHighLow(window)
	atr := ATR(s.prices, 14)
	avgVolume := tailAverage(s.volumes, 20)
	rsi := RSI(s.prices, 7)
	if atr == 0 || avgVolume == 0 {
		return holdSignal()
	}

	tolerance := math.Max(atr*0.35, candle.Price*0.0008)
	nearSupport := math.Abs(candle.Price-support) <= tolerance*1.2
	nearResistance := math.Abs(candle.Price-resistance) <= tolerance*1.2
	volRatio := candle.Quantity / avgVolume
	bullishBounce := candle.Price > s.prices[n-2]
	bearishBounce := candle.Price < s.prices[n-2]

	firstTapSupport := false
	firstTapResistance := false
	for i := 0; i < len(window)-4; i++ {
		if math.Abs(window[i]-support) <= tolerance && window[i+3] > support+tolerance {
			firstTapSupport = true
		}
		if math.Abs(window[i]-resistance) <= tolerance && window[i+3] < resistance-tolerance {
			firstTapResistance = true
		}
		if firstTapSupport && firstTapResistance {
			break
		}
	}

	if nearSupport && firstTapSupport && bullishBounce && rsi < 42 && volRatio >= 0.95 {
		s.lastSignalBar = n
		confidence := 0.95 + math.Min((42-rsi)/90+(volRatio-1.0)*0.15, 0.30)
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.30, 0.85, confidence)
	}
	if nearResistance && firstTapResistance && bearishBounce && rsi > 58 && volRatio >= 0.95 {
		s.lastSignalBar = n
		confidence := 0.95 + math.Min((rsi-58)/90+(volRatio-1.0)*0.15, 0.30)
		return signalWithConfidence(candle.Symbol, ActionSell, 0.30, 0.85, confidence)
	}

	return holdSignal()
}

func sliceHighLow(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	high := values[0]
	low := values[0]
	for _, value := range values[1:] {
		if value > high {
			high = value
		}
		if value < low {
			low = value
		}
	}
	return high, low
}
