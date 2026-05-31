package v2

import (
	"math"
	"sort"
	"time"
)

func nowUTC() time.Time { return time.Now().UTC() }

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func minPositive(values ...float64) float64 {
	out := 0.0
	for _, v := range values {
		if v <= 0 {
			continue
		}
		if out == 0 || v < out {
			out = v
		}
	}
	return out
}

func sum(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return sum(values) / float64(len(values))
}

func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := mean(values)
	var ss float64
	for _, v := range values {
		d := v - m
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(values)-1))
}

func percentile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	q = clamp(q, 0, 1)
	idx := int(q * float64(len(cp)-1))
	return cp[idx]
}

func tail(values []float64, n int) []float64 {
	if n <= 0 || len(values) <= n {
		return append([]float64(nil), values...)
	}
	return append([]float64(nil), values[len(values)-n:]...)
}

func appendBounded[T any](values []T, v T, maxLen int) []T {
	values = append(values, v)
	if maxLen > 0 && len(values) > maxLen {
		return append([]T(nil), values[len(values)-maxLen:]...)
	}
	return values
}

func annualizedSharpe(returns []float64) float64 {
	s := stddev(returns)
	if s == 0 {
		return 0
	}
	return mean(returns) / s * math.Sqrt(252)
}
