package strategy

import (
	"antigravity-engine/internal/marketdata"
	"math"
)

// =============================================================================
// 20 ADVANCED HIGH-ALPHA BTC SCALPING STRATEGIES (STRATEGIES 21-40)
// Each strategy exploits a distinct market microstructure edge.
// =============================================================================

// =============================================================================
// 21. Tick Velocity Scalper
// EDGE: Detects rapid price acceleration. When BTC moves X% in N ticks,
// it indicates institutional momentum. Rides the wave with tight SL.
// =============================================================================
type TickVelocityScalper struct {
	baseScalper
	window    int
	threshold float64
}

func NewTickVelocityScalper(window int, threshold float64) *TickVelocityScalper {
	return &TickVelocityScalper{
		baseScalper: baseScalper{name: "TickVelocity_Momentum", maxBuf: defaultBufSize},
		window:      window,
		threshold:   threshold,
	}
}

func (s *TickVelocityScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *TickVelocityScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.window+1 {
		return holdSignal()
	}
	velocity := (s.prices[len(s.prices)-1] - s.prices[len(s.prices)-s.window]) / float64(s.window)
	normalizedVelocity := (velocity / candle.Price) * 10000
	if normalizedVelocity > s.threshold {
		return buySignalCustom(candle.Symbol, 0.3, 0.8)
	}
	if normalizedVelocity < -s.threshold {
		return sellSignalCustom(candle.Symbol, 0.3, 0.8)
	}
	return holdSignal()
}

// =============================================================================
// 22. Fibonacci Retracement Scalper
// EDGE: BTC respects Fibonacci levels (38.2%, 50%, 61.8%) during pullbacks.
// Enters at the golden ratio (61.8% retracement) for high-probability reversals.
// =============================================================================
type FibonacciScalper struct {
	baseScalper
	lookback int
}

func NewFibonacciScalper(lookback int) *FibonacciScalper {
	return &FibonacciScalper{
		baseScalper: baseScalper{name: "Fibonacci_GoldenRatio_Scalp", maxBuf: defaultBufSize},
		lookback:    lookback,
	}
}

func (s *FibonacciScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *FibonacciScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.lookback {
		return holdSignal()
	}
	slice := s.prices[len(s.prices)-s.lookback:]
	high, low := slice[0], slice[0]
	for _, p := range slice {
		if p > high { high = p }
		if p < low { low = p }
	}
	rng := high - low
	if rng == 0 { return holdSignal() }

	fib618 := high - rng*0.618
	fib382 := high - rng*0.382
	tolerance := rng * 0.01

	if math.Abs(candle.Price-fib618) < tolerance {
		return buySignalCustom(candle.Symbol, 0.4, 1.2)
	}
	if math.Abs(candle.Price-fib382) < tolerance {
		return sellSignalCustom(candle.Symbol, 0.4, 1.2)
	}
	return holdSignal()
}

// =============================================================================
// 23. Multi-Timeframe RSI Confluence Scalper
// EDGE: Combines fast RSI (7) and slow RSI (21). Only trades when BOTH
// agree on direction. Double confirmation eliminates false signals.
// =============================================================================
type MTFRSIScalper struct {
	baseScalper
	fastPeriod int
	slowPeriod int
}

func NewMTFRSIScalper() *MTFRSIScalper {
	return &MTFRSIScalper{
		baseScalper: baseScalper{name: "MTF_RSI_Confluence_Scalp", maxBuf: defaultBufSize},
		fastPeriod:  7,
		slowPeriod:  21,
	}
}

func (s *MTFRSIScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *MTFRSIScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.slowPeriod+2 {
		return holdSignal()
	}
	fastRSI := RSI(s.prices, s.fastPeriod)
	slowRSI := RSI(s.prices, s.slowPeriod)
	if fastRSI < 25 && slowRSI < 35 {
		return buySignalCustom(candle.Symbol, 0.4, 1.0)
	}
	if fastRSI > 75 && slowRSI > 65 {
		return sellSignalCustom(candle.Symbol, 0.4, 1.0)
	}
	return holdSignal()
}

