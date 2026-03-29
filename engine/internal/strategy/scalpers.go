package strategy

import (
	"antigravity-engine/internal/marketdata"
)

// =============================================================================
// 20 PROFESSIONAL BTC SCALPING STRATEGIES
// Each strategy implements the Strategy interface and uses the shared indicators.
// =============================================================================

// --- Shared base for all scalpers ---
type baseScalper struct {
	name   string
	prices []float64
	maxBuf int
}

func (b *baseScalper) Name() string { return b.name }

func (b *baseScalper) feed(price float64) {
	b.prices = append(b.prices, price)
	if len(b.prices) > b.maxBuf {
		b.prices = b.prices[1:]
	}
}

const defaultBufSize = 500
const defaultQty = 0.01 // Conservative BTC position size for scalping

// Default SL/TP for scalping: tight stops suited to BTC's short-term volatility
// BTC moves ~0.1-0.3% per minute → 0.15% SL and 0.25% TP get hit within minutes
const defaultStopLossPct = 0.18   // 0.18% stop-loss
const defaultTakeProfitPct = 0.65 // 0.65% take-profit

func buySignal(symbol string) []Signal {
	return []Signal{{
		Symbol: symbol, Action: ActionBuy, TargetSize: defaultQty, Confidence: 1.0,
		StopLossPct: defaultStopLossPct, TakeProfitPct: defaultTakeProfitPct,
	}}
}

func sellSignal(symbol string) []Signal {
	return []Signal{{
		Symbol: symbol, Action: ActionSell, TargetSize: defaultQty, Confidence: 1.0,
		StopLossPct: defaultStopLossPct, TakeProfitPct: defaultTakeProfitPct,
	}}
}

func buySignalCustom(symbol string, slPct, tpPct float64) []Signal {
	return []Signal{{
		Symbol: symbol, Action: ActionBuy, TargetSize: defaultQty, Confidence: 1.0,
		StopLossPct: slPct, TakeProfitPct: tpPct,
	}}
}

func sellSignalCustom(symbol string, slPct, tpPct float64) []Signal {
	return []Signal{{
		Symbol: symbol, Action: ActionSell, TargetSize: defaultQty, Confidence: 1.0,
		StopLossPct: slPct, TakeProfitPct: tpPct,
	}}
}

func holdSignal() []Signal {
	return []Signal{{Action: ActionHold}}
}

// =============================================================================
// 0. Test Execution Scalper (Forces an immediate trade for testing)
// =============================================================================
type TestExecutionScalper struct {
	baseScalper
	hasTraded bool
}

func NewTestScalper() *TestExecutionScalper {
	return &TestExecutionScalper{
		baseScalper: baseScalper{name: "Test_Execution_Dumb_Scalper", maxBuf: 10},
	}
}

func (s *TestExecutionScalper) OnTick(tick marketdata.Tick) []Signal {
	if !s.hasTraded {
		s.hasTraded = true
		// Fire a BUY immediately with a tight stop-loss/take-profit to close it out quickly
		return buySignalCustom(tick.Symbol, 0.05, 0.05)
	}
	return holdSignal()
}

func (s *TestExecutionScalper) OnCandle(candle marketdata.Tick) []Signal {
	return s.OnTick(candle)
}

// =============================================================================
// 1. EMA Crossover Scalper
// Fast EMA crosses above Slow EMA → Buy. Crosses below → Sell.
// =============================================================================
type EMACrossScalper struct {
	baseScalper
	fastPeriod int
	slowPeriod int
	prevFast   float64
	prevSlow   float64
}

func NewEMACrossScalper(fast, slow int) *EMACrossScalper {
	return &EMACrossScalper{
		baseScalper: baseScalper{name: "EMA_Cross_Scalp", maxBuf: defaultBufSize},
		fastPeriod:  fast,
		slowPeriod:  slow,
	}
}

func (s *EMACrossScalper) OnTick(tick marketdata.Tick) []Signal {
	return s.OnCandle(tick)
}

