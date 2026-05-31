// Package regime provides deterministic market regime classification.
//
// Regime detection replaces the Macro Analyst AI agent that previously ran
// an LLM call on every signal.  All decisions are rule-based, sub-microsecond,
// and produce identical output for identical inputs.
//
// Regime is computed from 5 orthogonal dimensions:
//  1. Trend strength   — ADX
//  2. Volatility       — normalised ATR
//  3. Directional bias — EMA-slope sign
//  4. Mean deviation   — VWAP-distance
//  5. Volume context   — volume-ratio vs N-bar average
package regime

import (
	"math"
	"sync"
	"time"
)

// ── Regime constants ──────────────────────────────────────────────────────────

type Regime string

const (
	RegimeTrendingBull Regime = "TRENDING_BULL"
	RegimeTrendingBear Regime = "TRENDING_BEAR"
	RegimeRanging      Regime = "RANGING"
	RegimeHighVol      Regime = "HIGH_VOL"
	RegimeLowVol       Regime = "LOW_VOL"
	RegimeUnknown      Regime = "UNKNOWN"
)

// ── RegimeState ───────────────────────────────────────────────────────────────

// RegimeState is the output of one classification cycle.
type RegimeState struct {
	Regime       Regime
	ADX          float64
	ATR          float64
	ATRPct       float64 // ATR as % of price
	EMASlope     float64 // (EMAFast − EMASlow) / price
	VWAPDist     float64 // (price − VWAP) / price × 100
	VolumeRatio  float64 // current volume / N-bar average
	ComputedAt   time.Time
	BarCount     int64 // number of candles processed since startup
}

// ── Engine config ──────────────────────────────────────────────────────────────

// Config controls classification thresholds. All values have production defaults.
type Config struct {
	// ADX thresholds
	ADXTrending float64 // ADX above this → trending (default 22)
	ADXRanging  float64 // ADX below this → ranging  (default 18)

	// ATR-as-% thresholds
	ATRHighVol float64 // ATRPct above this → HIGH_VOL  (default 0.40)
	ATRLowVol  float64 // ATRPct below this → LOW_VOL   (default 0.08)

	// Indicator periods
	EMAFastPeriod int // default 9
	EMASlowPeriod int // default 21
	ADXPeriod     int // default 14
	ATRPeriod     int // default 14
	VWAPPeriod    int // default 50 — bars used for rolling VWAP
	VolAvgPeriod  int // default 20 — bars for volume average
}

func DefaultConfig() Config {
	return Config{
		ADXTrending:   22,
		ADXRanging:    18,
		ATRHighVol:    0.40,
		ATRLowVol:     0.08,
		EMAFastPeriod: 9,
		EMASlowPeriod: 21,
		ADXPeriod:     14,
		ATRPeriod:     14,
		VWAPPeriod:    50,
		VolAvgPeriod:  20,
	}
}

// ── Engine ────────────────────────────────────────────────────────────────────

// Engine maintains rolling indicator state and classifies market regime
// from closed candle data.  All methods are safe for concurrent use.
type Engine struct {
	mu  sync.RWMutex
	cfg Config

	// Indicator state
	emaFast    float64
	emaSlow    float64
	prevClose  float64
	atr        float64
	adx        float64
	plusDI     float64
	minusDI    float64
	smoothedTR float64
	smoothedPD float64
	smoothedMD float64

	// VWAP accumulator
	vwapTPV   float64 // Σ(typical_price × volume)
	vwapVol   float64 // Σ volume
	vwapBars  int

	// Volume average (ring buffer)
	volBuf    []float64
	volBufIdx int

	// History for DX smoothing
	dxHistory []float64

	barCount int64
	current  RegimeState
}

// NewEngine creates a regime engine with the given config.
func NewEngine(cfg Config) *Engine {
	return &Engine{
		cfg:       cfg,
		volBuf:    make([]float64, cfg.VolAvgPeriod),
		dxHistory: make([]float64, 0, cfg.ADXPeriod),
	}
}

// NewEngineDefault creates a regime engine with production defaults.
func NewEngineDefault() *Engine {
	return NewEngine(DefaultConfig())
}

// ── Public API ─────────────────────────────────────────────────────────────────

