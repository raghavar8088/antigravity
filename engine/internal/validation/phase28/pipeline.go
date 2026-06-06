package phase28

import (
	"fmt"
	"time"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
	"antigravity-engine/internal/validation/phase27"
)

// Run orchestrates all Phase 28 portfolio optimization steps.
func Run(
	p24 *phase24.Phase24Result,
	p25 *phase25.Phase25Result,
	p26 *phase26.Phase26Result,
	p27 *phase27.Phase27Result,
) (*Phase28Result, error) {
	if p24 == nil {
		return nil, fmt.Errorf("phase28: p24 result is nil")
	}
	if p25 == nil {
		return nil, fmt.Errorf("phase28: p25 result is nil")
	}
	if p26 == nil {
		return nil, fmt.Errorf("phase28: p26 result is nil")
	}
	if p27 == nil {
		return nil, fmt.Errorf("phase28: p27 result is nil")
	}

	result := &Phase28Result{
		GeneratedAt: time.Now().UTC(),
	}

	// Collect all certified strategies from phase26
	allCerts := append(p26.Tier1Strategies, p26.Tier2Strategies...)

	// Deduplicate
	seen := make(map[string]bool)
	var uniqueCerts []phase26.CertificationRecord
	for _, c := range allCerts {
		if !seen[c.StrategyName] {
			seen[c.StrategyName] = true
			uniqueCerts = append(uniqueCerts, c)
		}
	}

	// 28.1: Correlation matrix from live trades
	var stratNames []string
	for _, c := range uniqueCerts {
		stratNames = append(stratNames, c.StrategyName)
	}
	result.CorrelationMatrix = BuildCorrelationMatrix(stratNames, p25.LiveTrades)

	// 28.2: Portfolio optimization variants
	result.PortfolioA = BuildPortfolioMaxSharpe(uniqueCerts, result.CorrelationMatrix)
	result.PortfolioB = BuildPortfolioMaxCAGR(uniqueCerts, result.CorrelationMatrix)
	result.PortfolioC = BuildPortfolioMinDD(uniqueCerts, result.CorrelationMatrix)
	result.PortfolioD = BuildPortfolioInstitutional(uniqueCerts, result.CorrelationMatrix, p27)

	// 28.3: Allocation engine
	result.StrategyAllocations = ScoreStrategies(uniqueCerts, p25)

	// 28.4: Capital deployment plans
	capitals := []float64{1000, 5000, 25000, 100000, 1000000}
	result.CapitalPlans = BuildCapitalPlans(result.StrategyAllocations, capitals)

	// 28.5: Failure analysis (strategies outside the institutional portfolio)
	result.FailureAnalysis = AnalyzeFailures(uniqueCerts, result.PortfolioD)

	// 28.6: Final verdict
	result.FinalVerdict = BuildFinalVerdict(p26, p27, *result)

	return result, nil
}
