package strategy

// =============================================================================
// ELITE STRATEGIES V2 — 95 strategies using 20 reusable generic structs.
// All strategies are crossover/threshold-based with ADX + RSI guards.
// SL/TP geometry ensures R:R ≥ 1.5 on every position.
// =============================================================================

import (
	"math"

	"antigravity-engine/internal/marketdata"
)

// ─────────────────────────────────────────────────────────────────────────────
// GENERIC STRUCT TYPES
// ─────────────────────────────────────────────────────────────────────────────

// ── 1. EMA crossover (crossover-only signal, not steady state) ─────────────
type EMACrossV2 struct {
	baseScalper
	fast, slow         int
	adxMin             float64
	rsiLo, rsiHi       float64
	slPct, tpPct       float64
	prevAbove, prevSet bool
}

func newEMACrossV2(name string, fast, slow int, adxMin, rsiLo, rsiHi, slPct, tpPct float64) *EMACrossV2 {
	return &EMACrossV2{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		fast: fast, slow: slow, adxMin: adxMin,
		rsiLo: rsiLo, rsiHi: rsiHi, slPct: slPct, tpPct: tpPct,
	}
}
func (s *EMACrossV2) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *EMACrossV2) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.slow+16 {
		return holdSignal()
	}
	fast := EMA(s.prices, s.fast)
	slow := EMA(s.prices, s.slow)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	above := fast > slow
	if !s.prevSet {
		s.prevAbove = above
		s.prevSet = true
		return holdSignal()
	}
	up, dn := !s.prevAbove && above, s.prevAbove && !above
	s.prevAbove = above
	if adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.95+adx/200.0, 1.30)
	if up && rsi >= s.rsiLo && rsi <= s.rsiHi {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
	}
	if dn && rsi <= 100-s.rsiLo && rsi >= 100-s.rsiHi {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
	}
	return holdSignal()
}

// ── 2. RSI threshold crossover ─────────────────────────────────────────────
type RSIThresholdScalper struct {
	baseScalper
	period                   int
	buyBelow, buyAboveAfter  float64
	adxMin                   float64
	slPct, tpPct             float64
	prevRSI                  float64
}

func newRSIThreshold(name string, period int, buyLo, buyHi, adxMin, slPct, tpPct float64) *RSIThresholdScalper {
	return &RSIThresholdScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		period: period, buyBelow: buyLo, buyAboveAfter: buyHi,
		adxMin: adxMin, slPct: slPct, tpPct: tpPct,
	}
}
func (s *RSIThresholdScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *RSIThresholdScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+5 {
		return holdSignal()
	}
	rsi := RSI(s.prices, s.period)
	adx := ADX(s.prices, 14)
	prev := s.prevRSI
	s.prevRSI = rsi
	if prev == 0 || adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.94+adx/220.0, 1.25)
	if prev <= s.buyBelow && rsi > s.buyAboveAfter {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
	}
	sellHi := 100 - s.buyBelow
	sellLo := 100 - s.buyAboveAfter
	if prev >= sellHi && rsi < sellLo {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
	}
	return holdSignal()
}

// ── 3. RSI slope momentum ──────────────────────────────────────────────────
type RSISlopeScalper struct {
	baseScalper
	period, lookback int
	slopeThresh      float64
	adxMin           float64
	slPct, tpPct     float64
	rsiHistory       []float64
}

func newRSISlopeScalper(name string, period, lookback int, slopeThresh, adxMin, slPct, tpPct float64) *RSISlopeScalper {
	return &RSISlopeScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		period: period, lookback: lookback, slopeThresh: slopeThresh,
		adxMin: adxMin, slPct: slPct, tpPct: tpPct,
	}
}
func (s *RSISlopeScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *RSISlopeScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+s.lookback+2 {
		return holdSignal()
	}
	rsi := RSI(s.prices, s.period)
	s.rsiHistory = appendRollingFloat(s.rsiHistory, rsi, defaultBufSize)
	if len(s.rsiHistory) < s.lookback {
		return holdSignal()
	}
	adx := ADX(s.prices, 14)
	if adx < s.adxMin {
		return holdSignal()
	}
	prevRSI := s.rsiHistory[len(s.rsiHistory)-s.lookback]
	slope := rsi - prevRSI
	conf := math.Min(0.92+math.Abs(slope)/20.0, 1.25)
	if slope > s.slopeThresh && rsi > 50 && rsi < 72 {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
	}
	if slope < -s.slopeThresh && rsi < 50 && rsi > 28 {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
	}
	return holdSignal()
}

// ── 4. Bollinger Band signal (bounce, mid-cross, breakout) ─────────────────
type BBSignalScalper struct {
	baseScalper
	period               int
	mult                 float64
	mode                 string
	adxMax, adxMin       float64
	rsiMin, rsiMax       float64
	slPct, tpPct         float64
	prevAboveMid, prevSet bool
}

func newBBScalper(name, mode string, period int, mult, adxMin, adxMax, rsiMin, rsiMax, slPct, tpPct float64) *BBSignalScalper {
	return &BBSignalScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		period: period, mult: mult, mode: mode,
		adxMin: adxMin, adxMax: adxMax, rsiMin: rsiMin, rsiMax: rsiMax,
		slPct: slPct, tpPct: tpPct,
	}
}
func (s *BBSignalScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *BBSignalScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+5 {
		return holdSignal()
	}
	upper, mid, lower := BollingerBands(s.prices, s.period, s.mult)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if s.adxMin > 0 && adx < s.adxMin {
		return holdSignal()
	}
	if s.adxMax > 0 && adx > s.adxMax {
		return holdSignal()
	}
	conf := math.Min(0.92+adx/250.0, 1.25)
	price := c.Price
	switch s.mode {
	case "bounce_lower":
		if price <= lower*1.0003 && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if price >= upper*0.9997 && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "mid_cross":
		above := price > mid
		if !s.prevSet {
			s.prevAboveMid = above
			s.prevSet = true
			return holdSignal()
		}
		up, dn := !s.prevAboveMid && above, s.prevAboveMid && !above
		s.prevAboveMid = above
		if up && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if dn && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "breakout":
		if price > upper && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if price < lower && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	}
	return holdSignal()
}