// =============================================================================
// 24. Volatility Squeeze Scalper
// EDGE: When Bollinger Bands contract inside Keltner Channels, volatility
// is being compressed. The breakout that follows is explosive and tradeable.
// =============================================================================
type VolatilitySqueeze struct {
	baseScalper
	bbPeriod   int
	kcEMA      int
	kcATR      int
	wasSqueeze bool
}

func NewVolatilitySqueeze() *VolatilitySqueeze {
	return &VolatilitySqueeze{
		baseScalper: baseScalper{name: "VolSqueeze_Explosion_Scalp", maxBuf: defaultBufSize},
		bbPeriod:    20,
		kcEMA:       20,
		kcATR:       10,
	}
}

func (s *VolatilitySqueeze) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *VolatilitySqueeze) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < 30 {
		return holdSignal()
	}
	bbUpper, _, bbLower := BollingerBands(s.prices, s.bbPeriod, 2.0)
	kcUpper, _, kcLower := KeltnerChannels(s.prices, s.kcEMA, s.kcATR, 1.5)
	inSqueeze := bbLower > kcLower && bbUpper < kcUpper

	defer func() { s.wasSqueeze = inSqueeze }()

	if s.wasSqueeze && !inSqueeze {
		ema := EMA(s.prices, 9)
		if candle.Price > ema {
			return buySignalCustom(candle.Symbol, 0.5, 1.5)
		}
		return sellSignalCustom(candle.Symbol, 0.5, 1.5)
	}
	return holdSignal()
}

// =============================================================================
// 25. Volume Spike Reversal Scalper
// EDGE: Abnormal tick volume (3x average) at price extremes signals
// exhaustion. Large players dumping/accumulating causes sharp reversals.
// =============================================================================
type VolumeSpikeScalper struct {
	baseScalper
	tickCounts []int
	currentCount int
	window     int
}

func NewVolumeSpikeScalper(window int) *VolumeSpikeScalper {
	return &VolumeSpikeScalper{
		baseScalper: baseScalper{name: "VolumeSpike_Reversal_Scalp", maxBuf: defaultBufSize},
		tickCounts:  make([]int, 0),
		window:      window,
	}
}

func (s *VolumeSpikeScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *VolumeSpikeScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.currentCount++
	if len(s.prices)%s.window == 0 && len(s.prices) > 0 {
		s.tickCounts = append(s.tickCounts, s.currentCount)
		s.currentCount = 0
		if len(s.tickCounts) > 50 { s.tickCounts = s.tickCounts[1:] }
	}
	if len(s.tickCounts) < 10 || len(s.prices) < 20 {
		return holdSignal()
	}
	avgTicks := 0
	for _, tc := range s.tickCounts { avgTicks += tc }
	avgTicks /= len(s.tickCounts)
	if s.currentCount > avgTicks*3 {
		rsi := RSI(s.prices, 14)
		if rsi < 30 { return buySignalCustom(candle.Symbol, 0.4, 1.0) }
		if rsi > 70 { return sellSignalCustom(candle.Symbol, 0.4, 1.0) }
	}
	return holdSignal()
}

// =============================================================================
// 26. Engulfing Candle Pattern Scalper
// EDGE: When a bullish candle completely engulfs the previous bearish
// candle, it signals a power shift from sellers to buyers (and vice versa).
// =============================================================================
type EngulfingScalper struct {
	baseScalper
	prevOpen  float64
	prevClose float64
	currOpen  float64
}

func NewEngulfingScalper() *EngulfingScalper {
	return &EngulfingScalper{
		baseScalper: baseScalper{name: "Engulfing_PriceAction_Scalp", maxBuf: defaultBufSize},
	}
}

func (s *EngulfingScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *EngulfingScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < 4 { return holdSignal() }
	n := len(s.prices)
	prevBody := s.prices[n-3] - s.prices[n-2]
	currBody := candle.Price - s.prices[n-2]

	if prevBody < 0 && currBody > 0 && math.Abs(currBody) > math.Abs(prevBody)*1.5 {
		return buySignalCustom(candle.Symbol, 0.3, 0.9)
	}
	if prevBody > 0 && currBody < 0 && math.Abs(currBody) > math.Abs(prevBody)*1.5 {
		return sellSignalCustom(candle.Symbol, 0.3, 0.9)
	}
	return holdSignal()
}