func (s *EMACrossScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.slowPeriod+2 {
		return holdSignal()
	}
	fast := EMA(s.prices, s.fastPeriod)
	slow := EMA(s.prices, s.slowPeriod)
	defer func() { s.prevFast = fast; s.prevSlow = slow }()

	if s.prevFast != 0 && s.prevFast <= s.prevSlow && fast > slow {
		return buySignal(candle.Symbol)
	}
	if s.prevFast != 0 && s.prevFast >= s.prevSlow && fast < slow {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 2. RSI Reversal Scalper
// RSI < 30 → Buy (oversold). RSI > 70 → Sell (overbought).
// =============================================================================
type RSIReversalScalper struct {
	baseScalper
	period     int
	oversold   float64
	overbought float64
}

func NewRSIReversalScalper(period int) *RSIReversalScalper {
	return &RSIReversalScalper{
		baseScalper: baseScalper{name: "RSI_Reversal_Scalp", maxBuf: defaultBufSize},
		period:      period,
		oversold:    30,
		overbought:  70,
	}
}

func (s *RSIReversalScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *RSIReversalScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period+2 {
		return holdSignal()
	}
	rsi := RSI(s.prices, s.period)
	if rsi < s.oversold {
		return buySignal(candle.Symbol)
	}
	if rsi > s.overbought {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 3. Bollinger Band Squeeze Scalper
// Price touches lower band → Buy. Touches upper band → Sell.
// =============================================================================
type BollingerScalper struct {
	baseScalper
	period     int
	multiplier float64
}

func NewBollingerScalper(period int, mult float64) *BollingerScalper {
	return &BollingerScalper{
		baseScalper: baseScalper{name: "Bollinger_Squeeze_Scalp", maxBuf: defaultBufSize},
		period:      period,
		multiplier:  mult,
	}
}

func (s *BollingerScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *BollingerScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period {
		return holdSignal()
	}
	upper, _, lower := BollingerBands(s.prices, s.period, s.multiplier)
	if candle.Price <= lower {
		return buySignal(candle.Symbol)
	}
	if candle.Price >= upper {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 4. VWAP Mean Reversion Scalper
// Price significantly below VWAP → Buy. Significantly above → Sell.
// Uses SMA as VWAP proxy (true VWAP requires volume data).
// =============================================================================
type VWAPScalper struct {
	baseScalper
	period    int
	threshold float64
}

func NewVWAPScalper(period int, thresholdPct float64) *VWAPScalper {
	return &VWAPScalper{
		baseScalper: baseScalper{name: "VWAP_MeanRev_Scalp", maxBuf: defaultBufSize},
		period:      period,
		threshold:   thresholdPct,
	}
}

func (s *VWAPScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *VWAPScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period {
		return holdSignal()
	}
	vwap := SMA(s.prices[len(s.prices)-s.period:])
	deviation := ((candle.Price - vwap) / vwap) * 100
	if deviation < -s.threshold {
		return buySignal(candle.Symbol)
	}
	if deviation > s.threshold {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 5. MACD Histogram Scalper
// Histogram crosses zero upward → Buy. Crosses zero downward → Sell.
// =============================================================================
type MACDScalper struct {
	baseScalper
	fastPeriod   int
	slowPeriod   int
	signalPeriod int
	prevHist     float64
}

func NewMACDScalper() *MACDScalper {
	return &MACDScalper{
		baseScalper:  baseScalper{name: "MACD_Histogram_Scalp", maxBuf: defaultBufSize},
		fastPeriod:   12,
		slowPeriod:   26,
		signalPeriod: 9,
	}
}

func (s *MACDScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *MACDScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.slowPeriod+s.signalPeriod {
		return holdSignal()
	}
	_, _, hist := MACD(s.prices, s.fastPeriod, s.slowPeriod, s.signalPeriod)
	defer func() { s.prevHist = hist }()

	if s.prevHist < 0 && hist > 0 {
		return buySignal(candle.Symbol)
	}
	if s.prevHist > 0 && hist < 0 {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 6. Stochastic RSI Scalper
// StochRSI < 20 → Buy. StochRSI > 80 → Sell.
// =============================================================================
type StochRSIScalper struct {
	baseScalper
	rsiPeriod   int
	stochPeriod int
}

func NewStochRSIScalper() *StochRSIScalper {
	return &StochRSIScalper{
		baseScalper: baseScalper{name: "StochRSI_Scalp", maxBuf: defaultBufSize},
		rsiPeriod:   14,
		stochPeriod: 14,
	}
}

func (s *StochRSIScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *StochRSIScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.rsiPeriod+s.stochPeriod+2 {
		return holdSignal()
	}
	stochRSI := StochasticRSI(s.prices, s.rsiPeriod, s.stochPeriod)
	if stochRSI < 20 {
		return buySignal(candle.Symbol)
	}
	if stochRSI > 80 {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 7. Momentum Breakout Scalper
// Price momentum exceeds threshold → trend entry.
// =============================================================================
type MomentumScalper struct {
	baseScalper
	period    int
	threshold float64
}

func NewMomentumScalper(period int, thresholdPct float64) *MomentumScalper {
	return &MomentumScalper{
		baseScalper: baseScalper{name: "Momentum_Breakout_Scalp", maxBuf: defaultBufSize},
		period:      period,
		threshold:   thresholdPct,
	}
}

func (s *MomentumScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *MomentumScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) <= s.period {
		return holdSignal()
	}
	roc := ROC(s.prices, s.period)
	if roc > s.threshold {
		return buySignal(candle.Symbol)
	}
	if roc < -s.threshold {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 8. Mean Reversion Scalper
// Statistical z-score deviation from rolling mean.
// =============================================================================
type MeanReversionScalper struct {
	baseScalper
	period     int
	zThreshold float64
}

func NewMeanReversionScalper(period int, z float64) *MeanReversionScalper {
	return &MeanReversionScalper{
		baseScalper: baseScalper{name: "MeanReversion_ZScore_Scalp", maxBuf: defaultBufSize},
		period:      period,
		zThreshold:  z,
	}
}

func (s *MeanReversionScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *MeanReversionScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period {
		return holdSignal()
	}
	_, mid, lower := BollingerBands(s.prices, s.period, s.zThreshold)
	upper := 2*mid - lower
	if candle.Price < lower {
		return buySignal(candle.Symbol)
	}
	if candle.Price > upper {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 9. Ichimoku Cloud Scalper (Tenkan/Kijun Cross)
// Tenkan > Kijun → Buy. Tenkan < Kijun → Sell.
// =============================================================================
type IchimokuScalper struct {
	baseScalper
	tenkanPeriod int
	kijunPeriod  int
	prevTenkan   float64
	prevKijun    float64
}

func NewIchimokuScalper() *IchimokuScalper {
	return &IchimokuScalper{
		baseScalper:  baseScalper{name: "Ichimoku_TK_Cross_Scalp", maxBuf: defaultBufSize},
		tenkanPeriod: 9,
		kijunPeriod:  26,
	}
}

func (s *IchimokuScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *IchimokuScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.kijunPeriod {
		return holdSignal()
	}
	tenkanH, tenkanL := DonchianChannel(s.prices, s.tenkanPeriod)
	tenkan := (tenkanH + tenkanL) / 2
	kijunH, kijunL := DonchianChannel(s.prices, s.kijunPeriod)
	kijun := (kijunH + kijunL) / 2
	defer func() { s.prevTenkan = tenkan; s.prevKijun = kijun }()

	if s.prevTenkan != 0 && s.prevTenkan <= s.prevKijun && tenkan > kijun {
		return buySignal(candle.Symbol)
	}
	if s.prevTenkan != 0 && s.prevTenkan >= s.prevKijun && tenkan < kijun {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 10. ADX Trend Scalper
// Only trades when ADX > 25 (strong trend), using EMA direction.
// =============================================================================
type ADXTrendScalper struct {
	baseScalper
	adxPeriod    int
	emaPeriod    int
	adxThreshold float64
}

func NewADXTrendScalper() *ADXTrendScalper {
	return &ADXTrendScalper{
		baseScalper:  baseScalper{name: "ADX_Trend_Scalp", maxBuf: defaultBufSize},
		adxPeriod:    14,
		emaPeriod:    9,
		adxThreshold: 25,
	}
}

func (s *ADXTrendScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *ADXTrendScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.adxPeriod+2 {
		return holdSignal()
	}
	adx := ADX(s.prices, s.adxPeriod)
	if adx < s.adxThreshold {
		return holdSignal() // No trade in weak/choppy markets
	}
	ema := EMA(s.prices, s.emaPeriod)
	if candle.Price > ema {
		return buySignal(candle.Symbol)
	}
	if candle.Price < ema {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 11. Pivot Point Bounce Scalper
// Price bounces off S1 → Buy. Rejected at R1 → Sell.
// =============================================================================
type PivotScalper struct {
	baseScalper
	period int
}

func NewPivotScalper(period int) *PivotScalper {
	return &PivotScalper{
		baseScalper: baseScalper{name: "Pivot_Bounce_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *PivotScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *PivotScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period {
		return holdSignal()
	}
	_, r1, s1 := PivotPoints(s.prices, s.period)
	tolerance := (r1 - s1) * 0.02
	if candle.Price <= s1+tolerance {
		return buySignal(candle.Symbol)
	}
	if candle.Price >= r1-tolerance {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 12. Keltner Channel Breakout Scalper
// Price breaks above upper → Buy. Below lower → Sell.
// =============================================================================
type KeltnerScalper struct {
	baseScalper
	emaPeriod  int
	atrPeriod  int
	multiplier float64
}

func NewKeltnerScalper() *KeltnerScalper {
	return &KeltnerScalper{
		baseScalper: baseScalper{name: "Keltner_Breakout_Scalp", maxBuf: defaultBufSize},
		emaPeriod:   20,
		atrPeriod:   10,
		multiplier:  1.5,
	}
}

func (s *KeltnerScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *KeltnerScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.emaPeriod+s.atrPeriod {
		return holdSignal()
	}
	upper, _, lower := KeltnerChannels(s.prices, s.emaPeriod, s.atrPeriod, s.multiplier)
	if candle.Price > upper {
		return buySignal(candle.Symbol)
	}
	if candle.Price < lower {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 13. Williams %R Scalper
// %R < -80 → Buy (oversold). %R > -20 → Sell (overbought).
// =============================================================================
type WilliamsRScalper struct {
	baseScalper
	period int
}

func NewWilliamsRScalper(period int) *WilliamsRScalper {
	return &WilliamsRScalper{
		baseScalper: baseScalper{name: "WilliamsR_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *WilliamsRScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *WilliamsRScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period {
		return holdSignal()
	}
	wr := WilliamsR(s.prices, s.period)
	if wr < -80 {
		return buySignal(candle.Symbol)
	}
	if wr > -20 {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 14. CCI Divergence Scalper
// CCI < -100 → Buy. CCI > +100 → Sell.
// =============================================================================
type CCIScalper struct {
	baseScalper
	period int
}

func NewCCIScalper(period int) *CCIScalper {
	return &CCIScalper{
		baseScalper: baseScalper{name: "CCI_Divergence_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *CCIScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *CCIScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period {
		return holdSignal()
	}
	cci := CCI(s.prices, s.period)
	if cci < -100 {
		return buySignal(candle.Symbol)
	}
	if cci > 100 {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 15. Donchian Channel Breakout Scalper
// Price makes new period high → Buy. New period low → Sell.
// =============================================================================
type DonchianScalper struct {
	baseScalper
	period int
}

func NewDonchianScalper(period int) *DonchianScalper {
	return &DonchianScalper{
		baseScalper: baseScalper{name: "Donchian_Breakout_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *DonchianScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *DonchianScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period+1 {
		return holdSignal()
	}
	upper, lower := DonchianChannel(s.prices[:len(s.prices)-1], s.period)
	if candle.Price > upper {
		return buySignal(candle.Symbol)
	}
	if candle.Price < lower {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 16. Parabolic SAR Reversal Scalper
// Price crosses above SAR → Buy. Below SAR → Sell.
// =============================================================================
type ParabolicSARScalper struct {
	baseScalper
	af    float64
	maxAF float64
}

func NewParabolicSARScalper() *ParabolicSARScalper {
	return &ParabolicSARScalper{
		baseScalper: baseScalper{name: "ParabolicSAR_Reversal_Scalp", maxBuf: defaultBufSize},
		af:          0.02,
		maxAF:       0.20,
	}
}

func (s *ParabolicSARScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *ParabolicSARScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < 5 {
		return holdSignal()
	}
	sar := ParabolicSAR(s.prices, s.af, s.maxAF)
	if candle.Price > sar {
		return buySignal(candle.Symbol)
	}
	if candle.Price < sar {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 17. Hull Moving Average Scalper
// Hull MA rising → Buy. Falling → Sell. (Smoother, less lag than EMA.)
// =============================================================================
type HullMAScalper struct {
	baseScalper
	period  int
	prevHMA float64
}

func NewHullMAScalper(period int) *HullMAScalper {
	return &HullMAScalper{
		baseScalper: baseScalper{name: "HullMA_Trend_Scalp", maxBuf: defaultBufSize},
		period:      period,
	}
}

func (s *HullMAScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *HullMAScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.period+2 {
		return holdSignal()
	}
	hma := HullMA(s.prices, s.period)
	defer func() { s.prevHMA = hma }()

	if s.prevHMA != 0 && hma > s.prevHMA && candle.Price > hma {
		return buySignal(candle.Symbol)
	}
	if s.prevHMA != 0 && hma < s.prevHMA && candle.Price < hma {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 18. Triple EMA Ribbon Scalper
// All 3 EMAs align bullish → Buy. All bearish → Sell.
// =============================================================================
type TripleEMAScalper struct {
	baseScalper
	fast int
	mid  int
	slow int
}

func NewTripleEMAScalper(fast, mid, slow int) *TripleEMAScalper {
	return &TripleEMAScalper{
		baseScalper: baseScalper{name: "TripleEMA_Ribbon_Scalp", maxBuf: defaultBufSize},
		fast:        fast,
		mid:         mid,
		slow:        slow,
	}
}

func (s *TripleEMAScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *TripleEMAScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) < s.slow+2 {
		return holdSignal()
	}
	fastEMA := EMA(s.prices, s.fast)
	midEMA := EMA(s.prices, s.mid)
	slowEMA := EMA(s.prices, s.slow)

	if fastEMA > midEMA && midEMA > slowEMA && candle.Price > fastEMA {
		return buySignal(candle.Symbol)
	}
	if fastEMA < midEMA && midEMA < slowEMA && candle.Price < fastEMA {
		return sellSignal(candle.Symbol)
	}
	return holdSignal()
}

// =============================================================================
// 19. Rate of Change Reversal Scalper
// Extreme positive ROC → exhaustion sell. Extreme negative ROC → bounce buy.
// =============================================================================
type ROCReversalScalper struct {
	baseScalper
	period    int
	threshold float64
}

func NewROCReversalScalper(period int, threshold float64) *ROCReversalScalper {
	return &ROCReversalScalper{
		baseScalper: baseScalper{name: "ROC_Reversal_Scalp", maxBuf: defaultBufSize},
		period:      period,
		threshold:   threshold,
	}
}

func (s *ROCReversalScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *ROCReversalScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	if len(s.prices) <= s.period {
		return holdSignal()
	}
	roc := ROC(s.prices, s.period)
	if roc < -s.threshold {
		return buySignal(candle.Symbol) // Oversold bounce
	}
	if roc > s.threshold {
		return sellSignal(candle.Symbol) // Overbought exhaustion
	}
	return holdSignal()
}

// =============================================================================
// 20. Order Flow Imbalance Scalper
// Tracks buy vs sell tick pressure. Heavy buy pressure → Buy. Heavy sell → Sell.
// =============================================================================
type OrderFlowScalper struct {
	baseScalper
	buyTicks  int
	sellTicks int
	window    int
	threshold float64
	tickCount int
}

func NewOrderFlowScalper(window int, threshold float64) *OrderFlowScalper {
	return &OrderFlowScalper{
		baseScalper: baseScalper{name: "OrderFlow_Imbalance_Scalp", maxBuf: defaultBufSize},
		window:      window,
		threshold:   threshold,
	}
}

func (s *OrderFlowScalper) OnTick(tick marketdata.Tick) []Signal { return s.OnCandle(tick) }

func (s *OrderFlowScalper) OnCandle(candle marketdata.Tick) []Signal {
	s.feed(candle.Price)
	s.tickCount++

	// Classify as buy or sell tick
	if len(s.prices) >= 2 {
		if s.prices[len(s.prices)-1] >= s.prices[len(s.prices)-2] {
			s.buyTicks++
		} else {
			s.sellTicks++
		}
	}

	// Evaluate window
	if s.tickCount%s.window == 0 {
		total := s.buyTicks + s.sellTicks
		if total == 0 {
			return holdSignal()
		}
		buyRatio := float64(s.buyTicks) / float64(total)
		s.buyTicks = 0
		s.sellTicks = 0

		if buyRatio > s.threshold {
			return buySignal(candle.Symbol) // Heavy buying pressure
		}
		if buyRatio < (1 - s.threshold) {
			return sellSignal(candle.Symbol) // Heavy selling pressure
		}
	}
	return holdSignal()
}
