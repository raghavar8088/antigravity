// Package regime implements the Phase 19E Regime Analysis Engine.
// It classifies market regimes, tracks regime persistence, analyses
// regime transition probabilities, and evaluates strategy performance
// by regime — a prerequisite for every promotion gate.
package regime

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ─── Regime Classification ────────────────────────────────────────────────────

// Regime is the market regime label.
type Regime string

const (
	RegimeTrendingBull  Regime = "TRENDING_BULL"
	RegimeTrendingBear  Regime = "TRENDING_BEAR"
	RegimeMeanReverting Regime = "MEAN_REVERTING"
	RegimeHighVol       Regime = "HIGH_VOLATILITY"
	RegimeLowVol        Regime = "LOW_VOLATILITY"
	RegimeBullMarket    Regime = "BULL_MARKET"
	RegimeBearMarket    Regime = "BEAR_MARKET"
	RegimeRiskOff       Regime = "RISK_OFF"
	RegimeRiskOn        Regime = "RISK_ON"
	RegimeUnknown       Regime = "UNKNOWN"
)

var AllRegimes = []Regime{
	RegimeTrendingBull, RegimeTrendingBear, RegimeMeanReverting,
	RegimeHighVol, RegimeLowVol, RegimeBullMarket, RegimeBearMarket,
	RegimeRiskOff, RegimeRiskOn,
}

// ─── Configuration ────────────────────────────────────────────────────────────

// Config holds regime detection thresholds.
type Config struct {
	// ADX thresholds
	ADXTrending     float64 // ADX > this → trending
	ADXRanging      float64 // ADX < this → ranging/mean-reverting

	// Volatility thresholds (ATR as % of price)
	ATRHighVol      float64 // ATR% > this → high volatility
	ATRLowVol       float64 // ATR% < this → low volatility

	// Trend thresholds
	EMASlopeBull    float64 // positive EMA slope threshold
	EMASlopeBear    float64 // negative EMA slope threshold

	// Persistence: regime must last N bars before switching
	MinPersistBars  int

	// Risk-off indicators
	FundingRateRiskOff float64 // funding rate < this (bps) → risk-off
}

// DefaultConfig returns institutional-grade regime detection thresholds.
func DefaultConfig() Config {
	return Config{
		ADXTrending:        25.0,
		ADXRanging:         20.0,
		ATRHighVol:         0.025, // 2.5% ATR/price
		ATRLowVol:          0.008, // 0.8% ATR/price
		EMASlopeBull:       0.001,
		EMASlopeBear:       -0.001,
		MinPersistBars:     5,
		FundingRateRiskOff: -0.01,
	}
}

// ─── Input/Output ─────────────────────────────────────────────────────────────

// Bar is a market bar with the required indicators for regime detection.
type Bar struct {
	Time        time.Time
	Close       float64
	High        float64
	Low         float64
	Volume      float64
	ADX         float64 // pre-computed ADX (14-period)
	ATR         float64 // pre-computed ATR (14-period)
	EMA50       float64 // 50-period EMA
	EMA200      float64 // 200-period EMA
	FundingRate float64 // latest 8h funding rate (fractional)
}

// RegimeState captures regime classification at a single point in time.
type RegimeState struct {
	Regime      Regime
	Confidence  float64   // 0–1: certainty of classification
	ADX         float64
	ATRPct      float64
	EMASlope    float64
	Persistence int       // bars since last regime change
	ClassifiedAt time.Time
}

// RegimePeriod is a contiguous run of the same regime.
type RegimePeriod struct {
	Regime     Regime
	Start      time.Time
	End        time.Time
	Bars       int
	AvgADX     float64
	AvgATRPct  float64
	PnLInPeriod float64 // strategy PnL during this period (filled by performance analysis)
}

// TransitionMatrix captures regime-to-regime transition probabilities.
type TransitionMatrix map[Regime]map[Regime]float64

// RegimePerformance captures strategy performance metrics per regime.
type RegimePerformance struct {
	Regime       Regime
	TotalTrades  int
	WinRate      float64
	AvgPnLUSD    float64
	SharpeRatio  float64
	MaxDrawdown  float64
	TotalPnLUSD  float64
	BestFor      bool // true if this is a favourable regime for the strategy
}

// AnalysisReport is the complete regime analysis output.
type AnalysisReport struct {
	Symbol            string
	Periods           []RegimePeriod
	RegimeCoverage    map[Regime]float64       // fraction of bars in each regime
	TransitionMatrix  TransitionMatrix
	Performance       map[Regime]RegimePerformance
	DominantRegime    Regime
	MostProfitable    Regime
	LeastProfitable   Regime
	RegimeStability   float64 // 0–1: mean period length / total bars
}

// ─── Regime Engine ────────────────────────────────────────────────────────────

// Engine classifies market regimes from bar data.
type Engine struct {
	cfg Config
}

// NewEngine creates a regime engine with the given configuration.
func NewEngine(cfg Config) *Engine {
	return &Engine{cfg: cfg}
}

