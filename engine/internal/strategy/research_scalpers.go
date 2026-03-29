package strategy

import (
	"math"
	"time"

	"antigravity-engine/internal/marketdata"
)

// VWAPRSI2ReversionScalper fades stretched deviations from rolling VWAP when
// a very short RSI reaches exhaustion.
type VWAPRSI2ReversionScalper struct {
	baseScalper
	volumes []float64
	period  int
}

func NewVWAPRSI2ReversionScalper() *VWAPRSI2ReversionScalper {
	return &VWAPRSI2ReversionScalper{
		baseScalper: baseScalper{name: "VWAP_RSI2_Reversion_Scalp", maxBuf: defaultBufSize},
		period:      40,
	}
}

func (s *VWAPRSI2ReversionScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *VWAPRSI2ReversionScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.volumes = appendRollingFloat(s.volumes, candle.Quantity, defaultBufSize)
	if len(s.prices) < s.period || len(s.volumes) < s.period {
		return holdSignal()
	}

	atr := ATR(s.prices, 14)
	vwap := RollingVWAP(s.prices, s.volumes, s.period)
	avgVolume := tailAverage(s.volumes, 20)
	if atr == 0 || vwap == 0 || avgVolume == 0 {
		return holdSignal()
	}

	rsi2 := RSI(s.prices, 2)
	deviationPct := math.Abs((candle.Price - vwap) / vwap * 100)
	confidence := 0.95 + math.Min(deviationPct*0.6, 0.35)

	if candle.Price < vwap-0.45*atr && rsi2 < 10 && candle.Quantity >= avgVolume*0.85 {
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.28, 0.55, confidence)
	}
	if candle.Price > vwap+0.45*atr && rsi2 > 90 && candle.Quantity >= avgVolume*0.85 {
		return signalWithConfidence(candle.Symbol, ActionSell, 0.28, 0.55, confidence)
	}

	return holdSignal()
}

// BollingerRSIFadeScalper fades closes back inside the Bollinger envelope when
// RSI confirms an overextension and trend strength remains muted.
type BollingerRSIFadeScalper struct {
	baseScalper
	period     int
	multiplier float64
	widths     []float64
}

func NewBollingerRSIFadeScalper() *BollingerRSIFadeScalper {
	return &BollingerRSIFadeScalper{
		baseScalper: baseScalper{name: "Bollinger_RSI_Fade_Scalp", maxBuf: defaultBufSize},
		period:      20,
		multiplier:  2.0,
	}
}

func (s *BollingerRSIFadeScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *BollingerRSIFadeScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	n := len(s.prices)
	if n < s.period+2 {
		return holdSignal()
	}

	prevUpper, _, prevLower := BollingerBands(s.prices[:n-1], s.period, s.multiplier)
	upper, mid, lower := BollingerBands(s.prices, s.period, s.multiplier)
	if mid == 0 {
		return holdSignal()
	}

	widthPct := ((upper - lower) / mid) * 100
	s.widths = appendRollingFloat(s.widths, widthPct, defaultBufSize)
	avgWidth := tailAverage(s.widths, 10)
	rsi := RSI(s.prices, 14)
	adx := ADX(s.prices, 14)
	prevClose := s.prices[n-2]

	if adx > 22 || (avgWidth > 0 && widthPct > avgWidth*1.25) {
		return holdSignal()
	}

	if prevClose < prevLower && candle.Price > lower && rsi < 38 {
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.30, 0.60, 1.0+(38-rsi)/80)
	}
	if prevClose > prevUpper && candle.Price < upper && rsi > 62 {
		return signalWithConfidence(candle.Symbol, ActionSell, 0.30, 0.60, 1.0+(rsi-62)/80)
	}

	return holdSignal()
}

// MACDVWAPFlipScalper follows momentum inflections only when price is already
// positioned on the correct side of rolling VWAP.
type MACDVWAPFlipScalper struct {
	baseScalper
	volumes  []float64
	prevHist float64
}

func NewMACDVWAPFlipScalper() *MACDVWAPFlipScalper {
	return &MACDVWAPFlipScalper{
		baseScalper: baseScalper{name: "MACD_VWAP_Flip_Scalp", maxBuf: defaultBufSize},
	}
}

