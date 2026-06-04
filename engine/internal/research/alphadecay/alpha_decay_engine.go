// Package alphadecay implements the Phase 19I Alpha Decay Monitoring Engine.
// Detects signal deterioration, estimates half-life, and alerts before alpha
// fails — so strategies are retired or retrained before real losses occur.
package alphadecay

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ─── Decay States ─────────────────────────────────────────────────────────────

type DecayState string

const (
	DecayHealthy    DecayState = "HEALTHY"
	DecayWarning    DecayState = "WARNING"
	DecayCritical   DecayState = "CRITICAL"
	DecayExpired    DecayState = "EXPIRED"
)

// ─── Configuration ────────────────────────────────────────────────────────────

type Config struct {
	// IC thresholds (Information Coefficient)
	ICWarningThreshold  float64 // IC drops below this → WARNING
	ICCriticalThreshold float64 // IC drops below this → CRITICAL

	// Decay thresholds as % of baseline IC
	DecayWarningPct  float64 // e.g. 0.30 = 30% decay → WARNING
	DecayCriticalPct float64 // e.g. 0.50 = 50% decay → CRITICAL

	// Half-life thresholds (days)
	HalfLifeWarningDays  float64 // half-life below this → WARNING
	HalfLifeCriticalDays float64 // half-life below this → CRITICAL

	// Lookback windows
	ShortWindowDays int // short IC window (recent performance)
	LongWindowDays  int // long IC window (baseline)

	// Minimum observations before decay analysis
	MinObservations int
}

func DefaultConfig() Config {
	return Config{
		ICWarningThreshold:   0.05,
		ICCriticalThreshold:  0.02,
		DecayWarningPct:      0.30,
		DecayCriticalPct:     0.50,
		HalfLifeWarningDays:  30,
		HalfLifeCriticalDays: 14,
		ShortWindowDays:      21,
		LongWindowDays:       63,
		MinObservations:      20,
	}
}

// ─── Signal Observation ───────────────────────────────────────────────────────

// Observation is a single (signal, realised_return) data point.
type Observation struct {
	Timestamp       time.Time
	SignalValue     float64 // predicted direction or strength
	RealisedReturn  float64 // actual return in the holding period
	HoldingPeriod   time.Duration
	Regime          string
}

// ─── IC Calculation ───────────────────────────────────────────────────────────

// computeIC calculates the Information Coefficient (Spearman rank correlation)
// between signal values and realised returns for a window of observations.
func computeIC(obs []Observation) float64 {
	n := len(obs)
	if n < 5 {
		return 0
	}

	// Rank signal values and returns independently.
	signalRanks := rankData(mapSlice(obs, func(o Observation) float64 { return o.SignalValue }))
	returnRanks := rankData(mapSlice(obs, func(o Observation) float64 { return o.RealisedReturn }))

	// Spearman correlation of ranks.
	return pearsonCorr(signalRanks, returnRanks)
}

func pearsonCorr(a, b []float64) float64 {
	n := len(a)
	if n == 0 {
		return 0
	}
	meanA, meanB := mean(a), mean(b)
	num, varA, varB := 0.0, 0.0, 0.0
	for i := range a {
		da, db := a[i]-meanA, b[i]-meanB
		num += da * db
		varA += da * da
		varB += db * db
	}
	denom := math.Sqrt(varA * varB)
	if denom == 0 {
		return 0
	}
	return num / denom
}

func rankData(data []float64) []float64 {
	type pair struct{ val float64; idx int }
	pairs := make([]pair, len(data))
	for i, v := range data {
		pairs[i] = pair{v, i}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].val < pairs[j].val })
	ranks := make([]float64, len(data))
	for rank, p := range pairs {
		ranks[p.idx] = float64(rank + 1)
	}
	return ranks
}

func mapSlice(obs []Observation, fn func(Observation) float64) []float64 {
	out := make([]float64, len(obs))
	for i, o := range obs {
		out[i] = fn(o)
	}
	return out
}

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range data {
		s += v
	}
	return s / float64(len(data))
}

// ─── Half-Life Estimation ─────────────────────────────────────────────────────

