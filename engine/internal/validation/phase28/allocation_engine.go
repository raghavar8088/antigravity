package phase28

import (
	"math"
	"sort"

	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
)

// ScoreStrategies scores each certified strategy using weighted metrics
// and assigns allocation proportional to composite score.
func ScoreStrategies(certs []phase26.CertificationRecord, p25 *phase25.Phase25Result) []StrategyAllocation {
	if len(certs) == 0 {
		return nil
	}

	weights := DefaultAllocationWeights()

	// Normalize each metric relative to peer group (0–100 scale)
	// Collect raw values
	pfs := make([]float64, len(certs))
	sharpes := make([]float64, len(certs))
	expects := make([]float64, len(certs))
	rets := make([]float64, len(certs))

	for i, c := range certs {
		pfs[i] = c.ProfitFactor
		sharpes[i] = c.Sharpe
		expects[i] = c.Expectancy
		rets[i] = c.EdgeRetention.PFRetentionPct
	}

	pfMin, pfMax := minmax(pfs)
	shMin, shMax := minmax(sharpes)
	exMin, exMax := minmax(expects)
	erMin, erMax := minmax(rets)

	var allocs []StrategyAllocation

	for i, c := range certs {
		pfScore := normalize(pfs[i], pfMin, pfMax)
		shScore := normalize(sharpes[i], shMin, shMax)
		exScore := normalize(expects[i], exMin, exMax)

		// MC score
		mcScore := 0.0
		if c.MCStability == "STABLE" {
			mcScore = 100.0
		} else if c.MCStability == "UNSTABLE" {
			mcScore = 0.0
		} else {
			mcScore = 50.0
		}

		// WF score
		wfScore := 0.0
		if c.WFResult == "PASS" {
			wfScore = 100.0
		} else if c.WFResult == "FAIL" {
			wfScore = 0.0
		} else {
			wfScore = 50.0
		}

		erScore := normalize(rets[i], erMin, erMax)

		// ExecQuality from p25 live metrics
		execScore := 50.0 // default midpoint when not available
		if lm, ok := p25.LiveMetrics[c.StrategyName]; ok {
			// Use win rate as proxy for exec quality (simple, real-data-based)
			execScore = lm.WinRate * 100
		}

		composite := pfScore*weights.PFWeight +
			shScore*weights.SharpeWeight +
			exScore*weights.ExpectancyWeight +
			mcScore*weights.MCStabilityWeight +
			wfScore*weights.WFWeight +
			erScore*weights.EdgeRetentionWeight +
			execScore*weights.ExecQualityWeight

		allocs = append(allocs, StrategyAllocation{
			StrategyName:    c.StrategyName,
			CompositeScore:  composite,
			PFScore:         pfScore,
			SharpeScore:     shScore,
			ExpectancyScore: exScore,
			MCScore:         mcScore,
			WFScore:         wfScore,
			EdgeRetScore:    erScore,
			ExecScore:       execScore,
		})
	}

	// Total composite score for allocation
	total := 0.0
	for _, a := range allocs {
		total += a.CompositeScore
	}
	for i := range allocs {
		if total > 0 {
			allocs[i].AllocationPct = allocs[i].CompositeScore / total * 100
		} else {
			allocs[i].AllocationPct = 100.0 / float64(len(allocs))
		}
	}

	// Sort by AllocationPct descending
	sort.Slice(allocs, func(i, j int) bool {
		return allocs[i].AllocationPct > allocs[j].AllocationPct
	})

	return allocs
}

func normalize(v, min, max float64) float64 {
	if max == min {
		return 50.0
	}
	return math.Max(0, math.Min(100, (v-min)/(max-min)*100))
}

func minmax(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 1
	}
	mn, mx := vals[0], vals[0]
	for _, v := range vals[1:] {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}
	return mn, mx
}