// ── 5. BB width expansion (squeeze break) ─────────────────────────────────
type BBWidthScalper struct {
	baseScalper
	period         int
	mult           float64
	adxMin         float64
	rsiMin, rsiMax float64
	slPct, tpPct   float64
	widthHistory   []float64
}

func newBBWidth(name string, period int, mult, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *BBWidthScalper {
	return &BBWidthScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		period: period, mult: mult, adxMin: adxMin,
		rsiMin: rsiMin, rsiMax: rsiMax, slPct: slPct, tpPct: tpPct,
	}
}
func (s *BBWidthScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *BBWidthScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+10 {
		return holdSignal()
	}
	upper, mid, lower := BollingerBands(s.prices, s.period, s.mult)
	if mid == 0 {
		return holdSignal()
	}
	width := (upper - lower) / mid
	s.widthHistory = appendRollingFloat(s.widthHistory, width, defaultBufSize)
	if len(s.widthHistory) < 10 {
		return holdSignal()
	}
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if adx < s.adxMin {
		return holdSignal()
	}
	avgWidth := tailAverage(s.widthHistory[:len(s.widthHistory)-1], 10)
	if avgWidth == 0 {
		return holdSignal()
	}
	conf := math.Min(0.93+width/avgWidth*0.05, 1.25)
	if width > avgWidth*1.3 && c.Price > mid && rsi >= s.rsiMin && rsi <= s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
	}
	if width > avgWidth*1.3 && c.Price < mid && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
	}
	return holdSignal()
}

// ── 6. VWAP signal (cross, deviation, pullback) ────────────────────────────
type VWAPSignalScalper struct {
	baseScalper
	volumes            []float64
	vwapPeriod         int
	mode               string
	deviationPct       float64
	adxMin             float64
	rsiMin, rsiMax     float64
	slPct, tpPct       float64
	prevAbove, prevSet bool
}

func newVWAPScalper(name, mode string, vwapPeriod int, devPct, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *VWAPSignalScalper {
	return &VWAPSignalScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		vwapPeriod: vwapPeriod, mode: mode, deviationPct: devPct,
		adxMin: adxMin, rsiMin: rsiMin, rsiMax: rsiMax,
		slPct: slPct, tpPct: tpPct,
	}
}
func (s *VWAPSignalScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *VWAPSignalScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	s.volumes = appendRollingFloat(s.volumes, c.Quantity, defaultBufSize)
	if len(s.prices) < s.vwapPeriod+15 || len(s.volumes) < s.vwapPeriod {
		return holdSignal()
	}
	vwap := RollingVWAP(s.prices, s.volumes, s.vwapPeriod)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if vwap == 0 || (s.adxMin > 0 && adx < s.adxMin) {
		return holdSignal()
	}
	conf := math.Min(0.93+adx/220.0, 1.25)
	price := c.Price
	switch s.mode {
	case "cross":
		above := price > vwap
		if !s.prevSet {
			s.prevAbove = above
			s.prevSet = true
			return holdSignal()
		}
		up, dn := !s.prevAbove && above, s.prevAbove && !above
		s.prevAbove = above
		if up && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if dn && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "deviation":
		devDown := (vwap - price) / vwap * 100
		devUp := (price - vwap) / vwap * 100
		if devDown >= s.deviationPct && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if devUp >= s.deviationPct && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "pullback":
		ema20 := EMA(s.prices, 20)
		if price > vwap && price < ema20 && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if price < vwap && price > ema20 && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	}
	return holdSignal()
}

// ── 7. MACD signal (cross, zero-cross, histogram momentum) ────────────────
type MACDSignalScalperV2 struct {
	baseScalper
	fast, slow, sig        int
	mode                   string
	adxMin                 float64
	rsiMin, rsiMax         float64
	slPct, tpPct           float64
	prevHist, prevMACDLine float64
}

func newMACDScalperV2(name, mode string, fast, slow, sig int, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *MACDSignalScalperV2 {
	return &MACDSignalScalperV2{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		fast: fast, slow: slow, sig: sig, mode: mode,
		adxMin: adxMin, rsiMin: rsiMin, rsiMax: rsiMax,
		slPct: slPct, tpPct: tpPct,
	}
}
func (s *MACDSignalScalperV2) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *MACDSignalScalperV2) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.slow+s.sig+5 {
		return holdSignal()
	}
	macdLine, _, hist := MACD(s.prices, s.fast, s.slow, s.sig)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if s.adxMin > 0 && adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.93+adx/200.0, 1.25)
	prev := s.prevHist
	prevM := s.prevMACDLine
	s.prevHist = hist
	s.prevMACDLine = macdLine
	if prev == 0 && prevM == 0 {
		return holdSignal()
	}
	switch s.mode {
	case "cross":
		if prev < 0 && hist > 0 && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if prev > 0 && hist < 0 && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "zero_cross":
		if prevM < 0 && macdLine > 0 && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if prevM > 0 && macdLine < 0 && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "hist_momentum":
		if prev > 0 && hist > prev && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if prev < 0 && hist < prev && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	}
	return holdSignal()
}

// ── 8. Volume + price signal (breakout, climax reversal) ───────────────────
type VolumePriceScalper struct {
	baseScalper
	volumes        []float64
	volPeriod      int
	volMult        float64
	mode           string
	adxMin         float64
	rsiMin, rsiMax float64
	slPct, tpPct   float64
}