// estimateHalfLife fits an exponential decay curve to rolling IC values and
// returns the estimated half-life in days.
//
// IC(t) ≈ IC₀ × exp(-λt)  →  half_life = ln(2) / λ
func estimateHalfLife(rollingIC []float64, intervalDays float64) float64 {
	n := len(rollingIC)
	if n < 5 {
		return math.Inf(1)
	}

	// Remove non-positive IC values for log-linear fit.
	var x, y []float64
	for i, ic := range rollingIC {
		if ic > 0 {
			x = append(x, float64(i)*intervalDays)
			y = append(y, math.Log(ic))
		}
	}
	if len(x) < 3 {
		return math.Inf(1)
	}

	// Ordinary Least Squares on ln(IC) ~ a + b*t.
	slope := oLSSlope(x, y)
	if slope >= 0 {
		return math.Inf(1) // IC is not decaying
	}
	return math.Log(2) / (-slope) // half-life in days
}

func oLSSlope(x, y []float64) float64 {
	n := float64(len(x))
	if n == 0 {
		return 0
	}
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}

// ─── Regime Breakdown Detection ───────────────────────────────────────────────

// RegimeBreakdown captures alpha performance disaggregated by market regime.
type RegimeBreakdown struct {
	Regime     string
	IC         float64
	TradeCount int
	Decaying   bool
}

func analyseByRegime(obs []Observation) []RegimeBreakdown {
	byRegime := make(map[string][]Observation)
	for _, o := range obs {
		if o.Regime != "" {
			byRegime[o.Regime] = append(byRegime[o.Regime], o)
		}
	}
	var out []RegimeBreakdown
	for regime, regObs := range byRegime {
		ic := computeIC(regObs)
		out = append(out, RegimeBreakdown{
			Regime:     regime,
			IC:         ic,
			TradeCount: len(regObs),
			Decaying:   ic < 0.03,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].IC > out[j].IC
	})
	return out
}

// ─── Alpha Decay Result ───────────────────────────────────────────────────────

// DecayResult is the output of a single decay analysis run.
type DecayResult struct {
	StrategyID      string
	State           DecayState
	CurrentIC       float64
	BaselineIC      float64
	ShortWindowIC   float64
	LongWindowIC    float64
	DecayPct        float64   // (baseline - current) / baseline
	HalfLifeDays    float64
	ParameterDrift  float64   // 0–1: how much optimal params have shifted
	RegimeBreakdown []RegimeBreakdown
	DominantDecayRegime string
	Alert           string
	AnalysedAt      time.Time
}

// ─── Alpha Decay Engine ───────────────────────────────────────────────────────

// Engine monitors alpha decay for a single strategy.
type Engine struct {
	cfg         Config
	strategyID  string
	observations []Observation
}

// NewEngine creates an alpha decay engine for the given strategy.
func NewEngine(strategyID string, cfg Config) *Engine {
	return &Engine{strategyID: strategyID, cfg: cfg}
}

// AddObservation records a new signal → realised return data point.
func (e *Engine) AddObservation(obs Observation) {
	e.observations = append(e.observations, obs)
	// Keep only last 2×LongWindowDays observations to bound memory.
	maxObs := e.cfg.LongWindowDays * 3
	if len(e.observations) > maxObs {
		e.observations = e.observations[len(e.observations)-maxObs:]
	}
}