func (s *MACDVWAPFlipScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *MACDVWAPFlipScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.volumes = appendRollingFloat(s.volumes, candle.Quantity, defaultBufSize)
	if len(s.prices) < 40 || len(s.volumes) < 30 {
		return holdSignal()
	}

	vwap := RollingVWAP(s.prices, s.volumes, 30)
	fastEMA := EMA(s.prices, 8)
	slowEMA := EMA(s.prices, 21)
	atr := ATR(s.prices, 14)
	_, _, hist := MACD(s.prices, 5, 13, 6)
	defer func() { s.prevHist = hist }()

	if atr == 0 || vwap == 0 {
		return holdSignal()
	}

	histStrength := math.Min(math.Abs(hist)/atr, 1.5)
	confidence := 0.95 + histStrength*0.2

	if s.prevHist <= 0 && hist > 0 && candle.Price > vwap && fastEMA > slowEMA {
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.30, 0.85, confidence)
	}
	if s.prevHist >= 0 && hist < 0 && candle.Price < vwap && fastEMA < slowEMA {
		return signalWithConfidence(candle.Symbol, ActionSell, 0.30, 0.85, confidence)
	}

	return holdSignal()
}

// StochasticRangeScalper trades oscillator turns only when the market is
// sufficiently range-bound.
type StochasticRangeScalper struct {
	baseScalper
	rawKHist  []float64
	smoothedK []float64
}

func NewStochasticRangeScalper() *StochasticRangeScalper {
	return &StochasticRangeScalper{
		baseScalper: baseScalper{name: "Stochastic_Range_Scalp", maxBuf: defaultBufSize},
	}
}

func (s *StochasticRangeScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *StochasticRangeScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < 20 {
		return holdSignal()
	}

	if ADX(s.prices, 14) > 20 {
		return holdSignal()
	}

	rawK := CloseStochastic(s.prices, 14)
	s.rawKHist = appendRollingFloat(s.rawKHist, rawK, defaultBufSize)
	k := tailAverage(s.rawKHist, 3)
	s.smoothedK = appendRollingFloat(s.smoothedK, k, defaultBufSize)
	if len(s.smoothedK) < 4 {
		return holdSignal()
	}

	currentK := s.smoothedK[len(s.smoothedK)-1]
	currentD := tailAverage(s.smoothedK, 3)
	prevK := s.smoothedK[len(s.smoothedK)-2]
	prevD := tailAverage(s.smoothedK[:len(s.smoothedK)-1], 3)

	if prevK <= prevD && currentK > currentD && currentK < 25 && currentD < 30 {
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.25, 0.55, 1.0+(25-currentK)/100)
	}
	if prevK >= prevD && currentK < currentD && currentK > 75 && currentD > 70 {
		return signalWithConfidence(candle.Symbol, ActionSell, 0.25, 0.55, 1.0+(currentK-75)/100)
	}

	return holdSignal()
}

// ATRVolumeImpulseScalper trades breakouts only when both realized range and
// candle volume expand meaningfully above their rolling baselines.
type ATRVolumeImpulseScalper struct {
	baseScalper
	volumes          []float64
	breakoutLookback int
}

func NewATRVolumeImpulseScalper() *ATRVolumeImpulseScalper {
	return &ATRVolumeImpulseScalper{
		baseScalper:      baseScalper{name: "ATR_Volume_Impulse_Scalp", maxBuf: defaultBufSize},
		breakoutLookback: 10,
	}
}

func (s *ATRVolumeImpulseScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *ATRVolumeImpulseScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.volumes = appendRollingFloat(s.volumes, candle.Quantity, defaultBufSize)
	if len(s.prices) < s.breakoutLookback+2 || len(s.volumes) < 20 {
		return holdSignal()
	}

	atr := ATR(s.prices, 14)
	avgVolume := tailAverage(s.volumes, 20)
	if atr == 0 || avgVolume == 0 {
		return holdSignal()
	}

	n := len(s.prices)
	trueRange := math.Abs(s.prices[n-1] - s.prices[n-2])
	upper, lower := DonchianChannel(s.prices[:n-1], s.breakoutLookback)
	volRatio := candle.Quantity / avgVolume
	confidence := 0.95 + math.Min(volRatio*0.18, 0.4)

	if trueRange > 1.4*atr && volRatio > 1.5 && candle.Price > upper {
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.35, 1.10, confidence)
	}
	if trueRange > 1.4*atr && volRatio > 1.5 && candle.Price < lower {
		return signalWithConfidence(candle.Symbol, ActionSell, 0.35, 1.10, confidence)
	}

	return holdSignal()
}