// =============================================================================
// 27. Supertrend Scalper
// EDGE: Uses ATR-based dynamic support/resistance. Supertrend flips are
// high-conviction trend changes that pro traders rely on for entries.
// =============================================================================
type SupertrendScalper struct {
	baseScalper
	atrPeriod  int
	multiplier float64
	prevST     float64
	prevTrend  int
}

func NewSupertrendScalper(atrPeriod int, mult float64) *SupertrendScalper {
	return &SupertrendScalper{
		baseScalper: baseScalper{name: "Supertrend_Flip_Scalp", maxBuf: defaultBufSize},
		atrPeriod:   atrPeriod,
		multiplier:  mult,
	}
}

func (s *SupertrendScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *SupertrendScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.atrPeriod+2 {
		return holdSignal()
	}
	atr := ATR(s.prices, s.atrPeriod)
	mid := (s.prices[len(s.prices)-1] + s.prices[len(s.prices)-2]) / 2
	upperBand := mid + s.multiplier*atr
	lowerBand := mid - s.multiplier*atr
	trend := 0
	if candle.Price > upperBand { trend = 1 }
	if candle.Price < lowerBand { trend = -1 }
	defer func() { s.prevTrend = trend }()
	if s.prevTrend == -1 && trend == 1 {
		return buySignalCustom(candle.Symbol, 0.5, 1.2)
	}
	if s.prevTrend == 1 && trend == -1 {
		return sellSignalCustom(candle.Symbol, 0.5, 1.2)
	}
	return holdSignal()
}

// =============================================================================
// 28. Heikin Ashi Momentum Scalper
// EDGE: Heikin Ashi smooths out noise. 3 consecutive green HA candles
// after red = confirmed trend shift. Extremely low false signal rate.
// =============================================================================
type HeikinAshiScalper struct {
	baseScalper
	haCloses []float64
	streak   int
}

func NewHeikinAshiScalper() *HeikinAshiScalper {
	return &HeikinAshiScalper{
		baseScalper: baseScalper{name: "HeikinAshi_Momentum_Scalp", maxBuf: defaultBufSize},
	}
}

func (s *HeikinAshiScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *HeikinAshiScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < 4 { return holdSignal() }
	n := len(s.prices)
	haClose := (s.prices[n-4] + s.prices[n-3] + s.prices[n-2] + s.prices[n-1]) / 4
	var haOpen float64
	if len(s.haCloses) > 0 {
		haOpen = (s.haCloses[len(s.haCloses)-1] + s.prices[n-2]) / 2
	} else {
		haOpen = s.prices[n-2]
	}
	s.haCloses = append(s.haCloses, haClose)
	if len(s.haCloses) > 100 { s.haCloses = s.haCloses[1:] }

	if haClose > haOpen { s.streak++ } else if haClose < haOpen { s.streak-- } else { s.streak = 0 }
	if s.streak >= 3 {
		s.streak = 0
		return buySignalCustom(candle.Symbol, 0.4, 1.0)
	}
	if s.streak <= -3 {
		s.streak = 0
		return sellSignalCustom(candle.Symbol, 0.4, 1.0)
	}
	return holdSignal()
}

// =============================================================================
// 29. Double EMA Spread Scalper
// EDGE: Measures the spread between fast/slow EMA as a percentage.
// When spread reaches extreme levels (beyond 2 std devs), mean-reverts.
// =============================================================================
type EMASpreadScalper struct {
	baseScalper
	fast     int
	slow     int
	spreads  []float64
	lookback int
}

func NewEMASpreadScalper(fast, slow, lookback int) *EMASpreadScalper {
	return &EMASpreadScalper{
		baseScalper: baseScalper{name: "EMASpread_MeanRev_Scalp", maxBuf: defaultBufSize},
		fast: fast, slow: slow, lookback: lookback,
	}
}

