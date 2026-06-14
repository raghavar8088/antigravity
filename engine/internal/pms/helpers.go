package pms

// max64 returns the larger of a and b.
func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