// Classify classifies a single bar given its indicators.
func (e *Engine) Classify(bar Bar) RegimeState {
	state := RegimeState{
		ADX:          bar.ADX,
		ClassifiedAt: bar.Time,
	}

	if bar.Close > 0 {
		state.ATRPct = bar.ATR / bar.Close
	}
	if bar.EMA200 > 0 {
		state.EMASlope = (bar.EMA50 - bar.EMA200) / bar.EMA200
	}

	regime, confidence := e.classifyFromIndicators(state.ADX, state.ATRPct, state.EMASlope, bar.FundingRate, bar.EMA50, bar.EMA200)
	state.Regime = regime
	state.Confidence = confidence
	return state
}

func (e *Engine) classifyFromIndicators(adx, atrPct, emaSlope, funding, ema50, ema200 float64) (Regime, float64) {
	cfg := e.cfg

	// Risk-off takes precedence.
	if funding < cfg.FundingRateRiskOff {
		return RegimeRiskOff, 0.85
	}

	// High volatility override.
	if atrPct > cfg.ATRHighVol {
		if emaSlope < cfg.EMASlopeBear {
			return RegimeBearMarket, 0.80
		}
		return RegimeHighVol, 0.75
	}

	// Trending regime.
	if adx > cfg.ADXTrending {
		if emaSlope > cfg.EMASlopeBull {
			return RegimeTrendingBull, 0.82
		}
		if emaSlope < cfg.EMASlopeBear {
			return RegimeTrendingBear, 0.82
		}
	}

	// Ranging / mean-reverting.
	if adx < cfg.ADXRanging {
		if atrPct < cfg.ATRLowVol {
			return RegimeLowVol, 0.78
		}
		return RegimeMeanReverting, 0.70
	}

	// Broad market direction from EMA.
	if ema50 > 0 && ema200 > 0 {
		if ema50 > ema200 {
			if funding > 0.005 { // positive funding = risk-on
				return RegimeRiskOn, 0.68
			}
			return RegimeBullMarket, 0.65
		}
		return RegimeBearMarket, 0.65
	}

	return RegimeUnknown, 0.40
}

// ClassifyAll classifies a full bar series and returns a regime state per bar.
func (e *Engine) ClassifyAll(bars []Bar) []RegimeState {
	states := make([]RegimeState, len(bars))
	for i, bar := range bars {
		states[i] = e.Classify(bar)
		if i > 0 && states[i].Regime == states[i-1].Regime {
			states[i].Persistence = states[i-1].Persistence + 1
		} else {
			states[i].Persistence = 1
		}
	}
	return states
}

// ExtractPeriods converts a sequence of regime states into contiguous regime periods.
func ExtractPeriods(bars []Bar, states []RegimeState) []RegimePeriod {
	if len(states) == 0 {
		return nil
	}
	var periods []RegimePeriod
	current := RegimePeriod{
		Regime: states[0].Regime,
		Start:  states[0].ClassifiedAt,
	}
	adxSum, atrSum := 0.0, 0.0

	for i, s := range states {
		if s.Regime != current.Regime {
			current.End = states[i-1].ClassifiedAt
			if current.Bars > 0 {
				current.AvgADX = adxSum / float64(current.Bars)
				current.AvgATRPct = atrSum / float64(current.Bars)
			}
			periods = append(periods, current)
			current = RegimePeriod{
				Regime: s.Regime,
				Start:  s.ClassifiedAt,
			}
			adxSum, atrSum = 0, 0
		}
		current.Bars++
		adxSum += s.ADX
		atrSum += s.ATRPct
	}
	// Close last period.
	if current.Bars > 0 {
		current.End = states[len(states)-1].ClassifiedAt
		current.AvgADX = adxSum / float64(current.Bars)
		current.AvgATRPct = atrSum / float64(current.Bars)
		periods = append(periods, current)
	}
	return periods
}

// BuildTransitionMatrix computes regime-to-regime transition probabilities.
func BuildTransitionMatrix(periods []RegimePeriod) TransitionMatrix {
	matrix := make(TransitionMatrix)
	counts := make(map[Regime]map[Regime]int)

	for i := 1; i < len(periods); i++ {
		from := periods[i-1].Regime
		to := periods[i].Regime
		if counts[from] == nil {
			counts[from] = make(map[Regime]int)
		}
		counts[from][to]++
	}

	for from, tos := range counts {
		total := 0
		for _, c := range tos {
			total += c
		}
		matrix[from] = make(map[Regime]float64)
		for to, c := range tos {
			matrix[from][to] = float64(c) / float64(total)
		}
	}
	return matrix
}

// Trade is a single strategy trade for performance-by-regime analysis.
type Trade struct {
	EntryTime time.Time
	ExitTime  time.Time
	PnLUSD    float64
}