// OpeningRangeBreakoutScalper builds a close-based opening range anchored to a
// chosen UTC time and only trades the first confirmed breakout of that range.
type OpeningRangeBreakoutScalper struct {
	baseScalper
	volumes          []float64
	anchorHour       int
	anchorMinute     int
	rangeMinutes     int
	sessionKey       string
	rangeHigh        float64
	rangeLow         float64
	rangeReady       bool
	sessionTriggered bool
}

func NewOpeningRangeBreakoutScalper(anchorHour, anchorMinute, rangeMinutes int) *OpeningRangeBreakoutScalper {
	return &OpeningRangeBreakoutScalper{
		baseScalper:  baseScalper{name: "OpeningRange_Breakout_Scalp", maxBuf: defaultBufSize},
		anchorHour:   anchorHour,
		anchorMinute: anchorMinute,
		rangeMinutes: rangeMinutes,
	}
}

func (s *OpeningRangeBreakoutScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *OpeningRangeBreakoutScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.volumes = appendRollingFloat(s.volumes, candle.Quantity, defaultBufSize)
	if candle.TimeMs == 0 {
		return holdSignal()
	}

	ts := time.UnixMilli(candle.TimeMs).UTC()
	anchor := s.anchorFor(ts)
	sessionKey := anchor.Format(time.RFC3339)
	if sessionKey != s.sessionKey {
		s.sessionKey = sessionKey
		s.rangeHigh = 0
		s.rangeLow = 0
		s.rangeReady = false
		s.sessionTriggered = false
	}

	rangeEnd := anchor.Add(time.Duration(s.rangeMinutes) * time.Minute)
	if ts.After(anchor) && (ts.Before(rangeEnd) || ts.Equal(rangeEnd)) {
		s.updateRange(candle.Price)
		return holdSignal()
	}

	if !s.rangeReady {
		if s.rangeHigh == 0 || s.rangeLow == 0 || ts.Before(rangeEnd) {
			return holdSignal()
		}
		s.rangeReady = true
	}

	if s.sessionTriggered {
		return holdSignal()
	}

	avgVolume := tailAverage(s.volumes, 20)
	if avgVolume == 0 || candle.Quantity < avgVolume*1.15 || len(s.prices) < 14 {
		return holdSignal()
	}

	fastEMA := EMA(s.prices, 9)
	breakoutBuffer := math.Max(ATR(s.prices, 14)*0.10, candle.Price*0.00025)
	if candle.Price > s.rangeHigh+breakoutBuffer && candle.Price > fastEMA {
		s.sessionTriggered = true
		return signalWithConfidence(candle.Symbol, ActionBuy, 0.35, 1.00, 1.15)
	}
	if candle.Price < s.rangeLow-breakoutBuffer && candle.Price < fastEMA {
		s.sessionTriggered = true
		return signalWithConfidence(candle.Symbol, ActionSell, 0.35, 1.00, 1.15)
	}

	return holdSignal()
}

func (s *OpeningRangeBreakoutScalper) anchorFor(ts time.Time) time.Time {
	anchor := time.Date(ts.Year(), ts.Month(), ts.Day(), s.anchorHour, s.anchorMinute, 0, 0, time.UTC)
	if ts.Before(anchor) {
		anchor = anchor.Add(-24 * time.Hour)
	}
	return anchor
}

func (s *OpeningRangeBreakoutScalper) updateRange(price float64) {
	if s.rangeHigh == 0 || price > s.rangeHigh {
		s.rangeHigh = price
	}
	if s.rangeLow == 0 || price < s.rangeLow {
		s.rangeLow = price
	}
}
