package riskv3

import "math"

// CorrelationResult contains the correlation analysis of the current portfolio.
type CorrelationResult struct {
	// Pairwise correlation matrix (strategies vs strategies)
	Matrix CorrelationMatrix `json:"matrix"`

	// Highest |ρ| between any two open positions
	MaxPairwiseCorr float64 `json:"max_pairwise_corr"`

	// Correlated cluster: positions on the same side with |ρ| > threshold
	ClusterNotionalUSD  float64 `json:"cluster_notional_usd"`
	ClusterExposurePct  float64 `json:"cluster_exposure_pct"` // % of equity
	ClusterPositions    []string `json:"cluster_positions"`   // position IDs in cluster

	// Correlation of a proposed new position to the existing portfolio
	ProposedMaxCorr float64 `json:"proposed_max_corr"`

	// Breach flags
	MaxCorrBreached     bool `json:"max_corr_breached"`     // MaxPairwiseCorr > MaxCorrelationCoeff
	ClusterBreached     bool `json:"cluster_breached"`      // ClusterExposurePct > MaxCorrelatedExposurePct
	HiddenConcentration bool `json:"hidden_concentration"`  // either breach triggers this

	SampleCount int `json:"sample_count"` // return series length used
}

// AnalyseCorrelation computes the pairwise correlation between all open
// strategy return series and identifies correlated clusters.
//
// returnsByStrategy maps strategy name to its return series. The analysis
// uses the most recent 30 observations from each series (short-term correlation)
// and considers positions correlated when |ρ| > MaxCorrelationCoeff.
//
// The hidden concentration flag is set when a correlated cluster (same-side
// positions with high ρ) accounts for more than MaxCorrelatedExposurePct of equity.
func AnalyseCorrelation(snapshot PortfolioSnapshot, returnsByStrategy map[string][]ReturnSeries) CorrelationResult {
	result := CorrelationResult{}

	// Build correlation matrix from strategy return series (window: last 30)
	seriesInputs := flattenSeriesWithWindow(returnsByStrategy, 30)
	if len(seriesInputs) >= 2 {
		result.Matrix = BuildCorrelationMatrix(seriesInputs)
		result.MaxPairwiseCorr = result.Matrix.MaxAbsCorrelation()
		result.SampleCount = len(seriesInputs[0].Returns)
	}

	// Find correlated cluster among open positions
	equity := snapshot.EquityUSD
	if equity > 0 {
		clusterIDs, clusterNotional := findCorrelatedCluster(snapshot, result.Matrix)
		result.ClusterPositions = clusterIDs
		result.ClusterNotionalUSD = clusterNotional
		result.ClusterExposurePct = clusterNotional / equity * 100
	}

	result.MaxCorrBreached = result.MaxPairwiseCorr > MaxCorrelationCoeff
	result.ClusterBreached = result.ClusterExposurePct > MaxCorrelatedExposurePct
	result.HiddenConcentration = result.MaxCorrBreached || result.ClusterBreached

	return result
}

// CorrelationGuard checks whether a proposed new position is too correlated
// with the existing portfolio. Returns a Violation when the correlation
// exceeds MaxCorrelationCoeff.
func CorrelationGuard(proposedStrategyName, proposedSide string, snapshot PortfolioSnapshot,
	returnsByStrategy map[string][]ReturnSeries) (maxCorr float64, violation *Violation) {
	if len(snapshot.Positions) == 0 {
		return 0, nil
	}

	seriesInputs := flattenSeriesWithWindow(returnsByStrategy, 30)
	if len(seriesInputs) < 2 {
		return 0, nil
	}

	matrix := BuildCorrelationMatrix(seriesInputs)
	maxCorr = matrix.CorrelationWith(proposedStrategyName)

	if maxCorr > MaxCorrelationCoeff {
		return maxCorr, &Violation{
			Type:        ViolationCorrelationExceeded,
			Metric:      "max_pairwise_correlation",
			Current:     maxCorr,
			Limit:       MaxCorrelationCoeff,
			Description: "proposed strategy is too correlated with existing positions",
		}
	}
	return maxCorr, nil
}

// ─── Cluster detection ────────────────────────────────────────────────────────

