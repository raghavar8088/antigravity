package phase22f

import (
	"fmt"
	"math"
	"sort"
	"time"

	"antigravity-engine/internal/validation/phase22e"
)

// execMetrics is the ordered list of execution quality dimensions.
var execMetrics = []string{
	"Latency",
	"Slippage",
	"MissedEntry",
	"FillQuality",
	"TPOverride",
	"SignalAge",
}

// perfMetrics is the ordered list of performance metrics to correlate against.
var perfMetrics = []string{
	"ProfitFactor",
	"Sharpe",
	"Expectancy",
}

// CorrelateExecutionQuality computes Pearson correlations between execution
// quality dimensions (from execQuality records) and strategy performance metrics
// (derived from trades). Returns the full correlation report.
//
// If execQuality is nil or empty the function synthesises proxy exec metrics
// from trade duration and PnL consistency.
func CorrelateExecutionQuality(
	trades []phase22e.TradeRecord,
	execQuality []ExecQualityRecord,
) ExecutionCorrelationReport {
	rep := ExecutionCorrelationReport{GeneratedAt: time.Now().UTC()}

	if len(trades) == 0 {
		rep.Summary = "No trade data — execution correlation analysis skipped."
		return rep
	}

	// Build per-strategy performance map
	byStrat := GroupTradesByStrategy(trades)
	perfMap := buildPerfMap(byStrat)

	// Build per-strategy exec quality map
	execMap := buildExecMap(execQuality, byStrat)

	// We need parallel slices: one value per strategy per metric
	stratIDs := SortedStrategyIDs(byStrat)
	if len(stratIDs) < 3 {
		rep.Summary = "Fewer than 3 strategies — insufficient data for correlation analysis."
		return rep
	}

	// Collect series
	series := make(map[string][]float64) // metric → values per strategy
	for _, em := range execMetrics {
		series[em] = make([]float64, 0, len(stratIDs))
	}
	for _, pm := range perfMetrics {
		series[pm] = make([]float64, 0, len(stratIDs))
	}

	for _, id := range stratIDs {
		eq := execMap[id]
		pf := perfMap[id]
		series["Latency"] = append(series["Latency"], eq.AvgLatencyMs)
		series["Slippage"] = append(series["Slippage"], eq.AvgSlippageBps)
		series["MissedEntry"] = append(series["MissedEntry"], eq.MissedEntryRate)
		series["FillQuality"] = append(series["FillQuality"], eq.FillQuality)
		series["TPOverride"] = append(series["TPOverride"], eq.TPOverrideRate)
		series["SignalAge"] = append(series["SignalAge"], eq.AvgSignalAgeMs)
		series["ProfitFactor"] = append(series["ProfitFactor"], pf.ProfitFactor)
		series["Sharpe"] = append(series["Sharpe"], pf.Sharpe)
		series["Expectancy"] = append(series["Expectancy"], pf.Expectancy)
	}

	var entries []ExecCorrelationEntry
	for _, em := range execMetrics {
		for _, pm := range perfMetrics {
			r := pearsonR(series[em], series[pm])
			entry := ExecCorrelationEntry{
				ExecMetric:  em,
				PerfMetric:  pm,
				Correlation: r,
				Impact:      classifyImpact(em, r),
				Significance: classifySignificance(r, len(stratIDs)),
			}
			entries = append(entries, entry)
		}
	}

	// sort by |correlation| descending
	sort.Slice(entries, func(i, j int) bool {
		return math.Abs(entries[i].Correlation) > math.Abs(entries[j].Correlation)
	})

	rep.Entries = entries

	// top 5 by absolute correlation
	top := 5
	if len(entries) < top {
		top = len(entries)
	}
	rep.TopImpact = entries[:top]
	rep.Summary = buildExecCorrelationSummary(rep.TopImpact, len(stratIDs))
	return rep
}

// ── helpers ──────────────────────────────────────────────────────────────────

type stratPerf struct {
	ProfitFactor float64
	Sharpe       float64
	Expectancy   float64
}

func buildPerfMap(byStrat map[string][]phase22e.TradeRecord) map[string]stratPerf {
	m := make(map[string]stratPerf, len(byStrat))
	for id, ts := range byStrat {
		pf, sharpe, exp, _ := sampleMetrics(ts)
		m[id] = stratPerf{ProfitFactor: pf, Sharpe: sharpe, Expectancy: exp}
	}
	return m
}

