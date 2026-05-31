package risk

import (
	"math"
	"sort"
)

type CorrelationMatrix map[string]map[string]float64

func PearsonCorrelation(a, b []float64) float64 {
	n := minInt(len(a), len(b))
	if n < 2 {
		return 0
	}
	a = a[len(a)-n:]
	b = b[len(b)-n:]

	var sumA, sumB float64
	for i := 0; i < n; i++ {
		sumA += a[i]
		sumB += b[i]
	}
	meanA := sumA / float64(n)
	meanB := sumB / float64(n)

	var cov, varA, varB float64
	for i := 0; i < n; i++ {
		da := a[i] - meanA
		db := b[i] - meanB
		cov += da * db
		varA += da * da
		varB += db * db
	}
	denom := math.Sqrt(varA * varB)
	if denom == 0 {
		return 0
	}
	return cov / denom
}

func SpearmanCorrelation(a, b []float64) float64 {
	return PearsonCorrelation(rank(a), rank(b))
}

func RollingCorrelation(a, b []float64, window int) float64 {
	if window <= 0 {
		return PearsonCorrelation(a, b)
	}
	n := minInt(minInt(len(a), len(b)), window)
	if n < 2 {
		return 0
	}
	return PearsonCorrelation(a[len(a)-n:], b[len(b)-n:])
}

func BuildCorrelationMatrix(series map[string][]float64) CorrelationMatrix {
	matrix := make(CorrelationMatrix, len(series))
	for left, leftSeries := range series {
		matrix[left] = make(map[string]float64, len(series))
		for right, rightSeries := range series {
			if left == right {
				matrix[left][right] = 1
				continue
			}
			matrix[left][right] = PearsonCorrelation(leftSeries, rightSeries)
		}
	}
	return matrix
}

func MaxAbsCorrelation(matrix CorrelationMatrix) float64 {
	maxCorr := 0.0
	for left, row := range matrix {
		for right, corr := range row {
			if left == right {
				continue
			}
			if abs(corr) > maxCorr {
				maxCorr = abs(corr)
			}
		}
	}
	return maxCorr
}

func rank(values []float64) []float64 {
	type row struct {
		idx int
		val float64
	}
	rows := make([]row, len(values))
	for i, v := range values {
		rows[i] = row{idx: i, val: v}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].val < rows[j].val })
	ranks := make([]float64, len(values))
	for i, r := range rows {
		ranks[r.idx] = float64(i + 1)
	}
	return ranks
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
