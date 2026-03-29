package strategy

import (
	"antigravity-engine/internal/marketdata"
	"math"
)

// =============================================================================
// 50 ELITE HIGH-PROFIT BTC SCALPING STRATEGIES (41-90)
// =============================================================================

// 41. DEMA Crossover — Double EMA reduces lag vs standard EMA cross
type DEMACrossScalper struct {
	baseScalper
	fast, slow int
	prevFast, prevSlow float64
}
func NewDEMACrossScalper(fast, slow int) *DEMACrossScalper {
	return &DEMACrossScalper{baseScalper: baseScalper{name: "DEMA_Cross_Scalp", maxBuf: defaultBufSize}, fast: fast, slow: slow}
}
func (s *DEMACrossScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *DEMACrossScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.slow+2 { return holdSignal() }
	e1f := EMA(s.prices, s.fast); e2f := EMA(s.prices, s.fast) // simplified DEMA
	demaF := 2*e1f - e2f
	e1s := EMA(s.prices, s.slow); e2s := EMA(s.prices, s.slow)
	demaS := 2*e1s - e2s
	defer func() { s.prevFast = demaF; s.prevSlow = demaS }()
	if s.prevFast != 0 && s.prevFast <= s.prevSlow && demaF > demaS { return buySignal(c.Symbol) }
	if s.prevFast != 0 && s.prevFast >= s.prevSlow && demaF < demaS { return sellSignal(c.Symbol) }
	return holdSignal()
}