// Analyse runs the full alpha decay analysis and returns the current state.
func (e *Engine) Analyse() DecayResult {
	obs := e.observations
	result := DecayResult{
		StrategyID: e.strategyID,
		State:      DecayHealthy,
		AnalysedAt: time.Now().UTC(),
	}

	if len(obs) < e.cfg.MinObservations {
		result.Alert = fmt.Sprintf("insufficient observations (%d < %d)", len(obs), e.cfg.MinObservations)
		return result
	}

	// Sort by timestamp.
	sorted := make([]Observation, len(obs))
	copy(sorted, obs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	n := len(sorted)

	// Baseline IC: first LongWindowDays observations.
	longN := e.cfg.LongWindowDays
	if longN > n {
		longN = n / 2
	}
	baselineWindow := sorted[:longN]
	result.BaselineIC = computeIC(baselineWindow)
	result.LongWindowIC = computeIC(baselineWindow)

	// Short window IC: most recent ShortWindowDays observations.
	shortN := e.cfg.ShortWindowDays
	if shortN > n {
		shortN = n
	}
	recentWindow := sorted[n-shortN:]
	result.ShortWindowIC = computeIC(recentWindow)
	result.CurrentIC = result.ShortWindowIC

	// Decay percentage.
	if result.BaselineIC != 0 {
		result.DecayPct = (result.BaselineIC - result.CurrentIC) / math.Abs(result.BaselineIC)
	}

	// Rolling IC series for half-life estimation.
	rollingICs := e.computeRollingIC(sorted, e.cfg.ShortWindowDays)
	result.HalfLifeDays = estimateHalfLife(rollingICs, float64(e.cfg.ShortWindowDays)/float64(len(rollingICs)+1))

	// Regime breakdown.
	result.RegimeBreakdown = analyseByRegime(sorted)
	if len(result.RegimeBreakdown) > 0 {
		worstIC := math.Inf(1)
		for _, rb := range result.RegimeBreakdown {
			if rb.Decaying && rb.TradeCount >= 5 && rb.IC < worstIC {
				worstIC = rb.IC
				result.DominantDecayRegime = rb.Regime
			}
		}
	}

	// Classify decay state.
	result.State, result.Alert = e.classify(result)
	return result
}

func (e *Engine) classify(r DecayResult) (DecayState, string) {
	cfg := e.cfg

	// Expired: IC below critical AND half-life very short.
	if r.CurrentIC < cfg.ICCriticalThreshold && r.HalfLifeDays < float64(cfg.HalfLifeCriticalDays) {
		return DecayExpired, fmt.Sprintf(
			"ALPHA EXPIRED: IC=%.4f < %.4f critical threshold, half-life=%.1fd",
			r.CurrentIC, cfg.ICCriticalThreshold, r.HalfLifeDays)
	}

	// Critical: IC below critical OR decay > 50%.
	if r.CurrentIC < cfg.ICCriticalThreshold || r.DecayPct > cfg.DecayCriticalPct {
		return DecayCritical, fmt.Sprintf(
			"CRITICAL DECAY: IC=%.4f, decay=%.1f%%, half-life=%.1fd",
			r.CurrentIC, r.DecayPct*100, r.HalfLifeDays)
	}

	// Warning: IC below warning threshold OR decay > 30%.
	if r.CurrentIC < cfg.ICWarningThreshold || r.DecayPct > cfg.DecayWarningPct {
		return DecayWarning, fmt.Sprintf(
			"DECAY WARNING: IC=%.4f (baseline=%.4f), decay=%.1f%%, half-life=%.1fd",
			r.CurrentIC, r.BaselineIC, r.DecayPct*100, r.HalfLifeDays)
	}

	// Healthy.
	return DecayHealthy, fmt.Sprintf(
		"healthy: IC=%.4f, half-life=%.1fd", r.CurrentIC, r.HalfLifeDays)
}

// computeRollingIC returns IC values for overlapping windows across the full series.
func (e *Engine) computeRollingIC(sorted []Observation, windowSize int) []float64 {
	if len(sorted) < windowSize {
		return []float64{computeIC(sorted)}
	}
	var ics []float64
	step := windowSize / 3
	if step < 1 {
		step = 1
	}
	for start := 0; start+windowSize <= len(sorted); start += step {
		ic := computeIC(sorted[start : start+windowSize])
		ics = append(ics, ic)
	}
	return ics
}

// ─── Multi-Strategy Monitor ───────────────────────────────────────────────────

// Monitor manages alpha decay engines for multiple strategies.
type Monitor struct {
	engines map[string]*Engine
	cfg     Config
}

// NewMonitor creates a multi-strategy alpha decay monitor.
func NewMonitor(cfg Config) *Monitor {
	return &Monitor{engines: make(map[string]*Engine), cfg: cfg}
}

// AddObservation routes an observation to the correct strategy engine.
func (m *Monitor) AddObservation(strategyID string, obs Observation) {
	if _, ok := m.engines[strategyID]; !ok {
		m.engines[strategyID] = NewEngine(strategyID, m.cfg)
	}
	m.engines[strategyID].AddObservation(obs)
}

// AnalyseAll runs decay analysis on all monitored strategies and returns results.
func (m *Monitor) AnalyseAll() []DecayResult {
	results := make([]DecayResult, 0, len(m.engines))
	for _, eng := range m.engines {
		results = append(results, eng.Analyse())
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CurrentIC < results[j].CurrentIC
	})
	return results
}

// AlertedStrategies returns all strategies with WARNING or worse decay state.
func (m *Monitor) AlertedStrategies() []DecayResult {
	all := m.AnalyseAll()
	var alerted []DecayResult
	for _, r := range all {
		if r.State == DecayWarning || r.State == DecayCritical || r.State == DecayExpired {
			alerted = append(alerted, r)
		}
	}
	return alerted
}
