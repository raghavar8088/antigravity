package phase27

import "antigravity-engine/internal/validation/phase25"

// BuildPareto computes Pareto concentration of profit across alpha families.
// perfs must be sorted by ProfitContributionPct descending (or NetPnLUSD desc).
func BuildPareto(perfs []AlphaFamilyPerf, total float64) ParetoAnalysis {
	pa := ParetoAnalysis{}
	if len(perfs) == 0 || total <= 0 {
		return pa
	}

	var cumulative float64
	for i, p := range perfs {
		pct := 0.0
		if total > 0 {
			pct = p.NetPnLUSD / total * 100
		}
		cumulative += pct

		switch i {
		case 0:
			pa.Top1AlphaPct = cumulative
			pa.Top1Family = p.Family
			pa.Top3Families = append(pa.Top3Families, p.Family)
			pa.Top5Families = append(pa.Top5Families, p.Family)
		case 1, 2:
			pa.Top3Families = append(pa.Top3Families, p.Family)
			pa.Top5Families = append(pa.Top5Families, p.Family)
			if i == 2 {
				pa.Top3AlphaPct = cumulative
			}
		case 3, 4:
			pa.Top5Families = append(pa.Top5Families, p.Family)
			if i == 4 {
				pa.Top5AlphaPct = cumulative
			}
		}
	}

	// Handle cases with fewer than 3 or 5 families
	if len(perfs) < 3 {
		pa.Top3AlphaPct = cumulative
	}
	if len(perfs) < 5 {
		pa.Top5AlphaPct = cumulative
	}

	// Limit slice lengths
	if len(pa.Top3Families) > 3 {
		pa.Top3Families = pa.Top3Families[:3]
	}
	if len(pa.Top5Families) > 5 {
		pa.Top5Families = pa.Top5Families[:5]
	}

	// Ensure Top3Families doesn't include Top1 duplicate — already unique by iteration
	_ = phase25.FamilyFunding // keep import used
	return pa
}