func newVolumePrice(name, mode string, volPeriod int, volMult, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *VolumePriceScalper {
	return &VolumePriceScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		volPeriod: volPeriod, volMult: volMult, mode: mode,
		adxMin: adxMin, rsiMin: rsiMin, rsiMax: rsiMax,
		slPct: slPct, tpPct: tpPct,
	}
}
func (s *VolumePriceScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *VolumePriceScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	s.volumes = appendRollingFloat(s.volumes, c.Quantity, defaultBufSize)
	if len(s.prices) < s.volPeriod+15 || len(s.volumes) < s.volPeriod+1 {
		return holdSignal()
	}
	avgVol := tailAverage(s.volumes[:len(s.volumes)-1], s.volPeriod)
	if avgVol == 0 {
		return holdSignal()
	}
	volRatio := c.Quantity / avgVol
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if s.adxMin > 0 && adx < s.adxMin {
		return holdSignal()
	}
	upper, lower := DonchianChannel(s.prices[:len(s.prices)-1], 10)
	conf := math.Min(0.93+volRatio*0.08, 1.35)
	switch s.mode {
	case "breakout":
		if volRatio >= s.volMult && c.Price > upper && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if volRatio >= s.volMult && c.Price < lower && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "climax_reversal":
		if volRatio >= s.volMult && rsi <= s.rsiMin {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if volRatio >= s.volMult && rsi >= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	}
	return holdSignal()
}

// ── 9. N-bar Donchian breakout ─────────────────────────────────────────────
type NBarBreakoutScalper struct {
	baseScalper
	n              int
	adxMin         float64
	rsiMin, rsiMax float64
	slPct, tpPct   float64
}

func newNBarBreakout(name string, n int, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *NBarBreakoutScalper {
	return &NBarBreakoutScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		n: n, adxMin: adxMin, rsiMin: rsiMin, rsiMax: rsiMax,
		slPct: slPct, tpPct: tpPct,
	}
}
func (s *NBarBreakoutScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *NBarBreakoutScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.n+15 {
		return holdSignal()
	}
	upper, lower := DonchianChannel(s.prices[:len(s.prices)-1], s.n)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.93+adx/200.0, 1.25)
	if c.Price > upper && rsi >= s.rsiMin && rsi <= s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
	}
	if c.Price < lower && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
	}
	return holdSignal()
}

// ── 10. Triple EMA alignment crossover ────────────────────────────────────
type TripleEMAScalperV2 struct {
	baseScalper
	e1, e2, e3     int
	adxMin         float64
	rsiMin, rsiMax float64
	slPct, tpPct   float64
	prevAligned    int
}

func newTripleEMAV2(name string, e1, e2, e3 int, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *TripleEMAScalperV2 {
	return &TripleEMAScalperV2{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		e1: e1, e2: e2, e3: e3, adxMin: adxMin,
		rsiMin: rsiMin, rsiMax: rsiMax, slPct: slPct, tpPct: tpPct,
	}
}
func (s *TripleEMAScalperV2) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *TripleEMAScalperV2) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.e3+15 {
		return holdSignal()
	}
	ema1 := EMA(s.prices, s.e1)
	ema2 := EMA(s.prices, s.e2)
	ema3 := EMA(s.prices, s.e3)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if adx < s.adxMin {
		return holdSignal()
	}
	curr := 0
	if ema1 > ema2 && ema2 > ema3 {
		curr = 1
	}
	if ema1 < ema2 && ema2 < ema3 {
		curr = -1
	}
	prev := s.prevAligned
	s.prevAligned = curr
	conf := math.Min(0.93+adx/200.0, 1.30)
	if prev != 1 && curr == 1 && c.Price > ema1 && rsi >= s.rsiMin && rsi <= s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
	}
	if prev != -1 && curr == -1 && c.Price < ema1 && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
	}
	return holdSignal()
}

// ── 11. CCI signal (zero-cross, extreme bounce, trend) ────────────────────
type CCISignalScalper struct {
	baseScalper
	period         int
	mode           string
	adxMin         float64
	rsiMin, rsiMax float64
	slPct, tpPct   float64
	prevCCI        float64
}

func newCCIScalper(name, mode string, period int, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *CCISignalScalper {
	return &CCISignalScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		period: period, mode: mode, adxMin: adxMin,
		rsiMin: rsiMin, rsiMax: rsiMax, slPct: slPct, tpPct: tpPct,
	}
}
func (s *CCISignalScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *CCISignalScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+5 {
		return holdSignal()
	}
	cci := CCI(s.prices, s.period)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	prev := s.prevCCI
	s.prevCCI = cci
	if s.adxMin > 0 && adx < s.adxMin {
		return holdSignal()
	}
	if prev == 0 {
		return holdSignal()
	}
	conf := math.Min(0.92+adx/220.0, 1.25)
	switch s.mode {
	case "zero_cross":
		if prev <= 0 && cci > 0 && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if prev >= 0 && cci < 0 && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "extreme_bounce":
		if prev <= -100 && cci > -100 && rsi >= s.rsiMin {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if prev >= 100 && cci < 100 && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "trend":
		if cci > 100 && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if cci < -100 && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	}
	return holdSignal()
}

// ── 12. Stochastic signal (cross, oversold, trend) ─────────────────────────
type StochSignalScalper struct {
	baseScalper
	period, smooth int
	mode           string
	adxMin         float64
	rsiMin, rsiMax float64
	slPct, tpPct   float64
	rawKHist       []float64
	prevK, prevD   float64
}

func newStochScalper(name, mode string, period, smooth int, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *StochSignalScalper {
	return &StochSignalScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		period: period, smooth: smooth, mode: mode,
		adxMin: adxMin, rsiMin: rsiMin, rsiMax: rsiMax,
		slPct: slPct, tpPct: tpPct,
	}
}
func (s *StochSignalScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *StochSignalScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+s.smooth+5 {
		return holdSignal()
	}
	rawK := CloseStochastic(s.prices, s.period)
	s.rawKHist = appendRollingFloat(s.rawKHist, rawK, defaultBufSize)
	k := tailAverage(s.rawKHist, s.smooth)
	d := tailAverage(s.rawKHist, s.smooth*3)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if s.adxMin > 0 && adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.92+adx/220.0, 1.25)
	prevK, prevD := s.prevK, s.prevD
	s.prevK = k
	s.prevD = d
	if prevK == 0 && prevD == 0 {
		return holdSignal()
	}
	switch s.mode {
	case "cross":
		if prevK <= prevD && k > d && k < 70 && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if prevK >= prevD && k < d && k > 30 && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "oversold":
		if prevK <= 25 && k > 25 && rsi >= s.rsiMin {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if prevK >= 75 && k < 75 && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "trend":
		if k > 50 && k < 80 && d > 50 && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if k < 50 && k > 20 && d < 50 && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	}
	return holdSignal()
}

// ── 13. ATR momentum / channel break / contraction ─────────────────────────
type ATRSignalScalper struct {
	baseScalper
	atrPeriod, emaPeriod int
	mode                 string
	adxMin               float64
	rsiMin, rsiMax       float64
	slPct, tpPct         float64
	prevATR              float64
}

func newATRScalper(name, mode string, atrPeriod, emaPeriod int, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *ATRSignalScalper {
	return &ATRSignalScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		atrPeriod: atrPeriod, emaPeriod: emaPeriod, mode: mode,
		adxMin: adxMin, rsiMin: rsiMin, rsiMax: rsiMax,
		slPct: slPct, tpPct: tpPct,
	}
}
func (s *ATRSignalScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *ATRSignalScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.atrPeriod+s.emaPeriod+5 {
		return holdSignal()
	}
	atr := ATR(s.prices, s.atrPeriod)
	ema := EMA(s.prices, s.emaPeriod)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	prev := s.prevATR
	s.prevATR = atr
	if s.adxMin > 0 && adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.93+adx/200.0, 1.25)
	switch s.mode {
	case "momentum":
		if prev > 0 && atr > prev*1.2 && c.Price > ema && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if prev > 0 && atr > prev*1.2 && c.Price < ema && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "channel_break":
		upper := ema + atr*1.5
		lower := ema - atr*1.5
		if c.Price > upper && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if c.Price < lower && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "contraction":
		if prev > 0 && atr > prev*1.35 && c.Price > ema && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if prev > 0 && atr > prev*1.35 && c.Price < ema && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	}
	return holdSignal()
}

