package phase22e

import (
	"fmt"
	"sort"
)

// RankStrategies scores, filters, and ranks strategies for deployment.
// Returns strategies sorted by composite score (highest first) with
// ranks and capital allocations filled in.
func RankStrategies(strategies []StrategyMetrics, totalCapital float64) []StrategyMetrics {
	// 1. filter out unapproved strategies
	eligible := make([]StrategyMetrics, 0, len(strategies))
	for _, s := range strategies {
		s.Approved, s.RejectReason = isApproved(s)
		eligible = append(eligible, s)
	}

	// 2. score eligible strategies
	type scored struct {
		idx   int
		score float64
	}
	scores := make([]scored, len(eligible))
	for i, s := range eligible {
		scores[i] = scored{i, compositeScore(s)}
	}
	sort.Slice(scores, func(a, b int) bool { return scores[a].score > scores[b].score })

	// 3. assign ranks and compute capital allocations
	approvedKellySum := 0.0
	for _, sc := range scores {
		s := eligible[sc.idx]
		if s.Approved {
			approvedKellySum += s.HalfKelly
		}
	}

	ranked := make([]StrategyMetrics, len(eligible))
	rank := 1
	for _, sc := range scores {
		s := eligible[sc.idx]
		s.Rank = rank
		if s.Approved && approvedKellySum > 0 {
			rawAlloc := s.HalfKelly / approvedKellySum * 100
			// hard cap per strategy at 20% to prevent concentration
			if rawAlloc > 20 {
				rawAlloc = 20
			}
			s.AllocatedPct = rawAlloc
		}
		ranked[rank-1] = s
		rank++
	}

	// re-normalise if cap clipping changed total
	totalAlloc := 0.0
	for _, s := range ranked {
		if s.Approved {
			totalAlloc += s.AllocatedPct
		}
	}
	if totalAlloc > 0 && totalAlloc != 100 {
		for i := range ranked {
			if ranked[i].Approved {
				ranked[i].AllocatedPct = ranked[i].AllocatedPct / totalAlloc * 100
			}
		}
	}

	return ranked
}

// isApproved checks whether a strategy meets minimum deployment criteria.
func isApproved(s StrategyMetrics) (bool, string) {
	if !s.StatSig {
		return false, fmt.Sprintf("insufficient trades (%d < %d required)", s.TotalTrades, MinStrategyTrades)
	}
	if s.ProfitFactor < MinProfitFactor {
		return false, fmt.Sprintf("profit factor %.2f < %.2f threshold", s.ProfitFactor, MinProfitFactor)
	}
	if s.WinRate < MinWinRate {
		return false, fmt.Sprintf("win rate %.1f%% < %.0f%% threshold", s.WinRate*100, MinWinRate*100)
	}
	if s.MaxDrawdown > MaxDrawdownPct {
		return false, fmt.Sprintf("max drawdown %.1f%% > %.0f%% threshold", s.MaxDrawdown, MaxDrawdownPct)
	}
	if !s.LiveVsBacktestOK && s.LivePF > 0 {
		return false, fmt.Sprintf("live PF %.2f degrades %.0f%% vs backtest PF %.2f (limit %.0f%%)",
			s.LivePF, -s.PFDegradation*100, s.BacktestPF, MaxLiveVsBacktestDeg*100)
	}
	if s.KellyFraction <= 0 {
		return false, "negative Kelly fraction — edge not confirmed"
	}
	return true, ""
}

// compositeScore produces a single ranking number for a strategy.
// Higher is better.
//
// Weighting:
//   35% Profit Factor (normalised to cap 3.0)
//   25% Sharpe Ratio  (normalised to cap 4.0)
//   20% Win Rate
//   10% Expectancy rank (normalised vs portfolio avg $10)
//   10% Drawdown penalty (inverted)
func compositeScore(s StrategyMetrics) float64 {
	if !s.Approved {
		return -1
	}
	pfScore := clamp(s.ProfitFactor/3.0, 0, 1) * 35
	shrScore := clamp(s.Sharpe/4.0, 0, 1) * 25
	wrScore := clamp(s.WinRate/0.70, 0, 1) * 20
	exScore := clamp(s.Expectancy/50.0, 0, 1) * 10
	ddPenalty := clamp(1-(s.MaxDrawdown/MaxDrawdownPct), 0, 1) * 10
	return pfScore + shrScore + wrScore + exScore + ddPenalty
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// GroupByFamily aggregates strategy metrics by strategy family.
func GroupByFamily(strategies []StrategyMetrics) map[string]FamilyMetrics {
	families := make(map[string]FamilyMetrics)
	for _, s := range strategies {
		fm := families[s.Family]
		fm.Family = s.Family
		fm.Strategies++
		fm.TotalTrades += s.TotalTrades
		fm.NetPnLUSD += s.Expectancy * float64(s.TotalTrades)
		fm.ApprovedCount += boolInt(s.Approved)
		if s.ProfitFactor > 0 {
			fm.AvgProfitFactor = (fm.AvgProfitFactor*float64(fm.Strategies-1) + s.ProfitFactor) / float64(fm.Strategies)
		}
		families[s.Family] = fm
	}
	return families
}

// FamilyMetrics is the aggregated view for a strategy family.
type FamilyMetrics struct {
	Family          string
	Strategies      int
	ApprovedCount   int
	TotalTrades     int
	NetPnLUSD       float64
	AvgProfitFactor float64
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CapitalDeploymentPlan converts percentage allocations to USD amounts.
func CapitalDeploymentPlan(strategies []StrategyMetrics, totalCapital float64) []CapitalSlot {
	out := make([]CapitalSlot, 0, len(strategies))
	for _, s := range strategies {
		if !s.Approved {
			continue
		}
		out = append(out, CapitalSlot{
			Rank:          s.Rank,
			StrategyID:    s.StrategyID,
			StrategyName:  s.StrategyName,
			Family:        s.Family,
			AllocatedPct:  s.AllocatedPct,
			AllocatedUSD:  totalCapital * s.AllocatedPct / 100,
			KellyFraction: s.KellyFraction,
			HalfKelly:     s.HalfKelly,
			ProfitFactor:  s.ProfitFactor,
			WinRate:       s.WinRate,
			Sharpe:        s.Sharpe,
		})
	}
	return out
}

// CapitalSlot describes how much capital is assigned to one strategy.
type CapitalSlot struct {
	Rank          int
	StrategyID    string
	StrategyName  string
	Family        string
	AllocatedPct  float64
	AllocatedUSD  float64
	KellyFraction float64
	HalfKelly     float64
	ProfitFactor  float64
	WinRate       float64
	Sharpe        float64
}
