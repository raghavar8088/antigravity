package phase22e

import (
	"math"
	"sort"
)

// CorrelationMatrix holds pairwise Pearson correlations between strategy equity curves.
type CorrelationMatrix struct {
	StrategyIDs  []string
	Matrix       [][]float64 // [i][j] = correlation of i and j
	FamilyMatrix map[string]map[string]float64
	Clusters     []CorrelationCluster
	DiversScore  float64 // 0–100; higher = more diversified
}

// CorrelationCluster groups highly correlated strategies (|r| > 0.70).
type CorrelationCluster struct {
	Label       string
	StrategyIDs []string
	AvgCorr     float64
}

// ComputeCorrelation builds the full correlation matrix from per-strategy equity curves.
func ComputeCorrelation(trades []TradeRecord) CorrelationMatrix {
	// build per-strategy cumulative P&L series keyed to a shared timeline
	stratMap := groupByStrategy(trades)
	if len(stratMap) < 2 {
		return CorrelationMatrix{}
	}

	// collect all unique exit times (global timeline)
	timeSet := make(map[int64]struct{})
	for _, ts := range stratMap {
		for _, t := range ts {
			timeSet[t.ExitTime.Unix()] = struct{}{}
		}
	}
	timestamps := make([]int64, 0, len(timeSet))
	for ts := range timeSet {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })

	// for each strategy build an equity-curve slice over the global timeline
	// by filling forward (last known cumPnL at each timestamp)
	type equitySeries struct {
		id     string
		values []float64
	}
	stratIDs := make([]string, 0, len(stratMap))
	for id := range stratMap {
		stratIDs = append(stratIDs, id)
	}
	sort.Strings(stratIDs)

	equityCurves := make([]equitySeries, len(stratIDs))
	for si, id := range stratIDs {
		stratTrades := stratMap[id]
		// build ts -> pnl map
		pnlAtTime := make(map[int64]float64, len(stratTrades))
		for _, t := range stratTrades {
			pnlAtTime[t.ExitTime.Unix()] += t.NetPnLUSD
		}
		curve := make([]float64, len(timestamps))
		cum := 0.0
		for ti, ts := range timestamps {
			cum += pnlAtTime[ts]
			curve[ti] = cum
		}
		equityCurves[si] = equitySeries{id: id, values: curve}
	}

	// compute n×n Pearson correlation matrix
	n := len(equityCurves)
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
		matrix[i][i] = 1.0
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			r := pearson(equityCurves[i].values, equityCurves[j].values)
			matrix[i][j] = r
			matrix[j][i] = r
		}
	}

	// family-level correlation (average within/between families)
	familyMap := make(map[string][]int)
	for i, id := range stratIDs {
		// find family for this strategy via stratMap
		if ts := stratMap[id]; len(ts) > 0 {
			familyMap[ts[0].Family] = append(familyMap[ts[0].Family], i)
		}
	}
	familyMatrix := buildFamilyCorrelationMatrix(familyMap, matrix)

	// clustering: find groups of strategies with |r| > 0.70
	clusters := buildClusters(stratIDs, matrix)

	// diversification score: 100 - avg |off-diagonal correlation| * 100
	totalCorr := 0.0
	pairs := 0
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			totalCorr += math.Abs(matrix[i][j])
			pairs++
		}
	}
	diversScore := 100.0
	if pairs > 0 {
		diversScore = (1 - totalCorr/float64(pairs)) * 100
	}

	return CorrelationMatrix{
		StrategyIDs:  stratIDs,
		Matrix:       matrix,
		FamilyMatrix: familyMatrix,
		Clusters:     clusters,
		DiversScore:  diversScore,
	}
}

func pearson(a, b []float64) float64 {
	n := len(a)
	if n != len(b) || n < 2 {
		return 0
	}
	ma, mb := mean(a), mean(b)
	num, da, db := 0.0, 0.0, 0.0
	for i := range a {
		xa := a[i] - ma
		xb := b[i] - mb
		num += xa * xb
		da += xa * xa
		db += xb * xb
	}
	denom := math.Sqrt(da * db)
	if denom == 0 {
		return 0
	}
	return num / denom
}

func buildFamilyCorrelationMatrix(familyMap map[string][]int, matrix [][]float64) map[string]map[string]float64 {
	families := make([]string, 0, len(familyMap))
	for f := range familyMap {
		families = append(families, f)
	}
	sort.Strings(families)

	result := make(map[string]map[string]float64)
	for _, f1 := range families {
		result[f1] = make(map[string]float64)
		for _, f2 := range families {
			total, cnt := 0.0, 0
			for _, i := range familyMap[f1] {
				for _, j := range familyMap[f2] {
					if i == j {
						continue
					}
					total += matrix[i][j]
					cnt++
				}
			}
			if cnt > 0 {
				result[f1][f2] = total / float64(cnt)
			}
		}
	}
	return result
}

func buildClusters(ids []string, matrix [][]float64) []CorrelationCluster {
	n := len(ids)
	assigned := make([]bool, n)
	var clusters []CorrelationCluster

	for i := 0; i < n; i++ {
		if assigned[i] {
			continue
		}
		members := []int{i}
		for j := i + 1; j < n; j++ {
			if !assigned[j] && math.Abs(matrix[i][j]) > 0.70 {
				members = append(members, j)
			}
		}
		if len(members) < 2 {
			continue
		}
		for _, m := range members {
			assigned[m] = true
		}
		// compute average intra-cluster correlation
		total, cnt := 0.0, 0
		for a := 0; a < len(members); a++ {
			for b := a + 1; b < len(members); b++ {
				total += matrix[members[a]][members[b]]
				cnt++
			}
		}
		avgCorr := 0.0
		if cnt > 0 {
			avgCorr = total / float64(cnt)
		}
		memberIDs := make([]string, len(members))
		for k, m := range members {
			memberIDs[k] = ids[m]
		}
		clusters = append(clusters, CorrelationCluster{
			Label:       ids[members[0]],
			StrategyIDs: memberIDs,
			AvgCorr:     avgCorr,
		})
	}
	return clusters
}