// ── 14. ROC signal ─────────────────────────────────────────────────────────
type ROCSignalScalper struct {
	baseScalper
	period         int
	threshold      float64
	adxMin         float64
	rsiMin, rsiMax float64
	slPct, tpPct   float64
	prevROC        float64
}

func newROCScalper(name string, period int, threshold, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *ROCSignalScalper {
	return &ROCSignalScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		period: period, threshold: threshold, adxMin: adxMin,
		rsiMin: rsiMin, rsiMax: rsiMax, slPct: slPct, tpPct: tpPct,
	}
}
func (s *ROCSignalScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *ROCSignalScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+5 {
		return holdSignal()
	}
	roc := ROC(s.prices, s.period)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	prev := s.prevROC
	s.prevROC = roc
	if s.adxMin > 0 && adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.92+math.Abs(roc)/20.0, 1.25)
	if prev <= 0 && roc > s.threshold && rsi >= s.rsiMin && rsi <= s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
	}
	if prev >= 0 && roc < -s.threshold && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
	}
	return holdSignal()
}

// ── 15. Williams %R signal ─────────────────────────────────────────────────
type WilliamsRScalperV2 struct {
	baseScalper
	period         int
	mode           string
	adxMin         float64
	rsiMin, rsiMax float64
	slPct, tpPct   float64
	prevWR         float64
}

func newWilliamsRV2(name, mode string, period int, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *WilliamsRScalperV2 {
	return &WilliamsRScalperV2{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		period: period, mode: mode, adxMin: adxMin,
		rsiMin: rsiMin, rsiMax: rsiMax, slPct: slPct, tpPct: tpPct,
	}
}
func (s *WilliamsRScalperV2) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *WilliamsRScalperV2) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+5 {
		return holdSignal()
	}
	wr := WilliamsR(s.prices, s.period)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	prev := s.prevWR
	s.prevWR = wr
	if s.adxMin > 0 && adx < s.adxMin {
		return holdSignal()
	}
	if prev == 0 {
		return holdSignal()
	}
	conf := math.Min(0.92+adx/220.0, 1.25)
	switch s.mode {
	case "bounce":
		if prev <= -80 && wr > -80 && rsi >= s.rsiMin {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if prev >= -20 && wr < -20 && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "trend":
		if wr > -30 && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if wr < -70 && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	}
	return holdSignal()
}

// ── 16. Parabolic SAR + EMA confirmation ──────────────────────────────────
type PsarEMAScalper struct {
	baseScalper
	emaPeriod                int
	af, maxAF                float64
	adxMin                   float64
	rsiMin, rsiMax           float64
	slPct, tpPct             float64
	prevPsarBelow, prevSet   bool
}

func newPsarEMA(name string, emaPeriod int, af, maxAF, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *PsarEMAScalper {
	return &PsarEMAScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		emaPeriod: emaPeriod, af: af, maxAF: maxAF, adxMin: adxMin,
		rsiMin: rsiMin, rsiMax: rsiMax, slPct: slPct, tpPct: tpPct,
	}
}
func (s *PsarEMAScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *PsarEMAScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.emaPeriod+20 {
		return holdSignal()
	}
	psar := ParabolicSAR(s.prices, s.af, s.maxAF)
	ema := EMA(s.prices, s.emaPeriod)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.93+adx/200.0, 1.25)
	psarBelow := psar < c.Price
	if !s.prevSet {
		s.prevPsarBelow = psarBelow
		s.prevSet = true
		return holdSignal()
	}
	up := !s.prevPsarBelow && psarBelow
	dn := s.prevPsarBelow && !psarBelow
	s.prevPsarBelow = psarBelow
	if up && c.Price > ema && rsi >= s.rsiMin && rsi <= s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
	}
	if dn && c.Price < ema && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
	}
	return holdSignal()
}

// ── 17. Hull MA slope signal ───────────────────────────────────────────────
type HullMAScalperV2 struct {
	baseScalper
	period         int
	adxMin         float64
	rsiMin, rsiMax float64
	slPct, tpPct   float64
	prevHull       float64
}

func newHullMAV2(name string, period int, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *HullMAScalperV2 {
	return &HullMAScalperV2{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		period: period, adxMin: adxMin,
		rsiMin: rsiMin, rsiMax: rsiMax, slPct: slPct, tpPct: tpPct,
	}
}
func (s *HullMAScalperV2) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *HullMAScalperV2) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+5 {
		return holdSignal()
	}
	hull := HullMA(s.prices, s.period)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.93+adx/200.0, 1.25)
	prev := s.prevHull
	s.prevHull = hull
	if prev == 0 {
		return holdSignal()
	}
	if prev <= hull && c.Price > hull && rsi >= s.rsiMin && rsi <= s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
	}
	if prev >= hull && c.Price < hull && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
	}
	return holdSignal()
}

// ── 18. Keltner Channel (break, bounce, midline) ───────────────────────────
type KeltnerScalperV2 struct {
	baseScalper
	emaPeriod, atrPeriod     int
	mult                     float64
	mode                     string
	adxMin                   float64
	rsiMin, rsiMax           float64
	slPct, tpPct             float64
	prevAboveMid, prevSet    bool
}

