package strategy

import (
	"math"
	"time"

	"antigravity-engine/internal/marketdata"
)

// =============================================================================
// PRO2 STRATEGIES — Session, Divergence, Confluence, Pattern-based
// =============================================================================

// SessionOpenMomentumScalper catches the directional impulse that fires at the
// start of the three major trading sessions: Asia (00:00 UTC), Europe (08:00),
// US (13:00). Within the first N minutes of each session open it measures:
//   - Whether volume is expanding vs the prior-session average
//   - Whether price has moved more than 0.3x ATR in one direction
//   - Whether EMA5 > EMA13 (or <) to confirm directional bias
//
// These session-open moves are institutionally driven and often extend 0.5–1%+.
type SessionOpenMomentumScalper struct {
	baseScalper
	volumes        []float64
	sessionMinutes int // How many minutes after open the window stays active
	sessionHours   []int
	lastSessionKey string // "session-YYYYMMDD-HH" to avoid re-entry same session
	cooldownBars   int
	lastBar        int
}

func NewSessionOpenMomentumScalper() *SessionOpenMomentumScalper {
	return &SessionOpenMomentumScalper{
		baseScalper:    baseScalper{name: "SessionOpen_Momentum_Scalp", maxBuf: defaultBufSize},
		sessionMinutes: 18,
		sessionHours:   []int{0, 8, 13}, // UTC: Asia, Europe, US
		cooldownBars:   10,
	}
}

func (s *SessionOpenMomentumScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *SessionOpenMomentumScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	vol := candle.Quantity
	if vol <= 0 {
		vol = 1
	}
	s.volumes = appendRollingFloat(s.volumes, vol, defaultBufSize)

	n := len(s.prices)
	if n < 30 || len(s.volumes) < 20 {
		return holdSignal()
	}
	if n-s.lastBar < s.cooldownBars {
		return holdSignal()
	}

	now := time.Now().UTC()
	inSessionWindow := false
	sessionKey := ""
	for _, h := range s.sessionHours {
		if now.Hour() == h && now.Minute() < s.sessionMinutes {
			inSessionWindow = true
			sessionKey = now.Format("2006-01-02") + "-" + string(rune('0'+h))
			break
		}
	}
	if !inSessionWindow {
		return holdSignal()
	}
	if sessionKey == s.lastSessionKey {
		return holdSignal() // Already traded this session open
	}

	atr := ATR(s.prices, 14)
	if atr == 0 {
		return holdSignal()
	}

	fastEMA := EMA(s.prices, 5)
	slowEMA := EMA(s.prices, 13)
	rsi := RSI(s.prices, 7)
	avgVol := tailAverage(s.volumes, 20)
	if avgVol == 0 {
		return holdSignal()
	}
	volRatio := vol / avgVol

	// Require meaningful volume expansion at session open
	if volRatio < 1.20 {
		return holdSignal()
	}

	// Require price moved at least 0.3x ATR in the direction
	priceMove := candle.Price - s.prices[n-2]
	moveToATR := math.Abs(priceMove) / atr

	if moveToATR < 0.30 {
		return holdSignal()
	}

	atrPct := (atr / candle.Price) * 100
	slPct := math.Max(atrPct*1.0, 0.22)
	if slPct > 0.85 {
		slPct = 0.85
	}
	tpPct := slPct * 2.5 // Session opens run further
	confidence := 0.94 + math.Min(math.Max(volRatio-1.2, 0)*0.12+moveToATR*0.10, 0.26)

	if fastEMA > slowEMA && priceMove > 0 && rsi > 45 {
		s.lastSessionKey = sessionKey
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionBuy, slPct, tpPct, confidence)
	}
	if fastEMA < slowEMA && priceMove < 0 && rsi < 55 {
		s.lastSessionKey = sessionKey
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionSell, slPct, tpPct, confidence)
	}
	return holdSignal()
}

// RSIMACDDivergenceScalper looks for classic hidden divergence:
//   - Price makes a higher high but RSI makes a lower high → bearish
//   - Price makes a lower low but RSI makes a higher low → bullish
//
// Combined with MACD histogram flip for timing precision. This pattern is one
// of the highest-probability setups in technical analysis — it signals that the
// trend is losing internal momentum before price has reversed visibly.
type RSIMACDDivergenceScalper struct {
	baseScalper
	lookback     int
	cooldownBars int
	lastBar      int
}

func NewRSIMACDDivergenceScalper() *RSIMACDDivergenceScalper {
	return &RSIMACDDivergenceScalper{
		baseScalper:  baseScalper{name: "RSI_MACD_Divergence_Scalp", maxBuf: defaultBufSize},
		lookback:     25,
		cooldownBars: 8,
	}
}

