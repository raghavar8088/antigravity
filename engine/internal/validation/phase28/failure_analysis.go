package phase28

import (
	"fmt"
	"math"

	"antigravity-engine/internal/validation/phase26"
)

// AnalyzeFailures evaluates whether adding each non-portfolio strategy would
// reduce portfolio quality. All comparisons use actual certified metrics.
func AnalyzeFailures(allStrategies []phase26.CertificationRecord, portfolio Portfolio28) []FailureAnalysis28 {
	// Build portfolio strategy set
	inPortfolio := make(map[string]bool)
	for _, pw := range portfolio.Weights {
		inPortfolio[pw.StrategyName] = true
	}

	var result []FailureAnalysis28

	for _, cand := range allStrategies {
		if inPortfolio[cand.StrategyName] {
			continue
		}

		// Would adding this strategy to the portfolio reduce quality?
		// Simulate adding it with equal weight to existing strategies
		n := float64(len(portfolio.Weights) + 1)
		simSharpe := (portfolio.ExpectedSharpe*float64(len(portfolio.Weights)) + cand.Sharpe) / n
		simDD := math.Max(portfolio.ExpectedMaxDD, cand.MaxDD)

		decreasesSharpe := simSharpe < portfolio.ExpectedSharpe
		increasesDD := simDD > portfolio.ExpectedMaxDD

		unstableEdge := cand.MCStability != "STABLE" || cand.WFResult != "PASS"
		poorRetention := false
		avgRet := 0.0
		cnt := 0
		if cand.EdgeRetention.HistoricalPF > 0 {
			avgRet += cand.EdgeRetention.PFRetentionPct
			cnt++
		}
		if cand.EdgeRetention.HistoricalSharpe > 0 {
			avgRet += cand.EdgeRetention.SharpeRetentionPct
			cnt++
		}
		if cnt > 0 {
			avgRet /= float64(cnt)
			poorRetention = avgRet < 80.0
		}

		reducesQuality := decreasesSharpe || increasesDD

		recommendation := "KEEP"
		switch {
		case reducesQuality && unstableEdge && poorRetention:
			recommendation = "REMOVE"
		case reducesQuality:
			recommendation = "REDUCE"
		case unstableEdge || poorRetention:
			recommendation = "WATCH"
		}

		evidence := ""
		if decreasesSharpe {
			evidence += fmt.Sprintf("Adding %s reduces portfolio Sharpe from %.3f to %.3f. ", cand.StrategyName, portfolio.ExpectedSharpe, simSharpe)
		}
		if increasesDD {
			evidence += fmt.Sprintf("Increases MaxDD from %.2f%% to %.2f%%. ", portfolio.ExpectedMaxDD, simDD)
		}
		if evidence == "" {
			evidence = fmt.Sprintf("INSUFFICIENT_EVIDENCE: minimal impact detected for %s", cand.StrategyName)
		}

		result = append(result, FailureAnalysis28{
			StrategyName:            cand.StrategyName,
			ReducesPortfolioQuality: reducesQuality,
			IncreasesDrawdown:       increasesDD,
			DecreasesSharpe:         decreasesSharpe,
			UnstableEdge:            unstableEdge,
			PoorRetention:           poorRetention,
			Recommendation:          recommendation,
			Evidence:                evidence,
		})
	}

	return result
}