func newKeltnerV2(name, mode string, emaPeriod, atrPeriod int, mult, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *KeltnerScalperV2 {
	return &KeltnerScalperV2{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		emaPeriod: emaPeriod, atrPeriod: atrPeriod, mult: mult, mode: mode,
		adxMin: adxMin, rsiMin: rsiMin, rsiMax: rsiMax,
		slPct: slPct, tpPct: tpPct,
	}
}
func (s *KeltnerScalperV2) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *KeltnerScalperV2) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.emaPeriod+s.atrPeriod+5 {
		return holdSignal()
	}
	upper, mid, lower := KeltnerChannels(s.prices, s.emaPeriod, s.atrPeriod, s.mult)
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if s.adxMin > 0 && adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.93+adx/200.0, 1.25)
	switch s.mode {
	case "break":
		if c.Price > upper && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if c.Price < lower && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "bounce":
		if c.Price <= lower*1.0003 && rsi >= s.rsiMin && rsi <= 52 {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if c.Price >= upper*0.9997 && rsi >= 48 && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	case "midline":
		above := c.Price > mid
		if !s.prevSet {
			s.prevAboveMid = above
			s.prevSet = true
			return holdSignal()
		}
		up, dn := !s.prevAboveMid && above, s.prevAboveMid && !above
		s.prevAboveMid = above
		if up && rsi >= s.rsiMin && rsi <= s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
		}
		if dn && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
			return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
		}
	}
	return holdSignal()
}

// ── 19. Momentum divergence (price vs RSI) ─────────────────────────────────
type MomentumDivScalper struct {
	baseScalper
	period, lookback int
	adxMax           float64
	slPct, tpPct     float64
	priceHist        []float64
	rsiHist          []float64
}

func newMomDiv(name string, period, lookback int, adxMax, slPct, tpPct float64) *MomentumDivScalper {
	return &MomentumDivScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		period: period, lookback: lookback, adxMax: adxMax,
		slPct: slPct, tpPct: tpPct,
	}
}
func (s *MomentumDivScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *MomentumDivScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.period+s.lookback+2 {
		return holdSignal()
	}
	rsi := RSI(s.prices, s.period)
	s.priceHist = appendRollingFloat(s.priceHist, c.Price, defaultBufSize)
	s.rsiHist = appendRollingFloat(s.rsiHist, rsi, defaultBufSize)
	if len(s.priceHist) < s.lookback {
		return holdSignal()
	}
	adx := ADX(s.prices, 14)
	if s.adxMax > 0 && adx > s.adxMax {
		return holdSignal()
	}
	pricePrev := s.priceHist[len(s.priceHist)-s.lookback]
	rsiPrev := s.rsiHist[len(s.rsiHist)-s.lookback]
	if c.Price < pricePrev && rsi > rsiPrev && rsi < 45 {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, 0.95)
	}
	if c.Price > pricePrev && rsi < rsiPrev && rsi > 55 {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, 0.95)
	}
	return holdSignal()
}

// ── 20. Consecutive candles momentum ──────────────────────────────────────
type ConsecCandlesScalper struct {
	baseScalper
	n              int
	adxMin         float64
	rsiMin, rsiMax float64
	slPct, tpPct   float64
}

func newConsecCandles(name string, n int, adxMin, rsiMin, rsiMax, slPct, tpPct float64) *ConsecCandlesScalper {
	return &ConsecCandlesScalper{
		baseScalper: baseScalper{name: name, maxBuf: defaultBufSize},
		n: n, adxMin: adxMin, rsiMin: rsiMin, rsiMax: rsiMax,
		slPct: slPct, tpPct: tpPct,
	}
}
func (s *ConsecCandlesScalper) OnTick(t marketdata.Tick) []Signal  { return s.OnCandle(t) }
func (s *ConsecCandlesScalper) OnCandle(c marketdata.Tick) []Signal {
	s.feed(c.Price)
	if len(s.prices) < s.n+15 {
		return holdSignal()
	}
	adx := ADX(s.prices, 14)
	rsi := RSI(s.prices, 14)
	if adx < s.adxMin {
		return holdSignal()
	}
	conf := math.Min(0.92+adx/200.0, 1.25)
	n := len(s.prices)
	allUp, allDown := true, true
	for i := 0; i < s.n; i++ {
		if s.prices[n-1-i] <= s.prices[n-2-i] {
			allUp = false
		}
		if s.prices[n-1-i] >= s.prices[n-2-i] {
			allDown = false
		}
	}
	if allUp && rsi >= s.rsiMin && rsi <= s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionBuy, s.slPct, s.tpPct, conf)
	}
	if allDown && rsi <= 100-s.rsiMin && rsi >= 100-s.rsiMax {
		return signalWithConfidence(c.Symbol, ActionSell, s.slPct, s.tpPct, conf)
	}
	return holdSignal()
}

// =============================================================================
// CONSTRUCTORS — 95 strategies from the generic structs above
// =============================================================================