func (s *RSIMACDDivergenceScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *RSIMACDDivergenceScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	n := len(s.prices)
	if n < s.lookback+15 {
		return holdSignal()
	}
	if n-s.lastBar < s.cooldownBars {
		return holdSignal()
	}

	rsi := RSI(s.prices, 14)
	_, _, macdHist := MACD(s.prices, 12, 26, 9)
	atr := ATR(s.prices, 14)
	if atr == 0 {
		return holdSignal()
	}

	// Scan the lookback window for the prior swing high/low
	window := s.prices[n-s.lookback : n-3]
	priorHigh, priorHighIdx := window[0], 0
	priorLow, priorLowIdx := window[0], 0
	for i, p := range window {
		if p > priorHigh {
			priorHigh = p
			priorHighIdx = i
		}
		if p < priorLow {
			priorLow = p
			priorLowIdx = i
		}
	}

	// RSI at the prior swing points
	priorHighRSI := RSI(s.prices[:n-s.lookback+priorHighIdx+1], 14)
	priorLowRSI := RSI(s.prices[:n-s.lookback+priorLowIdx+1], 14)

	atrPct := (atr / candle.Price) * 100
	slPct := math.Max(atrPct*1.15, 0.25)
	if slPct > 0.90 {
		slPct = 0.90
	}
	tpPct := slPct * 2.0

	// Bullish divergence: price lower low, RSI higher low → trend reversal up
	if candle.Price < priorLow && rsi > priorLowRSI && rsi < 45 && macdHist > 0 {
		confidence := 0.93 + math.Min((45-rsi)/60+(rsi-priorLowRSI)/40, 0.27)
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionBuy, slPct, tpPct, confidence)
	}

	// Bearish divergence: price higher high, RSI lower high → trend reversal down
	if candle.Price > priorHigh && rsi < priorHighRSI && rsi > 55 && macdHist < 0 {
		confidence := 0.93 + math.Min((rsi-55)/60+(priorHighRSI-rsi)/40, 0.27)
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionSell, slPct, tpPct, confidence)
	}

	return holdSignal()
}

// TripleTrendConfluenceScalper requires three independent trend indicators to
// all point in the same direction simultaneously before firing a signal.
//
//   - HullMA direction (fast, low-lag)
//   - Supertrend direction (ATR-based dynamic S/R flip)
//   - EMA ribbon (5 > 13 > 34 or inverted)
//
// When all three agree the market is clearly trending. The ATR-adaptive SL keeps
// risk proportional and the 2.5R target captures the extension move.
type TripleTrendConfluenceScalper struct {
	baseScalper
	prevHMA   float64
	prevTrend int // +1 bullish, -1 bearish
	cooldown  int
	lastBar   int
}

func NewTripleTrendConfluenceScalper() *TripleTrendConfluenceScalper {
	return &TripleTrendConfluenceScalper{
		baseScalper: baseScalper{name: "TripleTrend_Confluence_Scalp", maxBuf: defaultBufSize},
		cooldown:    7,
	}
}

func (s *TripleTrendConfluenceScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *TripleTrendConfluenceScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	n := len(s.prices)
	if n < 50 {
		return holdSignal()
	}
	if n-s.lastBar < s.cooldown {
		return holdSignal()
	}

	hma := HullMA(s.prices, 16)
	e5 := EMA(s.prices, 5)
	e13 := EMA(s.prices, 13)
	e34 := EMA(s.prices, 34)
	atr := ATR(s.prices, 14)
	rsi := RSI(s.prices, 14)

	if atr == 0 {
		return holdSignal()
	}

	// Supertrend: mid ± 2*ATR, trend determined by whether price is above/below
	mid := (s.prices[n-1] + s.prices[n-2]) / 2
	upperBand := mid + 2.2*atr
	lowerBand := mid - 2.2*atr
	trend := 0
	if candle.Price > upperBand {
		trend = 1
	} else if candle.Price < lowerBand {
		trend = -1
	}

	// HMA direction
	hmaRising := s.prevHMA != 0 && hma > s.prevHMA
	hmaFalling := s.prevHMA != 0 && hma < s.prevHMA
	defer func() { s.prevHMA = hma; s.prevTrend = trend }()

	// EMA ribbon
	ribbonBull := e5 > e13 && e13 > e34
	ribbonBear := e5 < e13 && e13 < e34

	atrPct := (atr / candle.Price) * 100
	slPct := math.Max(atrPct*1.10, 0.22)
	if slPct > 0.85 {
		slPct = 0.85
	}
	tpPct := slPct * 2.5
	confidence := 0.94 + math.Min(math.Abs(float64(trend))*0.06, 0.16)

	if trend == 1 && hmaRising && ribbonBull && rsi > 45 && rsi < 70 {
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionBuy, slPct, tpPct, confidence)
	}
	if trend == -1 && hmaFalling && ribbonBear && rsi < 55 && rsi > 30 {
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionSell, slPct, tpPct, confidence)
	}
	return holdSignal()
}