// 42. RSI Divergence — Price makes new low but RSI makes higher low
type RSIDivergenceScalper struct {
	baseScalper
	period int
	prevPrice, prevRSI float64
}
func NewRSIDivergenceScalper(period int) *RSIDivergenceScalper {
	return &RSIDivergenceScalper{baseScalper: baseScalper{name: "RSI_Divergence_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *RSIDivergenceScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *RSIDivergenceScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+2 { return holdSignal() }
	rsi := RSI(s.prices, s.period)
	defer func() { s.prevPrice = c.Price; s.prevRSI = rsi }()
	if s.prevPrice != 0 && c.Price < s.prevPrice && rsi > s.prevRSI && rsi < 35 {
		return buySignalCustom(c.Symbol, 0.4, 1.2)
	}
	if s.prevPrice != 0 && c.Price > s.prevPrice && rsi < s.prevRSI && rsi > 65 {
		return sellSignalCustom(c.Symbol, 0.4, 1.2)
	}
	return holdSignal()
}

// 43. ATR Breakout — Enters when move exceeds 1.5x ATR
type ATRBreakoutScalper struct {
	baseScalper
	atrPeriod int
	mult      float64
}
func NewATRBreakoutScalper(period int, mult float64) *ATRBreakoutScalper {
	return &ATRBreakoutScalper{baseScalper: baseScalper{name: "ATR_Breakout_Scalp", maxBuf: defaultBufSize}, atrPeriod: period, mult: mult}
}
func (s *ATRBreakoutScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *ATRBreakoutScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.atrPeriod+2 { return holdSignal() }
	atr := ATR(s.prices, s.atrPeriod)
	move := c.Price - s.prices[len(s.prices)-2]
	if move > atr*s.mult { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if move < -atr*s.mult { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 44. Squeeze Momentum — BB width contraction then expansion
type SqueezeMomentumScalper struct {
	baseScalper
	period int
	prevWidth float64
}
func NewSqueezeMomentumScalper(period int) *SqueezeMomentumScalper {
	return &SqueezeMomentumScalper{baseScalper: baseScalper{name: "Squeeze_Momentum_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *SqueezeMomentumScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *SqueezeMomentumScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+2 { return holdSignal() }
	upper, mid, lower := BollingerBands(s.prices, s.period, 2.0)
	width := (upper - lower) / mid * 100
	defer func() { s.prevWidth = width }()
	if s.prevWidth != 0 && s.prevWidth < 0.5 && width > 0.5 {
		if c.Price > mid { return buySignalCustom(c.Symbol, 0.4, 1.2) }
		return sellSignalCustom(c.Symbol, 0.4, 1.2)
	}
	return holdSignal()
}

// 45. TRIX Momentum — Triple-smoothed EMA rate of change
type TRIXScalper struct {
	baseScalper
	period int
	prevTrix float64
}
func NewTRIXScalper(period int) *TRIXScalper {
	return &TRIXScalper{baseScalper: baseScalper{name: "TRIX_Momentum_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *TRIXScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *TRIXScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period*3+2 { return holdSignal() }
	e1 := EMA(s.prices, s.period)
	ema1s := make([]float64, 0, len(s.prices)-s.period)
	for i := s.period; i <= len(s.prices); i++ { ema1s = append(ema1s, EMA(s.prices[:i], s.period)) }
	if len(ema1s) < s.period { return holdSignal() }
	e2 := EMA(ema1s, s.period)
	_ = e1
	trix := e2
	defer func() { s.prevTrix = trix }()
	if s.prevTrix != 0 && trix > s.prevTrix { return buySignalCustom(c.Symbol, 0.3, 0.9) }
	if s.prevTrix != 0 && trix < s.prevTrix { return sellSignalCustom(c.Symbol, 0.3, 0.9) }
	return holdSignal()
}

// 46. Elder Ray — Bull/Bear power based on EMA
type ElderRayScalper struct {
	baseScalper
	period int
}
func NewElderRayScalper(period int) *ElderRayScalper {
	return &ElderRayScalper{baseScalper: baseScalper{name: "ElderRay_Power_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *ElderRayScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *ElderRayScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+2 { return holdSignal() }
	ema := EMA(s.prices, s.period)
	high, low := DonchianChannel(s.prices[len(s.prices)-s.period:], s.period)
	bullPower := high - ema
	bearPower := low - ema
	if bearPower < 0 && bearPower > s.prices[len(s.prices)-2]-EMA(s.prices[:len(s.prices)-1], s.period)-ema {
		return buySignalCustom(c.Symbol, 0.4, 1.0)
	}
	if bullPower > 0 && c.Price < ema { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 47. Vortex Indicator — Trend direction via positive/negative vortex
type VortexScalper struct {
	baseScalper
	period int
	prevVIPlus, prevVIMinus float64
}
func NewVortexScalper(period int) *VortexScalper {
	return &VortexScalper{baseScalper: baseScalper{name: "Vortex_Trend_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *VortexScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *VortexScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+2 { return holdSignal() }
	vmPlus, vmMinus, tr := 0.0, 0.0, 0.0
	for i := len(s.prices) - s.period; i < len(s.prices); i++ {
		vmPlus += math.Abs(s.prices[i] - s.prices[i-1])
		vmMinus += math.Abs(s.prices[i-1] - s.prices[i])
		tr += math.Abs(s.prices[i] - s.prices[i-1])
	}
	if tr == 0 { return holdSignal() }
	viPlus := vmPlus / tr; viMinus := vmMinus / tr
	defer func() { s.prevVIPlus = viPlus; s.prevVIMinus = viMinus }()
	if s.prevVIPlus != 0 && s.prevVIPlus <= s.prevVIMinus && viPlus > viMinus {
		return buySignalCustom(c.Symbol, 0.4, 1.0)
	}
	if s.prevVIPlus != 0 && s.prevVIPlus >= s.prevVIMinus && viPlus < viMinus {
		return sellSignalCustom(c.Symbol, 0.4, 1.0)
	}
	return holdSignal()
}

// 48. Price Channel Breakout — New N-period high/low
type PriceChannelScalper struct {
	baseScalper
	period int
}
func NewPriceChannelScalper(period int) *PriceChannelScalper {
	return &PriceChannelScalper{baseScalper: baseScalper{name: "PriceChannel_Breakout_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *PriceChannelScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *PriceChannelScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+2 { return holdSignal() }
	high, low := DonchianChannel(s.prices[len(s.prices)-s.period-1:len(s.prices)-1], s.period)
	if c.Price > high { return buySignalCustom(c.Symbol, 0.5, 1.5) }
	if c.Price < low { return sellSignalCustom(c.Symbol, 0.5, 1.5) }
	return holdSignal()
}

// 49. Momentum RSI — RSI of momentum values
type MomentumRSIScalper struct {
	baseScalper
	momPeriod, rsiPeriod int
}
func NewMomentumRSIScalper(mom, rsi int) *MomentumRSIScalper {
	return &MomentumRSIScalper{baseScalper: baseScalper{name: "MomentumRSI_Scalp", maxBuf: defaultBufSize}, momPeriod: mom, rsiPeriod: rsi}
}
func (s *MomentumRSIScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *MomentumRSIScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.momPeriod+s.rsiPeriod+2 { return holdSignal() }
	moms := make([]float64, 0)
	for i := s.momPeriod; i < len(s.prices); i++ {
		moms = append(moms, s.prices[i]-s.prices[i-s.momPeriod])
	}
	if len(moms) < s.rsiPeriod+1 { return holdSignal() }
	rsi := RSI(moms, s.rsiPeriod)
	if rsi < 25 { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if rsi > 75 { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 50. Double Bottom/Top Pattern
type DoublePatternScalper struct {
	baseScalper
	window    int
	tolerance float64
}
func NewDoublePatternScalper(window int, tol float64) *DoublePatternScalper {
	return &DoublePatternScalper{baseScalper: baseScalper{name: "DoublePattern_Scalp", maxBuf: defaultBufSize}, window: window, tolerance: tol}
}
func (s *DoublePatternScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *DoublePatternScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.window { return holdSignal() }
	slice := s.prices[len(s.prices)-s.window:]
	_, low := DonchianChannel(slice[:len(slice)/2], len(slice)/2)
	_, low2 := DonchianChannel(slice[len(slice)/2:], len(slice)/2)
	high, _ := DonchianChannel(slice[:len(slice)/2], len(slice)/2)
	high2, _ := DonchianChannel(slice[len(slice)/2:], len(slice)/2)
	if math.Abs(low-low2)/low < s.tolerance && c.Price > SMA(slice) {
		return buySignalCustom(c.Symbol, 0.4, 1.2)
	}
	if math.Abs(high-high2)/high < s.tolerance && c.Price < SMA(slice) {
		return sellSignalCustom(c.Symbol, 0.4, 1.2)
	}
	return holdSignal()
}

// 51. Stochastic Oscillator Cross
type StochasticCrossScalper struct {
	baseScalper
	period int
	prevK, prevD float64
}
func NewStochasticCrossScalper(period int) *StochasticCrossScalper {
	return &StochasticCrossScalper{baseScalper: baseScalper{name: "Stochastic_Cross_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *StochasticCrossScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *StochasticCrossScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+3 { return holdSignal() }
	slice := s.prices[len(s.prices)-s.period:]
	high, low := DonchianChannel(slice, s.period)
	if high == low { return holdSignal() }
	k := ((c.Price - low) / (high - low)) * 100
	d := SMA([]float64{k, s.prevK, s.prevD})
	defer func() { s.prevK = k; s.prevD = d }()
	if s.prevK != 0 && s.prevK < s.prevD && k > d && k < 20 { return buySignalCustom(c.Symbol, 0.3, 0.9) }
	if s.prevK != 0 && s.prevK > s.prevD && k < d && k > 80 { return sellSignalCustom(c.Symbol, 0.3, 0.9) }
	return holdSignal()
}

// 52. Coppock Curve — Long-term momentum oscillator
type CoppockScalper struct {
	baseScalper
	long, short, signal int
	prevCoppock float64
}
func NewCoppockScalper() *CoppockScalper {
	return &CoppockScalper{baseScalper: baseScalper{name: "Coppock_Curve_Scalp", maxBuf: defaultBufSize}, long: 14, short: 11, signal: 10}
}
func (s *CoppockScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *CoppockScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.long+s.signal+2 { return holdSignal() }
	roc1 := ROC(s.prices, s.long)
	roc2 := ROC(s.prices, s.short)
	coppock := roc1 + roc2
	defer func() { s.prevCoppock = coppock }()
	if s.prevCoppock < 0 && coppock > 0 { return buySignalCustom(c.Symbol, 0.5, 1.5) }
	if s.prevCoppock > 0 && coppock < 0 { return sellSignalCustom(c.Symbol, 0.5, 1.5) }
	return holdSignal()
}

// 53. Mass Index — Detects trend reversals via range expansion
type MassIndexScalper struct {
	baseScalper
	period int
	prevMI float64
}
func NewMassIndexScalper(period int) *MassIndexScalper {
	return &MassIndexScalper{baseScalper: baseScalper{name: "MassIndex_Reversal_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *MassIndexScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *MassIndexScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+10 { return holdSignal() }
	mi := 0.0
	for i := len(s.prices) - s.period; i < len(s.prices); i++ {
		r := math.Abs(s.prices[i] - s.prices[i-1])
		ema9 := ATR(s.prices[:i+1], 9)
		if ema9 > 0 { mi += r / ema9 }
	}
	defer func() { s.prevMI = mi }()
	if s.prevMI > 27 && mi < 26.5 {
		ema := EMA(s.prices, 9)
		if c.Price > ema { return buySignalCustom(c.Symbol, 0.4, 1.2) }
		return sellSignalCustom(c.Symbol, 0.4, 1.2)
	}
	return holdSignal()
}

// 54. Percentage Price Oscillator
type PPOScalper struct {
	baseScalper
	fast, slow int
	prevPPO    float64
}
func NewPPOScalper(fast, slow int) *PPOScalper {
	return &PPOScalper{baseScalper: baseScalper{name: "PPO_Oscillator_Scalp", maxBuf: defaultBufSize}, fast: fast, slow: slow}
}
func (s *PPOScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *PPOScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.slow+2 { return holdSignal() }
	fastEMA := EMA(s.prices, s.fast)
	slowEMA := EMA(s.prices, s.slow)
	if slowEMA == 0 { return holdSignal() }
	ppo := ((fastEMA - slowEMA) / slowEMA) * 100
	defer func() { s.prevPPO = ppo }()
	if s.prevPPO < 0 && ppo > 0 { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if s.prevPPO > 0 && ppo < 0 { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 55. Klinger Volume Oscillator (simplified)
type KlingerScalper struct {
	baseScalper
	fast, slow int
	kvo        []float64
}
func NewKlingerScalper() *KlingerScalper {
	return &KlingerScalper{baseScalper: baseScalper{name: "Klinger_Volume_Scalp", maxBuf: defaultBufSize}, fast: 34, slow: 55}
}
func (s *KlingerScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *KlingerScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 2 { return holdSignal() }
	vol := 1.0
	if c.Price < s.prices[len(s.prices)-2] { vol = -1.0 }
	s.kvo = append(s.kvo, vol)
	if len(s.kvo) > s.slow+2 { s.kvo = s.kvo[1:] }
	if len(s.kvo) < s.slow { return holdSignal() }
	fastE := EMA(s.kvo, s.fast)
	slowE := EMA(s.kvo, s.slow)
	if fastE > slowE && fastE > 0 { return buySignalCustom(c.Symbol, 0.3, 0.9) }
	if fastE < slowE && fastE < 0 { return sellSignalCustom(c.Symbol, 0.3, 0.9) }
	return holdSignal()
}

// 56. Ultimate Oscillator — Multi-timeframe momentum
type UltimateOscScalper struct {
	baseScalper
}
func NewUltimateOscScalper() *UltimateOscScalper {
	return &UltimateOscScalper{baseScalper: baseScalper{name: "UltimateOsc_Momentum_Scalp", maxBuf: defaultBufSize}}
}
func (s *UltimateOscScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *UltimateOscScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 30 { return holdSignal() }
	bp7, tr7, bp14, tr14, bp28, tr28 := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0
	for i := len(s.prices) - 7; i < len(s.prices); i++ {
		d := s.prices[i] - s.prices[i-1]
		tr7 += math.Abs(d)
		if d > 0 { bp7 += d }
	}
	for i := len(s.prices) - 14; i < len(s.prices); i++ {
		d := s.prices[i] - s.prices[i-1]
		tr14 += math.Abs(d)
		if d > 0 { bp14 += d }
	}
	for i := len(s.prices) - 28; i < len(s.prices); i++ {
		d := s.prices[i] - s.prices[i-1]
		tr28 += math.Abs(d)
		if d > 0 { bp28 += d }
	}
	if tr7 == 0 || tr14 == 0 || tr28 == 0 { return holdSignal() }
	uo := 100 * ((4*bp7/tr7 + 2*bp14/tr14 + bp28/tr28) / 7)
	if uo < 30 { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if uo > 70 { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 57. Ehlers Fisher Transform
type FisherTransformScalper struct {
	baseScalper
	period    int
	prevFish  float64
}
func NewFisherTransformScalper(period int) *FisherTransformScalper {
	return &FisherTransformScalper{baseScalper: baseScalper{name: "Fisher_Transform_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *FisherTransformScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *FisherTransformScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period { return holdSignal() }
	high, low := DonchianChannel(s.prices[len(s.prices)-s.period:], s.period)
	if high == low { return holdSignal() }
	val := 2*((c.Price-low)/(high-low)) - 1
	if val > 0.99 { val = 0.99 }
	if val < -0.99 { val = -0.99 }
	fish := 0.5 * math.Log((1+val)/(1-val))
	defer func() { s.prevFish = fish }()
	if s.prevFish < 0 && fish > 0 { return buySignalCustom(c.Symbol, 0.3, 1.0) }
	if s.prevFish > 0 && fish < 0 { return sellSignalCustom(c.Symbol, 0.3, 1.0) }
	return holdSignal()
}

// 58. Connors RSI — Composite RSI + streak + percentile
type ConnorsRSIScalper struct {
	baseScalper
	streak int
}
func NewConnorsRSIScalper() *ConnorsRSIScalper {
	return &ConnorsRSIScalper{baseScalper: baseScalper{name: "ConnorsRSI_Composite_Scalp", maxBuf: defaultBufSize}}
}
func (s *ConnorsRSIScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *ConnorsRSIScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 20 { return holdSignal() }
	n := len(s.prices)
	if s.prices[n-1] > s.prices[n-2] { if s.streak > 0 { s.streak++ } else { s.streak = 1 } } else if s.prices[n-1] < s.prices[n-2] { if s.streak < 0 { s.streak-- } else { s.streak = -1 } }
	rsi3 := RSI(s.prices, 3)
	rsi14 := RSI(s.prices, 14)
	crsi := (rsi3 + rsi14 + 50) / 3
	if crsi < 20 { return buySignalCustom(c.Symbol, 0.3, 0.8) }
	if crsi > 80 { return sellSignalCustom(c.Symbol, 0.3, 0.8) }
	return holdSignal()
}

// 59. Chande Momentum Oscillator
type ChandeMOScalper struct {
	baseScalper
	period int
}
func NewChandeMOScalper(period int) *ChandeMOScalper {
	return &ChandeMOScalper{baseScalper: baseScalper{name: "ChandeMO_Momentum_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *ChandeMOScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *ChandeMOScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+1 { return holdSignal() }
	up, dn := 0.0, 0.0
	for i := len(s.prices) - s.period; i < len(s.prices); i++ {
		d := s.prices[i] - s.prices[i-1]
		if d > 0 { up += d } else { dn += math.Abs(d) }
	}
	if up+dn == 0 { return holdSignal() }
	cmo := ((up - dn) / (up + dn)) * 100
	if cmo < -50 { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if cmo > 50 { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 60. Detrended Price Oscillator
type DPOScalper struct {
	baseScalper
	period int
}
func NewDPOScalper(period int) *DPOScalper {
	return &DPOScalper{baseScalper: baseScalper{name: "DPO_Cycle_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *DPOScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *DPOScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	offset := s.period/2 + 1
	if len(s.prices) < s.period+offset { return holdSignal() }
	sma := SMA(s.prices[len(s.prices)-s.period-offset : len(s.prices)-offset])
	dpo := c.Price - sma
	atr := ATR(s.prices, 14)
	if atr == 0 { return holdSignal() }
	if dpo < -atr*1.5 { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if dpo > atr*1.5 { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}

// 61. Know Sure Thing (KST) Oscillator
type KSTScalper struct {
	baseScalper
	prevKST float64
}
func NewKSTScalper() *KSTScalper {
	return &KSTScalper{baseScalper: baseScalper{name: "KST_Oscillator_Scalp", maxBuf: defaultBufSize}}
}
func (s *KSTScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *KSTScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 30 { return holdSignal() }
	roc10 := ROC(s.prices, 10)
	roc15 := ROC(s.prices, 15)
	roc20 := ROC(s.prices, 20)
	roc25 := ROC(s.prices, 25)
	kst := roc10*1 + roc15*2 + roc20*3 + roc25*4
	defer func() { s.prevKST = kst }()
	if s.prevKST < 0 && kst > 0 { return buySignalCustom(c.Symbol, 0.4, 1.2) }
	if s.prevKST > 0 && kst < 0 { return sellSignalCustom(c.Symbol, 0.4, 1.2) }
	return holdSignal()
}

// 62. Psychological Round Number Scalper
type RoundNumberScalper struct {
	baseScalper
	tolerance float64
}
func NewRoundNumberScalper(tol float64) *RoundNumberScalper {
	return &RoundNumberScalper{baseScalper: baseScalper{name: "RoundNumber_Support_Scalp", maxBuf: defaultBufSize}, tolerance: tol}
}
func (s *RoundNumberScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *RoundNumberScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 10 { return holdSignal() }
	round := math.Round(c.Price/1000) * 1000
	dist := math.Abs(c.Price - round)
	if dist < s.tolerance*c.Price/100 {
		rsi := RSI(s.prices, 14)
		if rsi < 40 && c.Price < round { return buySignalCustom(c.Symbol, 0.3, 0.8) }
		if rsi > 60 && c.Price > round { return sellSignalCustom(c.Symbol, 0.3, 0.8) }
	}
	return holdSignal()
}

// 63. Volatility Ratio Breakout
type VolRatioScalper struct {
	baseScalper
	shortP, longP int
}
func NewVolRatioScalper(short, long int) *VolRatioScalper {
	return &VolRatioScalper{baseScalper: baseScalper{name: "VolRatio_Breakout_Scalp", maxBuf: defaultBufSize}, shortP: short, longP: long}
}
func (s *VolRatioScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *VolRatioScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.longP+2 { return holdSignal() }
	shortATR := ATR(s.prices, s.shortP)
	longATR := ATR(s.prices, s.longP)
	if longATR == 0 { return holdSignal() }
	ratio := shortATR / longATR
	if ratio > 2.0 {
		ema := EMA(s.prices, 9)
		if c.Price > ema { return buySignalCustom(c.Symbol, 0.5, 1.5) }
		return sellSignalCustom(c.Symbol, 0.5, 1.5)
	}
	return holdSignal()
}

// 64. Moving Average Ribbon — All short MAs aligned
type MARibbonScalper struct {
	baseScalper
	prevAligned int
}
func NewMARibbonScalper() *MARibbonScalper {
	return &MARibbonScalper{baseScalper: baseScalper{name: "MARibbon_Alignment_Scalp", maxBuf: defaultBufSize}}
}
func (s *MARibbonScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *MARibbonScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < 55 { return holdSignal() }
	e5 := EMA(s.prices, 5); e10 := EMA(s.prices, 10)
	e20 := EMA(s.prices, 20); e30 := EMA(s.prices, 30); e50 := EMA(s.prices, 50)
	aligned := 0
	if e5 > e10 && e10 > e20 && e20 > e30 && e30 > e50 { aligned = 1 }
	if e5 < e10 && e10 < e20 && e20 < e30 && e30 < e50 { aligned = -1 }
	defer func() { s.prevAligned = aligned }()
	if s.prevAligned != 1 && aligned == 1 { return buySignalCustom(c.Symbol, 0.5, 1.5) }
	if s.prevAligned != -1 && aligned == -1 { return sellSignalCustom(c.Symbol, 0.5, 1.5) }
	return holdSignal()
}

// 65. Momentum Divergence Index
type MomDivScalper struct {
	baseScalper
	period    int
	prevMom   float64
	prevPrice float64
}
func NewMomDivScalper(period int) *MomDivScalper {
	return &MomDivScalper{baseScalper: baseScalper{name: "MomentumDivergence_Scalp", maxBuf: defaultBufSize}, period: period}
}
func (s *MomDivScalper) OnTick(t marketdata.Tick) []Signal { return s.OnCandle(t) }
func (s *MomDivScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+2 { return holdSignal() }
	mom := c.Price - s.prices[len(s.prices)-s.period-1]
	defer func() { s.prevMom = mom; s.prevPrice = c.Price }()
	if s.prevPrice != 0 && c.Price < s.prevPrice && mom > s.prevMom { return buySignalCustom(c.Symbol, 0.4, 1.0) }
	if s.prevPrice != 0 && c.Price > s.prevPrice && mom < s.prevMom { return sellSignalCustom(c.Symbol, 0.4, 1.0) }
	return holdSignal()
}