// ── EMA Cross family (15) ──────────────────────────────────────────────────
func NewEMA3_8CrossScalp()   *EMACrossV2 { return newEMACrossV2("EMA_3_8_Cross_Scalp",    3,  8,  18, 48, 68, 0.16, 0.38) }
func NewEMA5_13CrossScalp()  *EMACrossV2 { return newEMACrossV2("EMA_5_13_Cross_Scalp",   5, 13,  20, 46, 70, 0.17, 0.40) }
func NewEMA10_30CrossScalp() *EMACrossV2 { return newEMACrossV2("EMA_10_30_Cross_Scalp", 10, 30,  22, 45, 70, 0.18, 0.42) }
func NewEMA13_34CrossScalp() *EMACrossV2 { return newEMACrossV2("EMA_13_34_Cross_Scalp", 13, 34,  22, 45, 70, 0.18, 0.44) }
func NewEMA21_55CrossScalp() *EMACrossV2 { return newEMACrossV2("EMA_21_55_Cross_Scalp", 21, 55,  25, 44, 72, 0.20, 0.48) }
func NewEMA5_20CrossScalp()  *EMACrossV2 { return newEMACrossV2("EMA_5_20_Cross_Scalp",   5, 20,  20, 46, 70, 0.17, 0.40) }
func NewEMA8_34CrossScalp()  *EMACrossV2 { return newEMACrossV2("EMA_8_34_Cross_Scalp",   8, 34,  22, 45, 70, 0.18, 0.42) }
func NewEMA3_15CrossScalp()  *EMACrossV2 { return newEMACrossV2("EMA_3_15_Cross_Scalp",   3, 15,  18, 48, 68, 0.16, 0.38) }
func NewEMA7_21CrossScalp()  *EMACrossV2 { return newEMACrossV2("EMA_7_21_Cross_Scalp",   7, 21,  20, 46, 70, 0.17, 0.40) }
func NewEMA12_26CrossScalp() *EMACrossV2 { return newEMACrossV2("EMA_12_26_Cross_Scalp", 12, 26,  22, 45, 70, 0.18, 0.42) }
func NewEMA20_50CrossScalp() *EMACrossV2 { return newEMACrossV2("EMA_20_50_Cross_Scalp", 20, 50,  25, 44, 72, 0.20, 0.48) }
func NewEMA4_12CrossScalp()  *EMACrossV2 { return newEMACrossV2("EMA_4_12_Cross_Scalp",   4, 12,  18, 47, 69, 0.16, 0.38) }
func NewEMA6_18CrossScalp()  *EMACrossV2 { return newEMACrossV2("EMA_6_18_Cross_Scalp",   6, 18,  20, 46, 70, 0.17, 0.40) }
func NewEMA15_45CrossScalp() *EMACrossV2 { return newEMACrossV2("EMA_15_45_Cross_Scalp", 15, 45,  23, 44, 71, 0.19, 0.45) }
func NewEMA9_26CrossScalp()  *EMACrossV2 { return newEMACrossV2("EMA_9_26_Cross_Scalp",   9, 26,  22, 45, 70, 0.18, 0.42) }

// ── RSI threshold family (8) ───────────────────────────────────────────────
func NewRSIOversold30Scalp()  *RSIThresholdScalper { return newRSIThreshold("RSI_Oversold30_Scalp",  14, 32, 38, 15, 0.17, 0.40) }
func NewRSIOversold35Scalp()  *RSIThresholdScalper { return newRSIThreshold("RSI_Oversold35_Scalp",  14, 35, 42, 15, 0.17, 0.38) }
func NewRSICross50Scalp()     *RSIThresholdScalper { return newRSIThreshold("RSI_Cross50_Scalp",     14, 48, 52, 20, 0.18, 0.42) }
func NewRSICross55Scalp()     *RSIThresholdScalper { return newRSIThreshold("RSI_Cross55_Scalp",     14, 53, 57, 22, 0.18, 0.42) }
func NewRSIZoneBull60Scalp()  *RSIThresholdScalper { return newRSIThreshold("RSI_BullZone60_Scalp",  14, 58, 63, 22, 0.19, 0.44) }
func NewRSI9Cross50Scalp()    *RSIThresholdScalper { return newRSIThreshold("RSI9_Cross50_Scalp",     9, 48, 52, 18, 0.17, 0.40) }
func NewRSI7Oversold28Scalp() *RSIThresholdScalper { return newRSIThreshold("RSI7_Oversold28_Scalp",  7, 30, 36, 15, 0.16, 0.38) }
func NewRSI21BullScalp()      *RSIThresholdScalper { return newRSIThreshold("RSI21_Bull_Scalp",      21, 45, 52, 18, 0.20, 0.46) }

// ── RSI slope family (5) ───────────────────────────────────────────────────
func NewRSI14Slope5Scalp()  *RSISlopeScalper { return newRSISlopeScalper("RSI14_Slope5_Scalp",  14, 3, 5.0, 18, 0.18, 0.42) }
func NewRSI14Slope8Scalp()  *RSISlopeScalper { return newRSISlopeScalper("RSI14_Slope8_Scalp",  14, 2, 8.0, 20, 0.18, 0.42) }
func NewRSISlope3_3Scalp()  *RSISlopeScalper { return newRSISlopeScalper("RSI_Slope3_3_Scalp",  14, 3, 3.0, 15, 0.17, 0.40) }
func NewRSISlope5_10Scalp() *RSISlopeScalper { return newRSISlopeScalper("RSI_Slope5_10_Scalp", 14, 5, 10.0, 22, 0.19, 0.44) }
func NewRSI9Slope3_5Scalp() *RSISlopeScalper { return newRSISlopeScalper("RSI9_Slope3_5_Scalp",  9, 3, 5.0, 15, 0.16, 0.38) }

// ── Bollinger Band family (12) ─────────────────────────────────────────────
func NewBBBounce20_2Scalp()     *BBSignalScalper { return newBBScalper("BB_Bounce20_2_Scalp",   "bounce_lower", 20, 2.0, 0, 22, 28, 48, 0.18, 0.42) }
func NewBBBounce14_2Scalp()     *BBSignalScalper { return newBBScalper("BB_Bounce14_2_Scalp",   "bounce_lower", 14, 2.0, 0, 20, 28, 48, 0.17, 0.40) }
func NewBBBounce20_1p5Scalp()   *BBSignalScalper { return newBBScalper("BB_Bounce20_1p5_Scalp", "bounce_lower", 20, 1.5, 0, 18, 30, 50, 0.17, 0.40) }
func NewBBBounce30_2Scalp()     *BBSignalScalper { return newBBScalper("BB_Bounce30_2_Scalp",   "bounce_lower", 30, 2.0, 0, 22, 28, 48, 0.19, 0.44) }
func NewBBMidCross20Scalp()     *BBSignalScalper { return newBBScalper("BB_MidCross20_Scalp",   "mid_cross",    20, 2.0, 18, 0, 45, 70, 0.18, 0.42) }
func NewBBMidCross14Scalp()     *BBSignalScalper { return newBBScalper("BB_MidCross14_Scalp",   "mid_cross",    14, 2.0, 18, 0, 45, 70, 0.17, 0.40) }
func NewBBBreakout20_2Scalp()   *BBSignalScalper { return newBBScalper("BB_Breakout20_2_Scalp", "breakout",     20, 2.0, 22, 0, 52, 75, 0.20, 0.48) }
func NewBBBreakout20_2p5Scalp() *BBSignalScalper { return newBBScalper("BB_Break20_2p5_Scalp",  "breakout",     20, 2.5, 22, 0, 52, 78, 0.20, 0.50) }
func NewBBBreakout14_2Scalp()   *BBSignalScalper { return newBBScalper("BB_Break14_2_Scalp",    "breakout",     14, 2.0, 20, 0, 52, 75, 0.18, 0.45) }
func NewBBWidth20_2Scalp()      *BBWidthScalper  { return newBBWidth("BB_Width20_2_Scalp",      20, 2.0, 18, 44, 72, 0.18, 0.44) }
func NewBBWidth14_2Scalp()      *BBWidthScalper  { return newBBWidth("BB_Width14_2_Scalp",      14, 2.0, 16, 44, 72, 0.17, 0.42) }
func NewBBWidth30_2Scalp()      *BBWidthScalper  { return newBBWidth("BB_Width30_2_Scalp",      30, 2.0, 20, 44, 72, 0.19, 0.46) }

