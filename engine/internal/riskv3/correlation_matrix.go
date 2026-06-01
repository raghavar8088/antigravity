package riskv3

import "math"

// ReturnSeries holds a named return time-series used for correlation calculation.
// All series must be the same length for matrix computation.
type ReturnSeries struct {
	Name    string
	Returns []float64
}

// CorrelationMatrix is a symmetric n×n matrix of Pearson correlation coefficients
// between n return series.
type CorrelationMatrix struct {
	Labels []string    // name of each series
	Data   [][]float64 // Data[i][j] = correlation(series[i], series[j])
	N      int         // number of series
}

// Get returns the correlation between series i and j.
func (m CorrelationMatrix) Get(i, j int) float64 {
	if i < 0 || i >= m.N || j < 0 || j >= m.N {
		return 0
	}
	return m.Data[i][j]
}

// MaxAbsCorrelation returns the highest |ρ| off-diagonal in the matrix.
// Returns 0 for matrices with fewer than 2 series.
func (m CorrelationMatrix) MaxAbsCorrelation() float64 {
	max := 0.0
	for i := 0; i < m.N; i++ {
		for j := i + 1; j < m.N; j++ {
			if abs := math.Abs(m.Data[i][j]); abs > max {
				max = abs
			}
		}
	}
	return max
}

// CorrelationWith returns the maximum |ρ| between the named series and all others.
// Returns 0 when the series name is not found.
func (m CorrelationMatrix) CorrelationWith(name string) float64 {
	idx := -1
	for i, l := range m.Labels {
		if l == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0
	}
	max := 0.0
	for j := 0; j < m.N; j++ {
		if j == idx {
			continue
		}
		if abs := math.Abs(m.Data[idx][j]); abs > max {
			max = abs
		}
	}
	return max
}

// BuildCorrelationMatrix computes the full n×n Pearson correlation matrix
// for the given set of named return series.
// All series must have the same length; series with zero variance are skipped
// (their row/column will contain 0s).
func BuildCorrelationMatrix(series []ReturnSeries) CorrelationMatrix {
	n := len(series)
	if n == 0 {
		return CorrelationMatrix{}
	}

	labels := make([]string, n)
	data := make([][]float64, n)
	for i := range data {
		data[i] = make([]float64, n)
		labels[i] = series[i].Name
		data[i][i] = 1.0 // diagonal: ρ(x,x) = 1
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			rho := PearsonCorrelation(series[i].Returns, series[j].Returns)
			data[i][j] = rho
			data[j][i] = rho // symmetric
		}
	}

	return CorrelationMatrix{Labels: labels, Data: data, N: n}
}

// PearsonCorrelation computes the Pearson correlation coefficient ρ between
// two equal-length return series. Returns 0 when the series are too short,
// have zero variance, or have different lengths.
//
// ρ = Σ((x-μx)(y-μy)) / (n * σx * σy)
func PearsonCorrelation(a, b []float64) float64 {
	n := len(a)
	if n < 2 || len(b) != n {
		return 0
	}

	sumA, sumB := 0.0, 0.0
	for i := 0; i < n; i++ {
		sumA += a[i]
		sumB += b[i]
	}
	meanA := sumA / float64(n)
	meanB := sumB / float64(n)

	cov, varA, varB := 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		da := a[i] - meanA
		db := b[i] - meanB
		cov += da * db
		varA += da * da
		varB += db * db
	}
	if varA == 0 || varB == 0 {
		return 0
	}
	return cov / math.Sqrt(varA*varB)
}

// RollingCorrelation computes the Pearson correlation over the most recent
// `window` observations from each series. If either series is shorter than
// window, uses the full shorter length.
func RollingCorrelation(a, b []float64, window int) float64 {
	n := len(a)
	m := len(b)
	if n < m {
		m = n
	}
	if window > m {
		window = m
	}
	if window < 2 {
		return 0
	}
	return PearsonCorrelation(a[n-window:], b[m-window:])
}

// SpearmanCorrelation computes rank-based correlation between two series.
// More robust to outliers than Pearson; returns 0 for series of length < 2.
func SpearmanCorrelation(a, b []float64) float64 {
	n := len(a)
	if n < 2 || len(b) != n {
		return 0
	}
	rankA := rankSlice(a)
	rankB := rankSlice(b)
	return PearsonCorrelation(rankA, rankB)
}

// rankSlice returns the rank of each element (1-based, ties get average rank).
func rankSlice(v []float64) []float64 {
	n := len(v)
	type indexed struct {
		val float64
		idx int
	}
	sorted := make([]indexed, n)
	for i, x := range v {
		sorted[i] = indexed{x, i}
	}
	// Sort by value
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n; j++ {
			if sorted[j].val < sorted[i].val {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	ranks := make([]float64, n)
	for i := 0; i < n; {
		j := i
		for j < n && sorted[j].val == sorted[i].val {
			j++
		}
		// Average rank for tied values
		avg := float64(i+j+1) / 2.0 // 1-based midpoint
		for k := i; k < j; k++ {
			ranks[sorted[k].idx] = avg
		}
		i = j
	}
	return ranks
}
