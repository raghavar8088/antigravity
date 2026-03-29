package strategy

import (
	"antigravity-engine/internal/marketdata"
	"math"
)

// =============================================================================
// ELITE STRATEGIES 66-90
// =============================================================================

// 66. Intraday Momentum Index
type IMIScalper struct {
	baseScalper
	period int
}
func NewIMIScalper(period int) *IMIScalper {
	return &IMIScalper{baseScalper: baseScalper{name: "IMI_Intraday_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *IMIScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *IMIScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+1 { return holdSignal() }
	gains, losses := 0.0, 0.0
	for i := len(s.prices) - s.period; i < len(s.prices); i++ {
		if s.prices[i] > s.prices[i-1] { gains += s.prices[i] - s.prices[i-1] } else { losses += s.prices[i-1] - s.prices[i] }
	}
	if gains+losses == 0 { return holdSignal() }
	imi := (gains / (gains + losses)) * 100
	if imi < 25 { return buySignalCustom(c.Symbol, 0.3, 0.9) }
	if imi > 75 { return sellSignalCustom(c.Symbol, 0.3, 0.9) }
	return holdSignal()
}

// 67. Adaptive RSI — RSI with dynamic overbought/oversold levels
type AdaptiveRSIScalper struct {
	baseScalper
	period int
}
func NewAdaptiveRSIScalper(period int) *AdaptiveRSIScalper {
	return &AdaptiveRSIScalper{baseScalper: baseScalper{name: "AdaptiveRSI_Dynamic_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *AdaptiveRSIScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *AdaptiveRSIScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period*2 { return holdSignal() }
	rsi := RSI(s.prices, s.period)
	atr := ATR(s.prices, s.period)
	sma := SMA(s.prices[len(s.prices)-s.period:])
	if sma == 0 { return holdSignal() }
	volNorm := (atr / sma) * 100
	ob := 70.0 + volNorm*2; os := 30.0 - volNorm*2
	if ob > 90 { ob = 90 }; if os < 10 { os = 10 }
	if rsi < os { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if rsi > ob { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 68. Fractal Breakout — Williams fractal highs/lows
type FractalScalper struct {
	baseScalper
	lastFractalHigh, lastFractalLow float64
}
func NewFractalScalper() *FractalScalper {
	return &FractalScalper{baseScalper: baseScalper{name: "Fractal_Breakout_Scalp", maxBuf: defaultBufSize}}
}
func (s *FractalScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *FractalScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 7 { return holdSignal() }
	n := len(s.prices)
	mid := s.prices[n-3]
	if mid > s.prices[n-5] && mid > s.prices[n-4] && mid > s.prices[n-2] && mid > s.prices[n-1] {
		s.lastFractalHigh = mid
	}
	if mid < s.prices[n-5] && mid < s.prices[n-4] && mid < s.prices[n-2] && mid < s.prices[n-1] {
		s.lastFractalLow = mid
	}
	if s.lastFractalHigh > 0 && c.Price > s.lastFractalHigh { return buySignalCustom(c.Symbol, 0.4, 1.2) }
	if s.lastFractalLow > 0 && c.Price < s.lastFractalLow { return sellSignalCustom(c.Symbol, 0.4, 1.2) }
	return holdSignal()
}

// 69. Mean Reversion Bands — Z-score with adaptive bands
type ZScoreBandScalper struct {
	baseScalper
	period int
	zThresh float64
}
func NewZScoreBandScalper(period int, z float64) *ZScoreBandScalper {
	return &ZScoreBandScalper{baseScalper: baseScalper{name: "ZScoreBand_MeanRev_Scalp", maxBuf: defaultBufSize}, period: period, zThresh: z}
}
func (s *ZScoreBandScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *ZScoreBandScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period { return holdSignal() }
	slice := s.prices[len(s.prices)-s.period:]
	mean := SMA(slice)
	variance := 0.0
	for _, p := range slice { variance += (p - mean) * (p - mean) }
	sd := math.Sqrt(variance / float64(s.period))
	if sd == 0 { return holdSignal() }
	z := (c.Price - mean) / sd
	if z < -s.zThresh { return buySignalCustom(c.Symbol, 0.3, 0.8) }
	if z > s.zThresh { return sellSignalCustom(c.Symbol, 0.3, 0.8) }
	return holdSignal()
}

// 70. Trend Intensity Index
type TIIScalper struct {
	baseScalper
	period int
}
func NewTIIScalper(period int) *TIIScalper {
	return &TIIScalper{baseScalper: baseScalper{name: "TII_TrendIntensity_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *TIIScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *TIIScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period { return holdSignal() }
	sma := SMA(s.prices[len(s.prices)-s.period:])
	up, dn := 0.0, 0.0
	for _, p := range s.prices[len(s.prices)-s.period:] {
		if p > sma { up++ } else { dn++ }
	}
	if up+dn == 0 { return holdSignal() }
	tii := (up / (up + dn)) * 100
	if tii > 80 { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if tii < 20 { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 71. Acceleration Bands — Wider during volatility
type AccelBandScalper struct {
	baseScalper
	period int
}
func NewAccelBandScalper(period int) *AccelBandScalper {
	return &AccelBandScalper{baseScalper: baseScalper{name: "AccelBand_Breakout_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *AccelBandScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *AccelBandScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+2 { return holdSignal() }
	slice := s.prices[len(s.prices)-s.period:]
	high, low := DonchianChannel(slice, s.period)
	mid := SMA(slice)
	if low == 0 { return holdSignal() }
	factor := (high - low) / low
	upper := mid * (1 + factor); lower := mid * (1 - factor)
	if c.Price > upper { return buySignalCustom(c.Symbol, 0.4, 1.2) }
	if c.Price < lower { return sellSignalCustom(c.Symbol, 0.4, 1.2) }
	return holdSignal()
}

// 72. RSI + BB Confluence
type RSIBBScalper struct{ baseScalper }
func NewRSIBBScalper() *RSIBBScalper {
	return &RSIBBScalper{baseScalper: baseScalper{name: "RSI_BB_Confluence_Scalp", maxBuf: defaultBufSize}}
}
func (s *RSIBBScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *RSIBBScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 22 { return holdSignal() }
	rsi := RSI(s.prices, 14)
	upper, _, lower := BollingerBands(s.prices, 20, 2.0)
	if rsi < 30 && c.Price <= lower { return buySignalCustom(c.Symbol, 0.3, 1.0) }
	if rsi > 70 && c.Price >= upper { return sellSignalCustom(c.Symbol, 0.3, 1.0) }
	return holdSignal()
}

// 73. EMA + MACD + ADX Triple Filter
type TripleFilterScalper struct{ baseScalper }
func NewTripleFilterScalper() *TripleFilterScalper {
	return &TripleFilterScalper{baseScalper: baseScalper{name: "TripleFilter_Alpha_Scalp", maxBuf: defaultBufSize}}
}
func (s *TripleFilterScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *TripleFilterScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 40 { return holdSignal() }
	ema20 := EMA(s.prices, 20)
	_, _, hist := MACD(s.prices, 12, 26, 9)
	adx := ADX(s.prices, 14)
	if adx < 25 { return holdSignal() }
	if c.Price > ema20 && hist > 0 { return buySignalCustom(c.Symbol, 0.4, 1.2) }
	if c.Price < ema20 && hist < 0 { return sellSignalCustom(c.Symbol, 0.4, 1.2) }
	return holdSignal()
}

// 74. Exhaustion Move Detector
type ExhaustionScalper struct {
	baseScalper
	period int
}
func NewExhaustionScalper(period int) *ExhaustionScalper {
	return &ExhaustionScalper{baseScalper: baseScalper{name: "Exhaustion_Reversal_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *ExhaustionScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *ExhaustionScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+2 { return holdSignal() }
	roc := ROC(s.prices, s.period)
	rsi := RSI(s.prices, 14)
	if roc > 1.5 && rsi > 75 { return sellSignalCustom(c.Symbol, 0.3, 1.0) }
	if roc < -1.5 && rsi < 25 { return buySignalCustom(c.Symbol, 0.3, 1.0) }
	return holdSignal()
}

// 75. Pivot Fibonacci — S/R from pivot + fib levels
type PivotFibScalper struct {
	baseScalper
	period int
}
func NewPivotFibScalper(period int) *PivotFibScalper {
	return &PivotFibScalper{baseScalper: baseScalper{name: "PivotFib_SR_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *PivotFibScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *PivotFibScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period { return holdSignal() }
	pivot, r1, s1 := PivotPoints(s.prices, s.period)
	rng := r1 - s1
	fib382 := pivot - rng*0.382; fib618 := pivot - rng*0.618
	tol := rng * 0.02
	if math.Abs(c.Price-fib618) < tol { return buySignalCustom(c.Symbol, 0.3, 1.0) }
	if math.Abs(c.Price-fib382) < tol && c.Price > pivot { return sellSignalCustom(c.Symbol, 0.3, 1.0) }
	return holdSignal()
}

// 76. Chandelier Exit — Trailing stop-based entries
type ChandelierScalper struct {
	baseScalper
	period int
	mult   float64
	prevCE float64
}
func NewChandelierScalper(period int, mult float64) *ChandelierScalper {
	return &ChandelierScalper{baseScalper: baseScalper{name: "Chandelier_Exit_Scalp", maxBuf: defaultBufSize}, period: period, mult: mult}
}
func (s *ChandelierScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *ChandelierScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+2 { return holdSignal() }
	high, _ := DonchianChannel(s.prices[len(s.prices)-s.period:], s.period)
	atr := ATR(s.prices, s.period)
	ce := high - s.mult*atr
	defer func() { s.prevCE = ce }()
	if s.prevCE != 0 && c.Price > ce && s.prices[len(s.prices)-2] <= s.prevCE {
		return buySignalCustom(c.Symbol, 0.4, 1.2)
	}
	if s.prevCE != 0 && c.Price < ce && s.prices[len(s.prices)-2] >= s.prevCE {
		return sellSignalCustom(c.Symbol, 0.4, 1.2)
	}
	return holdSignal()
}

// 77. Relative Vigor Index
type RVIScalper struct {
	baseScalper
	period  int
	prevRVI float64
}
func NewRVIScalper(period int) *RVIScalper {
	return &RVIScalper{baseScalper: baseScalper{name: "RVI_Vigor_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *RVIScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *RVIScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+4 { return holdSignal() }
	num, den := 0.0, 0.0
	for i := len(s.prices) - s.period; i < len(s.prices); i++ {
		num += s.prices[i] - s.prices[i-1]
		den += math.Abs(s.prices[i] - s.prices[i-1])
	}
	if den == 0 { return holdSignal() }
	rvi := num / den
	defer func() { s.prevRVI = rvi }()
	if s.prevRVI < 0 && rvi > 0 { return buySignalCustom(c.Symbol, 0.3, 0.9) }
	if s.prevRVI > 0 && rvi < 0 { return sellSignalCustom(c.Symbol, 0.3, 0.9) }
	return holdSignal()
}

// 78. Weighted Close Oscillator
type WCOScalper struct {
	baseScalper
	fast, slow int
	prevWCO    float64
}
func NewWCOScalper(fast, slow int) *WCOScalper {
	return &WCOScalper{baseScalper: baseScalper{name: "WCO_Weighted_Scalp", maxBuf: defaultBufSize}, fast: fast, slow: slow}
}
func (s *WCOScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *WCOScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.slow+2 { return holdSignal() }
	wco := EMA(s.prices, s.fast) - SMA(s.prices[len(s.prices)-s.slow:])
	defer func() { s.prevWCO = wco }()
	if s.prevWCO < 0 && wco > 0 { return buySignalCustom(c.Symbol, 0.3, 0.9) }
	if s.prevWCO > 0 && wco < 0 { return sellSignalCustom(c.Symbol, 0.3, 0.9) }
	return holdSignal()
}

// 79. Schaff Trend Cycle
type SchaffScalper struct {
	baseScalper
	prevSTC float64
}
func NewSchaffScalper() *SchaffScalper {
	return &SchaffScalper{baseScalper: baseScalper{name: "Schaff_TrendCycle_Scalp", maxBuf: defaultBufSize}}
}
func (s *SchaffScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *SchaffScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 40 { return holdSignal() }
	macdLine, _, _ := MACD(s.prices, 23, 50, 9)
	rsi := RSI(s.prices, 14)
	stc := (macdLine + rsi) / 2
	defer func() { s.prevSTC = stc }()
	if s.prevSTC < 25 && stc > 25 { return buySignalCustom(c.Symbol, 0.4, 1.2) }
	if s.prevSTC > 75 && stc < 75 { return sellSignalCustom(c.Symbol, 0.4, 1.2) }
	return holdSignal()
}

// 80. Anchored VWAP Mean Reversion
type AnchoredVWAPScalper struct {
	baseScalper
	anchorPeriod int
}
func NewAnchoredVWAPScalper(period int) *AnchoredVWAPScalper {
	return &AnchoredVWAPScalper{baseScalper: baseScalper{name: "AnchoredVWAP_MeanRev_Scalp", maxBuf: defaultBufSize}, anchorPeriod: period}
}
func (s *AnchoredVWAPScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *AnchoredVWAPScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.anchorPeriod { return holdSignal() }
	vwap := SMA(s.prices[len(s.prices)-s.anchorPeriod:])
	dev := math.Abs(c.Price-vwap) / vwap * 100
	if dev > 0.3 && c.Price < vwap { return buySignalCustom(c.Symbol, 0.3, 0.8) }
	if dev > 0.3 && c.Price > vwap { return sellSignalCustom(c.Symbol, 0.3, 0.8) }
	return holdSignal()
}

// 81. Directional Movement Scalper — +DI/-DI cross
type DMIScalper struct {
	baseScalper
	period               int
	prevPlusDI, prevMinusDI float64
}
func NewDMIScalper(period int) *DMIScalper {
	return &DMIScalper{baseScalper: baseScalper{name: "DMI_Directional_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *DMIScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *DMIScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+2 { return holdSignal() }
	plusDM, minusDM, tr := 0.0, 0.0, 0.0
	for i := len(s.prices) - s.period; i < len(s.prices); i++ {
		up := s.prices[i] - s.prices[i-1]; dn := s.prices[i-1] - s.prices[i]
		if up > dn && up > 0 { plusDM += up }
		if dn > up && dn > 0 { minusDM += dn }
		tr += math.Abs(s.prices[i] - s.prices[i-1])
	}
	if tr == 0 { return holdSignal() }
	pdi := plusDM / tr * 100; mdi := minusDM / tr * 100
	defer func() { s.prevPlusDI = pdi; s.prevMinusDI = mdi }()
	if s.prevPlusDI != 0 && s.prevPlusDI <= s.prevMinusDI && pdi > mdi { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if s.prevPlusDI != 0 && s.prevPlusDI >= s.prevMinusDI && pdi < mdi { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 82. Normalized ATR Volatility Filter
type NATRScalper struct {
	baseScalper
	period    int
	threshold float64
}
func NewNATRScalper(period int, thresh float64) *NATRScalper {
	return &NATRScalper{baseScalper: baseScalper{name: "NATR_VolFilter_Scalp", maxBuf: defaultBufSize}, period: period, threshold: thresh}
}
func (s *NATRScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *NATRScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+14 { return holdSignal() }
	atr := ATR(s.prices, s.period)
	natr := (atr / c.Price) * 100
	if natr < s.threshold { return holdSignal() }
	rsi := RSI(s.prices, 14)
	if rsi < 35 { return buySignalCustom(c.Symbol, 0.5, 1.5) }
	if rsi > 65 { return sellSignalCustom(c.Symbol, 0.5, 1.5) }
	return holdSignal()
}

// 83. Price Action Pinbar Detection
type PinbarScalper struct{ baseScalper }
func NewPinbarScalper() *PinbarScalper {
	return &PinbarScalper{baseScalper: baseScalper{name: "Pinbar_Reversal_Scalp", maxBuf: defaultBufSize}}
}
func (s *PinbarScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *PinbarScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 5 { return holdSignal() }
	n := len(s.prices)
	body := math.Abs(s.prices[n-1] - s.prices[n-2])
	high, low := DonchianChannel(s.prices[n-4:], 4)
	upperWick := high - math.Max(s.prices[n-1], s.prices[n-2])
	lowerWick := math.Min(s.prices[n-1], s.prices[n-2]) - low
	if body == 0 { return holdSignal() }
	if lowerWick > body*2.5 && upperWick < body { return buySignalCustom(c.Symbol, 0.3, 1.0) }
	if upperWick > body*2.5 && lowerWick < body { return sellSignalCustom(c.Symbol, 0.3, 1.0) }
	return holdSignal()
}

// 84. Smoothed RSI Cross
type SmoothedRSIScalper struct {
	baseScalper
	period  int
	prevSRSI float64
}
func NewSmoothedRSIScalper(period int) *SmoothedRSIScalper {
	return &SmoothedRSIScalper{baseScalper: baseScalper{name: "SmoothedRSI_Cross_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *SmoothedRSIScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *SmoothedRSIScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+10 { return holdSignal() }
	rsi := RSI(s.prices, s.period)
	srsi := rsi*0.7 + s.prevSRSI*0.3
	defer func() { s.prevSRSI = srsi }()
	if s.prevSRSI < 30 && srsi > 30 { return buySignalCustom(c.Symbol, 0.3, 0.9) }
	if s.prevSRSI > 70 && srsi < 70 { return sellSignalCustom(c.Symbol, 0.3, 0.9) }
	return holdSignal()
}

// 85. Momentum Squeeze — BB inside KC with momentum direction
type MomSqueezeScalper struct {
	baseScalper
	wasSqueezing bool
}
func NewMomSqueezeScalper() *MomSqueezeScalper {
	return &MomSqueezeScalper{baseScalper: baseScalper{name: "MomSqueeze_Breakout_Scalp", maxBuf: defaultBufSize}}
}
func (s *MomSqueezeScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *MomSqueezeScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 30 { return holdSignal() }
	bbU, _, bbL := BollingerBands(s.prices, 20, 2.0)
	kcU, _, kcL := KeltnerChannels(s.prices, 20, 10, 1.5)
	squeezing := bbL > kcL && bbU < kcU
	mom := c.Price - SMA(s.prices[len(s.prices)-20:])
	defer func() { s.wasSqueezing = squeezing }()
	if s.wasSqueezing && !squeezing {
		if mom > 0 { return buySignalCustom(c.Symbol, 0.5, 1.5) }
		return sellSignalCustom(c.Symbol, 0.5, 1.5)
	}
	return holdSignal()
}

// 86. Triple RSI Period Confluence
type TripleRSIScalper struct{ baseScalper }
func NewTripleRSIScalper() *TripleRSIScalper {
	return &TripleRSIScalper{baseScalper: baseScalper{name: "TripleRSI_Confluence_Scalp", maxBuf: defaultBufSize}}
}
func (s *TripleRSIScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *TripleRSIScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 30 { return holdSignal() }
	r7 := RSI(s.prices, 7); r14 := RSI(s.prices, 14); r21 := RSI(s.prices, 21)
	if r7 < 30 && r14 < 40 && r21 < 45 { return buySignalCustom(c.Symbol, 0.3, 1.2) }
	if r7 > 70 && r14 > 60 && r21 > 55 { return sellSignalCustom(c.Symbol, 0.3, 1.2) }
	return holdSignal()
}

// 87. Rate of Change Acceleration
type ROCAccelScalper struct {
	baseScalper
	period  int
	prevROC float64
}
func NewROCAccelScalper(period int) *ROCAccelScalper {
	return &ROCAccelScalper{baseScalper: baseScalper{name: "ROCAccel_Momentum_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *ROCAccelScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *ROCAccelScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) <= s.period+1 { return holdSignal() }
	roc := ROC(s.prices, s.period)
	accel := roc - s.prevROC
	defer func() { s.prevROC = roc }()
	if s.prevROC != 0 && accel > 0.5 && roc > 0 { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if s.prevROC != 0 && accel < -0.5 && roc < 0 { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 88. Multi-MA Consensus — 4 different MA types agree
type MultiMAScalper struct{ baseScalper }
func NewMultiMAScalper() *MultiMAScalper {
	return &MultiMAScalper{baseScalper: baseScalper{name: "MultiMA_Consensus_Scalp", maxBuf: defaultBufSize}}
}
func (s *MultiMAScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *MultiMAScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 55 { return holdSignal() }
	sma20 := SMA(s.prices[len(s.prices)-20:])
	ema20 := EMA(s.prices, 20)
	hma16 := HullMA(s.prices, 16)
	sma50 := SMA(s.prices[len(s.prices)-50:])
	bullish := 0; if c.Price > sma20 { bullish++ }; if c.Price > ema20 { bullish++ }
	if c.Price > hma16 { bullish++ }; if c.Price > sma50 { bullish++ }
	if bullish == 4 { return buySignalCustom(c.Symbol, 0.4, 1.2) }
	if bullish == 0 { return sellSignalCustom(c.Symbol, 0.4, 1.2) }
	return holdSignal()
}

// 89. Volatility Contraction Pattern
type VCPScalper struct {
	baseScalper
	windows []int
}
func NewVCPScalper() *VCPScalper {
	return &VCPScalper{baseScalper: baseScalper{name: "VCP_Contraction_Scalp", maxBuf: defaultBufSize}, windows: []int{30, 20, 10}}
}
func (s *VCPScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *VCPScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 35 { return holdSignal() }
	ranges := make([]float64, len(s.windows))
	for i, w := range s.windows {
		h, l := DonchianChannel(s.prices[len(s.prices)-w:], w)
		if l > 0 { ranges[i] = (h - l) / l * 100 }
	}
	if ranges[0] > ranges[1] && ranges[1] > ranges[2] && ranges[2] < 0.5 {
		ema := EMA(s.prices, 9)
		if c.Price > ema { return buySignalCustom(c.Symbol, 0.5, 1.5) }
	}
	return holdSignal()
}

// 90. Quad Indicator Sniper — Only fires with 4/4 agreement
type QuadSniperScalper struct{ baseScalper }
func NewQuadSniperScalper() *QuadSniperScalper {
	return &QuadSniperScalper{baseScalper: baseScalper{name: "QuadSniper_UltraAlpha_Scalp", maxBuf: defaultBufSize}}
}
func (s *QuadSniperScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *QuadSniperScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 50 { return holdSignal() }
	rsi := RSI(s.prices, 14)
	_, _, hist := MACD(s.prices, 12, 26, 9)
	adx := ADX(s.prices, 14)
	ema20 := EMA(s.prices, 20)
	if adx < 25 { return holdSignal() }
	buyVotes, sellVotes := 0, 0
	if rsi < 40 { buyVotes++ }; if rsi > 60 { sellVotes++ }
	if hist > 0 { buyVotes++ }; if hist < 0 { sellVotes++ }
	if c.Price > ema20 { buyVotes++ }; if c.Price < ema20 { sellVotes++ }
	cci := CCI(s.prices, 20)
	if cci < -50 { buyVotes++ }; if cci > 50 { sellVotes++ }
	if buyVotes >= 4 { return buySignalCustom(c.Symbol, 0.3, 1.5) }
	if sellVotes >= 4 { return sellSignalCustom(c.Symbol, 0.3, 1.5) }
	return holdSignal()
}