// ── VWAP family (10) ───────────────────────────────────────────────────────
func NewVWAPCross30Scalp()    *VWAPSignalScalper { return newVWAPScalper("VWAP_Cross30_Scalp",    "cross",     30, 0,    20, 45, 70, 0.18, 0.42) }
func NewVWAPCross50Scalp()    *VWAPSignalScalper { return newVWAPScalper("VWAP_Cross50_Scalp",    "cross",     50, 0,    22, 45, 70, 0.18, 0.42) }
func NewVWAPDev0p3Scalp()     *VWAPSignalScalper { return newVWAPScalper("VWAP_Dev0p3_Scalp",     "deviation", 30, 0.3,  15, 32, 55, 0.18, 0.42) }
func NewVWAPDev0p5Scalp()     *VWAPSignalScalper { return newVWAPScalper("VWAP_Dev0p5_Scalp",     "deviation", 40, 0.5,  15, 30, 52, 0.20, 0.46) }
func NewVWAPDev0p4Scalp()     *VWAPSignalScalper { return newVWAPScalper("VWAP_Dev0p4_Scalp",     "deviation", 35, 0.4,  15, 31, 53, 0.19, 0.44) }
func NewVWAPPullback30Scalp() *VWAPSignalScalper { return newVWAPScalper("VWAP_Pullback30_Scalp",  "pullback",  30, 0,    22, 45, 65, 0.18, 0.42) }
func NewVWAPPullback50Scalp() *VWAPSignalScalper { return newVWAPScalper("VWAP_Pullback50_Scalp",  "pullback",  50, 0,    22, 45, 65, 0.18, 0.42) }
func NewVWAPCross20Scalp()    *VWAPSignalScalper { return newVWAPScalper("VWAP_Cross20_Scalp",    "cross",     20, 0,    18, 46, 70, 0.17, 0.40) }
func NewVWAPDev0p2Scalp()     *VWAPSignalScalper { return newVWAPScalper("VWAP_Dev0p2_Scalp",     "deviation", 25, 0.2,  12, 35, 55, 0.17, 0.38) }
func NewVWAPPullback20Scalp() *VWAPSignalScalper { return newVWAPScalper("VWAP_Pullback20_Scalp",  "pullback",  20, 0,    20, 46, 65, 0.17, 0.40) }

// ── MACD family (10) ───────────────────────────────────────────────────────
func NewMACDCross5_13_3Scalp()  *MACDSignalScalperV2 { return newMACDScalperV2("MACD_Cross5_13_3_Scalp",  "cross",         5, 13, 3, 18, 45, 70, 0.18, 0.42) }
func NewMACDCross8_17_9Scalp()  *MACDSignalScalperV2 { return newMACDScalperV2("MACD_Cross8_17_9_Scalp",  "cross",         8, 17, 9, 20, 45, 70, 0.18, 0.42) }
func NewMACDCross12_26_9Scalp() *MACDSignalScalperV2 { return newMACDScalperV2("MACD_Cross12_26_9_Scalp", "cross",        12, 26, 9, 22, 45, 70, 0.20, 0.46) }
func NewMACDZero5_13Scalp()     *MACDSignalScalperV2 { return newMACDScalperV2("MACD_Zero5_13_Scalp",     "zero_cross",    5, 13, 3, 18, 45, 70, 0.18, 0.42) }
func NewMACDZero12_26Scalp()    *MACDSignalScalperV2 { return newMACDScalperV2("MACD_Zero12_26_Scalp",    "zero_cross",   12, 26, 9, 22, 45, 70, 0.20, 0.46) }
func NewMACDHistMom5_13Scalp()  *MACDSignalScalperV2 { return newMACDScalperV2("MACD_HistMom5_13_Scalp",  "hist_momentum", 5, 13, 3, 18, 47, 68, 0.17, 0.40) }
func NewMACDHistMom8_17Scalp()  *MACDSignalScalperV2 { return newMACDScalperV2("MACD_HistMom8_17_Scalp",  "hist_momentum", 8, 17, 9, 20, 47, 68, 0.18, 0.42) }
func NewMACDHistMom12_26Scalp() *MACDSignalScalperV2 { return newMACDScalperV2("MACD_HistMom12_26_Scalp", "hist_momentum",12, 26, 9, 22, 47, 68, 0.19, 0.44) }
func NewMACDCross3_10_3Scalp()  *MACDSignalScalperV2 { return newMACDScalperV2("MACD_Cross3_10_3_Scalp",  "cross",         3, 10, 3, 15, 46, 70, 0.16, 0.38) }
func NewMACDZero3_10Scalp()     *MACDSignalScalperV2 { return newMACDScalperV2("MACD_Zero3_10_Scalp",     "zero_cross",    3, 10, 3, 15, 46, 70, 0.16, 0.38) }

