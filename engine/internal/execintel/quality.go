package execintel

import "math"

// ExecutionQualityScore is the composite 0–100 execution quality grade.
type ExecutionQualityScore struct {
	Score          float64            `json:"score"`
	Classification string             `json:"classification"`
	Components     map[string]float64 `json:"components"`
	Targets        map[string]string  `json:"targets"`
}

// classify maps a 0–100 score to the Phase 22D tier labels.
func classify(score float64) string {
	switch {
	case score >= 90:
		return "Institutional"
	case score >= 75:
		return "Production"
	case score >= 60:
		return "Watchlist"
	default:
		return "Requires Fixes"
	}
}

// qualityInputs are the raw measurements the score is computed from.
type qualityInputs struct {
	e2eLatencyP95Ms float64
	fillQualityPct  float64 // executed / (executed + brokerRejected)
	avgSlippageBps  float64
	missedEntryPct  float64
	tpAccuracyPct   float64 // share of TP-eligible overrides that did not tighten winners
	avgSignalAgeMs  float64
	haveData        bool
}

// scoreLatency: full marks at/under 150ms P95, zero at/over 500ms (linear).
func scoreLatency(p95 float64) float64 {
	if p95 <= 150 {
		return 100
	}
	if p95 >= 500 {
		return 0
	}
	return 100 * (500 - p95) / (500 - 150)
}

// scoreSlippage: full marks at/under 5bps (=0.05%), zero at/over 25bps.
func scoreSlippage(bps float64) float64 {
	bps = math.Abs(bps)
	if bps <= 5 {
		return 100
	}
	if bps >= 25 {
		return 0
	}
	return 100 * (25 - bps) / (25 - 5)
}

// scoreMissed: full marks at/under 5% missed, zero at/over 60%.
func scoreMissed(pct float64) float64 {
	if pct <= 5 {
		return 100
	}
	if pct >= 60 {
		return 0
	}
	return 100 * (60 - pct) / (60 - 5)
}

// scoreFreshness: full marks at/under 500ms avg age, zero at/over 5000ms.
func scoreFreshness(ms float64) float64 {
	if ms <= 500 {
		return 100
	}
	if ms >= 5000 {
		return 0
	}
	return 100 * (5000 - ms) / (5000 - 500)
}

// computeQuality builds the weighted composite execution quality score.
// Weights (sum=100): Latency 20, Fill 20, Slippage 20, MissedEntries 20,
// TP Accuracy 10, Freshness 10.
func computeQuality(in qualityInputs) ExecutionQualityScore {
	comp := map[string]float64{
		"latency":       scoreLatency(in.e2eLatencyP95Ms),
		"fillQuality":   clamp100(in.fillQualityPct),
		"slippage":      scoreSlippage(in.avgSlippageBps),
		"missedEntries": scoreMissed(in.missedEntryPct),
		"tpAccuracy":    clamp100(in.tpAccuracyPct),
		"freshness":     scoreFreshness(in.avgSignalAgeMs),
	}
	score := comp["latency"]*0.20 +
		comp["fillQuality"]*0.20 +
		comp["slippage"]*0.20 +
		comp["missedEntries"]*0.20 +
		comp["tpAccuracy"]*0.10 +
		comp["freshness"]*0.10

	targets := map[string]string{
		"latency":       "P95 signal→fill < 500ms (full marks ≤150ms)",
		"fillQuality":   "> 95%",
		"slippage":      "avg |slippage| < 5bps (0.05%)",
		"missedEntries": "< 5%",
		"tpAccuracy":    "winners not tightened by TP override",
		"freshness":     "avg signal age < 500ms at fill",
	}
	return ExecutionQualityScore{
		Score:          round2(score),
		Classification: classify(score),
		Components:     comp,
		Targets:        targets,
	}
}

func clamp100(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
