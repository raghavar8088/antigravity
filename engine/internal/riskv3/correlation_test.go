package riskv3

import (
	"math"
	"testing"
)

func TestPearsonCorrelation_Perfect(t *testing.T) {
	// Identical series: ρ = 1.0
	a := []float64{1, 2, 3, 4, 5}
	rho := PearsonCorrelation(a, a)
	if math.Abs(rho-1.0) > 1e-9 {
		t.Errorf("identical series: want 1.0 got %.6f", rho)
	}
}

func TestPearsonCorrelation_PerfectNegative(t *testing.T) {
	// Perfectly anti-correlated: ρ = -1.0
	a := []float64{1, 2, 3, 4, 5}
	b := []float64{5, 4, 3, 2, 1}
	rho := PearsonCorrelation(a, b)
	if math.Abs(rho-(-1.0)) > 1e-9 {
		t.Errorf("anti-correlated series: want -1.0 got %.6f", rho)
	}
}

func TestPearsonCorrelation_Zero(t *testing.T) {
	// Constant series → zero variance → return 0
	a := []float64{1, 1, 1, 1}
	b := []float64{1, 2, 3, 4}
	rho := PearsonCorrelation(a, b)
	if rho != 0 {
		t.Errorf("constant series: want 0 got %.6f", rho)
	}
}

func TestPearsonCorrelation_LengthMismatch(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{1, 2}
	rho := PearsonCorrelation(a, b)
	if rho != 0 {
		t.Errorf("length mismatch: want 0 got %.6f", rho)
	}
}

func TestPearsonCorrelation_Symmetry(t *testing.T) {
	// ρ(a,b) = ρ(b,a)
	a := []float64{1.5, -0.3, 2.1, 0.7, -1.2}
	b := []float64{0.8, 1.2, -0.5, 2.0, 1.1}
	if math.Abs(PearsonCorrelation(a, b)-PearsonCorrelation(b, a)) > 1e-10 {
		t.Error("Pearson correlation should be symmetric")
	}
}

func TestBuildCorrelationMatrix_Size(t *testing.T) {
	series := []ReturnSeries{
		{Name: "A", Returns: []float64{1, 2, 3, 4, 5}},
		{Name: "B", Returns: []float64{5, 4, 3, 2, 1}},
		{Name: "C", Returns: []float64{1, 1, 1, 2, 3}},
	}
	matrix := BuildCorrelationMatrix(series)
	if matrix.N != 3 {
		t.Errorf("N: want 3 got %d", matrix.N)
	}
	if len(matrix.Labels) != 3 {
		t.Errorf("Labels: want 3 got %d", len(matrix.Labels))
	}
}

func TestBuildCorrelationMatrix_Diagonal(t *testing.T) {
	// Diagonal must be 1.0 (self-correlation)
	series := []ReturnSeries{
		{Name: "A", Returns: []float64{1, 2, 3, 4, 5}},
		{Name: "B", Returns: []float64{5, 4, 3, 2, 1}},
	}
	m := BuildCorrelationMatrix(series)
	for i := 0; i < m.N; i++ {
		if math.Abs(m.Data[i][i]-1.0) > 1e-9 {
			t.Errorf("diagonal[%d]: want 1.0 got %.6f", i, m.Data[i][i])
		}
	}
}

func TestBuildCorrelationMatrix_Symmetric(t *testing.T) {
	series := []ReturnSeries{
		{Name: "A", Returns: []float64{1, 2, 3, 4, 5}},
		{Name: "B", Returns: []float64{5, 3, 2, 4, 1}},
		{Name: "C", Returns: []float64{2, 1, 4, 3, 5}},
	}
	m := BuildCorrelationMatrix(series)
	for i := 0; i < m.N; i++ {
		for j := 0; j < m.N; j++ {
			if math.Abs(m.Data[i][j]-m.Data[j][i]) > 1e-10 {
				t.Errorf("matrix not symmetric at [%d][%d]", i, j)
			}
		}
	}
}

func TestCorrelationMatrix_MaxAbsCorrelation(t *testing.T) {
	// A and B are perfectly anti-correlated; C is independent
	series := []ReturnSeries{
		{Name: "A", Returns: []float64{1, 2, 3, 4, 5}},
		{Name: "B", Returns: []float64{5, 4, 3, 2, 1}},
		{Name: "C", Returns: []float64{3, 1, 4, 1, 5}},
	}
	m := BuildCorrelationMatrix(series)
	maxCorr := m.MaxAbsCorrelation()
	if math.Abs(maxCorr-1.0) > 1e-9 {
		t.Errorf("MaxAbsCorrelation: want 1.0 got %.6f", maxCorr)
	}
}

func TestCorrelationMatrix_CorrelationWith(t *testing.T) {
	series := []ReturnSeries{
		{Name: "A", Returns: []float64{1, 2, 3, 4, 5}},
		{Name: "B", Returns: []float64{5, 4, 3, 2, 1}},
	}
	m := BuildCorrelationMatrix(series)

	// A vs B: |ρ| = 1.0
	corrA := m.CorrelationWith("A")
	if math.Abs(corrA-1.0) > 1e-9 {
		t.Errorf("CorrelationWith(A): want 1.0 got %.6f", corrA)
	}

	// Non-existent: 0
	corrX := m.CorrelationWith("X")
	if corrX != 0 {
		t.Errorf("CorrelationWith(X): want 0 got %.6f", corrX)
	}
}

func TestRollingCorrelation_Window(t *testing.T) {
	a := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	b := []float64{10, 9, 8, 7, 6, 5, 4, 3, 2, 1} // perfect negative corr

	// 5-element window: last 5 pairs are still perfectly anti-correlated
	rho := RollingCorrelation(a, b, 5)
	if math.Abs(rho-(-1.0)) > 1e-9 {
		t.Errorf("RollingCorrelation window=5: want -1.0 got %.6f", rho)
	}
}

func TestSpearmanCorrelation_Monotone(t *testing.T) {
	// Monotonically increasing → Spearman = 1.0
	a := []float64{1, 3, 2, 5, 4}
	b := []float64{2, 6, 4, 10, 8}
	rho := SpearmanCorrelation(a, b)
	if math.Abs(rho-1.0) > 1e-9 {
		t.Errorf("monotone series Spearman: want 1.0 got %.6f", rho)
	}
}
