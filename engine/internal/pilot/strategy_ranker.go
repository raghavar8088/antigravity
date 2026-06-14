package pilot

import "sort"

// RankedStrategy pairs a StrategyLiveMetrics entry with its computed composite score
// and ordinal rank (1 = best).
type RankedStrategy struct {
	StrategyID   string
	StrategyName string
	Rank         int
	Score        float64
	Metrics      StrategyLiveMetrics
}

// StrategyRanker scores and sorts strategies by a composite quality metric.
// Only strategies with at least MinTrades are eligible for a non-zero score.
type StrategyRanker struct {
	MinTrades int
}

// NewStrategyRanker returns a StrategyRanker with Phase 22E minimum-trades threshold.
func NewStrategyRanker() *StrategyRanker {
	return &StrategyRanker{MinTrades: 30}
}

// Rank scores all candidates and returns them sorted best-first.
// Strategies below MinTrades receive score 0 and sort to the bottom.
func (r *StrategyRanker) Rank(candidates []StrategyLiveMetrics) []RankedStrategy {
	scored := make([]RankedStrategy, len(candidates))
	for i, c := range candidates {
		score := 0.0
		if c.Trades >= r.MinTrades {
			// Composite: Sharpe × ProfitFactor × WinRate, penalised by MaxDrawdown
			dd := c.MaxDrawdown
			if dd <= 0 {
				dd = 0.001
			}
			score = c.Sharpe * c.ProfitFactor * c.WinRate / dd
		}
		scored[i] = RankedStrategy{
			StrategyID:   c.StrategyID,
			StrategyName: c.StrategyName,
			Score:        score,
			Metrics:      c,
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})
	for i := range scored {
		scored[i].Rank = i + 1
	}
	return scored
}

// TopN returns the top n strategies by composite score.
// If n ≥ len(candidates), all ranked strategies are returned.
func (r *StrategyRanker) TopN(candidates []StrategyLiveMetrics, n int) []RankedStrategy {
	ranked := r.Rank(candidates)
	if n >= len(ranked) {
		return ranked
	}
	return ranked[:n]
}