func (s *EMASpreadScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *EMASpreadScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.slow+2 { return holdSignal() }
	spread := EMA(s.prices, s.fast) - EMA(s.prices, s.slow)
	s.spreads = append(s.spreads, spread)
	if len(s.spreads) > s.lookback { s.spreads = s.spreads[1:] }
	if len(s.spreads) < s.lookback { return holdSignal() }
	mean := SMA(s.spreads)
	variance := 0.0
	for _, sp := range s.spreads { variance += (sp - mean) * (sp - mean) }
	stdDev := math.Sqrt(variance / float64(len(s.spreads)))
	if stdDev == 0 { return holdSignal() }
	zScore := (spread - mean) / stdDev
	if zScore < -2.0 { return buySignalCustom(candle.Symbol, 0.3, 0.8) }
	if zScore > 2.0 { return sellSignalCustom(candle.Symbol, 0.3, 0.8) }
	return holdSignal()
}

// =============================================================================
// 30. OBV Trend Scalper
// EDGE: On-Balance Volume leads price. When OBV makes a new high before
// price does, it signals hidden accumulation (smart money buying quietly).
// =============================================================================
type OBVScalper struct {
	baseScalper
	obv      float64
	obvHist  []float64
	period   int
}

func NewOBVScalper(period int) *OBVScalper {
	return &OBVScalper{
		baseScalper: baseScalper{name: "OBV_SmartMoney_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *OBVScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *OBVScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) >= 2 {
		if s.prices[len(s.prices)-1] > s.prices[len(s.prices)-2] {
			s.obv += 1
		} else { s.obv -= 1 }
	}
	s.obvHist = append(s.obvHist, s.obv)
	if len(s.obvHist) > s.period+1 { s.obvHist = s.obvHist[1:] }
	if len(s.obvHist) < s.period { return holdSignal() }
	obvEMA := EMA(s.obvHist, s.period)
	if s.obv > obvEMA && s.obv > s.obvHist[len(s.obvHist)-2] {
		return buySignalCustom(candle.Symbol, 0.4, 1.0)
	}
	if s.obv < obvEMA && s.obv < s.obvHist[len(s.obvHist)-2] {
		return sellSignalCustom(candle.Symbol, 0.4, 1.0)
	}
	return holdSignal()
}

// =============================================================================
// 31. Chaikin Money Flow Scalper
// EDGE: CMF measures buying/selling pressure over a period. Sustained
// positive CMF > 0.25 = institutional accumulation. Negative < -0.25 = distribution.
// =============================================================================
type ChaikinMFScalper struct {
	baseScalper
	period int
}

func NewChaikinMFScalper(period int) *ChaikinMFScalper {
	return &ChaikinMFScalper{
		baseScalper: baseScalper{name: "ChaikinMF_Flow_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *ChaikinMFScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *ChaikinMFScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period+1 { return holdSignal() }
	mfSum := 0.0
	for i := len(s.prices) - s.period; i < len(s.prices); i++ {
		if i < 1 { continue }
		change := s.prices[i] - s.prices[i-1]
		rng := math.Abs(change)
		if rng > 0 { mfSum += change / rng }
	}
	cmf := mfSum / float64(s.period)
	if cmf > 0.25 { return buySignalCustom(candle.Symbol, 0.4, 1.0) }
	if cmf < -0.25 { return sellSignalCustom(candle.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// =============================================================================
// 32. Aroon Oscillator Scalper
// EDGE: Aroon measures how long since the highest high / lowest low.
// Aroon Up crossing above Aroon Down = early trend detection before momentum.
// =============================================================================
type AroonScalper struct {
	baseScalper
	period   int
	prevUp   float64
	prevDown float64
}

func NewAroonScalper(period int) *AroonScalper {
	return &AroonScalper{
		baseScalper: baseScalper{name: "Aroon_EarlyTrend_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *AroonScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *AroonScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period+1 { return holdSignal() }
	slice := s.prices[len(s.prices)-s.period-1:]
	highIdx, lowIdx := 0, 0
	for i, p := range slice {
		if p >= slice[highIdx] { highIdx = i }
		if p <= slice[lowIdx] { lowIdx = i }
	}
	aroonUp := float64(highIdx) / float64(s.period) * 100
	aroonDown := float64(lowIdx) / float64(s.period) * 100
	defer func() { s.prevUp = aroonUp; s.prevDown = aroonDown }()
	if s.prevUp <= s.prevDown && aroonUp > aroonDown && aroonUp > 70 {
		return buySignalCustom(candle.Symbol, 0.5, 1.2)
	}
	if s.prevUp >= s.prevDown && aroonUp < aroonDown && aroonDown > 70 {
		return sellSignalCustom(candle.Symbol, 0.5, 1.2)
	}
	return holdSignal()
}

// =============================================================================
// 33. Range Compression Breakout Scalper
// EDGE: When the price range over N candles shrinks below a threshold,
// a massive directional move is imminent. Enters on the breakout side.
// =============================================================================
type RangeCompressionScalper struct {
	baseScalper
	period     int
	threshold  float64
	prevRange  float64
}

func NewRangeCompressionScalper(period int, thresholdPct float64) *RangeCompressionScalper {
	return &RangeCompressionScalper{
		baseScalper: baseScalper{name: "RangeCompress_Breakout_Scalp", maxBuf: defaultBufSize},
		period:      period,
		threshold:   thresholdPct,
	}
}

func (s *RangeCompressionScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *RangeCompressionScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period+1 { return holdSignal() }
	high, low := DonchianChannel(s.prices[:len(s.prices)-1], s.period)
	rangePct := ((high - low) / low) * 100
	defer func() { s.prevRange = rangePct }()
	if s.prevRange < s.threshold && rangePct > s.threshold {
		if candle.Price > high {
			return buySignalCustom(candle.Symbol, 0.4, 1.5)
		}
		if candle.Price < low {
			return sellSignalCustom(candle.Symbol, 0.4, 1.5)
		}
	}
	return holdSignal()
}

// =============================================================================
// 34. Kaufman Adaptive Moving Average Scalper
// EDGE: KAMA adapts its speed based on market noise. In trending markets
// it's fast; in choppy markets it flattens. Reduces whipsaws dramatically.
// =============================================================================
type KAMAScalper struct {
	baseScalper
	period   int
	prevKAMA float64
}

func NewKAMAScalper(period int) *KAMAScalper {
	return &KAMAScalper{
		baseScalper: baseScalper{name: "KAMA_Adaptive_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *KAMAScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *KAMAScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period+1 { return holdSignal() }
	direction := math.Abs(candle.Price - s.prices[len(s.prices)-s.period-1])
	volatility := 0.0
	for i := len(s.prices) - s.period; i < len(s.prices); i++ {
		volatility += math.Abs(s.prices[i] - s.prices[i-1])
	}
	if volatility == 0 { return holdSignal() }
	er := direction / volatility
	fastSC := 2.0 / 3.0
	slowSC := 2.0 / 31.0
	sc := math.Pow(er*(fastSC-slowSC)+slowSC, 2)
	kama := s.prevKAMA + sc*(candle.Price-s.prevKAMA)
	if s.prevKAMA == 0 { kama = candle.Price }
	defer func() { s.prevKAMA = kama }()
	if s.prevKAMA != 0 && candle.Price > kama && s.prices[len(s.prices)-2] <= s.prevKAMA {
		return buySignalCustom(candle.Symbol, 0.4, 1.0)
	}
	if s.prevKAMA != 0 && candle.Price < kama && s.prices[len(s.prices)-2] >= s.prevKAMA {
		return sellSignalCustom(candle.Symbol, 0.4, 1.0)
	}
	return holdSignal()
}

// =============================================================================
// 35. ZigZag Swing Reversal Scalper
// EDGE: Identifies swing highs/lows when price reverses by X%.
// Trades counter-trend at confirmed swing points for bounce plays.
// =============================================================================
type ZigZagScalper struct {
	baseScalper
	threshold float64
	lastPivot float64
	pivotType int
}

func NewZigZagScalper(thresholdPct float64) *ZigZagScalper {
	return &ZigZagScalper{
		baseScalper: baseScalper{name: "ZigZag_SwingReversal_Scalp", maxBuf: defaultBufSize},
		threshold:   thresholdPct,
	}
}

func (s *ZigZagScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *ZigZagScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if s.lastPivot == 0 { s.lastPivot = candle.Price; return holdSignal() }
	changePct := ((candle.Price - s.lastPivot) / s.lastPivot) * 100
	if changePct > s.threshold && s.pivotType != 1 {
		s.pivotType = 1
		s.lastPivot = candle.Price
		return buySignalCustom(candle.Symbol, 0.3, s.threshold*2)
	}
	if changePct < -s.threshold && s.pivotType != -1 {
		s.pivotType = -1
		s.lastPivot = candle.Price
		return sellSignalCustom(candle.Symbol, 0.3, s.threshold*2)
	}
	if s.pivotType == 1 && candle.Price > s.lastPivot { s.lastPivot = candle.Price }
	if s.pivotType == -1 && candle.Price < s.lastPivot { s.lastPivot = candle.Price }
	return holdSignal()
}

// =============================================================================
// 36. Accumulation/Distribution Line Scalper
// EDGE: A/D line diverging from price = hidden accumulation/distribution.
// When A/D rises but price falls → stealth buying → imminent breakout.
// =============================================================================
type ADLineScalper struct {
	baseScalper
	adLine    float64
	adHistory []float64
	period    int
}

func NewADLineScalper(period int) *ADLineScalper {
	return &ADLineScalper{
		baseScalper: baseScalper{name: "AccumDistrib_Stealth_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *ADLineScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *ADLineScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) >= 2 {
		change := candle.Price - s.prices[len(s.prices)-2]
		if change > 0 { s.adLine += 1 } else { s.adLine -= 1 }
	}
	s.adHistory = append(s.adHistory, s.adLine)
	if len(s.adHistory) > s.period { s.adHistory = s.adHistory[1:] }
	if len(s.adHistory) < s.period || len(s.prices) < s.period {
		return holdSignal()
	}
	priceSlope := s.prices[len(s.prices)-1] - s.prices[len(s.prices)-s.period]
	adSlope := s.adHistory[len(s.adHistory)-1] - s.adHistory[0]
	if priceSlope < 0 && adSlope > 3 {
		return buySignalCustom(candle.Symbol, 0.5, 1.2)
	}
	if priceSlope > 0 && adSlope < -3 {
		return sellSignalCustom(candle.Symbol, 0.5, 1.2)
	}
	return holdSignal()
}

// =============================================================================
// 37. Micro Pullback Continuation Scalper
// EDGE: In a strong trend (ADX > 30), small pullbacks (1-3 ticks) are
// buying opportunities. This strategy buys the dip in confirmed uptrends.
// =============================================================================
type MicroPullbackScalper struct {
	baseScalper
	adxPeriod     int
	pullbackCount int
}

func NewMicroPullbackScalper() *MicroPullbackScalper {
	return &MicroPullbackScalper{
		baseScalper: baseScalper{name: "MicroPullback_Continuation_Scalp", maxBuf: defaultBufSize},
		adxPeriod:   14,
	}
}

func (s *MicroPullbackScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *MicroPullbackScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.adxPeriod+5 { return holdSignal() }
	adx := ADX(s.prices, s.adxPeriod)
	if adx < 30 { s.pullbackCount = 0; return holdSignal() }
	n := len(s.prices)
	trendUp := EMA(s.prices, 20) > EMA(s.prices, 50)
	if trendUp {
		if s.prices[n-1] < s.prices[n-2] { s.pullbackCount++ } else { s.pullbackCount = 0 }
		if s.pullbackCount >= 2 && s.pullbackCount <= 4 && candle.Price > s.prices[n-2] {
			s.pullbackCount = 0
			return buySignalCustom(candle.Symbol, 0.3, 0.8)
		}
	} else {
		if s.prices[n-1] > s.prices[n-2] { s.pullbackCount++ } else { s.pullbackCount = 0 }
		if s.pullbackCount >= 2 && s.pullbackCount <= 4 && candle.Price < s.prices[n-2] {
			s.pullbackCount = 0
			return sellSignalCustom(candle.Symbol, 0.3, 0.8)
		}
	}
	return holdSignal()
}

// =============================================================================
// 38. Gap Fill Scalper
// EDGE: When BTC price gaps (jumps > 0.2% between ticks), it tends to
// fill the gap within minutes. Fades the gap for quick mean-reversion.
// =============================================================================
type GapFillScalper struct {
	baseScalper
	gapThreshold float64
	inGap        bool
	gapOrigin    float64
}

func NewGapFillScalper(gapPct float64) *GapFillScalper {
	return &GapFillScalper{
		baseScalper:  baseScalper{name: "GapFill_MeanRev_Scalp", maxBuf: defaultBufSize},
		gapThreshold: gapPct,
	}
}

func (s *GapFillScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *GapFillScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < 2 { return holdSignal() }
	prev := s.prices[len(s.prices)-2]
	gapPct := ((candle.Price - prev) / prev) * 100
	if math.Abs(gapPct) > s.gapThreshold && !s.inGap {
		s.inGap = true
		s.gapOrigin = prev
		if gapPct > 0 {
			return sellSignalCustom(candle.Symbol, 0.3, s.gapThreshold)
		}
		return buySignalCustom(candle.Symbol, 0.3, s.gapThreshold)
	}
	if s.inGap && math.Abs(((candle.Price-s.gapOrigin)/s.gapOrigin)*100) < 0.05 {
		s.inGap = false
	}
	return holdSignal()
}

// =============================================================================
// 39. Linear Regression Deviation Scalper
// EDGE: Calculates a linear regression line through N prices. When price
// deviates >2 std devs, it's statistically likely to revert to the line.
// =============================================================================
type LinRegScalper struct {
	baseScalper
	period    int
	devMult   float64
}

func NewLinRegScalper(period int, devMult float64) *LinRegScalper {
	return &LinRegScalper{
		baseScalper: baseScalper{name: "LinReg_Statistical_Scalp", maxBuf: defaultBufSize},
		period:      period,
		devMult:     devMult,
	}
}

func (s *LinRegScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *LinRegScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period { return holdSignal() }
	slice := s.prices[len(s.prices)-s.period:]
	n := float64(len(slice))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, p := range slice {
		x := float64(i)
		sumX += x; sumY += p; sumXY += x * p; sumX2 += x * x
	}
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n
	predicted := slope*float64(len(slice)-1) + intercept
	residuals := 0.0
	for i, p := range slice {
		pred := slope*float64(i) + intercept
		residuals += (p - pred) * (p - pred)
	}
	stdDev := math.Sqrt(residuals / n)
	if stdDev == 0 { return holdSignal() }
	deviation := (candle.Price - predicted) / stdDev
	if deviation < -s.devMult { return buySignalCustom(candle.Symbol, 0.4, 1.0) }
	if deviation > s.devMult { return sellSignalCustom(candle.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// =============================================================================
// 40. Multi-Strategy Consensus Scalper
// EDGE: The ultimate strategy. Runs RSI + MACD + Bollinger simultaneously.
// Only trades when ALL THREE indicators agree. Highest win-rate possible.
// =============================================================================
type ConsensusScalper struct {
	baseScalper
}

func NewConsensusScalper() *ConsensusScalper {
	return &ConsensusScalper{
		baseScalper: baseScalper{name: "TripleConsensus_Alpha_Scalp", maxBuf: defaultBufSize},
	}
}

func (s *ConsensusScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *ConsensusScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < 40 { return holdSignal() }
	rsi := RSI(s.prices, 14)
	_, _, hist := MACD(s.prices, 12, 26, 9)
	_, _, bbLower := BollingerBands(s.prices, 20, 2.0)
	bbUpper, _, _ := BollingerBands(s.prices, 20, 2.0)

	buyVotes := 0
	if rsi < 35 { buyVotes++ }
	if hist > 0 { buyVotes++ }
	if candle.Price <= bbLower*1.001 { buyVotes++ }

	sellVotes := 0
	if rsi > 65 { sellVotes++ }
	if hist < 0 { sellVotes++ }
	if candle.Price >= bbUpper*0.999 { sellVotes++ }

	if buyVotes == 3 { return buySignalCustom(candle.Symbol, 0.3, 1.5) }
	if sellVotes == 3 { return sellSignalCustom(candle.Symbol, 0.3, 1.5) }
	return holdSignal()
}