// CandleInput is one closed OHLCV bar.
type CandleInput struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// Update feeds one closed candle into the engine and returns the new regime.
// Call this on every closed 1m candle.
func (e *Engine) Update(c CandleInput) RegimeState {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.barCount++
	price := c.Close

	// ── EMA fast/slow ──
	k9 := 2.0 / float64(e.cfg.EMAFastPeriod+1)
	k21 := 2.0 / float64(e.cfg.EMASlowPeriod+1)
	if e.barCount == 1 {
		e.emaFast = price
		e.emaSlow = price
		e.prevClose = price
		e.atr = c.High - c.Low
	} else {
		e.emaFast = price*k9 + e.emaFast*(1-k9)
		e.emaSlow = price*k21 + e.emaSlow*(1-k21)
	}
	emaSlope := 0.0
	if price > 0 {
		emaSlope = (e.emaFast - e.emaSlow) / price * 100
	}

	// ── ATR (Wilder smoothing) ──
	tr := trueRange(c.High, c.Low, e.prevClose)
	kATR := 1.0 / float64(e.cfg.ATRPeriod)
	e.atr = e.atr*(1-kATR) + tr*kATR
	atrPct := 0.0
	if price > 0 {
		atrPct = e.atr / price * 100
	}

	// ── ADX / DI (Wilder) ──
	up := c.High - e.prevClose
	dn := e.prevClose - c.Low
	pdm := 0.0
	if up > 0 && up > dn {
		pdm = up
	}
	mdm := 0.0
	if dn > 0 && dn > up {
		mdm = dn
	}

	kDI := 1.0 / float64(e.cfg.ADXPeriod)
	if e.barCount < 3 {
		e.smoothedTR = tr
		e.smoothedPD = pdm
		e.smoothedMD = mdm
	} else {
		e.smoothedTR = e.smoothedTR*(1-kDI) + tr*kDI
		e.smoothedPD = e.smoothedPD*(1-kDI) + pdm*kDI
		e.smoothedMD = e.smoothedMD*(1-kDI) + mdm*kDI
	}
	if e.smoothedTR > 0 {
		e.plusDI = 100 * e.smoothedPD / e.smoothedTR
		e.minusDI = 100 * e.smoothedMD / e.smoothedTR
	}
	diSum := e.plusDI + e.minusDI
	dx := 0.0
	if diSum > 0 {
		dx = 100 * math.Abs(e.plusDI-e.minusDI) / diSum
	}
	e.dxHistory = append(e.dxHistory, dx)
	if len(e.dxHistory) > e.cfg.ADXPeriod {
		e.dxHistory = e.dxHistory[1:]
	}
	e.adx = mean(e.dxHistory)

	// ── Rolling VWAP ──
	tp := (c.High + c.Low + c.Close) / 3
	e.vwapTPV += tp * c.Volume
	e.vwapVol += c.Volume
	e.vwapBars++
	if e.vwapBars > e.cfg.VWAPPeriod {
		// Rolling: remove oldest approximation (weighted removal not exact
		// but sufficiently accurate for regime classification)
		e.vwapTPV *= float64(e.cfg.VWAPPeriod) / float64(e.vwapBars)
		e.vwapVol *= float64(e.cfg.VWAPPeriod) / float64(e.vwapBars)
		e.vwapBars = e.cfg.VWAPPeriod
	}
	vwap := 0.0
	if e.vwapVol > 0 {
		vwap = e.vwapTPV / e.vwapVol
	}
	vwapDist := 0.0
	if vwap > 0 {
		vwapDist = (price - vwap) / vwap * 100
	}

	// ── Volume ratio ──
	e.volBuf[e.volBufIdx%e.cfg.VolAvgPeriod] = c.Volume
	e.volBufIdx++
	avgVol := mean(e.volBuf)
	volRatio := 1.0
	if avgVol > 0 {
		volRatio = c.Volume / avgVol
	}

	e.prevClose = c.Close

	// ── Classification ──
	regime := classify(e.adx, atrPct, emaSlope, e.plusDI, e.minusDI, e.cfg, e.barCount)

	e.current = RegimeState{
		Regime:      regime,
		ADX:         e.adx,
		ATR:         e.atr,
		ATRPct:      atrPct,
		EMASlope:    emaSlope,
		VWAPDist:    vwapDist,
		VolumeRatio: volRatio,
		ComputedAt:  time.Now(),
		BarCount:    e.barCount,
	}

	return e.current
}

// Current returns the most recently computed regime without updating state.
func (e *Engine) Current() RegimeState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.current
}

// Reset clears all accumulated state (e.g. at session open).
func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.emaFast = 0
	e.emaSlow = 0
	e.prevClose = 0
	e.atr = 0
	e.adx = 0
	e.plusDI = 0
	e.minusDI = 0
	e.smoothedTR = 0
	e.smoothedPD = 0
	e.smoothedMD = 0
	e.vwapTPV = 0
	e.vwapVol = 0
	e.vwapBars = 0
	e.volBuf = make([]float64, e.cfg.VolAvgPeriod)
	e.volBufIdx = 0
	e.dxHistory = e.dxHistory[:0]
	e.barCount = 0
	e.current = RegimeState{}
}

// ── Classification logic ───────────────────────────────────────────────────────

func classify(adx, atrPct, emaSlope, plusDI, minusDI float64, cfg Config, barCount int64) Regime {
	// Need minimum warmup before committing to a regime
	if barCount < 30 {
		return RegimeUnknown
	}

	// Volatility dominates
	if atrPct > cfg.ATRHighVol {
		return RegimeHighVol
	}
	if atrPct < cfg.ATRLowVol {
		return RegimeLowVol
	}

	// Trend vs range based on ADX
	if adx >= cfg.ADXTrending {
		if emaSlope > 0 && plusDI > minusDI {
			return RegimeTrendingBull
		}
		if emaSlope < 0 && minusDI > plusDI {
			return RegimeTrendingBear
		}
		// ADX says trend but DI disagrees — cautious ranging
		return RegimeRanging
	}

	if adx < cfg.ADXRanging {
		return RegimeRanging
	}

	// ADX in transition zone (ADXRanging..ADXTrending)
	if emaSlope > 0.05 {
		return RegimeTrendingBull
	}
	if emaSlope < -0.05 {
		return RegimeTrendingBear
	}
	return RegimeRanging
}

// ── helpers ────────────────────────────────────────────────────────────────────

func trueRange(high, low, prevClose float64) float64 {
	return math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, v := range xs {
		s += v
	}
	return s / float64(len(xs))
}
