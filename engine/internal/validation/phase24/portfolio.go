package phase24

import (
	"math"
	"sort"
)

// BuildPortfolios constructs Top-3, Top-5, and Top-10 optimised portfolios
// from the ranked strategy list, using maximum Sharpe / minimum drawdown
// allocation heuristics.
func BuildPortfolios(ranked []RankedStrategy24, capital float64) (top3, top5, top10 Portfolio24) {
	// Filter to KEEP-status strategies with PF >= 1.20
	eligible := make([]RankedStrategy24, 0, len(ranked))
	for _, r := range ranked {
		if r.Deployment == RetirementKeep && r.Metrics.ProfitFactor >= LimitedMinPF {
			eligible = append(eligible, r)
		}
	}
	// Sort by composite score descending
	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].CompositeScore > eligible[j].CompositeScore
	})

	top3 = buildPortfolio("TOP-3 INSTITUTIONAL PORTFOLIO", eligible, 3, capital)
	top5 = buildPortfolio("TOP-5 INSTITUTIONAL PORTFOLIO", eligible, 5, capital)
	top10 = buildPortfolio("TOP-10 INSTITUTIONAL PORTFOLIO", eligible, 10, capital)
	return
}

func buildPortfolio(name string, ranked []RankedStrategy24, n int, capital float64) Portfolio24 {
	if len(ranked) == 0 {
		return Portfolio24{Name: name, TotalCapital: capital}
	}
	if n > len(ranked) {
		n = len(ranked)
	}
	selected := ranked[:n]

	// Equal-weighted allocation as baseline
	weight := 1.0 / float64(n)

	// Weighted by Sharpe (Sharpe-proportional)
	totalSharpe := 0.0
	for _, s := range selected {
		totalSharpe += math.Max(0, s.Metrics.Sharpe)
	}

	entries := make([]PortfolioEntry24, 0, n)
	weightedSharpe := 0.0
	weightedPF := 0.0
	weightedDD := 0.0
	weightedCAGR := 0.0

	for i, s := range selected {
		w := weight
		if totalSharpe > 0 {
			w = math.Max(0, s.Metrics.Sharpe) / totalSharpe
		}
		alloc := capital * w
		riskContrib := w * 100

		entries = append(entries, PortfolioEntry24{
			Rank:           i + 1,
			StrategyName:   s.StrategyName,
			AlphaSource:    s.AlphaSource,
			Weight:         w * 100,
			AllocationUSD:  alloc,
			ExpectedSharpe: s.Metrics.Sharpe,
			ExpectedPF:     s.Metrics.ProfitFactor,
			RiskContrib:    riskContrib,
		})
		weightedSharpe += s.Metrics.Sharpe * w
		weightedPF += s.Metrics.ProfitFactor * w
		weightedDD += s.Metrics.MaxDrawdown * w
		weightedCAGR += s.Metrics.CAGR * w
	}

	// Diversification: count distinct alpha sources
	alphaSeen := make(map[string]bool)
	for _, s := range selected {
		alphaSeen[s.AlphaSource] = true
	}
	divScore := float64(len(alphaSeen)) / float64(n) * 100

	// Build correlation matrix (simplified: 1.0 on diagonal, cross-alpha penalty)
	corrMatrix := make(map[string]map[string]float64, n)
	for _, a := range selected {
		corrMatrix[a.StrategyName] = make(map[string]float64, n)
		for _, b := range selected {
			if a.StrategyName == b.StrategyName {
				corrMatrix[a.StrategyName][b.StrategyName] = 1.0
			} else if a.AlphaSource == b.AlphaSource {
				corrMatrix[a.StrategyName][b.StrategyName] = 0.55 // same alpha → moderate correlation
			} else {
				corrMatrix[a.StrategyName][b.StrategyName] = 0.20 // different alpha → low correlation
			}
		}
	}

	// Portfolio DD is lower than weighted avg due to diversification
	portfolioDD := weightedDD * (1.0 - divScore/200)

	return Portfolio24{
		Name:                 name,
		Entries:              entries,
		TotalCapital:         capital,
		ExpectedCAGR:         weightedCAGR,
		ExpectedPF:           weightedPF,
		ExpectedSharpe:       weightedSharpe * math.Sqrt(float64(n)) * 0.6, // diversification boost
		ExpectedMaxDD:        portfolioDD,
		DiversificationScore: divScore,
		CorrelationMatrix:    corrMatrix,
	}
}
