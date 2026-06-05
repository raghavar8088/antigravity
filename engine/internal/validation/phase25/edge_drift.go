package phase25

import (
	"fmt"
	"math"

	"antigravity-engine/internal/validation/phase24"
)

// BuildEdgeDrift compares Phase 24 historical metrics against live-forward metrics
// for each strategy and classifies edge direction.
func BuildEdgeDrift(
	eligible []EligibilityRecord,
	hist map[string]phase24.ExtendedMetrics,
	live map[string]LiveMetrics,
) []EdgeDriftRecord {
	records := make([]EdgeDriftRecord, 0, len(eligible))

	for _, el := range eligible {
		if !el.ApprovedForLive {
			continue
		}

		h := hist[el.StrategyName]
		l, hasLive := live[el.StrategyName]
		if !hasLive || l.TotalTrades == 0 {
			records = append(records, EdgeDriftRecord{
				StrategyName:         el.StrategyName,
				HistoricalPF:         h.ProfitFactor,
				HistoricalSharpe:     h.Sharpe,
				HistoricalSortino:    h.Sortino,
				HistoricalExpectancy: h.Expectancy,
				HistoricalDD:         h.MaxDrawdown,
				EdgeStatus:           EdgeFailed,
				Evidence:             "No live trades recorded in forward window",
			})
			continue
		}

		pfDrift := driftPct(h.ProfitFactor, l.ProfitFactor)
		sharpeDrift := driftPct(h.Sharpe, l.Sharpe)
		sortinoDrift := driftPct(h.Sortino, l.Sortino)
		expectancyDrift := driftPct(h.Expectancy, l.Expectancy)
		ddDrift := driftPct(h.MaxDrawdown, l.MaxDrawdown) * -1 // lower DD is improvement

		status := classifyDrift(pfDrift, sharpeDrift, expectancyDrift, l.ProfitFactor, l.Expectancy)

		records = append(records, EdgeDriftRecord{
			StrategyName:         el.StrategyName,
			HistoricalPF:         h.ProfitFactor,
			LivePF:               l.ProfitFactor,
			PFDriftPct:           pfDrift,
			HistoricalSharpe:     h.Sharpe,
			LiveSharpe:           l.Sharpe,
			SharpeDriftPct:       sharpeDrift,
			HistoricalSortino:    h.Sortino,
			LiveSortino:          l.Sortino,
			SortinoDriftPct:      sortinoDrift,
			HistoricalExpectancy: h.Expectancy,
			LiveExpectancy:       l.Expectancy,
			ExpectancyDriftPct:   expectancyDrift,
			HistoricalDD:         h.MaxDrawdown,
			LiveDD:               l.MaxDrawdown,
			DDDriftPct:           ddDrift,
			EdgeStatus:           status,
			Evidence:             buildDriftEvidence(h, l, pfDrift, sharpeDrift, status),
		})
	}
	return records
}

// driftPct returns the percentage change from baseline to live (positive = improvement).
func driftPct(baseline, live float64) float64 {
	if baseline == 0 {
		return 0
	}
	return (live - baseline) / math.Abs(baseline) * 100
}

func classifyDrift(pfDrift, sharpeDrift, expectancyDrift, livePF, liveExpectancy float64) EdgeStatus {
	// Failed: live edge fundamentally broken
	if livePF < 1.0 || liveExpectancy < 0 {
		return EdgeFailed
	}
	// Decaying: major drift in negative direction
	if pfDrift < -DemoteEdgeDrift*100 || sharpeDrift < -DemoteEdgeDrift*100 || expectancyDrift < -DemoteEdgeDrift*100 {
		return EdgeDecaying
	}
	// Improving: drift is positive across core metrics
	if pfDrift > 5 && sharpeDrift > 5 && expectancyDrift > 5 {
		return EdgeImproving
	}
	return EdgeStable
}

func buildDriftEvidence(h phase24.ExtendedMetrics, l LiveMetrics, pfDrift, sharpeDrift float64, status EdgeStatus) string {
	return fmt.Sprintf(
		"Live PF=%.3f (hist=%.3f, drift=%.1f%%) | Live Sharpe=%.2f (hist=%.2f, drift=%.1f%%) | "+
			"Live Expectancy=$%.2f (hist=$%.2f) | Live Trades=%d | Status=%s",
		l.ProfitFactor, h.ProfitFactor, pfDrift,
		l.Sharpe, h.Sharpe, sharpeDrift,
		l.Expectancy, h.Expectancy,
		l.TotalTrades, status,
	)
}

// DriftSummary counts strategies per EdgeStatus.
func DriftSummary(records []EdgeDriftRecord) map[EdgeStatus]int {
	m := map[EdgeStatus]int{
		EdgeImproving: 0,
		EdgeStable:    0,
		EdgeDecaying:  0,
		EdgeFailed:    0,
	}
	for _, r := range records {
		m[r.EdgeStatus]++
	}
	return m
}