// AnalysePerformanceByRegime maps each trade to its regime and computes
// per-regime performance statistics.
func AnalysePerformanceByRegime(trades []Trade, periods []RegimePeriod) map[Regime]RegimePerformance {
	// Build a quick lookup: for each trade entry time, find the regime.
	tradeRegimes := make(map[int]Regime, len(trades))
	for i, t := range trades {
		tradeRegimes[i] = regimeAtTime(t.EntryTime, periods)
	}

	type stats struct {
		trades []float64
	}
	byRegime := make(map[Regime]*stats)
	for _, r := range AllRegimes {
		byRegime[r] = &stats{}
	}
	byRegime[RegimeUnknown] = &stats{}

	for i, t := range trades {
		r := tradeRegimes[i]
		byRegime[r].trades = append(byRegime[r].trades, t.PnLUSD)
	}

	perf := make(map[Regime]RegimePerformance)
	for r, s := range byRegime {
		if len(s.trades) == 0 {
			continue
		}
		p := computeRegimePerf(r, s.trades)
		perf[r] = p
	}
	return perf
}

func regimeAtTime(t time.Time, periods []RegimePeriod) Regime {
	for _, p := range periods {
		if !t.Before(p.Start) && !t.After(p.End) {
			return p.Regime
		}
	}
	return RegimeUnknown
}

func computeRegimePerf(r Regime, pnls []float64) RegimePerformance {
	wins := 0
	total := 0.0
	grossProfit, grossLoss := 0.0, 0.0
	for _, p := range pnls {
		total += p
		if p > 0 {
			wins++
			grossProfit += p
		} else {
			grossLoss -= p
		}
	}
	mean := total / float64(len(pnls))
	variance := 0.0
	for _, p := range pnls {
		d := p - mean
		variance += d * d
	}
	std := math.Sqrt(variance / float64(len(pnls)))
	sharpe := 0.0
	if std > 0 {
		sharpe = (mean / std) * math.Sqrt(252)
	}
	// Max drawdown
	equity, peak, maxDD := 0.0, 0.0, 0.0
	for _, p := range pnls {
		equity += p
		if equity > peak {
			peak = equity
		}
		if peak > 0 {
			dd := (peak - equity) / peak
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	pf := 0.0
	if grossLoss > 0 {
		pf = grossProfit / grossLoss
	}
	_ = pf
	return RegimePerformance{
		Regime:      r,
		TotalTrades: len(pnls),
		WinRate:     float64(wins) / float64(len(pnls)),
		AvgPnLUSD:   mean,
		SharpeRatio: sharpe,
		MaxDrawdown: maxDD * 100,
		TotalPnLUSD: total,
		BestFor:     sharpe > 0.5 && float64(wins)/float64(len(pnls)) > 0.5,
	}
}

// Analyse runs a full regime analysis over bars and strategy trades.
func (e *Engine) Analyse(symbol string, bars []Bar, trades []Trade) AnalysisReport {
	states := e.ClassifyAll(bars)
	periods := ExtractPeriods(bars, states)
	matrix := BuildTransitionMatrix(periods)
	perf := AnalysePerformanceByRegime(trades, periods)

	// Regime coverage: fraction of bars.
	coverage := make(map[Regime]float64)
	for _, s := range states {
		coverage[s.Regime]++
	}
	total := float64(len(states))
	for r := range coverage {
		coverage[r] /= total
	}

	// Dominant regime.
	dominant := RegimeUnknown
	maxBars := 0.0
	for r, f := range coverage {
		if f > maxBars {
			maxBars = f
			dominant = r
		}
	}

	// Most/least profitable regime.
	mostProfit, leastProfit := RegimeUnknown, RegimeUnknown
	bestSharpe := math.Inf(-1)
	worstSharpe := math.Inf(1)
	for r, p := range perf {
		if p.TotalTrades >= 5 {
			if p.SharpeRatio > bestSharpe {
				bestSharpe = p.SharpeRatio
				mostProfit = r
			}
			if p.SharpeRatio < worstSharpe {
				worstSharpe = p.SharpeRatio
				leastProfit = r
			}
		}
	}

	// Regime stability: mean period length / total bars.
	totalBars := 0
	for _, p := range periods {
		totalBars += p.Bars
	}
	stability := 0.0
	if len(periods) > 0 && totalBars > 0 {
		meanPeriodLen := float64(totalBars) / float64(len(periods))
		stability = meanPeriodLen / float64(totalBars)
	}

	return AnalysisReport{
		Symbol:          symbol,
		Periods:         periods,
		RegimeCoverage:  coverage,
		TransitionMatrix: matrix,
		Performance:     perf,
		DominantRegime:  dominant,
		MostProfitable:  mostProfit,
		LeastProfitable: leastProfit,
		RegimeStability: stability,
	}
}

// ─── Regime-Specific Performance Summary ─────────────────────────────────────

// FavourableRegimes returns all regimes where the strategy is profitable
// (Sharpe > threshold and win rate > threshold).
func FavourableRegimes(perf map[Regime]RegimePerformance, minSharpe, minWinRate float64) []Regime {
	var out []Regime
	for r, p := range perf {
		if p.TotalTrades >= 5 && p.SharpeRatio >= minSharpe && p.WinRate >= minWinRate {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return perf[out[i]].SharpeRatio > perf[out[j]].SharpeRatio
	})
	return out
}

// PrettyRegime returns a human-readable regime label.
func PrettyRegime(r Regime) string {
	return fmt.Sprintf("[%s]", r)
}
