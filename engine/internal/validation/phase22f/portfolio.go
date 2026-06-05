package phase22f

import (
	"math/rand"
	"sort"

	"antigravity-engine/internal/validation/phase22e"
)

// ConstructPortfolios builds Top5, Top10, Top20 portfolio variants using
// correlation-aware strategy selection.
func ConstructPortfolios(
	trades []phase22e.TradeRecord,
	top20 Top20Selection,
	initialNAV float64,
	rng *rand.Rand,
) []PortfolioVariant {
	byStrat := GroupTradesByStrategy(trades)
	corrMatrix := buildCorrelationMatrix(byStrat)

	variants := []struct {
		name  string
		count int
	}{
		{"Top5", 5},
		{"Top10", 10},
		{"Top20", 20},
	}

	result := make([]PortfolioVariant, 0, len(variants))
	for _, v := range variants {
		limit := v.count
		if len(top20.Entries) < limit {
			limit = len(top20.Entries)
		}
		if limit == 0 {
			continue
		}

		// select least-correlated subset from top-N candidates
		selected := selectLeastCorrelated(top20.Entries[:limit], corrMatrix, limit)
		pv := buildPortfolioVariant(v.name, selected, byStrat, corrMatrix, initialNAV, rng)
		result = append(result, pv)
	}
	return result
}

func buildPortfolioVariant(
	name string,
	entries []Top20Entry,
	byStrat map[string][]phase22e.TradeRecord,
	corr map[string]map[string]float64,
	initialNAV float64,
	rng *rand.Rand,
) PortfolioVariant {
	pv := PortfolioVariant{Name: name}

	ids := make([]string, 0, len(entries))
	var combined []phase22e.TradeRecord
	for _, e := range entries {
		ids = append(ids, e.StrategyID)
		combined = append(combined, byStrat[e.StrategyID]...)
	}
	pv.Strategies = ids

	if len(combined) == 0 {
		return pv
	}

	pf, sharpe, exp, _ := sampleMetrics(combined)
	pnls := make([]float64, len(combined))
	for i, t := range combined {
		pnls[i] = t.NetPnLUSD
	}
	pv.ProfitFactor = pf
	pv.Sharpe = sharpe
	pv.Expectancy = exp
	pv.MaxDrawdown = maxDrawdownPctLocal(pnls, initialNAV)
	pv.DiversScore = diversificationScore(ids, corr)
	pv.TotalCapitalPct = float64(len(ids)) * 5.0 // ~5% per strategy
	if pv.TotalCapitalPct > 100 {
		pv.TotalCapitalPct = 100
	}
	pv.MonteCarlo = RunMonteCarloF22(name, combined, initialNAV, rng)
	return pv
}

// selectLeastCorrelated greedily picks strategies that minimise average correlation.
func selectLeastCorrelated(candidates []Top20Entry, corr map[string]map[string]float64, n int) []Top20Entry {
	if len(candidates) <= n {
		return candidates
	}
	selected := []Top20Entry{candidates[0]}
	remaining := candidates[1:]

	for len(selected) < n && len(remaining) > 0 {
		bestIdx := 0
		bestAvgCorr := 2.0
		for i, c := range remaining {
			avg := avgCorrelationWith(c.StrategyID, selected, corr)
			if avg < bestAvgCorr {
				bestAvgCorr = avg
				bestIdx = i
			}
		}
		selected = append(selected, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}
	return selected
}

func avgCorrelationWith(id string, selected []Top20Entry, corr map[string]map[string]float64) float64 {
	if len(selected) == 0 {
		return 0
	}
	sum := 0.0
	for _, s := range selected {
		if r, ok := corr[id][s.StrategyID]; ok {
			sum += absFloat(r)
		}
	}
	return sum / float64(len(selected))
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func diversificationScore(ids []string, corr map[string]map[string]float64) float64 {
	if len(ids) <= 1 {
		return 100
	}
	total, count := 0.0, 0
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if r, ok := corr[ids[i]][ids[j]]; ok {
				total += absFloat(r)
				count++
			}
		}
	}
	if count == 0 {
		return 100
	}
	avgCorr := total / float64(count)
	return (1 - avgCorr) * 100
}

// buildCorrelationMatrix computes pairwise Pearson correlation of equity curves.
func buildCorrelationMatrix(byStrat map[string][]phase22e.TradeRecord) map[string]map[string]float64 {
	ids := SortedStrategyIDs(byStrat)
	// Build equity-curve series for each strategy
	curves := make(map[string][]float64, len(ids))
	for _, id := range ids {
		pnls := make([]float64, len(byStrat[id]))
		for i, t := range byStrat[id] {
			pnls[i] = t.NetPnLUSD
		}
		curves[id] = cumulativeSum(pnls)
	}

	// Align series to common length (use shortest)
	minLen := len(curves[ids[0]])
	for _, id := range ids[1:] {
		if len(curves[id]) < minLen {
			minLen = len(curves[id])
		}
	}
	// Trim
	for id := range curves {
		if len(curves[id]) > minLen {
			curves[id] = curves[id][:minLen]
		}
	}

	corr := make(map[string]map[string]float64, len(ids))
	for _, id := range ids {
		corr[id] = make(map[string]float64, len(ids))
		corr[id][id] = 1.0
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			a, b := ids[i], ids[j]
			r := pearsonR(curves[a], curves[b])
			corr[a][b] = r
			corr[b][a] = r
		}
	}
	return corr
}

func cumulativeSum(xs []float64) []float64 {
	out := make([]float64, len(xs))
	sum := 0.0
	for i, x := range xs {
		sum += x
		out[i] = sum
	}
	return out
}

// SortPortfoliosByPF sorts portfolio variants by profit factor descending.
func SortPortfoliosByPF(variants []PortfolioVariant) {
	sort.Slice(variants, func(i, j int) bool {
		return variants[i].ProfitFactor > variants[j].ProfitFactor
	})
}