// VolumeDeltaSpike Scalper models order-flow imbalance from signed volume.
// When buy-volume dramatically exceeds sell-volume (or vice versa) over a
// rolling window AND price confirms the direction, it fires a momentum entry.
//
// The key differentiator: it weights recent candles 2× vs old candles in the
// window, so a spike that's building (not decaying) gets higher weight.
type VolumeDeltaSpikeScalper struct {
	baseScalper
	deltaWindow []float64
	volumes     []float64
	window      int
	cooldown    int
	lastBar     int
}

func NewVolumeDeltaSpikeScalper() *VolumeDeltaSpikeScalper {
	return &VolumeDeltaSpikeScalper{
		baseScalper: baseScalper{name: "VolumeDelta_Spike_Scalp", maxBuf: defaultBufSize},
		window:      20,
		cooldown:    6,
	}
}

func (s *VolumeDeltaSpikeScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *VolumeDeltaSpikeScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	vol := candle.Quantity
	if vol <= 0 {
		vol = 1
	}
	s.volumes = appendRollingFloat(s.volumes, vol, defaultBufSize)

	n := len(s.prices)
	// Signed volume delta: positive if up-candle, negative if down-candle
	delta := vol
	if n >= 2 && candle.Price < s.prices[n-2] {
		delta = -vol
	}
	s.deltaWindow = appendRollingFloat(s.deltaWindow, delta, defaultBufSize)

	if n < s.window+10 || len(s.volumes) < s.window {
		return holdSignal()
	}
	if n-s.lastBar < s.cooldown {
		return holdSignal()
	}

	// Weighted delta: recent candles counted 2×
	dWindow := s.deltaWindow[len(s.deltaWindow)-s.window:]
	half := s.window / 2
	weightedDelta := 0.0
	totalWeight := 0.0
	for i, d := range dWindow {
		w := 1.0
		if i >= half {
			w = 2.0 // Recent half weighted double
		}
		weightedDelta += d * w
		totalWeight += math.Abs(d) * w
	}
	if totalWeight == 0 {
		return holdSignal()
	}
	imbalance := weightedDelta / totalWeight // -1 to +1

	avgVol := tailAverage(s.volumes, s.window)
	if avgVol == 0 {
		return holdSignal()
	}
	volRatio := vol / avgVol

	fastEMA := EMA(s.prices, 8)
	slowEMA := EMA(s.prices, 21)
	rsi := RSI(s.prices, 14)
	atr := ATR(s.prices, 14)
	if atr == 0 {
		return holdSignal()
	}

	atrPct := (atr / candle.Price) * 100
	slPct := math.Max(atrPct*1.05, 0.20)
	if slPct > 0.75 {
		slPct = 0.75
	}
	tpPct := slPct * 2.2
	confidence := 0.93 + math.Min(math.Abs(imbalance)*0.20+math.Max(volRatio-1, 0)*0.08, 0.27)

	if imbalance > 0.30 && fastEMA > slowEMA && rsi > 45 && rsi < 72 && volRatio > 1.10 {
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionBuy, slPct, tpPct, confidence)
	}
	if imbalance < -0.30 && fastEMA < slowEMA && rsi < 55 && rsi > 28 && volRatio > 1.10 {
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionSell, slPct, tpPct, confidence)
	}
	return holdSignal()
}

// MACDZeroCrossConfluenceScalper triggers on MACD histogram zero-line crosses
// only when supported by two confirming conditions:
//  1. Price is on the correct side of the 20-period VWAP proxy
//  2. ADX ≥ 22 (avoid crosses in flat/ranging markets)
//
// Zero-line crosses are stronger signals than histogram direction flips alone
// because they represent full momentum reversal, not just deceleration.
type MACDZeroCrossConfluenceScalper struct {
	baseScalper
	volumes  []float64
	prevHist float64
	cooldown int
	lastBar  int
}

func NewMACDZeroCrossConfluenceScalper() *MACDZeroCrossConfluenceScalper {
	return &MACDZeroCrossConfluenceScalper{
		baseScalper: baseScalper{name: "MACD_ZeroCross_Confluence_Scalp", maxBuf: defaultBufSize},
		cooldown:    8,
	}
}

func (s *MACDZeroCrossConfluenceScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *MACDZeroCrossConfluenceScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	vol := candle.Quantity
	if vol <= 0 {
		vol = 1
	}
	s.volumes = appendRollingFloat(s.volumes, vol, defaultBufSize)

	n := len(s.prices)
	if n < 45 || len(s.volumes) < 25 {
		return holdSignal()
	}
	if n-s.lastBar < s.cooldown {
		return holdSignal()
	}

	_, _, hist := MACD(s.prices, 12, 26, 9)
	adx := ADX(s.prices, 14)
	vwap := RollingVWAP(s.prices, s.volumes, 30)
	rsi := RSI(s.prices, 14)
	atr := ATR(s.prices, 14)

	defer func() { s.prevHist = hist }()

	if atr == 0 || vwap == 0 {
		return holdSignal()
	}
	if adx < 22 {
		return holdSignal() // Skip choppy markets
	}

	// Require actual zero-line cross (not just direction change within same side)
	zeroCrossBull := s.prevHist < 0 && hist > 0
	zeroCrossBear := s.prevHist > 0 && hist < 0

	if !zeroCrossBull && !zeroCrossBear {
		return holdSignal()
	}

	atrPct := (atr / candle.Price) * 100
	slPct := math.Max(atrPct*1.10, 0.22)
	if slPct > 0.80 {
		slPct = 0.80
	}
	tpPct := slPct * 2.2
	confidence := 0.94 + math.Min(adx/200, 0.20)

	if zeroCrossBull && candle.Price > vwap && rsi > 45 {
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionBuy, slPct, tpPct, confidence)
	}
	if zeroCrossBear && candle.Price < vwap && rsi < 55 {
		s.lastBar = n
		return signalWithConfidence(candle.Symbol, ActionSell, slPct, tpPct, confidence)
	}
	return holdSignal()
}

// BollingerWalkScalper detects "Bollinger Band walking" — when price closes
// outside the band for 3+ consecutive candles while RSI confirms momentum.
// Walking the bands is the signature of a genuine strong trend continuation,
// not an overextension ready to reverse. The strategy rides the walk until
// price closes back inside the band (managed by the SL/trailing stop).
type BollingerWalkScalper struct {
	baseScalper
	consecutiveAbove int
	consecutiveBelow int
	cooldown         int
	lastBar          int
}

func NewBollingerWalkScalper() *BollingerWalkScalper {
	return &BollingerWalkScalper{
		baseScalper: baseScalper{name: "BollingerWalk_Trend_Scalp", maxBuf: defaultBufSize},
		cooldown:    5,
	}
}

func (s *BollingerWalkScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *BollingerWalkScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	n := len(s.prices)
	if n < 25 {
		return holdSignal()
	}
	if n-s.lastBar < s.cooldown {
		return holdSignal()
	}

	upper, mid, lower := BollingerBands(s.prices, 20, 2.0)
	rsi := RSI(s.prices, 14)
	adx := ADX(s.prices, 14)
	atr := ATR(s.prices, 14)

	if mid == 0 || atr == 0 {
		return holdSignal()
	}

	// Track consecutive closes outside the band
	if candle.Price > upper {
		s.consecutiveAbove++
		s.consecutiveBelow = 0
	} else if candle.Price < lower {
		s.consecutiveBelow++
		s.consecutiveAbove = 0
	} else {
		s.consecutiveAbove = 0
		s.consecutiveBelow = 0
	}

	// Need 3 consecutive closes outside band to confirm the walk
	if s.consecutiveAbove < 3 && s.consecutiveBelow < 3 {
		return holdSignal()
	}

	atrPct := (atr / candle.Price) * 100
	slPct := math.Max(atrPct*1.20, 0.25)
	if slPct > 1.00 {
		slPct = 1.00
	}
	tpPct := slPct * 2.0
	confidence := 0.92 + math.Min(adx/150, 0.25)

	// Walking upper band: strong uptrend, RSI allowed to be extended (60–80 is OK when walking)
	if s.consecutiveAbove >= 3 && rsi > 55 && adx > 20 {
		s.lastBar = n
		s.consecutiveAbove = 0 // Reset so we don't keep re-entering
		return signalWithConfidence(candle.Symbol, ActionBuy, slPct, tpPct, confidence)
	}
	// Walking lower band: strong downtrend
	if s.consecutiveBelow >= 3 && rsi < 45 && adx > 20 {
		s.lastBar = n
		s.consecutiveBelow = 0
		return signalWithConfidence(candle.Symbol, ActionSell, slPct, tpPct, confidence)
	}
	return holdSignal()
}
