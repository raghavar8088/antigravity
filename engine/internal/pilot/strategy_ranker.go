package pilot

import "sort"

// RankedStrategy pairs a strategy with its computed composite score and final rank.
type RankedStrategy struct {
	StrategyID   string
	StrategyName string
	Metrics      StrategyLiveMetrics
	Score        float64
	Rank         int
}

// StrategyRanker ranks live strategies by weighted composite score.
// Weights are tuned for the Phase 23 pilot: Sharpe-heavy with drawdown penalty.
type StrategyRanker struct {
	WeightSharpe   float64
	WeightPF       float64
	WeightWinRate  float64
	WeightDrawdown float64
	MinTrades      int // strategies with fewer trades get score 0
}

func NewStrategyRanker() *StrategyRanker {
	return &StrategyRanker{
		WeightSharpe:   0.35,
		WeightPF:       0.30,
		WeightWinRate:  0.20,
		WeightDrawdown: 0.15,
		MinTrades:      30,
	}
}

// Rank computes composite scores for all strategies and returns them sorted descending.
func (r *StrategyRanker) Rank(strategies []StrategyLiveMetrics) []RankedStrategy {
	ranked := make([]RankedStrategy, len(strategies))
	for i, s := range strategies {
		ranked[i] = RankedStrategy{
			StrategyID:   s.StrategyID,
			StrategyName: s.StrategyName,
			Metrics:      s,
			Score:        r.score(s),
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})
	for i := range ranked {
		ranked[i].Rank = i + 1
	}
	return ranked
}

// TopN returns at most n strategies sorted by composite score (highest first).
func (r *StrategyRanker) TopN(strategies []StrategyLiveMetrics, n int) []RankedStrategy {
	ranked := r.Rank(strategies)
	if n <= 0 || n > len(ranked) {
		return ranked
	}
	return ranked[:n]
}

// score computes a single composite score in [0, 1].
// Strategies below MinTrades threshold receive 0 to prevent noise from entering pilot.
func (r *StrategyRanker) score(s StrategyLiveMetrics) float64 {
	if s.Trades < r.MinTrades {
		return 0
	}
	// Normalize each metric into [0, 1]
	sharpeNorm := clamp(s.Sharpe/3.0, 0, 1)         // Sharpe 0–3 → 0–1
	pfNorm := clamp((s.ProfitFactor-1.0)/2.0, 0, 1) // PF 1–3 → 0–1
	wrNorm := clamp((s.WinRate-0.30)/0.40, 0, 1)    // WR 30–70% → 0–1
	ddNorm := 1.0 - clamp(s.MaxDrawdown/0.20, 0, 1) // DD 0–20% → 1–0 (inverted)

	return r.WeightSharpe*sharpeNorm +
		r.WeightPF*pfNorm +
		r.WeightWinRate*wrNorm +
		r.WeightDrawdown*ddNorm
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