// ── Volume + Price family (8) ──────────────────────────────────────────────
func NewVolBreak1p5xScalp()     *VolumePriceScalper { return newVolumePrice("Vol_Break1p5x_Scalp",     "breakout",        20, 1.5, 20, 52, 72, 0.18, 0.44) }
func NewVolBreak2xScalp()       *VolumePriceScalper { return newVolumePrice("Vol_Break2x_Scalp",       "breakout",        20, 2.0, 22, 52, 72, 0.18, 0.44) }
func NewVolBreak2p5xScalp()     *VolumePriceScalper { return newVolumePrice("Vol_Break2p5x_Scalp",     "breakout",        20, 2.5, 22, 52, 74, 0.19, 0.46) }
func NewVolBreak3xScalp()       *VolumePriceScalper { return newVolumePrice("Vol_Break3x_Scalp",       "breakout",        20, 3.0, 22, 52, 74, 0.20, 0.48) }
func NewVolClimaxRev3xScalp()   *VolumePriceScalper { return newVolumePrice("Vol_ClimaxRev3x_Scalp",   "climax_reversal", 20, 3.0,  0, 28, 72, 0.18, 0.44) }
func NewVolClimaxRev4xScalp()   *VolumePriceScalper { return newVolumePrice("Vol_ClimaxRev4x_Scalp",   "climax_reversal", 20, 4.0,  0, 25, 75, 0.19, 0.46) }
func NewVolBreak10_1p5Scalp()   *VolumePriceScalper { return newVolumePrice("Vol_Break10_1p5_Scalp",   "breakout",        10, 1.5, 18, 52, 72, 0.17, 0.42) }
func NewVolBreak30_2Scalp()     *VolumePriceScalper { return newVolumePrice("Vol_Break30_2_Scalp",     "breakout",        30, 2.0, 22, 52, 72, 0.20, 0.48) }

// ── N-bar breakout family (10) ─────────────────────────────────────────────
func NewNBar3Break()  *NBarBreakoutScalper { return newNBarBreakout("NBar3_Break_Scalp",   3, 15, 52, 74, 0.16, 0.38) }
func NewNBar5Break()  *NBarBreakoutScalper { return newNBarBreakout("NBar5_Break_Scalp",   5, 18, 52, 74, 0.17, 0.40) }
func NewNBar7Break()  *NBarBreakoutScalper { return newNBarBreakout("NBar7_Break_Scalp",   7, 18, 52, 74, 0.17, 0.40) }
func NewNBar8Break()  *NBarBreakoutScalper { return newNBarBreakout("NBar8_Break_Scalp",   8, 20, 52, 74, 0.18, 0.42) }
func NewNBar10Break() *NBarBreakoutScalper { return newNBarBreakout("NBar10_Break_Scalp", 10, 20, 52, 74, 0.18, 0.42) }
func NewNBar12Break() *NBarBreakoutScalper { return newNBarBreakout("NBar12_Break_Scalp", 12, 22, 52, 74, 0.18, 0.44) }
func NewNBar15Break() *NBarBreakoutScalper { return newNBarBreakout("NBar15_Break_Scalp", 15, 22, 52, 74, 0.19, 0.44) }
func NewNBar20Break() *NBarBreakoutScalper { return newNBarBreakout("NBar20_Break_Scalp", 20, 22, 52, 74, 0.20, 0.46) }
func NewNBar25Break() *NBarBreakoutScalper { return newNBarBreakout("NBar25_Break_Scalp", 25, 25, 52, 74, 0.20, 0.48) }
func NewNBar30Break() *NBarBreakoutScalper { return newNBarBreakout("NBar30_Break_Scalp", 30, 25, 52, 74, 0.22, 0.50) }

// ── Triple EMA family (8) ──────────────────────────────────────────────────
func NewTriple3_8_21Scalp()   *TripleEMAScalperV2 { return newTripleEMAV2("Triple3_8_21_Scalp",    3,  8, 21, 18, 46, 70, 0.17, 0.40) }
func NewTriple5_13_34Scalp()  *TripleEMAScalperV2 { return newTripleEMAV2("Triple5_13_34_Scalp",   5, 13, 34, 20, 45, 70, 0.18, 0.42) }
func NewTriple8_21_55Scalp()  *TripleEMAScalperV2 { return newTripleEMAV2("Triple8_21_55_Scalp",   8, 21, 55, 22, 45, 70, 0.19, 0.44) }
func NewTriple10_30_60Scalp() *TripleEMAScalperV2 { return newTripleEMAV2("Triple10_30_60_Scalp", 10, 30, 60, 25, 44, 72, 0.20, 0.46) }
func NewTriple4_9_18Scalp()   *TripleEMAScalperV2 { return newTripleEMAV2("Triple4_9_18_Scalp",    4,  9, 18, 18, 46, 70, 0.17, 0.40) }
func NewTriple5_10_20Scalp()  *TripleEMAScalperV2 { return newTripleEMAV2("Triple5_10_20_Scalp",   5, 10, 20, 18, 46, 70, 0.17, 0.40) }
func NewTriple6_14_30Scalp()  *TripleEMAScalperV2 { return newTripleEMAV2("Triple6_14_30_Scalp",   6, 14, 30, 20, 45, 70, 0.18, 0.42) }
func NewTriple7_21_50Scalp()  *TripleEMAScalperV2 { return newTripleEMAV2("Triple7_21_50_Scalp",   7, 21, 50, 22, 45, 70, 0.19, 0.44) }

// ── CCI family (8) ─────────────────────────────────────────────────────────
func NewCCIZeroCross14Scalp() *CCISignalScalper { return newCCIScalper("CCI_ZeroCross14_Scalp",  "zero_cross",     14, 18, 45, 70, 0.18, 0.42) }
func NewCCIZeroCross20Scalp() *CCISignalScalper { return newCCIScalper("CCI_ZeroCross20_Scalp",  "zero_cross",     20, 18, 45, 70, 0.18, 0.42) }
func NewCCIZeroCross10Scalp() *CCISignalScalper { return newCCIScalper("CCI_ZeroCross10_Scalp",  "zero_cross",     10, 15, 46, 70, 0.17, 0.40) }
func NewCCIExtreme14Scalp()   *CCISignalScalper { return newCCIScalper("CCI_Extreme14_Scalp",    "extreme_bounce", 14, 15, 30, 72, 0.18, 0.42) }
func NewCCIExtreme20Scalp()   *CCISignalScalper { return newCCIScalper("CCI_Extreme20_Scalp",    "extreme_bounce", 20, 15, 30, 72, 0.19, 0.44) }
func NewCCIExtreme10Scalp()   *CCISignalScalper { return newCCIScalper("CCI_Extreme10_Scalp",    "extreme_bounce", 10, 12, 30, 72, 0.17, 0.40) }
func NewCCITrend14Scalp()     *CCISignalScalper { return newCCIScalper("CCI_Trend14_Scalp",      "trend",          14, 22, 50, 75, 0.18, 0.42) }
func NewCCITrend20Scalp()     *CCISignalScalper { return newCCIScalper("CCI_Trend20_Scalp",      "trend",          20, 22, 50, 75, 0.19, 0.44) }
