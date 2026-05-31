package execution

func SupportedPartialFillRatios() []float64 {
	return []float64{0.25, 0.50, 0.75, 1.0}
}

func BucketFillRatio(ratio float64) float64 {
	switch {
	case ratio <= 0.25:
		return 0.25
	case ratio <= 0.50:
		return 0.50
	case ratio <= 0.75:
		return 0.75
	default:
		return 1.0
	}
}