func buildExecMap(recs []ExecQualityRecord, byStrat map[string][]phase22e.TradeRecord) map[string]ExecQualityRecord {
	m := make(map[string]ExecQualityRecord, len(byStrat))
	for _, r := range recs {
		m[r.StrategyID] = r
	}
	// synthesise proxy metrics for strategies without execintel data
	for id, ts := range byStrat {
		if _, ok := m[id]; ok {
			continue
		}
		m[id] = synthesiseExecMetrics(ts)
	}
	return m
}

func synthesiseExecMetrics(trades []phase22e.TradeRecord) ExecQualityRecord {
	if len(trades) == 0 {
		return ExecQualityRecord{}
	}
	// proxy: latency ∝ 1/hold_time, slippage ∝ price variation
	var holdSum, priceVar float64
	for _, t := range trades {
		holdSum += t.HoldMinutes
		if t.EntryPrice > 0 {
			priceVar += math.Abs(t.ExitPrice-t.EntryPrice) / t.EntryPrice * 10000
		}
	}
	n := float64(len(trades))
	avgHold := holdSum / n
	avgVar := priceVar / n
	return ExecQualityRecord{
		AvgLatencyMs:    math.Max(1, 1000/math.Max(1, avgHold)), // proxy
		AvgSlippageBps:  avgVar,
		MissedEntryRate: 0.05, // default 5%
		FillQuality:     80,   // default 80/100
		TPOverrideRate:  0.02, // default 2%
		AvgSignalAgeMs:  avgHold * 60 * 1000,
	}
}

func pearsonR(xs, ys []float64) float64 {
	n := len(xs)
	if n != len(ys) || n < 3 {
		return 0
	}
	mx, my := meanLocal(xs), meanLocal(ys)
	num, dx2, dy2 := 0.0, 0.0, 0.0
	for i := range xs {
		dx := xs[i] - mx
		dy := ys[i] - my
		num += dx * dy
		dx2 += dx * dx
		dy2 += dy * dy
	}
	denom := math.Sqrt(dx2 * dy2)
	if denom == 0 {
		return 0
	}
	r := num / denom
	if math.IsNaN(r) || math.IsInf(r, 0) {
		return 0
	}
	return r
}

func classifyImpact(execMetric string, r float64) string {
	// For Latency/Slippage/MissedEntry/SignalAge: negative correlation with perf = NEGATIVE impact
	// For FillQuality: positive correlation = POSITIVE impact
	inverse := execMetric == "FillQuality"
	if inverse {
		switch {
		case r > 0.3:
			return "POSITIVE"
		case r < -0.3:
			return "NEGATIVE"
		default:
			return "NEUTRAL"
		}
	}
	switch {
	case r < -0.3:
		return "NEGATIVE" // higher latency/slippage → worse performance
	case r > 0.3:
		return "POSITIVE"
	default:
		return "NEUTRAL"
	}
}

func classifySignificance(r float64, n int) string {
	// t-statistic approximation: t = r * sqrt(n-2) / sqrt(1-r²)
	if n < 3 {
		return "NOT_SIGNIFICANT"
	}
	absR := math.Abs(r)
	t := absR * math.Sqrt(float64(n-2)) / math.Sqrt(math.Max(1e-9, 1-absR*absR))
	switch {
	case t > 2.576: // p < 0.01
		return "SIGNIFICANT"
	case t > 1.645: // p < 0.10
		return "MARGINAL"
	default:
		return "NOT_SIGNIFICANT"
	}
}

func buildExecCorrelationSummary(top []ExecCorrelationEntry, nStrategies int) string {
	if len(top) == 0 {
		return "No significant execution–performance correlations detected."
	}
	strongest := top[0]
	return fmt.Sprintf(
		"Analysis across %d strategies. Strongest: %s↔%s r=%.3f (%s, %s). "+
			"Execution quality is %s to portfolio performance.",
		nStrategies,
		strongest.ExecMetric, strongest.PerfMetric, strongest.Correlation,
		strongest.Impact, strongest.Significance,
		impactNarrative(top),
	)
}

func impactNarrative(top []ExecCorrelationEntry) string {
	neg, pos := 0, 0
	for _, e := range top {
		if e.Impact == "NEGATIVE" {
			neg++
		} else if e.Impact == "POSITIVE" {
			pos++
		}
	}
	switch {
	case neg > pos:
		return "MATERIALLY DAMAGING"
	case pos > neg:
		return "MATERIALLY BENEFICIAL"
	default:
		return "MIXED"
	}
}
