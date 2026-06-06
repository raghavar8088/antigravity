package phase27

import (
	"fmt"
	"time"

	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
)

// Run orchestrates all Phase 27 alpha concentration analysis.
func Run(p25 *phase25.Phase25Result, p26 *phase26.Phase26Result) (*Phase27Result, error) {
	if p25 == nil {
		return nil, fmt.Errorf("phase27: p25 result is nil")
	}
	if p26 == nil {
		return nil, fmt.Errorf("phase27: p26 result is nil")
	}

	result := &Phase27Result{
		GeneratedAt: time.Now().UTC(),
	}

	// 27.1: Build attribution records from all live trades
	result.AttributionRecords = BuildAttributionRecords(p25.LiveTrades)

	// 27.2: Compute per-family performance
	var insufficient []string
	result.AlphaPerformance, result.TotalPlatformPnL, insufficient = BuildAllFamilyPerfs(result.AttributionRecords)
	result.InsufficientEvidence = insufficient

	// 27.3: Pareto analysis
	result.ParetoAnalysis = BuildPareto(result.AlphaPerformance, result.TotalPlatformPnL)

	// 27.4: Championship ranking
	result.Championship = BuildChampionship(result.AlphaPerformance)

	// 27.5: Overlap analysis
	result.OverlapAnalysis = BuildOverlapAnalysis(result.AlphaPerformance, result.AttributionRecords)
	for _, ov := range result.OverlapAnalysis {
		if ov.Redundant {
			// Only add the lower-ranked one
			result.RedundantFamilies = append(result.RedundantFamilies, ov.FamilyB)
		}
	}

	return result, nil
}