// findCorrelatedCluster identifies positions that are on the same side AND
// whose strategy return series are correlated above MaxCorrelationCoeff.
// Returns the position IDs and total notional of the cluster.
func findCorrelatedCluster(snapshot PortfolioSnapshot, matrix CorrelationMatrix) ([]string, float64) {
	if len(snapshot.Positions) == 0 || matrix.N == 0 {
		return nil, 0
	}

	// Find the "long" and "short" sub-clusters separately, return the larger.
	longCluster := clusterForSide(snapshot, matrix, "BUY")
	shortCluster := clusterForSide(snapshot, matrix, "SELL")

	if longCluster.notional >= shortCluster.notional {
		return longCluster.ids, longCluster.notional
	}
	return shortCluster.ids, shortCluster.notional
}

type cluster struct {
	ids      []string
	notional float64
}

func clusterForSide(snapshot PortfolioSnapshot, matrix CorrelationMatrix, side string) cluster {
	var ids []string
	var notional float64

	for _, pos := range snapshot.Positions {
		if pos.Side != side {
			continue
		}
		// Include in cluster if correlated with any other same-side position
		corr := matrix.CorrelationWith(pos.StrategyName)
		if corr >= MaxCorrelationCoeff || matrix.N <= 1 {
			ids = append(ids, pos.ID)
			notional += pos.NotionalUSD
		}
	}
	return cluster{ids: ids, notional: notional}
}

// ─── Rolling correlation for 30-day / 90-day windows ─────────────────────────

// StrategyPairCorrelation30d returns the 30-day rolling Pearson correlation
// between two strategy return series.
func StrategyPairCorrelation30d(a, b []ReturnSeries, stratA, stratB string) float64 {
	serA := findSeries(a, stratA)
	serB := findSeries(b, stratB)
	if serA == nil || serB == nil {
		return 0
	}
	return RollingCorrelation(serA.Returns, serB.Returns, 30)
}

// StrategyPairCorrelation90d returns the 90-day rolling Pearson correlation.
func StrategyPairCorrelation90d(a, b []ReturnSeries, stratA, stratB string) float64 {
	serA := findSeries(a, stratA)
	serB := findSeries(b, stratB)
	if serA == nil || serB == nil {
		return 0
	}
	return RollingCorrelation(serA.Returns, serB.Returns, 90)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// flattenSeriesWithWindow collapses a map of named []ReturnSeries into a flat
// []ReturnSeries, using up to the most recent `window` returns from each series.
func flattenSeriesWithWindow(m map[string][]ReturnSeries, window int) []ReturnSeries {
	var out []ReturnSeries
	for name, serList := range m {
		if len(serList) == 0 {
			continue
		}
		ser := serList[len(serList)-1] // most recent series for this name
		returns := ser.Returns
		if len(returns) > window {
			returns = returns[len(returns)-window:]
		}
		if len(returns) < 2 {
			continue
		}
		out = append(out, ReturnSeries{Name: name, Returns: returns})
	}
	return out
}

func findSeries(series []ReturnSeries, name string) *ReturnSeries {
	for i := range series {
		if series[i].Name == name {
			return &series[i]
		}
	}
	return nil
}

// ─── Simple point-in-time correlation for a single proposed trade ─────────────

// PortfolioCorrelationScore returns a 0–1 score measuring how correlated a
// proposed new position is with the existing portfolio.
// Returns 0 when the portfolio is empty.
//
// Method: compute the average correlation between the proposed strategy and all
// open strategies, weighted by notional size.
func PortfolioCorrelationScore(proposedSeries ReturnSeries, snapshot PortfolioSnapshot,
	returnsByStrategy map[string][]ReturnSeries) float64 {
	if len(snapshot.Positions) == 0 || len(proposedSeries.Returns) < 2 {
		return 0
	}

	totalNotional := snapshot.TotalNotionalUSD()
	if totalNotional <= 0 {
		return 0
	}

	weightedCorr := 0.0
	for _, pos := range snapshot.Positions {
		serList, ok := returnsByStrategy[pos.StrategyName]
		if !ok || len(serList) == 0 {
			continue
		}
		ser := serList[len(serList)-1]
		rho := math.Abs(RollingCorrelation(proposedSeries.Returns, ser.Returns, 30))
		weight := pos.NotionalUSD / totalNotional
		weightedCorr += rho * weight
	}
	return weightedCorr
}
